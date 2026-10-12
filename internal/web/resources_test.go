package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestResourcesTemplateRenders exercises /resources so a bad field or method
// call fails the test rather than only showing at runtime.
func TestResourcesTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	d := resourcesData{
		baseData: baseData{Title: "Resources", ActivePlanetID: 5},
		Rows: []resProdRow{
			{Name: "Base production"},
			{Name: "Iron Mine (Level 41)", Metal: fmtSigned(18810.24)},
			{Name: "Chemical Factory (Level 5)", Water: fmtSigned(-326.40), Hydrogen: fmtSigned(65.28)},
		},
		Total:     resProdRow{Name: "Total", Metal: fmtSigned(18830.24), Lutinum: fmtSigned(12400.8), Water: fmtSigned(2754.96), Hydrogen: fmtSigned(5483.08)},
		Storage:   resProdRow{Name: "Storage capacity", Metal: fmtAmount(24300000)},
		Protected: resProdRow{Name: "(of which cannot be plundered)", Metal: fmtAmount(486000)},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "resources", d); err != nil {
		t.Fatalf("resources template: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Iron Mine (Level 41)", "18,810.24", "-326.40", "24,300,000", "cannot be plundered"} {
		if !strings.Contains(out, want) {
			t.Errorf("resources page missing %q", want)
		}
	}
}
