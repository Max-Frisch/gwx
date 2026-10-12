package game

import "math"

// ProtectedFraction is the share of a resource storage capacity that cannot be
// plundered. This is the signature GalaxyWars safety valve: a player can always
// keep a cushion no matter how badly they are raided.
const ProtectedFraction = 0.20

// StorageCapacity returns the resource ceiling granted by a storage building.
// Level 0 still grants the base capacity.
func StorageCapacity(base float64, level int) int64 {
	if level < 0 {
		level = 0
	}
	return int64(math.Round(base * math.Pow(2.0, float64(level))))
}

// Production returns a producer's hourly output at the given level, scaled by
// the universe speed. GalaxyWars has no energy/electricity system: a mine always
// runs at 100%. Values come from the captured per-level tables
// (production_tables.go); levels beyond the captured range extrapolate from the
// last observed growth ratio.
func Production(code string, level int, speed float64) float64 {
	if level <= 0 {
		return 0
	}
	t := productionTable[code]
	if len(t) == 0 {
		return 0
	}
	if level < len(t) {
		return t[level] * speed
	}
	v := t[len(t)-1]
	if len(t) >= 2 {
		ratio := t[len(t)-1] / t[len(t)-2]
		for l := len(t); l <= level; l++ {
			v *= ratio
		}
	}
	return v * speed
}

// PlanetEconomy is the fully derived production/capacity state of a planet,
// recomputed whenever a relevant building or research completes.
type PlanetEconomy struct {
	MetalProd    float64
	LutinumProd  float64
	WaterProd    float64
	HydrogenProd float64

	MetalCap    int64
	LutinumCap  int64
	WaterCap    int64
	HydrogenCap int64

	ProtectedMetal    int64
	ProtectedLutinum  int64
	ProtectedWater    int64
	ProtectedHydrogen int64
}

// Recompute derives the planet economy from its building levels. There is no
// energy gate: every extraction building always produces its full output.
func Recompute(structures map[string]int, techs map[string]int, speed float64) PlanetEconomy {
	lvl := func(code string) int { return structures[code] }

	var e PlanetEconomy
	e.MetalProd = Production("iron_mine", lvl("iron_mine"), speed)
	e.LutinumProd = Production("lutinum_mine", lvl("lutinum_mine"), speed)
	e.WaterProd = Production("water_extractor", lvl("water_extractor"), speed)
	e.HydrogenProd = Production("chemical_factory", lvl("chemical_factory"), speed)

	e.MetalCap = StorageCapacity(Catalog["iron_storage"].ProdBase, lvl("iron_storage"))
	e.LutinumCap = StorageCapacity(Catalog["lutinum_storage"].ProdBase, lvl("lutinum_storage"))
	e.WaterCap = StorageCapacity(Catalog["water_storage"].ProdBase, lvl("water_storage"))
	e.HydrogenCap = StorageCapacity(Catalog["hydrogen_storage"].ProdBase, lvl("hydrogen_storage"))

	e.ProtectedMetal = int64(float64(e.MetalCap) * ProtectedFraction)
	e.ProtectedLutinum = int64(float64(e.LutinumCap) * ProtectedFraction)
	e.ProtectedWater = int64(float64(e.WaterCap) * ProtectedFraction)
	e.ProtectedHydrogen = int64(float64(e.HydrogenCap) * ProtectedFraction)

	return e
}
