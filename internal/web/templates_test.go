package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"gwx/internal/store"
)

func TestTemplatesParse(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	if _, err := template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"); err != nil {
		t.Fatalf("template parse: %v", err)
	}
}

// TestOverviewTemplateRendersIncoming guards the incoming-fleet row against a
// field rename: the handler populates fleetRow.From, so the template must not
// reach for origin coordinates that aren't on the struct.
func TestOverviewTemplateRendersIncoming(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := overviewData{
		baseData: baseData{Title: "Overview", ActivePlanetID: 7},
		Planet:   &store.Planet{ID: 7, Name: "Home", Galaxy: 1, System: 2, Position: 3},
		Incoming: []fleetRow{{Mission: "ATTACK", From: "1:2:3", Remaining: "5m"}},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "overview", d); err != nil {
		t.Fatalf("overview template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ATTACK", "1:2:3", "5m"} {
		if !strings.Contains(out, want) {
			t.Errorf("overview incoming fleet missing %q", want)
		}
	}
}
