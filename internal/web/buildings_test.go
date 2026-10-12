package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestBuildingsTemplateRenders exercises the card layout so a bad field or
// method call in the sectioning/cost markup fails the test.
func TestBuildingsTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := buildingsData{
		baseData: baseData{Title: "Buildings", ActivePlanetID: 5},
		Sections: []buildingSection{{
			Name: "Main buildings",
			Rows: []buildRow{
				{
					Code: "command_center", Name: "Command Center", Level: 1, NextLevel: 2,
					Unlocked: true, Affordable: true, Icon: "i-home", Desc: "The heart of it all.",
					TimeStr: "1m 0s", Costs: []costPart{{Label: "Iron", Class: "m", Amount: "300", OK: true}},
				},
				{
					Code: "research_center", Name: "Research Center", NextLevel: 1,
					Unlocked: false, Icon: "i-flask", Desc: "Labs.",
					Missing: []string{"Command Center (needs level 1, have 0)"},
				},
			},
		}},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "buildings", d); err != nil {
		t.Fatalf("buildings template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Main buildings", "Command Center", "Upgrade to level 2", "300", "Requires:", "i-home"} {
		if !strings.Contains(out, want) {
			t.Errorf("buildings page missing %q", want)
		}
	}
}

// TestHtmxContentRenders exercises the progressive-enhancement fragments: the
// swapped content region, its out-of-band resource refresh, the self-polling
// queue panel and the topbar ticker cells.
func TestHtmxContentRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	q := queueView{ID: 9, Kind: "building", Name: "Iron Mine → 3", Remaining: "1m 0s", FinishUnix: 1700000000}
	d := buildingsData{
		baseData: baseData{
			Title: "Buildings", ActivePlanetID: 5, HX: true,
			Flash: "Construction started.", Resources: &resView{Metal: 500},
		},
		Queue: &q,
		Sections: []buildingSection{{
			Name: "Resource buildings",
			Rows: []buildRow{{Code: "iron_mine", Name: "Iron Mine", NextLevel: 3, Unlocked: true, Affordable: true, Icon: "i-mine"}},
		}},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "buildings-content", d); err != nil {
		t.Fatalf("buildings-content template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"hx-post=", `hx-target="#page-content"`, `data-countdown="1700000000"`,
		`hx-swap-oob="true"`, `id="res-strip"`, "Construction started.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("buildings-content missing %q", want)
		}
	}

	var qbuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&qbuf, "queue-panel", queuePartialData{ActivePlanetID: 5, Queue: &q}); err != nil {
		t.Fatalf("queue-panel template: %v", err)
	}
	if !strings.Contains(qbuf.String(), `hx-get="/partials/queue/5?kind=building"`) {
		t.Errorf("queue-panel missing self-poll attribute: %s", qbuf.String())
	}

	var rbuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&rbuf, "resource-cells", baseData{ActivePlanetID: 5, Resources: &resView{Metal: 500}}); err != nil {
		t.Fatalf("resource-cells template: %v", err)
	}
	if !strings.Contains(rbuf.String(), "Iron") {
		t.Errorf("resource-cells missing Iron cell")
	}
}
