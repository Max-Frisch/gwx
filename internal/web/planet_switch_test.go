package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"gwx/internal/store"
)

func TestPlanetSection(t *testing.T) {
	cases := map[string]string{
		"/buildings/5":       "buildings",
		"/buildings/5/build": "buildings",
		"/resources/7":       "resources",
		"/research/7":        "research",
		"/shipyard/7":        "shipyard",
		"/defense/7":         "defense",
		"/fleet/7":           "fleet",
		"/simulator/7":       "simulator",
		"/overview/7":        "overview",
		"/galaxy/1/2":        "overview",
		"/highscore":         "overview",
		"/settings/account":  "overview",
		"/":                  "overview",
		"":                   "overview",
	}
	for path, want := range cases {
		if got := planetSection(path); got != want {
			t.Errorf("planetSection(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestPlanetSwitcherPreservesSection ensures the sidebar planet dropdown keeps
// the viewer on the current planet page (e.g. Construction) rather than always
// bouncing them to the overview.
func TestPlanetSwitcherPreservesSection(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := buildingsData{
		baseData: baseData{
			Title: "Buildings", ActivePlanetID: 5, PlanetSection: "buildings",
			Planets: []store.OwnedPlanet{
				{ID: 5, Name: "Home", Galaxy: 1, System: 1, Position: 1},
				{ID: 9, Name: "Colony", Galaxy: 1, System: 2, Position: 3},
			},
		},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "buildings", d); err != nil {
		t.Fatalf("buildings template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{`value="/buildings/5"`, `value="/buildings/9"`} {
		if !strings.Contains(out, want) {
			t.Errorf("planet switcher missing %q", want)
		}
	}
	if strings.Contains(out, `value="/overview/9"`) {
		t.Errorf("planet switcher still links a non-active planet to the overview")
	}
}

// TestPlanetRouteFallback covers the default used by non planet-scoped pages.
func TestPlanetRouteFallback(t *testing.T) {
	if got := (baseData{}).PlanetRoute(); got != "overview" {
		t.Errorf("empty PlanetRoute = %q, want overview", got)
	}
	if got := (baseData{PlanetSection: "share"}).PlanetRoute(); got != "share" {
		t.Errorf("PlanetRoute = %q, want share", got)
	}
}
