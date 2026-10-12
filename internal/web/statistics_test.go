package web

import (
	"bytes"
	"html/template"
	"testing"

	"gwx/internal/game"
)

// TestStatisticsTemplateRenders exercises each statistics tab so a bad field or
// method call fails the build rather than only showing at runtime.
func TestStatisticsTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	battle := game.AggregateBattle([]game.CombatReport{
		{Role: "attacker", DefenderLosses: map[string]int64{"heavy_laser": 2}, Loot: game.Loot{Metal: 1000}},
		{Role: "defender", AttackerLosses: map[string]int64{"cougar": 1}, Loot: game.Loot{Hydrogen: 500}},
	})
	cases := []statisticsData{
		{baseData: baseData{Title: "Statistics"}, Tab: "battle", Battle: battle},
		{baseData: baseData{Title: "Statistics"}, Tab: "farming", Farming: game.AggregateFarming(nil)},
		{baseData: baseData{Title: "Statistics"}, Tab: "resources", ResourceBalance: game.AggregateResources(nil)},
	}
	for _, d := range cases {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "statistics", d); err != nil {
			t.Errorf("tab %s: %v", d.Tab, err)
		}
	}
}
