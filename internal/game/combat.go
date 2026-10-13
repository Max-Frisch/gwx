package game

import (
	"math"
	"math/rand"
)

// CombatResult summarises a resolved battle. Losses are recorded per unit so
// reports and statistics can price what was destroyed.
type CombatResult struct {
	AttackerSurvivors map[string]int64
	DefenderSurvivors map[string]int64
	AttackerLosses    map[string]int64
	DefenderLosses    map[string]int64
	Rounds            int
	AttackerLost      int64
	DefenderLost      int64
}

// CombatTechs are the research levels that modify one side's combat stats. The
// attacker and defender each carry their own set.
type CombatTechs struct {
	EnergyFocusing       int // +10·√L % attack to energy weapons
	ExplosiveProjectiles int // +10·√L % attack to explosive weapons
	ShipArmor            int // +10·√L % shield
}

// CombatTechsFromLevels reads the combat-relevant research out of a tech-level
// map (the layout used by user_techs).
func CombatTechsFromLevels(levels map[string]int) CombatTechs {
	return CombatTechs{
		EnergyFocusing:       levels["energy_focusing"],
		ExplosiveProjectiles: levels["explosive_projectiles"],
		ShipArmor:            levels["advanced_ship_armor"],
	}
}

// weaponMultiplier returns the attack multiplier this side's Weapons research
// grants a unit's weapon class (the captured 10·√L %). Civilian hulls and token
// guns (WeaponNone) are unaffected.
func (d Def) weaponMultiplier(t CombatTechs) float64 {
	switch d.Weapon {
	case WeaponEnergy:
		return 1 + 0.10*math.Sqrt(float64(t.EnergyFocusing))
	case WeaponExplosive:
		return 1 + 0.10*math.Sqrt(float64(t.ExplosiveProjectiles))
	default:
		return 1
	}
}

// Simulate resolves a battle with a simple aggregate, simultaneous-fire model
// and NO rapid-fire (as agreed for the MVP). Each round, both sides deal
// damage equal to their total attack minus the opposing side's total active
// shields; damage is spread across the target's units in proportion to their
// hull pool. Shields fully regenerate every round. Each side applies its own
// Weapons research (matched to each unit's weapon class) and its own Ship Armor
// to its shields.
//
// Simulate is the deterministic sample battle: it rolls nothing, so identical
// inputs always give the same result. WinChance samples the same model
// stochastically to estimate how often the attacker actually wins.
func Simulate(attacker, defender map[string]int64, attackerTechs, defenderTechs CombatTechs, maxRounds int) CombatResult {
	return resolveBattle(attacker, defender, attackerTechs, defenderTechs, maxRounds, nil)
}

// resolveBattle is the shared core of Simulate and WinChance. When rng is nil
// each round's attack is exact (the deterministic Simulate); otherwise the
// attack is rolled so two identical battles can still diverge.
func resolveBattle(attacker, defender map[string]int64, attackerTechs, defenderTechs CombatTechs, maxRounds int, rng *rand.Rand) CombatResult {
	att := copyCounts(attacker)
	def := copyCounts(defender)
	attStartCounts := copyCounts(att)
	defStartCounts := copyCounts(def)
	attStart := total(att)
	defStart := total(def)

	rounds := 0
	for rounds < maxRounds && total(att) > 0 && total(def) > 0 {
		rounds++
		attPower := sideAttack(att, attackerTechs)
		defPower := sideAttack(def, defenderTechs)
		if rng != nil {
			attPower *= combatRoll(rng)
			defPower *= combatRoll(rng)
		}
		attShield := sideShield(att, attackerTechs.ShipArmor)
		defShield := sideShield(def, defenderTechs.ShipArmor)

		damageToDef := math.Max(0, attPower-defShield)
		damageToAtt := math.Max(0, defPower-attShield)

		applyDamage(def, damageToDef)
		applyDamage(att, damageToAtt)
	}

	return CombatResult{
		AttackerSurvivors: att,
		DefenderSurvivors: def,
		AttackerLosses:    diffCounts(attStartCounts, att),
		DefenderLosses:    diffCounts(defStartCounts, def),
		Rounds:            rounds,
		AttackerLost:      attStart - total(att),
		DefenderLost:      defStart - total(def),
	}
}

// CombatTrials is how many stochastic battles WinChance samples. 1000 keeps the
// estimate stable to well under a percentage point while staying cheap enough
// to run on every simulator request.
const CombatTrials = 1000

// combatRoll is a per-round attack multiplier around 1: the sum of two uniforms
// is triangular on [0,2] with mean 1, so a round can swing either way but is
// usually near the average. This is the only randomness in the model.
func combatRoll(rng *rand.Rand) float64 {
	return rng.Float64() + rng.Float64()
}

// WinChance estimates the attacker's probability of clearing the defender
// (defender survivors == 0) by sampling CombatTrials stochastic battles through
// the same aggregate model as Simulate. It returns a value in [0,1]; the
// reference exposes this as a win/lose percentage because even an overwhelming
// force can, rarely, lose.
//
// Callers supply the rng so a seed makes it reproducible. Degenerate line-ups
// are answered exactly: no attackers is 0, an empty defence is 1.
func WinChance(attacker, defender map[string]int64, attackerTechs, defenderTechs CombatTechs, maxRounds int, rng *rand.Rand) float64 {
	if total(attacker) == 0 {
		return 0
	}
	if total(defender) == 0 {
		return 1
	}
	wins := 0
	for i := 0; i < CombatTrials; i++ {
		res := resolveBattle(attacker, defender, attackerTechs, defenderTechs, maxRounds, rng)
		if total(res.DefenderSurvivors) == 0 {
			wins++
		}
	}
	return float64(wins) / float64(CombatTrials)
}

// diffCounts returns the units present in start but no longer in end, keyed by
// unit code.
func diffCounts(start, end map[string]int64) map[string]int64 {
	out := make(map[string]int64)
	for code, n := range start {
		if left := end[code]; n > left {
			out[code] = n - left
		}
	}
	return out
}

func sideAttack(units map[string]int64, techs CombatTechs) float64 {
	var p float64
	for code, n := range units {
		if n > 0 {
			d := Catalog[code]
			p += float64(n) * d.Attack * d.weaponMultiplier(techs)
		}
	}
	return p
}

func sideShield(units map[string]int64, armorTech int) float64 {
	mult := 1 + 0.10*math.Sqrt(float64(armorTech))
	var p float64
	for code, n := range units {
		if n > 0 {
			p += float64(n) * Catalog[code].Shield * mult
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
