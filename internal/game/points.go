package game

// PointsDivisor converts invested resources into economy points: one point per
// this many resources spent. This mirrors the classic GalaxyWars scoring scale.
const PointsDivisor = 1000.0

// LevelPoints returns the cumulative economy points invested in reaching the
// given level of a building/research definition. Ships and defense are never
// scored: the game deliberately has no military points, so a fleet is invisible
// to the leaderboard.
func (d Def) LevelPoints(level int) float64 {
	if level <= 0 {
		return 0
	}
	var total float64
	for l := 1; l <= level; l++ {
		c := d.LevelCost(l)
		total += c.Metal + c.Lutinum + c.Water + c.Hydrogen
	}
	return total / PointsDivisor
}

// PointsFromLevels sums economy points for a set of catalog levels, keeping only
// entries of the given kind (building or research).
func PointsFromLevels(levels map[string]int, kind Kind) float64 {
	var total float64
	for code, lvl := range levels {
		d, ok := Catalog[code]
		if !ok || d.Kind != kind {
			continue
		}
		total += d.LevelPoints(lvl)
	}
	return total
}
