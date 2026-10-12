package game

// ProtectedFraction is the share of a resource storage capacity that cannot be
// plundered ("concealed" area). Captured exactly at 2% of Storage(L).
const ProtectedFraction = 0.02

// StorageCapacity returns the resource ceiling granted by a storage building,
// exactly as captured: Storage(L) = 60_000·L² + 300_000.
func StorageCapacity(level int) int64 {
	if level < 0 {
		level = 0
	}
	return int64(60000*level*level + 300000)
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
	// WaterUse is the hourly water draw of both hydrogen factories combined
	// (5 × base-factory H₂ + 2 × advanced-factory H₂).
	WaterUse float64

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

	// Both factories draw from the same water pool; outputs and draws add up.
	// Chemical Factory is 5:1, Advanced Chemical Factory is 2:1.
	hBase := Production("chemical_factory", lvl("chemical_factory"), speed)
	hAdv := Production("advanced_chemical_factory", lvl("advanced_chemical_factory"), speed)
	e.HydrogenProd = hBase + hAdv
	e.WaterUse = 5*hBase + 2*hAdv

	e.MetalCap = StorageCapacity(lvl("iron_storage"))
	e.LutinumCap = StorageCapacity(lvl("lutinum_storage"))
	e.WaterCap = StorageCapacity(lvl("water_storage"))
	e.HydrogenCap = StorageCapacity(lvl("hydrogen_storage"))

	e.ProtectedMetal = int64(float64(e.MetalCap) * ProtectedFraction)
	e.ProtectedLutinum = int64(float64(e.LutinumCap) * ProtectedFraction)
	e.ProtectedWater = int64(float64(e.WaterCap) * ProtectedFraction)
	e.ProtectedHydrogen = int64(float64(e.HydrogenCap) * ProtectedFraction)

	return e
}
