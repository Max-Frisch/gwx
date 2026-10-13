package engine

import (
	"context"
	"errors"
	"log"
	"time"

	"gwx/internal/game"
	"gwx/internal/store"
)

// npcBatch caps how many NPCs one tick will let think, so a large universe
// never stalls the queue/fleet resolvers behind the brain.
const npcBatch = 25

// processNPCs is the NPC brain's durable driver. It leases due NPCs and gives
// each one a single decision, then reschedules it for a human-ish interval
// later. Like every other engine phase it is safe to restart and to run on
// more than one instance (the lease uses FOR UPDATE SKIP LOCKED).
//
// The brain deliberately starts small: it only queues building levels today.
// The remaining actions from the design — unit/defense production, colonizing,
// reacting to incoming attacks and sloppy "fleet in the air" fleet-saves — hook
// in here next and reuse the same QueueStore/FleetStore calls a player uses.
func (e *EventEngine) processNPCs(ctx context.Context) {
	npcs := store.NewNPCStore(e.db)
	due, err := npcs.ClaimDue(ctx, npcBatch)
	if err != nil {
		log.Printf("[ENGINE] npc scan: %v", err)
		return
	}
	for _, n := range due {
		if err := e.runNPC(ctx, n); err != nil {
			log.Printf("[ENGINE] npc %d: %v", n.UserID, err)
		}
	}
}

// runNPC is one decision cycle for a single NPC.
func (e *EventEngine) runNPC(ctx context.Context, n store.NPC) error {
	planets, err := store.NewPlanetStore(e.db).ListOwned(ctx, n.UserID)
	if err != nil {
		return err
	}

	// Homeworld first, but if it is full/broke the brain simply falls through
	// to the next planet. Future actions (units, defense, colonize, fleet-save)
	// slot in as additional per-planet sweeps here.
	if len(planets) > 0 {
		e.npcBuildDecide(ctx, n, planets)
	}

	return store.NewNPCStore(e.db).Schedule(ctx, n.UserID, time.Now().Add(game.NextDecisionDelay(n.Profile, e.rng)))
}

// npcBuildDecide tries to queue the next building level somewhere. It walks the
// NPC's planets and, on each, attempts building candidates in weighted-random
// order until one is accepted; soft failures (no fields, too expensive, queue
// busy/full) just fall through to the next candidate or planet.
func (e *EventEngine) npcBuildDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet) bool {
	planets := store.NewPlanetStore(e.db)
	queues := store.NewQueueStore(e.db)
	techs, err := planets.Techs(ctx, n.UserID)
	if err != nil {
		return false
	}
	for _, p := range owned {
		structs, err := planets.Structures(ctx, p.ID)
		if err != nil {
			continue
		}
		for _, code := range game.RankedBuildings(structs, techs, n.Profile, e.rng) {
			err := queues.StartBuilding(ctx, n.Universe, n.UserID, p.ID, code)
			if err == nil {
				return true
			}
			if !npcSoftBuildFailure(err) {
				return false
			}
		}
	}
	return false
}

// npcSoftBuildFailure reports whether a failed StartBuilding attempt should let
// the brain try the next candidate/planet rather than giving up.
func npcSoftBuildFailure(err error) bool {
	return errors.Is(err, store.ErrInsufficientResources) ||
		errors.Is(err, store.ErrNoFields) ||
		errors.Is(err, store.ErrQueueFull) ||
		errors.Is(err, store.ErrQueueBusy) ||
		errors.Is(err, store.ErrPrerequisites) ||
		errors.Is(err, store.ErrOnVacation) ||
		errors.Is(err, store.ErrSanctioned)
}
