package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestSettingsTemplateRenders exercises each settings tab (account, game,
// vacation, sanctions) so a bad field or method call fails the test.
func TestSettingsTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	cases := []settingsData{
		{baseData: baseData{Title: "Settings"}, Tab: "game", MessagesPerPage: 10, SpyLimitOnSpy: 200},
		{baseData: baseData{Title: "Settings"}, Tab: "account", Username: "capy", Email: "capy@example.com"},
		{baseData: baseData{Title: "Settings"}, Tab: "vacation"},
		{
			baseData: baseData{Title: "Settings", OnVacation: true}, Tab: "vacation",
			VacationSince: "2026-10-12 12:00", VacationUntil: "2026-11-02 12:00", VacationReady: true,
		},
		{
			baseData: baseData{Title: "Settings"}, Tab: "vacation",
			FleetsInFlight: true,
		},
		{baseData: baseData{Title: "Settings", Sanctioned: true, SanctionNote: "farming"}, Tab: "sanction"},
		{baseData: baseData{Title: "Settings", Sanctioned: true, SanctionNote: "farming", OnVacation: true}, Tab: "account"},
	}
	for _, d := range cases {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "settings", d); err != nil {
			t.Errorf("tab %s: %v", d.Tab, err)
		}
	}

	// The header (banner) must surface the sanction reason and vacation state.
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "settings", cases[6]); err != nil {
		t.Fatalf("banner render: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "farming") || !strings.Contains(out, "sanctioned") {
		t.Error("banner did not surface the sanction reason")
	}
	if !strings.Contains(out, "vacation mode") {
		t.Error("banner did not surface vacation mode")
	}
}
