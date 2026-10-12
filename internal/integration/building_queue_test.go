package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestBuildingQueueChains queues several levels of the same building and checks
// that they target sequential levels and are scheduled back-to-back, then all
// land in order.
func TestBuildingQueueChains(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "builder")
	h.topUp(t, p)

	path := fmt.Sprintf("/buildings/%d/build", p.home)
	for i := 0; i < 3; i++ {
		h.action(t, p.client, path, url.Values{"code": {"command_center"}})
	}

	rows, err := h.db.Query(`
		SELECT target_level, start_at, finish_at FROM queues
		WHERE planet_id = $1 AND kind = 'building' AND status = 'IN_PROGRESS'
		ORDER BY start_at, id
	`, p.home)
	if err != nil {
		t.Fatalf("read build queue: %v", err)
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

	want := []int64{2, 3, 4}
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
	if _, body := h.get(t, p.client, fmt.Sprintf("/buildings/%d", p.home)); !strings.Contains(body, "Level 1 (+3)") {
		t.Errorf("buildings page does not show the queued-level badge:\n%s", snippet([]byte(body)))
	} else if !strings.Contains(body, "Construction queue") {
		t.Error("buildings page is missing the construction queue panel")
	}

	waitFor(t, "command center to reach level 4", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items
			WHERE planet_id = $1 AND kind = 'building' AND code = 'command_center'
		`, p.home) >= 4
	})
	waitFor(t, "build queue to drain", func() bool {
		return h.queryInt(t, `
			SELECT count(*) FROM queues
			WHERE planet_id = $1 AND kind = 'building' AND status = 'IN_PROGRESS'
		`, p.home) == 0
	})
}

// TestBuildingQueueFull rejects an order once the planet's build_queue_slots are
// all taken.
func TestBuildingQueueFull(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "crowded")

	if _, err := h.db.Exec(`UPDATE universes SET build_queue_slots = 2 WHERE code_name = $1`, h.universe); err != nil {
		t.Fatalf("shrink build queue: %v", err)
	}
	h.topUp(t, p)

	path := fmt.Sprintf("/buildings/%d/build", p.home)
	h.action(t, p.client, path, url.Values{"code": {"command_center"}})
	h.action(t, p.client, path, url.Values{"code": {"command_center"}})

	if _, errMsg := h.postFlash(t, p.client, path, url.Values{"code": {"command_center"}}); !strings.Contains(errMsg, "queue is full") {
		t.Fatalf("third order flash = %q, want a full-queue error", errMsg)
	}
	if n := h.queryInt(t, `
		SELECT count(*) FROM queues
		WHERE planet_id = $1 AND kind = 'building' AND status = 'IN_PROGRESS'
	`, p.home); n != 2 {
		t.Errorf("active building orders = %d, want 2", n)
	}
}

// TestBuildingQueueCancelRenumbers cancels the first of several queued levels
// and checks that the survivors close the gap and shift down one level.
func TestBuildingQueueCancelRenumbers(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "renumberer")
	h.topUp(t, p)

	path := fmt.Sprintf("/buildings/%d/build", p.home)
	for i := 0; i < 3; i++ {
		h.action(t, p.client, path, url.Values{"code": {"command_center"}})
	}

	var firstID int64
	if err := h.db.QueryRow(`
		SELECT id FROM queues
		WHERE planet_id = $1 AND kind = 'building' AND status = 'IN_PROGRESS'
		ORDER BY start_at, id LIMIT 1
	`, p.home).Scan(&firstID); err != nil {
		t.Fatalf("find first order: %v", err)
	}
	h.action(t, p.client, fmt.Sprintf("/queue/%d/cancel", firstID), nil)

	rows, err := h.db.Query(`
		SELECT target_level FROM queues
		WHERE planet_id = $1 AND kind = 'building' AND status = 'IN_PROGRESS'
		ORDER BY start_at, id
	`, p.home)
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
	if len(targets) != 2 || targets[0] != 2 || targets[1] != 3 {
		t.Fatalf("surviving targets = %v, want [2 3]", targets)
	}

	waitFor(t, "command center to reach level 3", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items
			WHERE planet_id = $1 AND kind = 'building' AND code = 'command_center'
		`, p.home) >= 3
	})
}

// postFlash POSTs a form and returns the redirect's flash query values without
// treating an "err" as a test failure (for negative-path checks).
func (h *harness) postFlash(t *testing.T, c *http.Client, path string, form url.Values) (okMsg, errMsg string) {
	t.Helper()
	resp, err := c.PostForm(h.server.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	q := resp.Request.URL.Query()
	return q.Get("ok"), q.Get("err")
}
