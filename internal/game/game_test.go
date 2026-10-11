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

func TestProductionAndEnergyGate(t *testing.T) {
	// A level-1 iron mine draws 10 energy; base supply is 30, so it runs.
	econ := Recompute(map[string]int{"iron_mine": 1}, map[string]int{}, 1)
	if econ.EnergyUsed != 11 {
		t.Fatalf("energy used: want 11, got %d", econ.EnergyUsed)
	}
	if econ.MetalProd <= 0 {
		t.Fatalf("expected positive metal production, got %v", econ.MetalProd)
	}

	// Overdraw energy: three mines (30) plus a chemical factory (20) exceed the
	// base 30, so production must be throttled below the unthrottled value.
	throttled := Recompute(map[string]int{
		"iron_mine": 5, "lutinum_mine": 5, "water_extractor": 5, "chemical_factory": 5,
	}, map[string]int{}, 1)
	unthrottled := Production(Catalog["iron_mine"].ProdBase, 5, 1)
	if throttled.MetalProd >= unthrottled {
		t.Fatalf("expected throttle: throttled=%v unthrottled=%v", throttled.MetalProd, unthrottled)
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
