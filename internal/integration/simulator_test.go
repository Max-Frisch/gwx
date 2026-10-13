package integration

import (
	"net/url"
	"strings"
	"testing"
)

// TestBattleSimulator drives the /simulator page through the real HTTP surface:
// a GET seeds the columns, and a POST runs the line-up and shows the win
// chance. It proves routing, ownership and form parsing line up end to end.
func TestBattleSimulator(t *testing.T) {
	h := newHarness(t)
	me := h.register(t, "sim-commander")
	h.giveUnit(t, me.home, "ship", "longeagle_x", 100)
	h.giveUnit(t, me.home, "defense", "light_laser", 1)

	status, body := h.get(t, me.client, "/simulator/"+itoa(int(me.home)))
	if status != 200 {
		t.Fatalf("GET /simulator -> %d", status)
	}
	if !strings.Contains(body, "Battle simulator") {
		t.Fatal("simulator page did not render")
	}
	if !strings.Contains(body, `name="a_longeagle_x"`) {
		t.Error("simulator page did not seed the attacker column")
	}

	body = h.action(t, me.client, "/simulator/"+itoa(int(me.home)), url.Values{
		"a_longeagle_x": {"100"},
		"d_light_laser": {"1"},
	})
	if !strings.Contains(body, "100.00%") {
		t.Errorf("simulator result missing the win chance:\n%s", snippet([]byte(body)))
	}
}
