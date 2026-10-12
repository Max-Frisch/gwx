package integration

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// TestFullLoop walks the whole BACKLOG full-loop acceptance path against a real
// Postgres and the real scheduler, at x1000 game speed / x100000 fleet speed:
//
//	register → build → research → shipyard → raid → loot → return → colonize
//
// Every game action goes through the HTTP UI (so routing, validation, queueing
// and the engine are all exercised); state is verified directly in the database.
func TestFullLoop(t *testing.T) {
	h := newHarness(t)

	var attacker, defender *player

	t.Run("register", func(t *testing.T) {
		attacker = h.register(t, "attacker")
		defender = h.register(t, "defender")
		if attacker.home == 0 || defender.home == 0 {
			t.Fatal("registration did not provision a homeworld")
		}
		if got := h.queryInt(t, `
			SELECT amount FROM planet_items
			WHERE planet_id = $1 AND kind = 'building' AND code = 'command_center'
		`, attacker.home); got != 1 {
			t.Fatalf("homeworld command center = %d, want 1", got)
		}
	})

	t.Run("build", func(t *testing.T) {
		// Research Center needs the starter Command Center 1.
		h.build(t, attacker, "research_center", 1)
	})

	t.Run("research", func(t *testing.T) {
		// Jackal gate: Ship Factory 1 + Combustion Drive 1.
		h.research(t, attacker, "combustion_drive", 1)
		h.build(t, attacker, "shipyard", 1)
	})

	t.Run("shipyard", func(t *testing.T) {
		h.buildUnits(t, attacker, "shipyard", "jackal", 50, 50)
		// The resolved production queue must leave the finished ships stationed.
		got := h.queryInt(t, `
			SELECT amount FROM planet_items
			WHERE planet_id = $1 AND kind = 'ship' AND code = 'jackal'
		`, attacker.home)
		if got != 50 {
			t.Fatalf("jackals stationed = %d, want 50", got)
		}
	})

	t.Run("raid_loot_return", func(t *testing.T) {
		// A defenceless neighbour sitting on a pile of resources.
		h.seed(t, defender.home, 100_000, 100_000, 100_000, 100_000)
		target := h.at(t, defender.home)

		h.topUp(t, attacker) // hydrogen for the round-trip fuel
		beforeAttacker := h.resourcesOf(t, attacker.home)
		beforeDefender := h.resourcesOf(t, defender.home)

		h.dispatch(t, attacker, "ATTACK", target, map[string]int64{"jackal": 50}, nil)

		waitFor(t, "attack fleet to turn around", func() bool {
			return h.queryInt(t, `
				SELECT count(*) FROM fleets
				WHERE user_id = $1 AND mission = 'ATTACK' AND phase IN ('RETURNING','RESOLVED')
			`, attacker.id) > 0
		})
		waitFor(t, "raid fleet to get home", func() bool {
			return h.queryInt(t, `
				SELECT count(*) FROM fleets
				WHERE user_id = $1 AND mission = 'ATTACK' AND phase = 'RESOLVED'
			`, attacker.id) > 0
		})

		afterAttacker := h.resourcesOf(t, attacker.home)
		afterDefender := h.resourcesOf(t, defender.home)

		if afterDefender.metal >= beforeDefender.metal {
			t.Errorf("defender metal not plundered: %d -> %d", beforeDefender.metal, afterDefender.metal)
		}
		if afterAttacker.metal <= beforeAttacker.metal {
			t.Errorf("loot not delivered home: %d -> %d", beforeAttacker.metal, afterAttacker.metal)
		}

		// Both sides get a combat report; the attacker also gets a message.
		if n := h.queryInt(t, `
			SELECT count(*) FROM reports WHERE user_id = $1 AND kind = 'combat'
		`, attacker.id); n == 0 {
			t.Error("attacker has no combat report")
		}
		if n := h.queryInt(t, `
			SELECT count(*) FROM reports WHERE user_id = $1 AND kind = 'combat'
		`, defender.id); n == 0 {
			t.Error("defender has no combat report")
		}
		if n := h.queryInt(t, `
			SELECT count(*) FROM messages WHERE user_id = $1 AND kind = 'attack'
		`, attacker.id); n == 0 {
			t.Error("attacker has no attack message")
		}
	})

	t.Run("colonize", func(t *testing.T) {
		// Colony Ship needs Ship Factory 1 + Ion Drive 4. Seed those directly so
		// the test focuses on the colonisation flight, not the deep tech grind
		// (a level-10 factory keeps the captured build time short at test speed).
		h.setBuilding(t, attacker.home, "shipyard", 10)
		h.setTech(t, attacker.id, "ion_drive", 4)

		h.buildUnits(t, attacker, "shipyard", "colony_ship", 1, 1)

		slot := h.freeSlot(t)
		h.topUp(t, attacker)
		h.dispatch(t, attacker, "COLONIZE", slot, map[string]int64{"colony_ship": 1}, nil)

		waitFor(t, "colony to be founded", func() bool {
			return h.queryInt(t, `
				SELECT count(*) FROM planets
				WHERE user_id = $1 AND galaxy = $2 AND system = $3 AND position = $4
			`, attacker.id, slot.g, slot.s, slot.pos) == 1
		})

		var isHome bool
		var metal int64
		if err := h.db.QueryRow(`
			SELECT is_homeworld, metal::bigint FROM planets
			WHERE user_id = $1 AND galaxy = $2 AND system = $3 AND position = $4
		`, attacker.id, slot.g, slot.s, slot.pos).Scan(&isHome, &metal); err != nil {
			t.Fatalf("read colony: %v", err)
		}
		if isHome {
			t.Error("colony must not be flagged as homeworld")
		}
		if metal < 1000 {
			t.Errorf("colony seed metal = %d, want at least 1000", metal)
		}
		if owned := h.queryInt(t, `SELECT count(*) FROM planets WHERE user_id = $1`, attacker.id); owned != 2 {
			t.Errorf("attacker owns %d planets, want 2", owned)
		}
	})

	// The loop touches most systems; make sure the derived feeds all render.
	t.Run("surfaces", func(t *testing.T) {
		for _, path := range []string{
			fmt.Sprintf("/overview/%d", attacker.home),
			fmt.Sprintf("/buildings/%d", attacker.home),
			fmt.Sprintf("/research/%d", attacker.home),
			fmt.Sprintf("/shipyard/%d", attacker.home),
			fmt.Sprintf("/defense/%d", attacker.home),
			"/messages",
			"/events",
			"/highscore",
			"/statistics",
			"/reports",
		} {
			if status, body := h.get(t, attacker.client, path); status != 200 {
				t.Errorf("GET %s -> %d: %s", path, status, snippet([]byte(body)))
			}
		}
		if _, body := h.get(t, attacker.client, "/reports"); !strings.Contains(body, "Attack on") {
			t.Error("reports page does not show the raid")
		}
	})
}

// TestTransportMovesResources is a smaller, independent check that the transport
// mission credits the target and returns the ships — the second most-used fleet
// path after attack.
func TestTransportMovesResources(t *testing.T) {
	h := newHarness(t)

	sender := h.register(t, "sender")
	receiver := h.register(t, "receiver")

	// Small Trader gate: Ship Factory 1 + Combustion Drive 1.
	h.setBuilding(t, sender.home, "shipyard", 1)
	h.setTech(t, sender.id, "combustion_drive", 1)
	h.buildUnits(t, sender, "shipyard", "small_trader", 1, 1)

	target := h.at(t, receiver.home)
	h.seed(t, receiver.home, 10_000, 10_000, 10_000, 10_000)
	before := h.resourcesOf(t, receiver.home)

	h.topUp(t, sender)
	h.dispatch(t, sender, "TRANSPORT", target, map[string]int64{"small_trader": 1}, url.Values{
		"cargo_metal":   {"4000"},
		"cargo_lutinum": {"1000"},
	})

	waitFor(t, "transport to be delivered", func() bool {
		return h.resourcesOf(t, receiver.home).metal >= before.metal+4000
	})
	if got := h.resourcesOf(t, receiver.home).lutinum; got < before.lutinum+1000 {
		t.Errorf("receiver lutinum = %d, want at least %d", got, before.lutinum+1000)
	}

	waitFor(t, "transport ships to return home", func() bool {
		return h.queryInt(t, `
			SELECT count(*) FROM fleets
			WHERE user_id = $1 AND mission = 'TRANSPORT' AND phase = 'RESOLVED'
		`, sender.id) > 0
	})
	if got := h.queryInt(t, `
		SELECT amount FROM planet_items
		WHERE planet_id = $1 AND kind = 'ship' AND code = 'small_trader'
	`, sender.home); got != 1 {
		t.Errorf("small trader did not return: stationed = %d, want 1", got)
	}
}
