package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestBuilderTemplateRenders exercises the Colony Builder editor markup: the
// enable toggle, per-building target inputs, implied-prerequisite note, per
// planet progress and both actions.
func TestBuilderTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := builderData{
		baseData:    baseData{Title: "Colony Builder", ActivePlanetID: 5},
		Enabled:     true,
		HasTemplate: true,
		Sections: []builderSection{{
			Name: "Resource buildings",
			Rows: []builderRow{
				{Code: "iron_mine", Name: "Iron Mine", Group: "Resource buildings", Icon: "i-mine", Current: 3, Target: 5, Effective: 5},
				{Code: "iron_storage", Name: "Iron Storage", Group: "Resource buildings", Icon: "i-tank", Current: 0, Target: 0, Effective: 5, Implied: true},
			},
		}},
		PlanetsProgress: []builderPlanetRow{{Name: "Home", Coords: "1:1:1", Done: 1, Total: 2}},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "builder", d); err != nil {
		t.Fatalf("builder template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Colony Builder",
		`name="enabled"`,
		"checked",
		"target_iron_mine",
		`value="5"`,
		"Iron Storage",
		"needed for another target",
		"1:1:1",
		"1 / 2",
		"Queue now",
		`/builder/5/run`,
		"Save builder plan",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("builder page missing %q", want)
		}
	}
}

// TestBuilderTemplateNoTemplateHidesRunButton checks the "queue now" action only
// appears once a plan exists.
func TestBuilderTemplateNoTemplateHidesRunButton(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := builderData{baseData: baseData{Title: "Colony Builder", ActivePlanetID: 5}}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "builder", d); err != nil {
		t.Fatalf("builder template: %v", err)
	}
	if out := buf.String(); strings.Contains(out, "Queue now") {
		t.Error("run button should be hidden without a template")
	}
}
