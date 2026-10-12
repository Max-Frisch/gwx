package web

import (
	"html/template"
	"testing"
)

func TestTemplatesParse(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	if _, err := template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"); err != nil {
		t.Fatalf("template parse: %v", err)
	}
}
