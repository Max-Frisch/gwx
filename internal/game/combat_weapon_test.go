package game

import (
	"math"
	"testing"
)

// TestWeaponClasses pins the gwx weapon-class assignment (the capture exposes
// none): rocket/missile/bomb hulls are explosive, laser/ion/plasma/EMP hulls
// are energy, and civilian/token-gun hulls carry no class.
func TestWeaponClasses(t *testing.T) {
	want := map[string]WeaponClass{
		// energy
		"falcon": WeaponEnergy, "tjuger": WeaponEnergy, "cougar": WeaponEnergy,
		"longeagle_v": WeaponEnergy, "raider": WeaponEnergy,
		"light_laser": WeaponEnergy, "heavy_laser": WeaponEnergy,
		"emp_launcher": WeaponEnergy, "plasma_turret": WeaponEnergy,
		// explosive
		"jackal": WeaponExplosive, "renegade": WeaponExplosive,
		"large_trader": WeaponExplosive, "noah": WeaponExplosive,
		"longeagle_x": WeaponExplosive, "rocket_launcher": WeaponExplosive,
		// none
		"spy_probe": WeaponNone, "small_trader": WeaponNone,
		"colony_ship": WeaponNone, "recycler": WeaponNone,
	}
	for code, class := range want {
		d, ok := Get(code)
		if !ok {
			t.Errorf("catalog code %q missing", code)
			continue
		}
		if d.Weapon != class {
			t.Errorf("%s weapon class = %q, want %q", code, d.Weapon, class)
		}
	}
}

func TestWeaponBonusOnlyMatchesClass(t *testing.T) {
	falcon := Catalog["falcon"] // energy
	if got := falcon.weaponMultiplier(CombatTechs{EnergyFocusing: 4}); math.Abs(got-1.20) > 1e-9 {
		t.Errorf("energy ×lvl4 = %v, want 1.20", got)
	}
	if got := falcon.weaponMultiplier(CombatTechs{ExplosiveProjectiles: 4}); got != 1 {
		t.Errorf("energy weapon boosted by explosives = %v, want 1", got)
	}

	rocket := Catalog["rocket_launcher"] // explosive
	if got := rocket.weaponMultiplier(CombatTechs{ExplosiveProjectiles: 9}); math.Abs(got-1.30) > 1e-9 {
		t.Errorf("explosive ×lvl9 = %v, want 1.30", got)
	}
	if got := rocket.weaponMultiplier(CombatTechs{EnergyFocusing: 9}); got != 1 {
		t.Errorf("explosive weapon boosted by energy = %v, want 1", got)
	}

	probe := Catalog["spy_probe"] // none
	if got := probe.weaponMultiplier(CombatTechs{EnergyFocusing: 20, ExplosiveProjectiles: 20}); got != 1 {
		t.Errorf("unarmed hull bonus = %v, want 1", got)
	}
}

func TestSideAttackUsesWeaponClass(t *testing.T) {
	falcons := map[string]int64{"falcon": 10}
	base := sideAttack(falcons, CombatTechs{})
	if base != 10*1300 {
		t.Fatalf("falcon base attack = %v, want 13000", base)
	}
	if got := sideAttack(falcons, CombatTechs{EnergyFocusing: 4}); math.Abs(got-15600) > 1e-6 {
		t.Errorf("falcon with Energy Focusing 4 = %v, want 15600", got)
	}
	if got := sideAttack(falcons, CombatTechs{ExplosiveProjectiles: 4}); got != base {
		t.Errorf("falcon boosted by explosives = %v, want unchanged %v", got, base)
	}
}

func TestShipArmorScalesShields(t *testing.T) {
	// Laser Turret defense 700 at Advanced Ship Armor 6: 10·√6 % = 24.49%.
	got := sideShield(map[string]int64{"heavy_laser": 1}, 6)
	if math.Abs(got-700*(1+0.10*math.Sqrt(6))) > 1e-9 {
		t.Errorf("sideShield = %v, want %v", got, 700*(1+0.10*math.Sqrt(6)))
	}
	if math.Abs(got-871.46) > 0.5 {
		t.Errorf("sideShield = %v, want ~871 (captured 700→871)", got)
	}
}

// TestSimulatePerSideTechs proves each side applies its own research: in an
// identical mirror match the side with the matching attack tech trades better
// and loses fewer hulls.
func TestSimulatePerSideTechs(t *testing.T) {
	fleet := map[string]int64{"longeagle_x": 10} // explosive weapons
	asIs := Simulate(copyCounts(fleet), copyCounts(fleet), CombatTechs{}, CombatTechs{}, 6)
	boosted := Simulate(copyCounts(fleet), copyCounts(fleet), CombatTechs{ExplosiveProjectiles: 9}, CombatTechs{}, 6)

	if asIs.DefenderLost != asIs.AttackerLost {
		t.Errorf("even mirror should be symmetric: defLost=%d attLost=%d",
			asIs.DefenderLost, asIs.AttackerLost)
	}
	if boosted.DefenderLost <= boosted.AttackerLost {
		t.Errorf("boosted attacker should trade better: defLost=%d attLost=%d",
			boosted.DefenderLost, boosted.AttackerLost)
	}
	if boosted.AttackerLost >= asIs.AttackerLost {
		t.Errorf("boosted attacker should lose fewer ships: %d >= %d", boosted.AttackerLost, asIs.AttackerLost)
	}
}
