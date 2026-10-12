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
	defender := map[string]int64{"defense_turret": 5}
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
