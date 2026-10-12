package game

import "math"

// Rounded returns the resources as whole units.
func (r Resources) Rounded() Resources {
	return Resources{
		Metal:    math.Round(r.Metal),
		Lutinum:  math.Round(r.Lutinum),
		Water:    math.Round(r.Water),
		Hydrogen: math.Round(r.Hydrogen),
	}
}

// LossResources is the per-resource resource cost of a per-unit loss map: the
// resource composition of the wreckage a battle leaves behind.
func LossResources(units map[string]int64) Resources {
	var r Resources
	for code, n := range units {
		if n <= 0 {
			continue
		}
		d, ok := Catalog[code]
		if !ok {
			continue
		}
		r.Metal += d.Cost.Metal * float64(n)
		r.Lutinum += d.Cost.Lutinum * float64(n)
		r.Water += d.Cost.Water * float64(n)
		r.Hydrogen += d.Cost.Hydrogen * float64(n)
	}
	return r
}

// TotalUnits is the number of units in a count map (non-positive entries are
// ignored).
func TotalUnits(units map[string]int64) int64 {
	var t int64
	for _, n := range units {
		if n > 0 {
			t += n
		}
	}
	return t
}

// RecyclerCargo is the haul a fleet's Recyclers can carry, boosted by cargo
// research exactly like CargoCapacity.
func RecyclerCargo(units map[string]int64, cargoTech int) int64 {
	n := units["recycler"]
	if n <= 0 {
		return 0
	}
	return CargoCapacity(map[string]int64{"recycler": n}, cargoTech)
}

// RecoverDebris computes the resources salvaged during a battle. Recovery draws
// 20% + 1% per level of Recycling Technology of the resource value of *both*
// sides' destroyed ships and defense, in the resource proportions of the
// wreckage, capped by cargo — overflow disperses and is lost. A salvager with no
// Recyclers (cargo <= 0) recovers nothing.
func RecoverDebris(attackerLosses, defenderLosses map[string]int64, recyclingTech int, cargo int64) Resources {
	if cargo <= 0 {
		return Resources{}
	}
	debris := LossResources(attackerLosses)
	other := LossResources(defenderLosses)
	debris.Metal += other.Metal
	debris.Lutinum += other.Lutinum
	debris.Water += other.Water
	debris.Hydrogen += other.Hydrogen

	pct := 0.20 + 0.01*float64(recyclingTech)
	out := Resources{
		Metal:    debris.Metal * pct,
		Lutinum:  debris.Lutinum * pct,
		Water:    debris.Water * pct,
		Hydrogen: debris.Hydrogen * pct,
	}
	if total := out.Total(); total > float64(cargo) {
		scale := float64(cargo) / total
		out.Metal *= scale
		out.Lutinum *= scale
		out.Water *= scale
		out.Hydrogen *= scale
	}
	return out
}
