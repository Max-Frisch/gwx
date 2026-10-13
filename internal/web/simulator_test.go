package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestSimulatorTemplateRenders exercises the simulator page so a bad field or
// method call fails the test rather than only showing at runtime.
func TestSimulatorTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := simulatorData{
		baseData: baseData{Title: "Simulator", ActivePlanetID: 5},
		Rows: []simulatorRow{
			{Code: "tjuger", Name: "Tjuger", Kind: "ship", Icon: "i-ship", Stats: "Atk 2,000 · Def 900 · Hull 900", Attacker: 10},
			{Code: "cougar", Name: "Cougar", Kind: "ship", Icon: "i-ship", Stats: "Atk 2,250 · Def 6,500 · Hull 6,500", Defender: 8},
		},
		Result: &simulatorResult{
			WinPct: "63.40%", Trials: 1000, Rounds: 4,
			AttackerSurvived: 6, DefenderSurvived: 0,
			Rows: []simulatorOutcomeRow{
				{Name: "Tjuger", AttackerBefore: 10, AttackerLost: 4, AttackerSurvived: 6},
			},
		},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "simulator", d); err != nil {
		t.Fatalf("simulator template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Battle simulator", "63.40%", "1,000",
		`name="a_tjuger"`, `name="d_cougar"`, "Tjuger", "Cougar", "Simulate battle",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("simulator page missing %q", want)
		}
	}
}

// TestSimulateBattleOverwhelming pins the deterministic sample battle and the
// exact degenerate win-chance answers.
func TestSimulateBattleOverwhelming(t *testing.T) {
	res := simulateBattle(map[string]int64{"longeagle_x": 100}, map[string]int64{"light_laser": 1}, nil)
	if res.WinPct != "100.00%" {
		t.Errorf("overwhelming win chance = %s, want 100.00%%", res.WinPct)
	}
	if res.DefenderSurvived != 0 {
		t.Errorf("defender survivors = %d, want 0", res.DefenderSurvived)
	}
	if res.Trials != 1000 {
		t.Errorf("trials = %d, want 1000", res.Trials)
	}

	empty := simulateBattle(nil, map[string]int64{"light_laser": 1}, nil)
	if empty.WinPct != "0.00%" {
		t.Errorf("no attackers win chance = %s, want 0.00%%", empty.WinPct)
	}
}
