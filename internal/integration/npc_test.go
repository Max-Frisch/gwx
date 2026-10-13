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
