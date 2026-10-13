package integration

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestResearchQueueChains queues several levels of the same tech and checks that
// they target sequential levels and are scheduled back-to-back, then all land in
// order.
func TestResearchQueueChains(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "researcher")
	h.setBuilding(t, p.home, "research_center", 1)
	h.topUp(t, p)

	path := fmt.Sprintf("/research/%d/start", p.home)
	for i := 0; i < 3; i++ {
		h.action(t, p.client, path, url.Values{"code": {"combustion_drive"}})
	}

	rows, err := h.db.Query(`
		SELECT target_level, start_at, finish_at FROM queues
		WHERE user_id = $1 AND kind = 'research' AND status = 'IN_PROGRESS'
		ORDER BY start_at, id
	`, p.id)
	if err != nil {
		t.Fatalf("read research queue: %v", err)
	}
	defer rows.Close()
	var targets []int64
	var starts, finishes []time.Time
	for rows.Next() {
		var lvl int64
		var start, finish time.Time
		if err := rows.Scan(&lvl, &start, &finish); err != nil {
			t.Fatalf("scan queue row: %v", err)
		}
		targets = append(targets, lvl)
		starts = append(starts, start)
		finishes = append(finishes, finish)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate queue rows: %v", err)
	}

	want := []int64{1, 2, 3}
	if len(targets) != len(want) {
		t.Fatalf("queued %d orders, want %d (%v)", len(targets), len(want), targets)
	}
	for i := range want {
		if targets[i] != want[i] {
			t.Errorf("order %d targets level %d, want %d", i+1, targets[i], want[i])
		}
		if !finishes[i].After(starts[i]) {
			t.Errorf("order %d finish %v not after start %v", i+1, finishes[i], starts[i])
		}
		if i > 0 && !starts[i].Equal(finishes[i-1]) {
			t.Errorf("order %d starts at %v, want previous finish %v", i+1, starts[i], finishes[i-1])
		}
	}

	// The card previews the level after everything queued.
	if _, body := h.get(t, p.client, fmt.Sprintf("/research/%d", p.home)); !strings.Contains(body, "Level 0 (+3)") {
		t.Errorf("research page does not show the queued-level badge:\n%s", snippet([]byte(body)))
	} else if !strings.Contains(body, "Research queue") {
		t.Error("research page is missing the research queue panel")
	}

	waitFor(t, "combustion drive to reach level 3", func() bool {
		return h.queryInt(t, `SELECT level FROM user_techs WHERE user_id = $1 AND code = 'combustion_drive'`, p.id) >= 3
	})
	waitFor(t, "research queue to drain", func() bool {
		return h.queryInt(t, `
			SELECT count(*) FROM queues
			WHERE user_id = $1 AND kind = 'research' AND status = 'IN_PROGRESS'
		`, p.id) == 0
	})
}

// TestResearchQueueFull rejects an order once the account's research_queue_slots
// are all taken.
func TestResearchQueueFull(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "crowded-lab")
	h.setBuilding(t, p.home, "research_center", 1)

	if _, err := h.db.Exec(`UPDATE universes SET research_queue_slots = 2 WHERE code_name = $1`, h.universe); err != nil {
		t.Fatalf("shrink research queue: %v", err)
	}
	h.topUp(t, p)

	path := fmt.Sprintf("/research/%d/start", p.home)
	h.action(t, p.client, path, url.Values{"code": {"combustion_drive"}})
	h.action(t, p.client, path, url.Values{"code": {"combustion_drive"}})

	if _, errMsg := h.postFlash(t, p.client, path, url.Values{"code": {"combustion_drive"}}); !strings.Contains(errMsg, "research queue is full") {
		t.Fatalf("third order flash = %q, want a full-queue error", errMsg)
	}
	if n := h.queryInt(t, `
		SELECT count(*) FROM queues
		WHERE user_id = $1 AND kind = 'research' AND status = 'IN_PROGRESS'
	`, p.id); n != 2 {
		t.Errorf("active research orders = %d, want 2", n)
	}
}

// TestResearchQueueCancelRenumbers cancels the first of several queued levels
// and checks that the survivors close the gap and shift down one level.
func TestResearchQueueCancelRenumbers(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "renumber-lab")
	h.setBuilding(t, p.home, "research_center", 1)
	h.topUp(t, p)

	path := fmt.Sprintf("/research/%d/start", p.home)
	for i := 0; i < 3; i++ {
		h.action(t, p.client, path, url.Values{"code": {"combustion_drive"}})
	}

	var firstID int64
	if err := h.db.QueryRow(`
		SELECT id FROM queues
		WHERE user_id = $1 AND kind = 'research' AND status = 'IN_PROGRESS'
		ORDER BY start_at, id LIMIT 1
	`, p.id).Scan(&firstID); err != nil {
		t.Fatalf("find first order: %v", err)
	}
	h.action(t, p.client, fmt.Sprintf("/queue/%d/cancel", firstID), nil)

	rows, err := h.db.Query(`
		SELECT target_level FROM queues
		WHERE user_id = $1 AND kind = 'research' AND status = 'IN_PROGRESS'
		ORDER BY start_at, id
	`, p.id)
	if err != nil {
		t.Fatalf("read surviving queue: %v", err)
	}
	defer rows.Close()
	var targets []int64
	for rows.Next() {
		var lvl int64
		if err := rows.Scan(&lvl); err != nil {
			t.Fatalf("scan survivor: %v", err)
		}
		targets = append(targets, lvl)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate survivors: %v", err)
	}
	if len(targets) != 2 || targets[0] != 1 || targets[1] != 2 {
		t.Fatalf("surviving targets = %v, want [1 2]", targets)
	}

	waitFor(t, "combustion drive to reach level 2", func() bool {
		return h.queryInt(t, `SELECT level FROM user_techs WHERE user_id = $1 AND code = 'combustion_drive'`, p.id) >= 2
	})
}
