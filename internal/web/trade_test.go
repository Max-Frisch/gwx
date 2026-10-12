package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"gwx/internal/game"
	"gwx/internal/store"
)

// TestTradeTemplateRenders exercises every trade tab so a bad field or method
// call fails the test rather than only showing at runtime.
func TestTradeTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	offer := tradeOfferRow{
		ID: 1, CreatorID: 9, CreatorName: "bandit", Planet: "1:2:3",
		GiveStr: "10,000 Iron", WantStr: "5,000 Lutinum",
		Status: "OPEN", Expires: "2026-10-19 12:00", Created: "2026-10-12 12:00",
		CanAccept: true,
	}
	mine := offer
	mine.CreatorID = 7
	mine.IsMine = true
	mine.CanAccept = false

	planets := []store.OwnedPlanet{{ID: 5, Name: "Home", Galaxy: 1, System: 2, Position: 3}}

	cases := []tradeData{
		{
			baseData: baseData{Title: "Trade"}, Tab: "market",
			Offers: []tradeOfferRow{offer}, Page: 1, Pages: 1, Total: 1,
			OfferedOptions: tradeResourceOptions,
		},
		{baseData: baseData{Title: "Trade"}, Tab: "new", Planets: planets, OfferedOptions: tradeResourceOptions},
		{baseData: baseData{Title: "Trade"}, Tab: "own", MyOffers: []tradeOfferRow{mine}},
		{
			baseData: baseData{Title: "Trade"}, Tab: "history",
			History: []tradeHistoryRow{{When: "2026-10-12 12:00", Action: "ACCEPT", Actor: "bandit", ActorID: 9, Counterparty: "capy", CounterpartyID: 7, GiveStr: "10,000 Iron", WantStr: "5,000 Lutinum"}},
		},
		{
			baseData: baseData{Title: "Trade"}, Tab: "storage",
			Storage: []tradeStoragePlanet{{PlanetID: 5, Name: "Home", Coords: "1:2:3", Total: "10,000 Iron", Entries: []tradeStorageEntryRow{{GiveStr: "10,000 Iron", Expires: "2026-10-19 12:00"}}}},
		},
		{baseData: baseData{Title: "Trade"}, Tab: "offer", Offer: &offer, Planets: planets},
		{baseData: baseData{Title: "Trade"}, Tab: "offer", Offer: &mine, Planets: planets},
	}
	for _, d := range cases {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "trade", d); err != nil {
			t.Errorf("tab %s: %v", d.Tab, err)
			continue
		}
		if !strings.Contains(buf.String(), "Trade") {
			t.Errorf("tab %s: page did not render", d.Tab)
		}
	}
}

func TestTradeResFromForm(t *testing.T) {
	if r := tradeResFromForm("iron", 100); r.Metal != 100 || r.Total() != 100 {
		t.Errorf("iron basket = %+v", r)
	}
	if r := tradeResFromForm("hydrogen", 50); r.Hydrogen != 50 {
		t.Errorf("hydrogen basket = %+v", r)
	}
	if r := tradeResFromForm("bogus", 100); !r.IsZero() {
		t.Errorf("unknown key should be empty, got %+v", r)
	}
	if r := tradeResFromForm("iron", 0); !r.IsZero() {
		t.Errorf("zero amount should be empty, got %+v", r)
	}
}

func TestTradeResString(t *testing.T) {
	if got := tradeResString(game.Resources{Metal: 1000, Hydrogen: 5}); got != "1,000 Iron + 5 Hydrogen" {
		t.Errorf("tradeResString = %q", got)
	}
}
