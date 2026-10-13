package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestGalaxyTemplateRenders exercises the galaxy page including the one-click
// spy action so a bad field or method call fails the test rather than only
// showing at runtime.
func TestGalaxyTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := galaxyData{
		baseData: baseData{Title: "Galaxy", ActivePlanetID: 5},
		Galaxy:   1,
		System:   2,
		SpyCount: 3,
		Rows: []galaxyRow{
			{Position: 1, PlanetID: 9, Name: "Home", OwnerID: 5, OwnerName: "me"},
			{Position: 2, PlanetID: 10, Name: "Target", OwnerID: 6, OwnerName: "rival", CanSpy: true},
			{Position: 3},
		},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "galaxy", d); err != nil {
		t.Fatalf("galaxy template: %v", err)
	}
	out := buf.String()

	if strings.Count(out, `action="/galaxy/spy"`) != 1 {
		t.Errorf("expected exactly one spy action for the rival planet, got %d", strings.Count(out, `action="/galaxy/spy"`))
	}
	for _, want := range []string{
		"Galaxy", "1:2", `name="p" value="2"`, "Send 3 spy probe(s)", "i-eye",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("galaxy page missing %q", want)
		}
	}
}
