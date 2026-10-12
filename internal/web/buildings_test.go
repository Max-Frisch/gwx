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
