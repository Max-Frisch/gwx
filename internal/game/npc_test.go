package game

import (
	"math/rand"
	"testing"
	"time"
)

func TestNPCProfileNormalized(t *testing.T) {
	got := (NPCProfile{}).Normalized()
	if got.Strategy != NPCBalanced || got.Difficulty != "normal" || got.ActiveTo != 24 {
		t.Errorf("zero profile normalized = %+v, want balanced/normal/0..24", got)
	}
	bad := (NPCProfile{Strategy: "nonsense", Difficulty: "epic", ActiveFrom: 20, ActiveTo: 3}).Normalized()
	if bad.Strategy != NPCBalanced || bad.Difficulty != "normal" || bad.ActiveFrom != 0 || bad.ActiveTo != 24 {
		t.Errorf("invalid profile normalized = %+v, want defaults", bad)
	}
}

func TestRankedBuildingsRespectPrereqs(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	got := RankedBuildings(map[string]int{}, map[string]int{}, DefaultNPCProfile(), rng)
	set := map[string]bool{}
	for _, code := range got {
		set[code] = true
	}
	// Fresh planet: unlockables with no prerequisites.
	for _, want := range []string{"command_center", "iron_mine", "lutinum_mine", "water_extractor", "shipyard", "orbital_defense"} {
		if !set[want] {
			t.Errorf("fresh planet candidates missing %q (got %v)", want, got)
		}
	}
	// Gated buildings must not be offered before their prerequisites.
	for _, blocked := range []string{"chemical_factory", "iron_storage", "fusion_reactor", "planetary_shield", "trading_post"} {
		if set[blocked] {
			t.Errorf("candidate %q offered without prerequisites", blocked)
		}
	}

	// Raising the water extractor unlocks the chemical factory.
	structs := map[string]int{"water_extractor": 1}
	got = RankedBuildings(structs, map[string]int{}, DefaultNPCProfile(), rng)
	set = map[string]bool{}
	for _, code := range got {
		set[code] = true
	}
	if !set["chemical_factory"] {
		t.Errorf("chemical_factory missing after water_extractor 1: %v", got)
	}
}

func TestRankedBuildingsIsAPermutation(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	structs := map[string]int{"command_center": 3, "iron_mine": 4}
	first := RankedBuildings(structs, map[string]int{}, DefaultNPCProfile(), rng)
	second := RankedBuildings(structs, map[string]int{}, DefaultNPCProfile(), rng)
	if len(first) != len(second) {
		t.Fatalf("candidate count drifted: %d vs %d", len(first), len(second))
	}
	seen := map[string]int{}
	for _, code := range first {
		seen[code]++
	}
	for _, code := range second {
		seen[code]--
	}
	for code, n := range seen {
		if n != 0 {
			t.Errorf("candidate %q appears %d extra time(s) across runs", code, n)
		}
	}
}

func TestNextDecisionDelayRange(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	p := DefaultNPCProfile()
	for i := 0; i < 2000; i++ {
		d := NextDecisionDelay(p, rng)
		if d < 4*time.Minute || d > 150*time.Minute {
			t.Fatalf("NextDecisionDelay = %v, want within 4..150m", d)
		}
	}
}

func TestRandomNPCProfileNormalized(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	for i := 0; i < 500; i++ {
		p := RandomNPCProfile(rng)
		if p != p.Normalized() {
			t.Fatalf("RandomNPCProfile not normalized: %+v", p)
		}
	}
}
