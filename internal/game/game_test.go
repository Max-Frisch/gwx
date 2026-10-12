package game

import (
	"math"
	"testing"
)

func TestCalculateDistance(t *testing.T) {
	if got := CalculateDistance(1, 1, 1, 1, 1, 5); got != 1020 {
		t.Errorf("same system: want 1020, got %v", got)
	}
	if got := CalculateDistance(1, 1, 1, 1, 3, 1); got != 2890 {
		t.Errorf("same galaxy diff system: want 2890, got %v", got)
	}
	if got := CalculateDistance(1, 1, 1, 2, 1, 1); got != 28100 {
		t.Errorf("cross galaxy: want 28100, got %v", got)
	}
}

func TestLevelCostGrowsGeometrically(t *testing.T) {
	def := Catalog["iron_mine"]
	c1 := def.LevelCost(1)
	c2 := def.LevelCost(2)
	if c1.Metal != 60 {
		t.Errorf("level 1 metal: want 60, got %v", c1.Metal)
	}
	if math.Abs(c2.Metal-90) > 1e-9 {
		t.Errorf("level 2 metal: want 90, got %v", c2.Metal)
	}
}

func TestProductionMatchesReference(t *testing.T) {
	// Ground truth from the Uni5 capture: Iron Mine L3 = 113/h, no energy gate.
	econ := Recompute(map[string]int{"iron_mine": 3}, map[string]int{}, 1)
	if econ.MetalProd != 113 {
		t.Fatalf("iron L3: want 113, got %v", econ.MetalProd)
	}
	if Production("lutinum_mine", 3, 1) != 70 {
		t.Errorf("lutinum L3: want 70, got %v", Production("lutinum_mine", 3, 1))
	}
	if Production("water_extractor", 3, 1) != 141 {
		t.Errorf("water L3: want 141, got %v", Production("water_extractor", 3, 1))
	}
	if Production("chemical_factory", 3, 1) != 28 {
		t.Errorf("hydrogen L3: want 28, got %v", Production("chemical_factory", 3, 1))
	}
	if Production("advanced_chemical_factory", 1, 1) != 100 {
		t.Errorf("advanced chem L1: want 100, got %v", Production("advanced_chemical_factory", 1, 1))
	}

	// Both hydrogen factories draw from the same pool: outputs add up, and the
	// water draw is 5 × base H₂ + 2 × advanced H₂.
	both := Recompute(map[string]int{"chemical_factory": 1, "advanced_chemical_factory": 1}, map[string]int{}, 1)
	if both.HydrogenProd != 108 {
		t.Fatalf("combined H₂ L1: want 108 (8+100), got %v", both.HydrogenProd)
	}
	if both.WaterUse != 240 {
		t.Fatalf("water use L1: want 240 (5·8 + 2·100), got %v", both.WaterUse)
	}

	// The speed knob scales output linearly.
	if got := Recompute(map[string]int{"iron_mine": 3}, map[string]int{}, 15).MetalProd; got != 113*15 {
		t.Fatalf("iron L3 @x15: want 1695, got %v", got)
	}

	// No energy gate: extra producers never throttle the iron mine.
	all := map[string]int{"iron_mine": 3, "lutinum_mine": 5, "water_extractor": 5, "chemical_factory": 5}
	if got := Recompute(all, map[string]int{}, 1).MetalProd; got != 113 {
		t.Fatalf("iron L3 must stay 113 with other buildings, got %v", got)
	}
}

func TestCargoCapacityScalesWithResearch(t *testing.T) {
	units := map[string]int64{"handelsschiff": 2}
	if got := CargoCapacity(units, 0); got != 20000 {
		t.Errorf("cargo no research: want 20000, got %d", got)
	}
	if got := CargoCapacity(units, 5); got != 30000 {
		t.Errorf("cargo +50%%: want 30000, got %d", got)
	}
}

func TestSimulateOverwhelmingForceWins(t *testing.T) {
	attacker := map[string]int64{"longeagle_x": 50}
	defender := map[string]int64{"light_laser": 5}
	res := Simulate(attacker, defender, 0, 0, 6)
	if res.DefenderLost != 5 {
		t.Errorf("expected all 5 turrets destroyed, got %d", res.DefenderLost)
	}
	if res.AttackerLost != 0 {
		t.Errorf("expected no attacker losses, got %d", res.AttackerLost)
	}
}

func TestPrereqsMet(t *testing.T) {
	def := Catalog["longeagle_x"]
	if PrereqsMet(def, map[string]int{"shipyard": 1}, map[string]int{}) {
		t.Error("expected unmet prereqs")
	}
	if !PrereqsMet(def, map[string]int{"shipyard": 4}, map[string]int{"spacefold_drive": 2}) {
		t.Error("expected met prereqs")
	}
}

// TestDefenseRoster pins the GalaxyWars-aligned defense units (real costs and
// stats from 03-combat.md), the dropped placeholders, and the prereqs.
func TestDefenseRoster(t *testing.T) {
	want := map[string]struct {
		attack, shield, hull  float64
		metal, lutinum, water float64
	}{
		"light_laser":     {350, 150, 500, 600, 0, 0},
		"heavy_laser":     {1000, 700, 1700, 2000, 200, 0},
		"emp_launcher":    {1800, 1000, 2800, 200, 5000, 0},
		"plasma_turret":   {700, 1400, 2100, 2000, 2000, 50},
		"rocket_launcher": {4500, 8100, 12600, 15000, 2000, 2000},
	}
	if n := len(DefsOf(KindDefense)); n != len(want) {
		t.Fatalf("defense roster size = %d, want %d", n, len(want))
	}
	for code, w := range want {
		d, ok := Get(code)
		if !ok {
			t.Fatalf("defense %q missing", code)
		}
		if d.Kind != KindDefense {
			t.Errorf("%s kind = %s, want defense", code, d.Kind)
		}
		if d.Attack != w.attack || d.Shield != w.shield || d.Hull != w.hull {
			t.Errorf("%s stats = %v/%v/%v, want %v/%v/%v", code, d.Attack, d.Shield, d.Hull, w.attack, w.shield, w.hull)
		}
		c := d.Cost
		if c.Metal != w.metal || c.Lutinum != w.lutinum || c.Water != w.water {
			t.Errorf("%s cost = Fe %v / Lu %v / Wa %v, want %v/%v/%v", code, c.Metal, c.Lutinum, c.Water, w.metal, w.lutinum, w.water)
		}
	}

	for _, dropped := range []string{"defense_turret", "ion_cannon", "gauss_cannon", "small_shield_dome", "large_shield_dome"} {
		if _, ok := Get(dropped); ok {
			t.Errorf("placeholder %q should be dropped", dropped)
		}
	}

	prereqs := map[string]map[string]int{
		"light_laser":     {"orbital_defense": 1},
		"heavy_laser":     {"orbital_defense": 1, "combustion_drive": 1},
		"emp_launcher":    {"orbital_defense": 1},
		"plasma_turret":   {"orbital_defense": 1, "ionization": 4},
		"rocket_launcher": {"orbital_defense": 1, "explosive_projectiles": 10},
	}
	for code, want := range prereqs {
		d := Catalog[code]
		if len(d.Prereqs) != len(want) {
			t.Errorf("%s prereqs = %v, want %v", code, d.Prereqs, want)
			continue
		}
		for k, v := range want {
			if d.Prereqs[k] != v {
				t.Errorf("%s prereq %s = %d, want %d", code, k, d.Prereqs[k], v)
			}
		}
	}
	for _, tech := range []string{"ionization", "explosive_projectiles"} {
		if _, ok := Get(tech); !ok {
			t.Errorf("prereq-only tech %q missing", tech)
		}
	}

	if PrereqsMet(Catalog["plasma_turret"], map[string]int{"orbital_defense": 1}, map[string]int{"ionization": 3}) {
		t.Error("plasma_turret should require ionization 4")
	}
	if !PrereqsMet(Catalog["plasma_turret"], map[string]int{"orbital_defense": 1}, map[string]int{"ionization": 4}) {
		t.Error("plasma_turret should build with ionization 4")
	}

	res := Simulate(map[string]int64{"cougar": 1}, map[string]int64{"rocket_launcher": 1}, 0, 0, 6)
	if res.AttackerLost != 1 {
		t.Errorf("rocket_launcher should destroy the cougar, attackerLost = %d", res.AttackerLost)
	}
}
