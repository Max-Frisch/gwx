package integration

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// unitOrder describes one active ship/defense batch read back from the queue.
type unitOrder struct {
	id       int64
	code     string
	quantity int64
	start    time.Time
	finish   time.Time
}

// activeUnitOrders reads a planet's in-progress batches of one kind in the
// order they will run.
func (h *harness) activeUnitOrders(t *testing.T, planetID int64, kind string) []unitOrder {
	t.Helper()
	rows, err := h.db.Query(`
		SELECT id, code, quantity, start_at, finish_at FROM queues
		WHERE planet_id = $1 AND kind = $2 AND status = 'IN_PROGRESS'
		ORDER BY start_at, id
	`, planetID, kind)
	if err != nil {
		t.Fatalf("read %s queue: %v", kind, err)
	}
	defer rows.Close()
	var out []unitOrder
	for rows.Next() {
		var o unitOrder
		if err := rows.Scan(&o.id, &o.code, &o.quantity, &o.start, &o.finish); err != nil {
			t.Fatalf("scan %s order: %v", kind, err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s orders: %v", kind, err)
	}
	return out
}

// TestUnitQueueChainsUnlimited queues several ship batches and checks the queue
// is unlimited and scheduled back-to-back (each batch starts when the previous
// finishes), then that the shipyard page lists every batch.
func TestUnitQueueChainsUnlimited(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "shipwright")
	// Slow the universe to 1x so the batches do not resolve before we inspect
	// them (the test universe otherwise runs at x10000).
	if _, err := h.db.Exec(`UPDATE universes SET game_speed = 1 WHERE code_name = $1`, h.universe); err != nil {
		t.Fatalf("set 1x speed: %v", err)
	}
	h.setBuilding(t, p.home, "shipyard", 1)
	h.topUp(t, p)

	path := fmt.Sprintf("/shipyard/%d/build", p.home)
	// More batches than any queue slot: the unit queue has no cap.
	for i := 0; i < 8; i++ {
		h.action(t, p.client, path, url.Values{"code": {"spy_probe"}, "qty": {"2"}})
	}

	orders := h.activeUnitOrders(t, p.home, "ship")
	if len(orders) != 8 {
		t.Fatalf("queued %d ship batches, want 8 (unlimited queue)", len(orders))
	}
	for i, o := range orders {
		if o.code != "spy_probe" || o.quantity != 2 {
			t.Errorf("order %d = %dx %s, want 2x spy_probe", i+1, o.quantity, o.code)
		}
		if !o.finish.After(o.start) {
			t.Errorf("order %d finish %v not after start %v", i+1, o.finish, o.start)
		}
		if i > 0 && !o.start.Equal(orders[i-1].finish) {
			t.Errorf("order %d starts at %v, want previous finish %v", i+1, o.start, orders[i-1].finish)
		}
	}

	// The shipyard page lists the whole queue with checkboxes and both buttons.
	if _, body := h.get(t, p.client, fmt.Sprintf("/shipyard/%d", p.home)); !strings.Contains(body, "Production queue") {
		t.Errorf("shipyard page is missing the production queue panel:\n%s", snippet([]byte(body)))
	} else if !strings.Contains(body, "Cancel selected") || !strings.Contains(body, "Cancel all") {
		t.Error("shipyard production queue is missing its cancel controls")
	}
}

// TestUnitQueueCancelSelected cancels two of several queued batches and checks
// that the survivors close the gap and keep running back-to-back.
func TestUnitQueueCancelSelected(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "selective")
	if _, err := h.db.Exec(`UPDATE universes SET game_speed = 1 WHERE code_name = $1`, h.universe); err != nil {
		t.Fatalf("set 1x speed: %v", err)
	}
	h.setBuilding(t, p.home, "shipyard", 1)
	h.topUp(t, p)

	path := fmt.Sprintf("/shipyard/%d/build", p.home)
	for i := 0; i < 4; i++ {
		h.action(t, p.client, path, url.Values{"code": {"spy_probe"}, "qty": {"1"}})
	}
	before := h.activeUnitOrders(t, p.home, "ship")
	if len(before) != 4 {
		t.Fatalf("queued %d batches, want 4", len(before))
	}

	// Cancel the first and the third.
	h.action(t, p.client, fmt.Sprintf("/queue/%d/cancel-selected", p.home), url.Values{
		"kind":  {"ship"},
		"queue": {fmt.Sprint(before[0].id), fmt.Sprint(before[2].id)},
	})

	after := h.activeUnitOrders(t, p.home, "ship")
	if len(after) != 2 {
		t.Fatalf("active batches after cancel = %d, want 2", len(after))
	}
	if after[0].id != before[1].id || after[1].id != before[3].id {
		t.Errorf("survivors = [%d %d], want [%d %d]", after[0].id, after[1].id, before[1].id, before[3].id)
	}
	// The remaining batches must still be chained back-to-back.
	if !after[1].start.Equal(after[0].finish) {
		t.Errorf("survivor 2 starts at %v, want survivor 1 finish %v", after[1].start, after[0].finish)
	}
	if !after[0].start.Before(before[1].start.Add(time.Second)) {
		t.Errorf("survivor 1 did not move earlier: now %v, was %v", after[0].start, before[1].start)
	}
}

// TestUnitQueueCancelAll empties a planet's ship production line, leaves its
// defense line untouched, and is driven by the confirmed Cancel all button.
func TestUnitQueueCancelAll(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "clearout")
	if _, err := h.db.Exec(`UPDATE universes SET game_speed = 1 WHERE code_name = $1`, h.universe); err != nil {
		t.Fatalf("set 1x speed: %v", err)
	}
	h.setBuilding(t, p.home, "shipyard", 1)
	h.setBuilding(t, p.home, "orbital_defense", 1)
	h.topUp(t, p)

	shipPath := fmt.Sprintf("/shipyard/%d/build", p.home)
	for i := 0; i < 3; i++ {
		h.action(t, p.client, shipPath, url.Values{"code": {"spy_probe"}, "qty": {"1"}})
	}
	h.action(t, p.client, fmt.Sprintf("/defense/%d/build", p.home), url.Values{"code": {"light_laser"}, "qty": {"1"}})

	if n := len(h.activeUnitOrders(t, p.home, "ship")); n != 3 {
		t.Fatalf("queued %d ship batches, want 3", n)
	}
	if n := len(h.activeUnitOrders(t, p.home, "defense")); n != 1 {
		t.Fatalf("queued %d defense batches, want 1", n)
	}

	h.action(t, p.client, fmt.Sprintf("/queue/%d/cancel-all", p.home), url.Values{"kind": {"ship"}})

	if n := len(h.activeUnitOrders(t, p.home, "ship")); n != 0 {
		t.Errorf("ship batches after Cancel all = %d, want 0", n)
	}
	if n := len(h.activeUnitOrders(t, p.home, "defense")); n != 1 {
		t.Errorf("Cancel all ships also cleared the defense line: %d batches left", n)
	}

	// The defense page still offers its own queue controls.
	if _, body := h.get(t, p.client, fmt.Sprintf("/defense/%d", p.home)); !strings.Contains(body, "Do you really want to cancel all defenses?") {
		t.Errorf("defense page is missing the production queue confirmation")
	}
}
