package game

import "math"

// ProtectedFraction is the share of a resource storage capacity that cannot be
// plundered. This is the signature GalaxyWars safety valve: a player can always
// keep a cushion no matter how badly they are raided.
const ProtectedFraction = 0.20

// BaseEnergy is a planet's free baseline power, enough to run the three
// starting mines. The fusion reactor is what lets production scale beyond it.
const BaseEnergy = 30

// Production returns the hourly output of a producing building. At level 0 it
// yields nothing; output grows ~10% per level on top of the linear term, and is
// scaled by the universe resource speed.
func Production(base float64, level int, resourceSpeed float64) float64 {
	if level <= 0 {
		return 0
	}
	l := float64(level)
	return base * l * math.Pow(1.1, l) * resourceSpeed
}

// StorageCapacity returns the resource ceiling granted by a storage building.
// Level 0 still grants the base capacity.
func StorageCapacity(base float64, level int) int64 {
	if level < 0 {
		level = 0
	}
	return int64(math.Round(base * math.Pow(2.0, float64(level))))
}

// EnergyProduction returns a reactor's hourly energy output, boosted by energy
// research (+10% per level).
func EnergyProduction(base float64, level int, energyTech int) float64 {
	if level <= 0 {
		return 0
	}
	l := float64(level)
	return base * l * math.Pow(1.1, l) * (1 + 0.1*float64(energyTech))
}

// EnergyUse returns a mine's hourly energy draw.
func EnergyUse(base float64, level int) float64 {
	if level <= 0 {
		return 0
	}
	l := float64(level)
	return base * l * math.Pow(1.1, l)
}

// PlanetEconomy is the fully derived production/capacity/energy state of a
// planet, recomputed whenever a relevant building or research completes.
type PlanetEconomy struct {
	MetalProd    float64
	LutinumProd  float64
	WaterProd    float64
	HydrogenProd float64

	MetalCap    int64
	LutinumCap  int64
	WaterCap    int64
	HydrogenCap int64

	EnergyUsed int64
	EnergyProd int64

	ProtectedMetal    int64
	ProtectedLutinum  int64
	ProtectedWater    int64
	ProtectedHydrogen int64
}

// Recompute derives the planet economy from its building levels and the
// account's research levels. Energy shortage proportionally throttles mines.
func Recompute(structures map[string]int, techs map[string]int, resourceSpeed float64) PlanetEconomy {
	lvl := func(code string) int { return structures[code] }

	var e PlanetEconomy

	// Energy first.
	for _, code := range []string{"iron_mine", "lutinum_mine", "water_extractor", "chemical_factory"} {
		e.EnergyUsed += int64(math.Round(EnergyUse(Catalog[code].EnergyUse, lvl(code))))
	}
	e.EnergyProd = BaseEnergy + int64(math.Round(EnergyProduction(Catalog["fusion_reactor"].EnergyProd, lvl("fusion_reactor"), techs["energy_tech"])))

	scale := 1.0
	if e.EnergyUsed > 0 && e.EnergyProd < e.EnergyUsed {
		scale = float64(e.EnergyProd) / float64(e.EnergyUsed)
	}

	e.MetalProd = Production(Catalog["iron_mine"].ProdBase, lvl("iron_mine"), resourceSpeed) * scale
	e.LutinumProd = Production(Catalog["lutinum_mine"].ProdBase, lvl("lutinum_mine"), resourceSpeed) * scale
	e.WaterProd = Production(Catalog["water_extractor"].ProdBase, lvl("water_extractor"), resourceSpeed) * scale
	e.HydrogenProd = Production(Catalog["chemical_factory"].ProdBase, lvl("chemical_factory"), resourceSpeed) * scale

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
