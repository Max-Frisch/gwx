package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestHighscoreTemplateRenders exercises every highscore tab so a bad field or
// method call fails the build rather than only showing at runtime.
func TestHighscoreTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	cases := []highscoreData{
		{
			baseData:  baseData{Title: "Highscore"},
			Tab:       "players",
			Sort:      "allPoints",
			Direction: "desc",
			Page:      1,
			Pages:     1,
			Players: []playerScoreRow{
				{Rank: 1, UserID: 7, Username: "capy", PlanetPoints: 96030, ResearchPoints: 22922, Total: 118952, Planets: 10, IsMe: true},
			},
		},
		{
			baseData: baseData{Title: "Highscore"},
			Tab:      "planets",
			Page:     1,
			Pages:    1,
			PlanetRows: []planetScoreRow{
				{Rank: 1, Name: "Homeworld", Coords: "1:1:1", OwnerID: 7, OwnerName: "capy", Points: 10790},
			},
		},
		{
			baseData: baseData{Title: "Highscore"},
			Tab:      "battles",
			Page:     1,
			Pages:    1,
			Battles: []battleRow{
				{Rank: 1, When: "2026-10-12 08:00", Attacker: "capy", Defender: "bandit", Coords: "1:1:2", Rounds: 3, Destroyed: 9100, Plundered: 1500},
			},
		},
	}
	for _, d := range cases {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "highscore", d); err != nil {
			t.Errorf("tab %s: %v", d.Tab, err)
			continue
		}
		if !strings.Contains(buf.String(), "Highscore") {
			t.Errorf("tab %s: page did not render", d.Tab)
		}
	}
}

func TestHighscoreSortKeys(t *testing.T) {
	if got := playerSort("planet"); got != "planetPoints" {
		t.Errorf("playerSort(planet) = %q, want planetPoints", got)
	}
	if got := playerSort("total"); got != "allPoints" {
		t.Errorf("playerSort(total) = %q, want allPoints", got)
	}
	if got := playerSort("bogus"); got != "allPoints" {
		t.Errorf("playerSort(bogus) = %q, want allPoints", got)
	}
	if got := planetSort("name"); got != "name" {
		t.Errorf("planetSort(name) = %q, want name", got)
	}
	if got := battleSort("loot"); got != "loot" {
		t.Errorf("battleSort(loot) = %q, want loot", got)
	}
	if got := battleSort("bogus"); got != "destroyed" {
		t.Errorf("battleSort(bogus) = %q, want destroyed", got)
	}
}

func TestHighscoreSortURL(t *testing.T) {
	d := highscoreData{Tab: "players", Sort: "allPoints", Direction: "desc"}
	if got := d.SortURL("allPoints"); got != "/highscore?tab=players&sort=allPoints&direction=asc" {
		t.Errorf("active SortURL = %q, want direction flip to asc", got)
	}
	// Name columns default ascending; scores descending.
	if got := d.SortURL("name"); got != "/highscore?tab=players&sort=name&direction=asc" {
		t.Errorf("name SortURL = %q", got)
	}
	if got := d.SortURL("planetPoints"); got != "/highscore?tab=players&sort=planetPoints&direction=desc" {
		t.Errorf("score SortURL = %q", got)
	}

	p := highscoreData{Tab: "battles", Sort: "destroyed", Direction: "desc", Page: 2}
	if got := p.PageURL(3); got != "/highscore?tab=battles&sort=destroyed&direction=desc&page=3" {
		t.Errorf("PageURL = %q", got)
	}
}

func TestPageWindow(t *testing.T) {
	start, end, pages, page := pageWindow(250, 2, 100)
	if start != 100 || end != 200 || pages != 3 || page != 2 {
		t.Errorf("pageWindow(250,2,100) = %d,%d,%d,%d", start, end, pages, page)
	}
	// Out-of-range page clamps to the last page.
	start, end, pages, page = pageWindow(250, 9, 100)
	if start != 200 || end != 250 || pages != 3 || page != 3 {
		t.Errorf("clamped pageWindow = %d,%d,%d,%d", start, end, pages, page)
	}
	// Empty data still reports one page.
	if _, _, pages, page := pageWindow(0, 1, 100); pages != 1 || page != 1 {
		t.Errorf("empty pageWindow pages/page = %d/%d, want 1/1", pages, page)
	}
}

func TestOrdered(t *testing.T) {
	if !ordered(-1, "asc") || ordered(-1, "desc") {
		t.Error("ascending direction should keep compare < 0")
	}
	if !ordered(1, "desc") || ordered(1, "asc") {
		t.Error("descending direction should keep compare > 0")
	}
}
