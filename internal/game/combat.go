package game

import "math"

// CombatResult summarises a resolved battle.
type CombatResult struct {
	AttackerSurvivors map[string]int64
	DefenderSurvivors map[string]int64
	Rounds            int
	AttackerLost      int64
	DefenderLost      int64
}

// Simulate resolves a battle with a simple aggregate, simultaneous-fire model
// and NO rapid-fire (as agreed for the MVP). Each round, both sides deal
// damage equal to their total attack minus the opposing side's total active
// shields; damage is spread across the target's units in proportion to their
// hull pool. Shields fully regenerate every round.
func Simulate(attacker, defender map[string]int64, weaponTech, shieldTech, maxRounds int) CombatResult {
	att := copyCounts(attacker)
	def := copyCounts(defender)
	attStart := total(att)
	defStart := total(def)

	rounds := 0
	for rounds < maxRounds && total(att) > 0 && total(def) > 0 {
		rounds++
		attPower := sideAttack(att, weaponTech)
		defPower := sideAttack(def, weaponTech)
		attShield := sideShield(att, shieldTech)
		defShield := sideShield(def, shieldTech)

		damageToDef := math.Max(0, attPower-defShield)
		damageToAtt := math.Max(0, defPower-attShield)

		applyDamage(def, damageToDef)
		applyDamage(att, damageToAtt)
	}

	return CombatResult{
		AttackerSurvivors: att,
		DefenderSurvivors: def,
		Rounds:            rounds,
		AttackerLost:      attStart - total(att),
		DefenderLost:      defStart - total(def),
	}
}

func sideAttack(units map[string]int64, weaponTech int) float64 {
	var p float64
	for code, n := range units {
		if n > 0 {
			p += float64(n) * Catalog[code].Attack * (1 + 0.1*float64(weaponTech))
		}
	}
	return p
}

func sideShield(units map[string]int64, shieldTech int) float64 {
	var p float64
	for code, n := range units {
		if n > 0 {
			p += float64(n) * Catalog[code].Shield * (1 + 0.1*float64(shieldTech))
		}
	}
	return p
}

// applyDamage removes units from side whose combined hull pool absorbs damage.
func applyDamage(side map[string]int64, damage float64) {
	if damage <= 0 {
		return
	}
	var hullPool float64
	for code, n := range side {
		if n > 0 {
			hullPool += float64(n) * Catalog[code].Hull
		}
	}
	if hullPool <= 0 {
		return
	}
	for code, n := range side {
		if n <= 0 {
			continue
		}
		hull := Catalog[code].Hull
		if hull <= 0 {
			continue
		}
		share := damage * (float64(n) * hull) / hullPool
		killed := int64(share / hull)
		if killed > n {
			killed = n
		}
		side[code] = n - killed
	}
}

func copyCounts(src map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(src))
	for k, v := range src {
		if v > 0 {
			out[k] = v
		}
	}
	return out
}

func total(m map[string]int64) int64 {
	var t int64
	for _, v := range m {
		if v > 0 {
			t += v
		}
	}
	return t
}
