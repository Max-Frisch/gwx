package engine

import (
	"context"
	"errors"
	"log"
	"time"

	"gwx/internal/game"
	"gwx/internal/store"
)

// defaultNPCBbatch caps how many NPCs one tick will let think, so a large
// universe never stalls the queue/fleet resolvers behind the brain. The actual
// cap is configurable at startup via EventEngine.SetNPCBatch (NPC_BATCH).
const defaultNPCBbatch = 25

// npcWindowMaxGameSpeed is the game speed above which online windows are
// ignored: real-time hours are meaningless in a compressed round, so a fast
// universe keeps every NPC on the clock.
const npcWindowMaxGameSpeed = 4

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
	due, err := npcs.ClaimDue(ctx, e.npcBatch)
	if err != nil {
		log.Printf("[ENGINE] npc scan: %v", err)
		return
	}
	e.npcRaidLeft = e.npcRaid.MaxPerTick
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

	now := time.Now()
	if len(planets) > 0 {
		// React first — an inbound attack is time-critical — then the growth
		// actions, each best-effort so a busy queue or a thin treasury on one
		// simply falls through to the next.
		e.npcReactDecide(ctx, n, planets)

		// Growth only happens inside the NPC's online window. Reactions above
		// still fire, so an attacked NPC is never a free farm.
		if npcGrowthAllowed(n.Profile, n.GameSpeed, now) {
			// Load the account's techs and each planet's structures once, then
			// hand them to every growth decide instead of re-querying inside.
			planetsStore := store.NewPlanetStore(e.db)
			techs, err := planetsStore.Techs(ctx, n.UserID)
			if err != nil {
				return err
			}
			structsByPlanet := make(map[int64]map[string]int, len(planets))
			for _, p := range planets {
				structs, err := planetsStore.Structures(ctx, p.ID)
				if err != nil {
					return err
				}
				structsByPlanet[p.ID] = structs
			}
			e.npcBuildDecide(ctx, n, planets, techs, structsByPlanet)
			e.npcResearchDecide(ctx, n, planets, techs, structsByPlanet)
			e.npcUnitDecide(ctx, n, planets, techs, structsByPlanet)
			e.npcColonizeDecide(ctx, n, planets)
			// Offense is opt-in: skip the extra ships query entirely when
			// raiding is off, so the default brain stays byte-for-byte.
			if e.npcRaid.Enabled {
				shipsByPlanet, err := planetsStore.ShipsByPlanet(ctx, n.UserID)
				if err != nil {
					return err
				}
				e.npcRaidDecide(ctx, n, planets, techs, shipsByPlanet)
			}
		}
	}

	delay := game.NextDecisionDelay(n.Profile, n.GameSpeed, e.rng)
	return store.NewNPCStore(e.db).Schedule(ctx, n.UserID, npcScheduleAt(n.Profile, n.GameSpeed, now, delay))
}

// npcGrowthAllowed reports whether the NPC may build/research/expand right now.
// At gameSpeed > npcWindowMaxGameSpeed the window is ignored entirely (a
// compressed round has no meaningful real-time hours).
func npcGrowthAllowed(profile game.NPCProfile, gameSpeed float64, now time.Time) bool {
	if gameSpeed > npcWindowMaxGameSpeed {
		return true
	}
	return game.NPCAwake(profile, now)
}

// npcScheduleAt pushes the next decision into the NPC's online window when the
// raw delay would land outside it. It is a no-op above npcWindowMaxGameSpeed.
func npcScheduleAt(profile game.NPCProfile, gameSpeed float64, now time.Time, delay time.Duration) time.Time {
	at := now.Add(delay)
	if gameSpeed > npcWindowMaxGameSpeed {
		return at
	}
	if !game.NPCAwake(profile, at) {
		return game.NPCNextWindowOpen(profile, now)
	}
	return at
}

// npcBuildDecide tries to queue the next building level somewhere. It walks the
// NPC's planets and, on each, attempts building candidates in weighted-random
// order until one is accepted; soft failures (no fields, too expensive, queue
// busy/full) just fall through to the next candidate or planet.
func (e *EventEngine) npcBuildDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, techs map[string]int, structsByPlanet map[int64]map[string]int) bool {
	queues := store.NewQueueStore(e.db)
	for _, p := range owned {
		structs := structsByPlanet[p.ID]
		local := n.Profile
		local.Phase = game.DevelopmentTier(structs, techs)
		for _, code := range game.RankedBuildings(structs, techs, local, e.rng) {
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
func (e *EventEngine) npcResearchDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, techs map[string]int, structsByPlanet map[int64]map[string]int) bool {
	queues := store.NewQueueStore(e.db)
	for _, p := range owned {
		structs := structsByPlanet[p.ID]
		local := n.Profile
		local.Phase = game.DevelopmentTier(structs, techs)
		for _, code := range game.RankedResearch(structs, techs, local, e.rng) {
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
func (e *EventEngine) npcUnitDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, techs map[string]int, structsByPlanet map[int64]map[string]int) bool {
	queues := store.NewQueueStore(e.db)
	for _, p := range owned {
		structs := structsByPlanet[p.ID]
		local := n.Profile
		local.Phase = game.DevelopmentTier(structs, techs)
		for _, code := range game.RankedUnits(structs, techs, local, e.rng) {
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

// npcRaidDecide is the opt-in offense. If the NPC already holds an espionage
// report it consumes the intel (attack or not); otherwise it scouts a fresh
// target. It returns whether it acted. It is only called when raiding is
// enabled, inside the growth window.
func (e *EventEngine) npcRaidDecide(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, techs map[string]int, shipsByPlanet map[int64]map[string]int64) bool {
	if !e.npcRaid.Enabled || e.npcRaidLeft <= 0 {
		return false
	}
	reports := store.NewReportStore(e.db)
	report, err := reports.LatestByKind(ctx, n.UserID, "espionage")
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return false
	}
	if report != nil {
		return e.npcRaidFromReport(ctx, n, owned, techs, shipsByPlanet, report)
	}
	return e.npcScout(ctx, n, owned, shipsByPlanet)
}

// npcRaidFromReport is Phase A: read the pending espionage report, decide
// whether a raid is worth it and dispatch ATTACK, then consume the report
// either way so stale intel never piles up.
func (e *EventEngine) npcRaidFromReport(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, techs map[string]int, shipsByPlanet map[int64]map[string]int64, report *store.Report) bool {
	defer func() {
		if err := store.NewReportStore(e.db).Consume(ctx, n.UserID, report.ID); err != nil {
			log.Printf("[ENGINE] npc %d: consume report %d: %v", n.UserID, report.ID, err)
		}
	}()

	esp, err := game.ParseEspionage(report.Payload)
	if err != nil {
		return false
	}
	tg, ts, tp, ok := esp.Coords()
	if !ok {
		return false
	}
	defender := make(map[string]int64, len(esp.Ships)+len(esp.Defense))
	for code, qty := range esp.Ships {
		defender[code] += qty
	}
	for code, qty := range esp.Defense {
		defender[code] += qty
	}
	atkTechs := game.CombatTechsFromLevels(techs)
	fleets := store.NewFleetStore(e.db)
	for _, p := range owned {
		force := game.NPCRaidForce(shipsByPlanet[p.ID], e.rng)
		if force == nil {
			continue
		}
		// The report carries no defender techs, so NPCRaidWorthIt fights the
		// defender with zero tech — deliberately conservative.
		if !game.NPCRaidWorthIt(force, defender, atkTechs) {
			continue
		}
		if _, err := fleets.Dispatch(ctx, store.DispatchParams{
			UniverseCode: n.Universe, UserID: n.UserID, OriginID: p.ID,
			Mission: "ATTACK", TargetG: tg, TargetS: ts, TargetP: tp,
			SpeedPct: 1, Ships: force,
		}); err == nil {
			e.npcRaidLeft--
			return true
		} else if !npcSoftFailure(err) {
			return false
		}
	}
	return false
}

// npcScout is Phase B: with a probe and some warships, gate on the raid chance
// and send one probe at a random eligible target. The next decision finds the
// report and runs Phase A.
func (e *EventEngine) npcScout(ctx context.Context, n store.NPC, owned []store.OwnedPlanet, shipsByPlanet map[int64]map[string]int64) bool {
	npcs := store.NewNPCStore(e.db)
	if has, err := npcs.HasOutboundAttack(ctx, n.UserID); err != nil || has {
		return false
	}
	hasProbe, hasWarships := false, false
	for _, ships := range shipsByPlanet {
		if ships["spy_probe"] > 0 {
			hasProbe = true
		}
		for code, qty := range ships {
			if qty > 0 && game.IsWarship(code) {
				hasWarships = true
				break
			}
		}
	}
	if !hasProbe || !hasWarships {
		return false
	}
	if e.rng.Float64() >= game.NPCRaidChance(n.Profile)*e.npcRaid.ChanceScale {
		return false
	}
	_, g, s, pos, ok, err := npcs.RandomRaidTarget(ctx, n.Universe, n.UserID)
	if err != nil || !ok {
		return false
	}
	fleets := store.NewFleetStore(e.db)
	for _, p := range owned {
		if shipsByPlanet[p.ID]["spy_probe"] <= 0 {
			continue
		}
		_, err := fleets.Dispatch(ctx, store.DispatchParams{
			UniverseCode: n.Universe, UserID: n.UserID, OriginID: p.ID,
			Mission: "ESPIONAGE", TargetG: g, TargetS: s, TargetP: pos,
			SpeedPct: 1, Ships: map[string]int64{"spy_probe": 1},
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
		errors.Is(err, store.ErrSanctioned) ||
		errors.Is(err, store.ErrTargetOnVacation)
}
