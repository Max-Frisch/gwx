package integration

import (
	"context"
	"strings"
	"testing"

	"gwx/internal/game"
	"gwx/internal/store"
)

// TestNPCBrainBuilds wires the durable brain against a real Postgres universe:
// an NPC is seeded, the scheduler gives it a decision, and it queues a building
// the same way a player would.
func TestNPCBrainBuilds(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	npcs := store.NewNPCStore(h.db)
	userID, homeID, err := npcs.Create(ctx, h.universe, game.DefaultNPCProfile())
	if err != nil {
		t.Fatalf("create npc: %v", err)
	}

	var isNPC bool
	var username string
	if err := h.db.QueryRow(`SELECT is_npc, username FROM users WHERE id = $1`, userID).Scan(&isNPC, &username); err != nil {
		t.Fatalf("load npc: %v", err)
	}
	if !isNPC {
		t.Fatal("npc account is not flagged is_npc")
	}

	// Even knowing the exact handle, an NPC can never authenticate.
	if _, _, err := store.NewAuthStore(h.db).UserAuth(ctx, username); err == nil {
		t.Fatal("NPC authenticated via UserAuth, want rejection")
	}

	// The brain should queue at least one building on the homeworld.
	waitFor(t, "npc building order", func() bool {
		return h.queryInt(t, `SELECT COUNT(*) FROM queues WHERE user_id = $1 AND kind = 'building'`, userID) > 0
	})

	// And the homeworld must actually be the one we created.
	if n := h.queryInt(t, `SELECT COUNT(*) FROM planets WHERE id = $1 AND user_id = $2`, homeID, userID); n != 1 {
		t.Fatalf("npc homeworld lookup = %d, want 1", n)
	}
}

// TestNPCBrainProducesUnits checks the brain arms itself: with a shipyard and
// orbital defense standing, a decision cycle queues a ship/defense batch the
// same way a player's form does.
func TestNPCBrainProducesUnits(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	npcs := store.NewNPCStore(h.db)
	userID, homeID, err := npcs.Create(ctx, h.universe, game.DefaultNPCProfile())
	if err != nil {
		t.Fatalf("create npc: %v", err)
	}
	h.seed(t, homeID, seedAmount, seedAmount, seedAmount, seedAmount)
	h.setBuilding(t, homeID, "shipyard", 1)
	h.setBuilding(t, homeID, "orbital_defense", 1)

	waitFor(t, "npc unit production", func() bool {
		h.forceNPCDecision(t, userID)
		return h.queryInt(t, `
			SELECT COUNT(*) FROM queues WHERE user_id = $1 AND kind IN ('ship','defense')
		`, userID) > 0
	})
}

// TestNPCBrainReactsWithEspionage checks the attack reaction: an inbound ATTACK
// eventually prompts the NPC to send a probe at the attacker.
func TestNPCBrainReactsWithEspionage(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	attacker := h.register(t, "npc-scout-target")
	npcs := store.NewNPCStore(h.db)
	userID, homeID, err := npcs.Create(ctx, h.universe, game.DefaultNPCProfile())
	if err != nil {
		t.Fatalf("create npc: %v", err)
	}
	h.seed(t, homeID, seedAmount, seedAmount, seedAmount, seedAmount)
	h.giveUnit(t, homeID, "ship", "spy_probe", 3)
	h.injectAttack(t, attacker, homeID)

	waitFor(t, "npc espionage reaction", func() bool {
		h.forceNPCDecision(t, userID)
		return h.queryInt(t, `
			SELECT COUNT(*) FROM fleets WHERE user_id = $1 AND mission = 'ESPIONAGE'
		`, userID) > 0
	})
}

// TestNPCBrainFleetSaves checks the sloppy "fleet in the air" save: on an
// inbound attack the NPC eventually lifts its ships off the planet.
func TestNPCBrainFleetSaves(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	attacker := h.register(t, "npc-save-target")
	npcs := store.NewNPCStore(h.db)
	userID, homeID, err := npcs.Create(ctx, h.universe, game.DefaultNPCProfile())
	if err != nil {
		t.Fatalf("create npc: %v", err)
	}
	h.seed(t, homeID, seedAmount, seedAmount, seedAmount, seedAmount)
	h.giveUnit(t, homeID, "ship", "jackal", 3)
	h.injectAttack(t, attacker, homeID)

	waitFor(t, "npc fleet-save", func() bool {
		h.forceNPCDecision(t, userID)
		return h.queryInt(t, `
			SELECT COUNT(*) FROM fleets WHERE user_id = $1 AND mission = 'TRANSPORT'
		`, userID) > 0
	})
}

// TestNPCBrainColonizes checks expansion: an NPC sitting on a colony ship
// eventually plants a second planet.
func TestNPCBrainColonizes(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	npcs := store.NewNPCStore(h.db)
	userID, homeID, err := npcs.Create(ctx, h.universe, game.DefaultNPCProfile())
	if err != nil {
		t.Fatalf("create npc: %v", err)
	}
	h.seed(t, homeID, seedAmount, seedAmount, seedAmount, seedAmount)
	h.giveUnit(t, homeID, "ship", "colony_ship", 1)

	waitFor(t, "npc colony", func() bool {
		h.forceNPCDecision(t, userID)
		return h.queryInt(t, `SELECT COUNT(*) FROM planets WHERE user_id = $1`, userID) > 1
	})
}

// forceNPCDecision makes an NPC due again so the fast test scheduler gives it
// another decision cycle without waiting the human-ish 4..30 minutes.
func (h *harness) forceNPCDecision(t *testing.T, userID int64) {
	t.Helper()
	if _, err := h.db.Exec(`UPDATE users SET npc_next_decision_at = NOW() WHERE id = $1`, userID); err != nil {
		t.Fatalf("force npc decision: %v", err)
	}
}

// giveUnit plants a ship/defense batch directly on a planet (test shortcut).
func (h *harness) giveUnit(t *testing.T, planetID int64, kind, code string, amount int64) {
	t.Helper()
	if _, err := h.db.Exec(`
		INSERT INTO planet_items (planet_id, kind, code, amount) VALUES ($1, $2, $3, $4)
		ON CONFLICT (planet_id, kind, code) DO UPDATE SET amount = EXCLUDED.amount
	`, planetID, kind, code, amount); err != nil {
		t.Fatalf("give unit %s: %v", code, err)
	}
}

// injectAttack plants an inbound ATTACK fleet from attacker's homeworld to the
// target, arriving far enough out that it only ever reads as "incoming".
func (h *harness) injectAttack(t *testing.T, attacker *player, targetID int64) {
	t.Helper()
	oc := h.at(t, attacker.home)
	tc := h.at(t, targetID)
	if _, err := h.db.Exec(`
		INSERT INTO fleets (
			universe_id, user_id, mission, phase, origin_id, target_id,
			origin_galaxy, origin_system, origin_pos,
			target_galaxy, target_system, target_pos,
			start_at, arrival_at, speed_pct, hydrogen_cost,
			cargo_metal, cargo_lutinum, cargo_water, cargo_hydrogen
		) VALUES (
			(SELECT id FROM universes WHERE code_name = $1), $2, 'ATTACK', 'OUTBOUND', $3, $4,
			$5, $6, $7, $8, $9, $10,
			NOW() - INTERVAL '1 hour', NOW() + INTERVAL '1 day', 1.00, 0,
			0, 0, 0, 0
		)
	`, h.universe, attacker.id, attacker.home, targetID,
		oc.g, oc.s, oc.pos, tc.g, tc.s, tc.pos); err != nil {
		t.Fatalf("inject attack: %v", err)
	}
}

// TestNPCShowsRobotMarker checks the highscore and galaxy views flag NPCs so
// players can tell them apart from humans.
func TestNPCShowsRobotMarker(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	npcs := store.NewNPCStore(h.db)
	npcs.EnsureCount(ctx, h.universe, 1)

	var g, s int
	if err := h.db.QueryRow(`
		SELECT p.galaxy, p.system FROM planets p
		JOIN users u ON u.id = p.user_id
		WHERE u.is_npc LIMIT 1
	`).Scan(&g, &s); err != nil {
		t.Fatalf("locate npc planet: %v", err)
	}

	me := h.register(t, "marker-scout")

	if _, body := h.get(t, me.client, "/highscore?tab=players"); !strings.Contains(body, "&#129302;") {
		t.Error("highscore players tab does not mark the NPC")
	}
	if _, body := h.get(t, me.client, "/galaxy/"+itoa(g)+"/"+itoa(s)); !strings.Contains(body, "&#129302;") {
		t.Error("galaxy view does not mark the NPC")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
