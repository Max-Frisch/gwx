package web

import (
	"database/sql"
	"testing"
	"time"

	"gwx/internal/store"
)

func TestQueueEventName(t *testing.T) {
	cases := []struct {
		name string
		q    store.Queue
		want string
	}{
		{"building level", store.Queue{Code: "iron_mine", TargetLevel: sql.NullInt64{Int64: 3, Valid: true}}, "Iron Mine → 3"},
		{"research level", store.Queue{Code: "energy_tech", TargetLevel: sql.NullInt64{Int64: 2, Valid: true}}, "Energy Research → 2"},
		{"unit batch", store.Queue{Code: "spy_probe", Quantity: sql.NullInt64{Int64: 5, Valid: true}}, "5 × Spy Probe"},
		{"unknown code", store.Queue{Code: "mystery"}, "mystery"},
	}
	for _, c := range cases {
		if got := queueEventName(c.q); got != c.want {
			t.Errorf("%s: queueEventName = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMissionLabel(t *testing.T) {
	cases := map[string]string{
		"TRANSPORT": "Transport",
		"DEPLOY":    "Deploy",
		"ATTACK":    "Attack",
		"ESPIONAGE": "Espionage",
		"COLONIZE":  "Colonize",
		"UNKNOWN":   "UNKNOWN",
	}
	for in, want := range cases {
		if got := missionLabel(in); got != want {
			t.Errorf("missionLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueueLink(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{"building", "/buildings/7"},
		{"research", "/research/7"},
		{"ship", "/shipyard/7"},
		{"defense", "/shipyard/7"},
	}
	for _, c := range cases {
		q := store.Queue{Kind: c.kind, PlanetID: 7}
		if got := queueLink(q); got != c.want {
			t.Errorf("queueLink(%q) = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestAgo(t *testing.T) {
	if got := ago(time.Now().Add(-90 * time.Second)); got != "1m 30s ago" {
		t.Errorf("ago = %q, want %q", got, "1m 30s ago")
	}
	if got := ago(time.Now().Add(time.Hour)); got != "0s ago" {
		t.Errorf("future ago = %q, want %q", got, "0s ago")
	}
}
