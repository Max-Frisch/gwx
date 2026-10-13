package integration

import (
	"net/url"
	"strings"
	"testing"
)

// TestGalaxyQuickDispatch drives the galaxy view's one-click spy action through
// the real HTTP surface: the stored settings.spyShortcutCount is offered on the
// row (title + button) and the dispatched ESPIONAGE fleet carries exactly that
// many probes.
func TestGalaxyQuickDispatch(t *testing.T) {
	h := newHarness(t)
	me := h.register(t, "spy-commander")
	rival := h.register(t, "spy-target")
	h.topUp(t, me)
	h.giveUnit(t, me.home, "ship", "spy_probe", 10)

	// Configure quick dispatch to 3 probes through the real settings form.
	h.action(t, me.client, "/settings/game", url.Values{
		"navigationPosition": {"left"},
		"locale":             {"en"},
		"messagesPerPage":    {"10"},
		"spyShortcutCount":   {"3"},
		"spyLimitOnSpy":      {"200"},
	})

	c := h.at(t, rival.home)
	status, body := h.get(t, me.client, "/galaxy/"+itoa(c.g)+"/"+itoa(c.s))
	if status != 200 {
		t.Fatalf("GET /galaxy -> %d", status)
	}
	if !strings.Contains(body, `action="/galaxy/spy"`) {
		t.Fatalf("galaxy page has no spy action:\n%s", snippet([]byte(body)))
	}
	if !strings.Contains(body, "Send 3 spy probe(s)") {
		t.Error("galaxy spy action did not use the configured probe count")
	}

	// Click it: the probe count comes from the settings, not a form field.
	h.action(t, me.client, "/galaxy/spy", url.Values{
		"g": {itoa(c.g)},
		"s": {itoa(c.s)},
		"p": {itoa(c.pos)},
	})

	got := h.queryInt(t, `
		SELECT COALESCE(SUM(fu.count), 0) FROM fleets f
		JOIN fleet_units fu ON fu.fleet_id = f.id
		WHERE f.user_id = $1 AND f.mission = 'ESPIONAGE' AND fu.code = 'spy_probe'
	`, me.id)
	if got != 3 {
		t.Fatalf("dispatched spy probe count = %d, want 3", got)
	}

	waitFor(t, "espionage report", func() bool {
		return h.queryInt(t, `
			SELECT COUNT(*) FROM reports WHERE user_id = $1 AND kind = 'espionage'
		`, me.id) >= 1
	})
}
