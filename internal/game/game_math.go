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

// driveForTech maps a drive research code to the ships it boosts. Ships without
// an explicit drive use combustion.
var shipDrive = map[string]string{
	"spy_probe":     "combustion_drive",
	"handelsschiff": "combustion_drive",
	"cougar":        "combustion_drive",
	"recycler":      "combustion_drive",
	"longeagle_x":   "spacefold_drive",
	"colony_ship":   "ion_drive",
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
