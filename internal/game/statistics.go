package game

import "sort"

// Loot is the resource map written into a combat report — what the attacker
// plundered from the target.
type Loot struct {
	Metal    int64 `json:"metal"`
	Lutinum  int64 `json:"lutinum"`
	Water    int64 `json:"water"`
	Hydrogen int64 `json:"hydrogen"`
}

// CombatReport is the JSON payload stored for each participant of a resolved
// battle. Role is the perspective of the report's owner ("attacker" or
// "defender"); loss maps hold per-unit destroyed counts. The engine writes one
// report per side with the same numbers but a different Role.
type CombatReport struct {
	Target            string           `json:"target"`
	Rounds            int              `json:"rounds"`
	Role              string           `json:"role"`
	AttackerLosses    map[string]int64 `json:"attackerLosses"`
	DefenderLosses    map[string]int64 `json:"defenderLosses"`
	AttackerSurvivors map[string]int64 `json:"attackerSurvivors"`
	Loot              Loot             `json:"loot"`
}

// UnitValue is the total resource cost (metal + lutinum + water + hydrogen) of
// count units of a catalog code. Unknown codes and non-positive counts are
// worth nothing.
func UnitValue(code string, count int64) float64 {
	if count <= 0 {
		return 0
	}
	d, ok := Catalog[code]
	if !ok {
		return 0
	}
	c := d.Cost
	return float64(count) * (c.Metal + c.Lutinum + c.Water + c.Hydrogen)
}

// LossValue is the total resource value of a per-unit loss map.
func LossValue(units map[string]int64) float64 {
	var total float64
	for code, n := range units {
		total += UnitValue(code, n)
	}
	return total
}

// UnitStat is one row of the battle-balance tables: how many units of a type
// the player destroyed and how many they lost.
type UnitStat struct {
	Code      string
	Name      string
	Kind      Kind
	Destroyed int64
	Lost      int64
}

// DestroyedValue is the resource value of the units this player destroyed.
func (u UnitStat) DestroyedValue() float64 { return UnitValue(u.Code, u.Destroyed) }

// LostValue is the resource value of this player's own units that were lost.
func (u UnitStat) LostValue() float64 { return UnitValue(u.Code, u.Lost) }

// ResourceStat is one row of a resource table.
type ResourceStat struct {
	Name      string
	Plundered float64
	Lost      float64
}

// Balance is plundered minus lost.
func (r ResourceStat) Balance() float64 { return r.Plundered - r.Lost }

// BattleBalance summarises a player's battles from their own combat reports.
type BattleBalance struct {
	Attacked  int
	Defended  int
	Won       int
	Lost      int
	Tied      int
	Ships     []UnitStat
	Defense   []UnitStat
	Resources []ResourceStat
}

// Farming summarises a player's raids: attacks that returned with loot.
type Farming struct {
	Raids       int
	Resources   []ResourceStat
	Total       float64
	AverageHaul float64
}

// ResourceBalance is the plundered-versus-lost resource ledger.
type ResourceBalance struct {
	Resources []ResourceStat
	Plundered float64
	Lost      float64
}

// resourceOrder fixes the display order (and labels) of the four resources.
var resourceOrder = []struct{ key, name string }{
	{"metal", "Iron"},
	{"lutinum", "Lutinum"},
	{"water", "Water"},
	{"hydrogen", "Hydrogen"},
}

// AggregateBattle folds a player's combat reports into the battle-balance view.
// A battle is won when the enemy lost more resource value than the player did,
// lost when the reverse, and a tie otherwise.
func AggregateBattle(reports []CombatReport) BattleBalance {
	var bb BattleBalance
	ships := map[string]*UnitStat{}
	defense := map[string]*UnitStat{}
	plundered := map[string]float64{}
	lost := map[string]float64{}

	for _, r := range reports {
		var own, enemy map[string]int64
		if r.Role == "defender" {
			bb.Defended++
			own, enemy = r.DefenderLosses, r.AttackerLosses
		} else {
			bb.Attacked++
			own, enemy = r.AttackerLosses, r.DefenderLosses
		}

		ownVal, enemyVal := LossValue(own), LossValue(enemy)
		switch {
		case enemyVal > ownVal:
			bb.Won++
		case enemyVal < ownVal:
			bb.Lost++
		default:
			bb.Tied++
		}

		for code, n := range enemy {
			addUnit(ships, defense, code, n, true)
		}
		for code, n := range own {
			addUnit(ships, defense, code, n, false)
		}

		dest := plundered
		if r.Role == "defender" {
			dest = lost
		}
		dest["metal"] += float64(r.Loot.Metal)
		dest["lutinum"] += float64(r.Loot.Lutinum)
		dest["water"] += float64(r.Loot.Water)
		dest["hydrogen"] += float64(r.Loot.Hydrogen)
	}

	bb.Ships = unitStats(ships, KindShip)
	bb.Defense = unitStats(defense, KindDefense)
	for _, res := range resourceOrder {
		bb.Resources = append(bb.Resources, ResourceStat{
			Name: res.name, Plundered: plundered[res.key], Lost: lost[res.key],
		})
	}
	return bb
}

// AggregateFarming folds the player's attacking reports into the farming view.
func AggregateFarming(reports []CombatReport) Farming {
	var f Farming
	totals := map[string]float64{}
	for _, r := range reports {
		if r.Role != "attacker" {
			continue
		}
		f.Raids++
		totals["metal"] += float64(r.Loot.Metal)
		totals["lutinum"] += float64(r.Loot.Lutinum)
		totals["water"] += float64(r.Loot.Water)
		totals["hydrogen"] += float64(r.Loot.Hydrogen)
	}
	for _, res := range resourceOrder {
		v := totals[res.key]
		f.Resources = append(f.Resources, ResourceStat{Name: res.name, Plundered: v})
		f.Total += v
	}
	if f.Raids > 0 {
		f.AverageHaul = f.Total / float64(f.Raids)
	}
	return f
}

// AggregateResources folds the player's combat reports into the plundered /
// lost resource ledger.
func AggregateResources(reports []CombatReport) ResourceBalance {
	var rb ResourceBalance
	plundered := map[string]float64{}
	lost := map[string]float64{}
	for _, r := range reports {
		dest := plundered
		if r.Role == "defender" {
			dest = lost
		}
		dest["metal"] += float64(r.Loot.Metal)
		dest["lutinum"] += float64(r.Loot.Lutinum)
		dest["water"] += float64(r.Loot.Water)
		dest["hydrogen"] += float64(r.Loot.Hydrogen)
	}
	for _, res := range resourceOrder {
		p, l := plundered[res.key], lost[res.key]
		rb.Resources = append(rb.Resources, ResourceStat{Name: res.name, Plundered: p, Lost: l})
		rb.Plundered += p
		rb.Lost += l
	}
	return rb
}

// addUnit records n destroyed (enemy) or lost (own) units of a code, routing
// ships and defense into their own maps and ignoring unknown codes.
func addUnit(ships, defense map[string]*UnitStat, code string, n int64, destroyed bool) {
	if n <= 0 {
		return
	}
	d, ok := Catalog[code]
	if !ok {
		return
	}
	m := ships
	if d.Kind == KindDefense {
		m = defense
	}
	st := m[code]
	if st == nil {
		st = &UnitStat{Code: code, Name: d.Name, Kind: d.Kind}
		m[code] = st
	}
	if destroyed {
		st.Destroyed += n
	} else {
		st.Lost += n
	}
}

// unitStats renders the full catalog of a kind (so zero rows are included) with
// any tallied counts merged in, sorted by display name.
func unitStats(counts map[string]*UnitStat, kind Kind) []UnitStat {
	defs := DefsOf(kind)
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	out := make([]UnitStat, 0, len(defs))
	for _, d := range defs {
		st := UnitStat{Code: d.Code, Name: d.Name, Kind: d.Kind}
		if c, ok := counts[d.Code]; ok {
			st.Destroyed, st.Lost = c.Destroyed, c.Lost
		}
		out = append(out, st)
	}
	return out
}
