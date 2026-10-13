package game

import (
	"math/rand"
	"testing"
	"time"
)

func TestNPCProfileNormalized(t *testing.T) {
	got := (NPCProfile{}).Normalized()
	if got.Strategy != NPCBalanced || got.Difficulty != "normal" || got.ActiveFrom != 0 || got.ActiveTo != 24 {
		t.Errorf("zero profile normalized = %+v, want balanced/normal/0..24", got)
	}
	// Genuinely out-of-range values fall back to defaults (phase clamps to 3).
	bad := (NPCProfile{Strategy: "nonsense", Difficulty: "epic", Phase: 9, ActiveFrom: 30, ActiveTo: -5}).Normalized()
	if bad.Strategy != NPCBalanced || bad.Difficulty != "normal" || bad.Phase != 3 || bad.ActiveFrom != 0 || bad.ActiveTo != 24 {
		t.Errorf("invalid profile normalized = %+v, want defaults with phase 3", bad)
	}
	// An overnight window survives: 20 -> 3 wraps past midnight.
	wrap := (NPCProfile{ActiveFrom: 20, ActiveTo: 3}).Normalized()
	if wrap.ActiveFrom != 20 || wrap.ActiveTo != 3 {
		t.Errorf("overnight window normalized = %+v, want 20..3", wrap)
	}
	// Equal endpoints mean always on.
	equal := (NPCProfile{ActiveFrom: 5, ActiveTo: 5}).Normalized()
	if equal.ActiveFrom != 0 || equal.ActiveTo != 24 {
		t.Errorf("equal window normalized = %+v, want 0..24", equal)
	}
}

func TestDevelopmentTier(t *testing.T) {
	if got := DevelopmentTier(map[string]int{"command_center": 1}, map[string]int{}); got != 0 {
		t.Errorf("fresh planet tier = %d, want 0", got)
	}
	// Score >= 10 is "economy online".
	if got := DevelopmentTier(map[string]int{"iron_mine": 5, "command_center": 3, "research_center": 2}, map[string]int{}); got != 1 {
		t.Errorf("developing planet tier = %d, want 1", got)
	}
	// A late-round power reaches the top tier.
	advancedStructs := map[string]int{
		"command_center": 8, "research_center": 6,
		"iron_mine": 12, "lutinum_mine": 11, "water_extractor": 10,
		"chemical_factory": 6, "shipyard": 4, "orbital_defense": 4,
	}
	advancedTechs := map[string]int{
		"combustion_drive": 6, "ion_drive": 5, "ionization": 4,
		"energy_focusing": 3, "advanced_ship_armor": 2,
	}
	if got := DevelopmentTier(advancedStructs, advancedTechs); got < 2 {
		t.Errorf("advanced planet tier = %d, want >= 2", got)
	}
}

func TestNPCAwake(t *testing.T) {
	at := func(hour int) time.Time { return time.Date(2026, 1, 1, hour, 30, 0, 0, time.UTC) }

	day := NPCProfile{ActiveFrom: 9, ActiveTo: 17}.Normalized()
	for _, tc := range []struct {
		hour int
		want bool
	}{{8, false}, {9, true}, {16, true}, {17, false}, {23, false}} {
		if got := NPCAwake(day, at(tc.hour)); got != tc.want {
			t.Errorf("NPCAwake(day, %02d:30) = %v, want %v", tc.hour, got, tc.want)
		}
	}

	night := NPCProfile{ActiveFrom: 20, ActiveTo: 3}.Normalized()
	for _, tc := range []struct {
		hour int
		want bool
	}{{19, false}, {20, true}, {23, true}, {0, true}, {2, true}, {3, false}, {12, false}} {
		if got := NPCAwake(night, at(tc.hour)); got != tc.want {
			t.Errorf("NPCAwake(night, %02d:30) = %v, want %v", tc.hour, got, tc.want)
		}
	}

	if !NPCAwake(DefaultNPCProfile(), at(3)) {
		t.Error("default profile should always be awake")
	}
}

func TestNPCNextWindowOpen(t *testing.T) {
	night := NPCProfile{ActiveFrom: 20, ActiveTo: 3}.Normalized()

	// From noon, the next opening is 20:00 the same day (plus jitter).
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next := NPCNextWindowOpen(night, from)
	open := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	if next.Before(open) || next.After(open.Add(2*time.Minute)) {
		t.Errorf("NPCNextWindowOpen(night, noon) = %v, want ~%v", next, open)
	}

	// From inside the window (22:00) the next opening is the following night.
	from = time.Date(2026, 1, 1, 22, 0, 0, 0, time.UTC)
	next = NPCNextWindowOpen(night, from)
	tomorrow := time.Date(2026, 1, 2, 20, 0, 0, 0, time.UTC)
	if next.Before(tomorrow) || next.After(tomorrow.Add(2*time.Minute)) {
		t.Errorf("NPCNextWindowOpen(night, 22:00) = %v, want ~%v", next, tomorrow)
	}
}

func TestRankedBuildingsPhaseBias(t *testing.T) {
	structs := map[string]int{"shipyard": 1, "orbital_defense": 1}
	resource := map[string]bool{
		"iron_mine": true, "lutinum_mine": true, "water_extractor": true,
	}
	firstIsResource := func(phase int, seed int64) bool {
		rng := rand.New(rand.NewSource(seed))
		order := RankedBuildings(structs, map[string]int{}, NPCProfile{Phase: phase, Strategy: NPCBalanced}, rng)
		return len(order) > 0 && resource[order[0]]
	}
	early, late := 0, 0
	for seed := int64(0); seed < 400; seed++ {
		if firstIsResource(0, seed) {
			early++
		}
		if firstIsResource(3, seed) {
			late++
		}
	}
	if early <= late {
		t.Errorf("phase 0 led with a resource building %d times vs phase 3 %d; want phase 0 > phase 3", early, late)
	}
}

func TestNextDecisionDelayDifficulty(t *testing.T) {
	avg := func(difficulty string, seed int64) time.Duration {
		p := NPCProfile{Difficulty: difficulty}.Normalized()
		rng := rand.New(rand.NewSource(seed))
		var total time.Duration
		const n = 5000
		for i := 0; i < n; i++ {
			total += NextDecisionDelay(p, 1, rng)
		}
		return total / n
	}
	hard, normal, easy := avg("hard", 1), avg("normal", 1), avg("easy", 1)
	if !(hard < normal && normal < easy) {
		t.Errorf("cadence hard=%v normal=%v easy=%v; want hard < normal < easy", hard, normal, easy)
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
		d := NextDecisionDelay(p, 1, rng)
		if d < 4*time.Minute || d > 60*time.Minute {
			t.Fatalf("NextDecisionDelay(speed 1) = %v, want within 4..60m", d)
		}
	}
}

func TestNextDecisionDelayScalesWithGameSpeed(t *testing.T) {
	p := DefaultNPCProfile()
	// At 10x every game-time pause compresses 10x, so the raw shape sits within
	// 24s..6m (before the real-time floor).
	rng := rand.New(rand.NewSource(30))
	for i := 0; i < 2000; i++ {
		d := NextDecisionDelay(p, 10, rng)
		if d < npcDecisionFloor || d > 6*time.Minute {
			t.Fatalf("NextDecisionDelay(speed 10) = %v, want within 30s..6m", d)
		}
	}
	// At 1000x everything is pinned to the floor.
	rng = rand.New(rand.NewSource(31))
	for i := 0; i < 2000; i++ {
		if d := NextDecisionDelay(p, 1000, rng); d != npcDecisionFloor {
			t.Fatalf("NextDecisionDelay(speed 1000) = %v, want the %v floor", d, npcDecisionFloor)
		}
	}
}

func TestNextDecisionDelayFloorsInvalidSpeed(t *testing.T) {
	rng := rand.New(rand.NewSource(32))
	if d := NextDecisionDelay(DefaultNPCProfile(), 0, rng); d < 4*time.Minute || d > 60*time.Minute {
		t.Fatalf("NextDecisionDelay(speed 0) = %v, want treated as 1x", d)
	}
}

func TestRankedResearchRespectPrereqs(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	// No Research Center: nothing is legal yet.
	got := RankedResearch(map[string]int{}, map[string]int{}, DefaultNPCProfile(), rng)
	if len(got) != 0 {
		t.Fatalf("fresh planet offered research without a Research Center: %v", got)
	}

	// A Research Center admits the root techs only.
	structs := map[string]int{"research_center": 1}
	got = RankedResearch(structs, map[string]int{}, DefaultNPCProfile(), rng)
	set := map[string]bool{}
	for _, code := range got {
		set[code] = true
	}
	for _, want := range []string{"combustion_drive", "spy_technology", "ionization"} {
		if !set[want] {
			t.Errorf("root research candidates missing %q (got %v)", want, got)
		}
	}
	for _, blocked := range []string{"ion_drive", "energy_focusing", "explosive_projectiles", "space_folding_drive"} {
		if set[blocked] {
			t.Errorf("candidate %q offered without prerequisites", blocked)
		}
	}

	// Combustion 1 unlocks the Ion Drive.
	got = RankedResearch(structs, map[string]int{"combustion_drive": 1}, DefaultNPCProfile(), rng)
	set = map[string]bool{}
	for _, code := range got {
		set[code] = true
	}
	if !set["ion_drive"] {
		t.Errorf("ion_drive missing after combustion_drive 1: %v", got)
	}
}

func TestRankedResearchIsAPermutation(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	structs := map[string]int{"research_center": 3}
	techs := map[string]int{"combustion_drive": 4, "ionization": 2}
	first := RankedResearch(structs, techs, DefaultNPCProfile(), rng)
	second := RankedResearch(structs, techs, DefaultNPCProfile(), rng)
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
