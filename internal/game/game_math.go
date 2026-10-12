package game

import "math"

// CalculateDistance returns the fleet travel distance between two coordinate
// vectors using the classic piecewise scheme.
func CalculateDistance(g1, s1, p1, g2, s2, p2 int) float64 {
	dg := math.Abs(float64(g1 - g2))
	ds := math.Abs(float64(s1 - s2))
	dp := math.Abs(float64(p1 - p2))

	if dg != 0 {
		return 28000.0 + 100.0*dg
	}
	if ds == 0 {
		return 1000.0 + 5.0*dp
	}
	d := 2700.0 + 95.0*ds
	if dp > 0 {
		d += 1000.0 + 5.0*dp
	}
	return d
}

// shipDrive maps a ship to the drive research that boosts it (from the captured
// ship prerequisites). Ships without an explicit drive default to combustion.
var shipDrive = map[string]string{
	"spy_probe":    "combustion_drive",
	"jackal":       "combustion_drive",
	"small_trader": "combustion_drive",
	"renegade":     "combustion_drive",
	"raider":       "ion_drive",
	"recycler":     "combustion_drive",
	"falcon":       "ion_drive",
	"colony_ship":  "ion_drive",
	"tjuger":       "space_curvature_drive",
	"cougar":       "space_curvature_drive",
	"longeagle_v":  "space_curvature_drive",
	"large_trader": "space_curvature_drive",
	"noah":         "space_folding_drive",
	"longeagle_x":  "space_folding_drive",
}

// FleetMaxSpeed returns the slowest ship's speed in the fleet, each boosted
// +10% per level of its drive research.
func FleetMaxSpeed(units map[string]int64, techs map[string]int) int {
	slowest := 0
	for code, n := range units {
		if n <= 0 {
			continue
		}
		def, ok := Catalog[code]
		if !ok || def.Speed <= 0 {
			continue
		}
		drive := shipDrive[code]
		speed := int(math.Round(float64(def.Speed) * (1 + 0.1*float64(techs[drive]))))
		if slowest == 0 || speed < slowest {
			slowest = speed
		}
	}
	if slowest == 0 {
		slowest = 10000
	}
	return slowest
}

// FlightDuration returns the one-way trip time in seconds.
func FlightDuration(distance float64, maxSpeed int, speedPercent float64, fleetSpeed float64) int64 {
	if maxSpeed <= 0 {
		maxSpeed = 10000
	}
	if speedPercent <= 0 {
		speedPercent = 1
	}
	if fleetSpeed <= 0 {
		fleetSpeed = 1
	}
	base := 10 + 3500*math.Sqrt(distance*10.0/float64(maxSpeed))
	return int64(math.Max(1, math.Round(base/(fleetSpeed*speedPercent))))
}

// FuelCost returns the hydrogen burned for a one-way trip.
func FuelCost(units map[string]int64, distance float64, speedPercent float64) int64 {
	var base float64
	for code, n := range units {
		if n > 0 {
			base += float64(Catalog[code].Fuel) * float64(n)
		}
	}
	if base <= 0 {
		return 0
	}
	if speedPercent <= 0 {
		speedPercent = 1
	}
	fuel := base * distance / 35000.0 * (0.1 + speedPercent)
	return int64(math.Max(1, math.Round(fuel)))
}

// CargoCapacity returns the total haul capacity of a fleet, boosted by cargo
// research (+10% per level).
func CargoCapacity(units map[string]int64, cargoTech int) int64 {
	var total float64
	for code, n := range units {
		if n > 0 {
			total += float64(Catalog[code].Cargo) * float64(n)
		}
	}
	return int64(math.Round(total * (1 + 0.1*float64(cargoTech))))
}

// BuildTimeMultiplier returns the building duration factor from the command
// center level (higher = faster).
func BuildTimeMultiplier(cmdLevel int) float64 {
	return 1.0 / (1.0 + 0.05*float64(cmdLevel))
}

// UnitFactoryCode returns the building whose level speeds up production of a
// unit kind: the Ship Factory for ships, the Orbital Defense Station for
// defense.
func UnitFactoryCode(kind Kind) string {
	if kind == KindDefense {
		return "orbital_defense"
	}
	return "shipyard"
}

// unitTimeDivisors is the exact shared per-factory-level divisor captured from
// the Uni5 ship and defense calculators: a unit's build time is
// TimeSecs / unitTimeDivisors[factoryLevel], where the factory is the Ship
// Factory (ships) or the Orbital Defense Station (defense). Level 1 leaves the
// captured per-unit duration unchanged; higher factory levels speed production
// up faster than linearly. All ships and all defense units share the identical
// series (verified against Jackal, Falcon, Longeagle X and every defense unit).
// Captured for factory levels 1..29; beyond the captured range it extrapolates
// with the closed form that reproduces every captured value exactly.
var unitTimeDivisors = []float64{
	0, // index 0 unused
	1, 2, 3.5, 5.5, 8, 11.5, 15.5, 20, 25, 30.5,
	37, 44, 51.5, 59.5, 68, 77.5, 87.5, 98, 109, 120.5,
	133, 146, 159.5, 173.5, 188, 203.5, 219.5, 236, 253,
}

// unitTimeDivisor returns the captured build-time divisor for a factory level.
func unitTimeDivisor(factoryLevel int) float64 {
	if factoryLevel < 1 {
		factoryLevel = 1
	}
	if factoryLevel < len(unitTimeDivisors) {
		return unitTimeDivisors[factoryLevel]
	}
	// Extrapolate with 1 + (F²+F−2)/4 + ½·Σ_{k=1}^{F−1} ⌊k/5⌋, the closed form
	// that matches the captured divisors exactly.
	f := float64(factoryLevel)
	var staircase float64
	for k := 1; k < factoryLevel; k++ {
		staircase += float64(k / 5)
	}
	return 1 + (f*f+f-2)/4 + staircase/2
}

// UnitTimeMultiplier returns the per-unit build-time factor for a factory
// level: the reciprocal of the captured divisor curve, so level 1 yields 1.0
// (the captured per-unit duration unchanged) and higher levels shorten it along
// the reference's exact curve.
func UnitTimeMultiplier(factoryLevel int) float64 {
	return 1 / unitTimeDivisor(factoryLevel)
}
