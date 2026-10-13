package game

import (
	"math"
	"math/rand"
	"testing"
)

func TestWinChanceDegenerateCases(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if got := WinChance(map[string]int64{}, map[string]int64{"light_laser": 5}, CombatTechs{}, CombatTechs{}, 6, rng); got != 0 {
		t.Errorf("no attackers win chance = %v, want 0", got)
	}
	if got := WinChance(map[string]int64{"jackal": 5}, map[string]int64{}, CombatTechs{}, CombatTechs{}, 6, rng); got != 1 {
		t.Errorf("empty defence win chance = %v, want 1", got)
	}
}

func TestWinChanceOverwhelmingForce(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	attacker := map[string]int64{"longeagle_x": 500}
	defender := map[string]int64{"light_laser": 2}
	got := WinChance(attacker, defender, CombatTechs{}, CombatTechs{}, 6, rng)
	if got != 1 {
		t.Errorf("overwhelming force win chance = %v, want 1", got)
	}
}

func TestWinChanceSymmetric(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	fleet := map[string]int64{"tjuger": 10, "raider": 20}
	got := WinChance(fleet, copyCounts(fleet), CombatTechs{}, CombatTechs{}, 6, rng)
	if math.Abs(got-0.5) > 0.08 {
		t.Errorf("mirror match win chance = %v, want ~0.5", got)
	}
}

func TestWinChanceDeterministicWithSeed(t *testing.T) {
	attacker := map[string]int64{"tjuger": 10}
	defender := map[string]int64{"cougar": 8}
	first := WinChance(attacker, defender, CombatTechs{}, CombatTechs{}, 6, rand.New(rand.NewSource(42)))
	second := WinChance(attacker, defender, CombatTechs{}, CombatTechs{}, 6, rand.New(rand.NewSource(42)))
	if first != second {
		t.Errorf("seeded WinChance differed: %v vs %v", first, second)
	}
}

func TestWinChanceMonotonic(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	defender := map[string]int64{"cougar": 10}
	small := WinChance(map[string]int64{"tjuger": 5}, defender, CombatTechs{}, CombatTechs{}, 6, rng)
	large := WinChance(map[string]int64{"tjuger": 60}, defender, CombatTechs{}, CombatTechs{}, 6, rng)
	if large < small {
		t.Errorf("more attackers reduced win chance: %v -> %v", small, large)
	}
	if large != 1 {
		t.Errorf("60 tjugers vs 10 cougars win chance = %v, want 1", large)
	}
}

func TestWinChanceTechsHelp(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	attacker := map[string]int64{"tjuger": 12}
	defender := map[string]int64{"tjuger": 14}
	plain := WinChance(attacker, defender, CombatTechs{}, CombatTechs{}, 6, rng)
	boosted := WinChance(attacker, defender, CombatTechs{ShipArmor: 20}, CombatTechs{}, 6, rng)
	if boosted < plain {
		t.Errorf("ship armor lowered the attacker's win chance: %v -> %v", plain, boosted)
	}
}
