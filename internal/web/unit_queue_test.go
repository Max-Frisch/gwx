package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestUnitQueuePanelRenders exercises the ship/defense production queue panel:
// the per-row checkboxes, the batch-delete buttons and the confirmation text.
func TestUnitQueuePanelRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := shipyardData{
		baseData:  baseData{Title: "Shipyard", ActivePlanetID: 5},
		QueueKind: "ship",
	}
	d.UnitQueue = []unitQueueRow{
		{Ordinal: 1, ID: 11, Name: "Colonization Ship", Quantity: 1, Duration: "57m 30s", Remaining: "57m 30s", FinishUnix: 1700000000},
		{Ordinal: 2, ID: 12, Name: "Spy Probe", Quantity: 2498, Duration: "41m 38s", Remaining: "1h 39m 8s", FinishUnix: 1700002498},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "shipyard", d); err != nil {
		t.Fatalf("shipyard template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Production queue", "(2)",
		`name="queue" value="11"`, `name="queue" value="12"`,
		`action="/queue/5/cancel-selected"`,
		`formaction="/queue/11/cancel"`,
		`formaction="/queue/5/cancel-all"`,
		"Cancel selected", "Cancel all",
		"Do you really want to cancel all ships?",
		`data-countdown="1700002498"`,
		"querySelectorAll", "return confirm(",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("shipyard unit queue missing %q", want)
		}
	}

	// The defense page asks for confirmation about defenses instead of ships.
	d2 := defenseData{
		baseData:  baseData{Title: "Defense", ActivePlanetID: 7},
		QueueKind: "defense",
	}
	d2.UnitQueue = []unitQueueRow{{Ordinal: 1, ID: 21, Name: "Rocket Launcher", Quantity: 3}}
	var dbuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&dbuf, "defense", d2); err != nil {
		t.Fatalf("defense template: %v", err)
	}
	if !strings.Contains(dbuf.String(), "Do you really want to cancel all defenses?") {
		t.Errorf("defense unit queue missing its confirmation text:\n%s", dbuf.String())
	}

	// The partial re-rendered by the self-poll/htmx keeps the same controls.
	ud := unitQueuePartialData{
		ActivePlanetID: 5, QueueKind: "ship",
		UnitQueue: []unitQueueRow{{Ordinal: 1, ID: 11, Name: "Colonization Ship", Quantity: 1, Remaining: "57m 30s", FinishUnix: 1700000000}},
	}
	var pbuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&pbuf, "unit-queue-panel", ud); err != nil {
		t.Fatalf("unit-queue-panel template: %v", err)
	}
	if !strings.Contains(pbuf.String(), `name="queue" value="11"`) {
		t.Errorf("unit-queue-panel missing row checkbox: %s", pbuf.String())
	}
}

// TestUnitQueuePanelHiddenWhenEmpty proves the panel stays out of the way when
// the production line is idle.
func TestUnitQueuePanelHiddenWhenEmpty(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "unit-queue-panel", unitQueuePartialData{ActivePlanetID: 5, QueueKind: "ship"}); err != nil {
		t.Fatalf("unit-queue-panel template: %v", err)
	}
	if strings.Contains(buf.String(), "Cancel all") {
		t.Errorf("empty queue should render nothing, got: %s", buf.String())
	}
}
