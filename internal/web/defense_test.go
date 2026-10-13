package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestDefenseTemplateRenders exercises the /defense page so a bad field or
// method call fails the test rather than only showing at runtime.
func TestDefenseTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	unlocked := unitRow{
		Code: "rocket_launcher", Name: "Rocket Launchers", Count: 2, Unlocked: true,
		CostStr: "15,000 Iron", Stats: "Atk 4,500 · Def 8,100 · Hull 12,600",
		PerTimeStr: "15m 9s", MaxAffordable: 3,
		Costs: []costPart{{Label: "Iron", Class: "m", Amount: "15,000", OK: true, Base: "15000", Have: "90000"}},
	}
	locked := unitRow{
		Code: "plasma_turret", Name: "Plasma Turret", Unlocked: false,
		Missing: []string{"Ionization (needs level 4, have 0)"},
	}

	d := defenseData{
		baseData: baseData{Title: "Defense", ActivePlanetID: 5},
		Units:    []unitRow{unlocked, locked},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "defense", d); err != nil {
		t.Fatalf("defense template: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Rocket Launchers") || !strings.Contains(out, "8,100") {
		t.Error("defense page did not render unit stats")
	}
	if !strings.Contains(out, "Ionization") {
		t.Error("defense page did not render missing prereqs")
	}
	if !strings.Contains(out, `data-base="15000"`) || !strings.Contains(out, `data-have="90000"`) {
		t.Error("defense page did not render dynamic cost attributes")
	}
}
