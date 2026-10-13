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

func TestRankedUnitsRespectPrereqs(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	got := RankedUnits(map[string]int{}, map[string]int{}, DefaultNPCProfile(), rng)
	if len(got) != 0 {
		t.Fatalf("fresh planet offered units without a shipyard: %v", got)
	}

	structs := map[string]int{"shipyard": 1, "orbital_defense": 1}
	got = RankedUnits(structs, map[string]int{}, DefaultNPCProfile(), rng)
	set := map[string]bool{}
	for _, code := range got {
		set[code] = true
	}
	for _, want := range []string{"spy_probe", "light_laser", "emp_launcher"} {
		if !set[want] {
			t.Errorf("shipyard candidates missing %q (got %v)", want, got)
		}
	}
	// Gated hulls/defense must not appear before their prerequisites.
	for _, blocked := range []string{"jackal", "renegade", "raider", "heavy_laser", "colony_ship", "rocket_launcher", "plasma_turret"} {
		if set[blocked] {
			t.Errorf("candidate %q offered without prerequisites", blocked)
		}
	}

	// Combustion drive unlocks the Jackal/Renegade/Laser Turret, ionization the
	// Plasma Turret.
	techs := map[string]int{"combustion_drive": 2, "ionization": 4}
	got = RankedUnits(structs, techs, DefaultNPCProfile(), rng)
	set = map[string]bool{}
	for _, code := range got {
		set[code] = true
	}
	for _, want := range []string{"jackal", "renegade", "heavy_laser", "plasma_turret"} {
		if !set[want] {
			t.Errorf("candidate %q missing after tech prerequisites: %v", want, got)
		}
	}
}

func TestRankedUnitsIsAPermutation(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	structs := map[string]int{"shipyard": 2, "orbital_defense": 2}
	techs := map[string]int{"combustion_drive": 3, "ion_drive": 2}
	first := RankedUnits(structs, techs, DefaultNPCProfile(), rng)
	second := RankedUnits(structs, techs, DefaultNPCProfile(), rng)
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

func TestNPCUnitBatchBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for code, def := range Catalog {
		if def.Kind != KindShip && def.Kind != KindDefense {
			continue
		}
		for _, strategy := range []NPCStrategy{NPCBalanced, NPCBuilder, NPCAggressive, NPCPassive} {
			batch := NPCUnitBatch(def, NPCProfile{Strategy: strategy}.Normalized(), rng)
			if batch < 1 {
				t.Fatalf("NPCUnitBatch(%s, %s) = %d, want >= 1", code, strategy, batch)
			}
		}
	}
	if got := NPCUnitBatch(Catalog["colony_ship"], DefaultNPCProfile(), rng); got != 1 {
		t.Errorf("colony ship batch = %d, want 1", got)
	}
}

func TestNPCReactionChancesInRange(t *testing.T) {
	strategies := []NPCStrategy{NPCBalanced, NPCBuilder, NPCAggressive, NPCPassive}
	difficulties := []string{"easy", "normal", "hard", ""}
	for _, s := range strategies {
		for _, d := range difficulties {
			p := NPCProfile{Strategy: s, Difficulty: d}
			for name, v := range map[string]float64{
				"spy":      p.SpyChance(),
				"save":     p.FleetSaveChance(),
				"colonize": p.ColonizeChance(),
			} {
				if v < 0 || v > 1 {
					t.Errorf("%s.%s chance = %v, want in [0,1]", s, name, v)
				}
			}
		}
	}
	// Aggressive NPCs scout more than builders; builders expand more.
	aggressive := NPCProfile{Strategy: NPCAggressive, Difficulty: "normal"}
	builder := NPCProfile{Strategy: NPCBuilder, Difficulty: "normal"}
	if aggressive.SpyChance() <= builder.SpyChance() {
		t.Error("aggressive SpyChance should exceed builder")
	}
	if builder.ColonizeChance() <= aggressive.ColonizeChance() {
		t.Error("builder ColonizeChance should exceed aggressive")
	}
}
