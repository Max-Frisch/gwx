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
// each one a decision cycle, then reschedules it for a human-ish interval
// later. Like every other engine phase it is safe to restart and to run on
// more than one instance (the lease uses FOR UPDATE SKIP LOCKED).
//
// A cycle runs a handful of best-effort actions through the same
// QueueStore/FleetStore calls a player uses: react to an inbound attack, then
// grow (buildings), advance (research), arm (ships/defense) and expand
// (colonize). Every step is probabilistic so NPCs vary; the profile's
// strategy/difficulty knobs steer them.
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

	if len(planets) > 0 {
		// React first — an inbound attack is time-critical — then the growth
		// actions, each best-effort so a busy queue or a thin treasury on one
		// simply falls through to the next.
		e.npcReactDecide(ctx, n, planets)
		e.npcBuildDecide(ctx, n, planets)
		e.npcResearchDecide(ctx, n, planets)
		e.npcUnitDecide(ctx, n, planets)
		e.npcColonizeDecide(ctx, n, planets)
	}

	return store.NewNPCStore(e.db).Schedule(ctx, n.UserID, time.Now().Add(game.NextDecisionDelay(n.Profile, n.GameSpeed, e.rng)))
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
			if !npcSoftFailure(err) {
				return false
			}
		}
	}
	return false
}

// npcResearchDecide tries to queue the next tech level. It walks the NPC's
// planets (research is account-wide but gated on a planet's Research Center and
// paid from its treasury) and, on each, attempts research candidates in
// weighted-random order until one is accepted; soft failures fall through.
func (e *EventEngine) npcResearchDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet) bool {
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
		for _, code := range game.RankedResearch(structs, techs, n.Profile, e.rng) {
			err := queues.StartResearch(ctx, n.Universe, n.UserID, p.ID, code)
			if err == nil {
				return true
			}
			if !npcSoftFailure(err) {
				return false
			}
		}
	}
	return false
}

// npcUnitDecide tries to queue one batch of ships or defense. It works through
// the NPC's planets and, on each, walks the legal units in weighted-random
// order until one is accepted; soft failures fall through.
func (e *EventEngine) npcUnitDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet) bool {
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
		for _, code := range game.RankedUnits(structs, techs, n.Profile, e.rng) {
			def, ok := game.Get(code)
			if !ok {
				continue
			}
			qty := game.NPCUnitBatch(def, n.Profile, e.rng)
			err := queues.StartUnits(ctx, n.Universe, n.UserID, p.ID, string(def.Kind), code, qty)
			if err == nil {
				return true
			}
			if !npcSoftFailure(err) {
				return false
			}
		}
	}
	return false
}

// npcColonizeDecide commits an already-built colony ship to a random empty
// slot, when the profile feels like expanding. It is the second half of
// expansion: the ship itself comes from the ordinary unit production above.
func (e *EventEngine) npcColonizeDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet) bool {
	if e.rng.Float64() >= n.Profile.ColonizeChance() {
		return false
	}
	planets := store.NewPlanetStore(e.db)
	fleets := store.NewFleetStore(e.db)
	npcs := store.NewNPCStore(e.db)
	for _, p := range owned {
		ships, err := planets.Units(ctx, p.ID, "ship")
		if err != nil || ships["colony_ship"] <= 0 {
			continue
		}
		g, s, pos, ok, err := npcs.RandomEmptySlot(ctx, n.Universe)
		if err != nil || !ok {
			return false
		}
		_, err = fleets.Dispatch(ctx, store.DispatchParams{
			UniverseCode: n.Universe, UserID: n.UserID, OriginID: p.ID,
			Mission: "COLONIZE", TargetG: g, TargetS: s, TargetP: pos,
			SpeedPct: 1, Ships: map[string]int64{"colony_ship": 1},
		})
		return err == nil
	}
	return false
}

// npcReactDecide handles an inbound attack. With a profile-weighted chance it
// scouts the attacker with a probe, and (rarely) deliberately lifts its fleet
// off an attacked planet so the attack lands on the ground defenses alone.
func (e *EventEngine) npcReactDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet) bool {
	incoming, err := store.NewFleetStore(e.db).IncomingForUser(ctx, n.UserID)
	if err != nil {
		return false
	}
	var attacks []store.Fleet
	for _, f := range incoming {
		if f.Mission == "ATTACK" {
			attacks = append(attacks, f)
		}
	}
	if len(attacks) == 0 {
		return false
	}

	did := false
	if e.rng.Float64() < n.Profile.SpyChance() {
		if e.npcSpyAttacker(ctx, n, owned, attacks[0]) {
			did = true
		}
	}
	if e.rng.Float64() < n.Profile.FleetSaveChance() {
		if e.npcFleetSave(ctx, n, owned, attacks) {
			did = true
		}
	}
	return did
}

// npcSpyAttacker sends one spy probe to the attacker's origin, so the NPC's
// next look at its reports shows what is inbound.
func (e *EventEngine) npcSpyAttacker(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, attack store.Fleet) bool {
	planets := store.NewPlanetStore(e.db)
	fleets := store.NewFleetStore(e.db)
	for _, p := range owned {
		ships, err := planets.Units(ctx, p.ID, "ship")
		if err != nil || ships["spy_probe"] <= 0 {
			continue
		}
		_, err = fleets.Dispatch(ctx, store.DispatchParams{
			UniverseCode: n.Universe, UserID: n.UserID, OriginID: p.ID,
			Mission: "ESPIONAGE",
			TargetG: attack.OriginG, TargetS: attack.OriginS, TargetP: attack.OriginP,
			SpeedPct: 1, Ships: map[string]int64{"spy_probe": 1},
		})
		return err == nil
	}
	return false
}

// npcFleetSave sends the ships off each attacked planet on a sloppy round trip
// to a random target. Probes stay behind. The fleet is in the air (and thus out
// of the battle) when the attack lands; it returns on its own afterwards.
func (e *EventEngine) npcFleetSave(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, attacks []store.Fleet) bool {
	planets := store.NewPlanetStore(e.db)
	fleets := store.NewFleetStore(e.db)
	npcs := store.NewNPCStore(e.db)
	underAttack := map[int64]bool{}
	for _, a := range attacks {
		if a.TargetID.Valid {
			underAttack[a.TargetID.Int64] = true
		}
	}
	saved := false
	for _, p := range owned {
		if len(underAttack) > 0 && !underAttack[p.ID] {
			continue
		}
		ships, err := planets.Units(ctx, p.ID, "ship")
		if err != nil || len(ships) == 0 {
			continue
		}
		delete(ships, "spy_probe")
		if len(ships) == 0 {
			continue
		}
		g, s, pos, ok, err := npcs.RandomTarget(ctx, n.Universe, n.UserID)
		if err != nil || !ok {
			continue
		}
		if _, err := fleets.Dispatch(ctx, store.DispatchParams{
			UniverseCode: n.Universe, UserID: n.UserID, OriginID: p.ID,
			Mission: "TRANSPORT", TargetG: g, TargetS: s, TargetP: pos,
			SpeedPct: 1, Ships: ships,
		}); err == nil {
			saved = true
		}
	}
	return saved
}

// npcSoftFailure reports whether a failed queue/dispatch attempt should let the
// brain try the next candidate/planet rather than giving up.
func npcSoftFailure(err error) bool {
	return errors.Is(err, store.ErrInsufficientResources) ||
		errors.Is(err, store.ErrNoFields) ||
		errors.Is(err, store.ErrQueueFull) ||
		errors.Is(err, store.ErrQueueBusy) ||
		errors.Is(err, store.ErrResearchQueueFull) ||
		errors.Is(err, store.ErrPrerequisites) ||
		errors.Is(err, store.ErrOnVacation) ||
		errors.Is(err, store.ErrSanctioned)
}
