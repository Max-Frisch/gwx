package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestResearchTemplateRenders exercises the research card layout and the
// multi-slot research queue panel so a bad field or method call fails the test.
func TestResearchTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := researchData{
		baseData:   baseData{Title: "Research", ActivePlanetID: 5},
		QueueSlots: 4, QueueUsed: 2,
		Sections: []buildingSection{{
			Name: "Drive research",
			Rows: []buildRow{
				{
					Code: "combustion_drive", Name: "Combustion Drive", Level: 1, NextLevel: 4,
					Queued: true, QueuedCount: 3,
					Unlocked: true, Affordable: true, Icon: "i-galaxy", Desc: "Reduces flight costs.",
					TimeStr: "1m 0s", Costs: []costPart{{Label: "Iron", Class: "m", Amount: "300", OK: true}},
				},
				{
					Code: "space_folding_drive", Name: "Space Folding Drive", NextLevel: 1,
					Unlocked: false, Icon: "i-galaxy", Desc: "Folds space.",
					Missing: []string{"Space Curvature Drive (needs level 10, have 0)"},
				},
			},
		}},
	}
	d.ResearchQueue = []buildQueueRow{
		{Ordinal: 1, ID: 1, Name: "Combustion Drive Level 2", Duration: "1m 0s", Remaining: "1m 0s", FinishUnix: 1700000000},
		{Ordinal: 2, ID: 2, Name: "Combustion Drive Level 3", Duration: "1m 0s", Remaining: "2m 0s", FinishUnix: 1700000060},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "research", d); err != nil {
		t.Fatalf("research template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Drive research", "Combustion Drive", "Level 1 (+3)", "Research to level 4",
		"300", "Requires:", "i-galaxy", "Research queue", "(2/4)",
		`data-countdown="1700000000"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("research page missing %q", want)
		}
	}

	var qbuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&qbuf, "research-queue-panel", d); err != nil {
		t.Fatalf("research-queue-panel template: %v", err)
	}
	if !strings.Contains(qbuf.String(), `hx-get="/partials/queue/5?kind=research"`) {
		t.Errorf("research-queue-panel missing self-poll attribute: %s", qbuf.String())
	}
}
