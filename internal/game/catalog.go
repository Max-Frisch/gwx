// Package game holds the pure, dependency-free game domain: the content
// catalog, economic formulas, tech tree, flight math and combat. Nothing here
// touches the database, so it is unit-testable in isolation.
package game

import "math"

// Kind classifies a catalog entry.
type Kind string

const (
	KindBuilding Kind = "building"
	KindResearch Kind = "research"
	KindShip     Kind = "ship"
	KindDefense  Kind = "defense"
)

// Resources is a four-resource amount. Water is a raw resource that the
// Chemical Factory converts 5:1 into hydrogen and the Advanced Chemical
// Factory converts 2:1.
type Resources struct {
	Metal    float64 `json:"metal"`
	Lutinum  float64 `json:"lutinum"`
	Water    float64 `json:"water"`
	Hydrogen float64 `json:"hydrogen"`
}

// Def is a single catalog definition. For buildings/research Cost is the level-1
// price and TimeSeconds the level-1 duration; higher levels scale by
// CostFactor/TimeFactor. For ships/defense Cost and TimeSeconds are per unit.
type Def struct {
	Code       string
	Name       string
	Kind       Kind
	Cost       Resources
	CostFactor float64
	TimeSecs   float64
	TimeFactor float64

	// Combat stats (ships/defense).
	Attack float64
	Shield float64
	Hull   float64
	Cargo  int64
	Speed  int
	Fuel   int64

	// Production (buildings). Extraction output comes from the captured
	// per-level tables in production_tables.go; ProdBase is only the storage
	// capacity base for storage buildings.
	Produces string // "metal","lutinum","water","hydrogen"
	ProdBase float64

	// Presentation (buildings): the category heading the card is grouped
	// under, a one-line blurb and the SVG sprite symbol used as its tile.
	Group string
	Desc  string
	Icon  string

	// Prerequisites: code -> minimum level (building level or tech level).
	Prereqs map[string]int
}

// Catalog holds every definition keyed by code.
var Catalog = func() map[string]Def {
	defs := []Def{
		// ---- Buildings: command + research ----
		{Code: "command_center", Name: "Command Center", Kind: KindBuilding, Cost: Resources{Metal: 200, Lutinum: 100}, CostFactor: 1.5, TimeSecs: 60, TimeFactor: 1.25, Group: "Main buildings", Icon: "i-home", Desc: "The heart of your planetary complex. Upgrading it speeds up every construction on this planet."},
		{Code: "research_center", Name: "Research Center", Kind: KindBuilding, Cost: Resources{Metal: 200, Lutinum: 200, Water: 100}, CostFactor: 1.8, TimeSecs: 90, TimeFactor: 1.3, Prereqs: map[string]int{"command_center": 1}, Group: "Main buildings", Icon: "i-flask", Desc: "The technological centre of your planet. Higher levels let you research new technologies faster."},

		// ---- Buildings: resource production ----
		{Code: "iron_mine", Name: "Iron Mine", Kind: KindBuilding, Cost: Resources{Metal: 60, Lutinum: 15}, CostFactor: 1.5, TimeSecs: 30, TimeFactor: 1.25, Produces: "metal", Group: "Resource buildings", Icon: "i-mine", Desc: "Extracts the basic material iron. The further it is upgraded, the more iron is mined."},
		{Code: "lutinum_mine", Name: "Lutinum Mine", Kind: KindBuilding, Cost: Resources{Metal: 48, Lutinum: 24}, CostFactor: 1.6, TimeSecs: 40, TimeFactor: 1.25, Produces: "lutinum", Group: "Resource buildings", Icon: "i-mine", Desc: "Extracts lutinum, needed by many fundamental and advanced technologies."},
		{Code: "water_extractor", Name: "Drilling Tower", Kind: KindBuilding, Cost: Resources{Metal: 60, Lutinum: 40}, CostFactor: 1.5, TimeSecs: 45, TimeFactor: 1.25, Produces: "water", Group: "Resource buildings", Icon: "i-drill", Desc: "Draws water from the depths of the planet with laser cutters and tractor beams."},
		{Code: "chemical_factory", Name: "Chemical Factory", Kind: KindBuilding, Cost: Resources{Metal: 200, Lutinum: 100, Water: 100}, CostFactor: 1.6, TimeSecs: 60, TimeFactor: 1.25, Produces: "hydrogen", Prereqs: map[string]int{"water_extractor": 1}, Group: "Resource buildings", Icon: "i-factory", Desc: "Converts water into hydrogen at a ratio of 5:1. Stops when the water runs dry."},
		{Code: "advanced_chemical_factory", Name: "Advanced Chemical Factory", Kind: KindBuilding, Cost: Resources{Metal: 20000, Lutinum: 15000, Hydrogen: 1000}, CostFactor: 1.7, TimeSecs: 3000, TimeFactor: 1.3, Produces: "hydrogen", Prereqs: map[string]int{"water_extractor": 10, "command_center": 6}, Group: "Resource buildings", Icon: "i-factory", Desc: "A more efficient refinery that converts water into hydrogen at a ratio of 2:1."},
		{Code: "fusion_reactor", Name: "Fusion Reactor", Kind: KindBuilding, Cost: Resources{Metal: 400, Lutinum: 200, Water: 300}, CostFactor: 1.6, TimeSecs: 90, TimeFactor: 1.3, Prereqs: map[string]int{"research_center": 1}, Group: "Resource buildings", Icon: "i-atom", Desc: "Feeds the planetary grid. A prerequisite for the Planetary Shield."},

		// ---- Buildings: storages ----
		{Code: "iron_storage", Name: "Iron Storage", Kind: KindBuilding, Cost: Resources{Metal: 100}, CostFactor: 1.5, TimeSecs: 30, TimeFactor: 1.25, Produces: "storage_metal", ProdBase: 20000, Group: "Resource buildings", Icon: "i-tank", Desc: "Increases the iron storage capacity of the planet."},
		{Code: "lutinum_storage", Name: "Lutinum Storage", Kind: KindBuilding, Cost: Resources{Metal: 100}, CostFactor: 1.5, TimeSecs: 30, TimeFactor: 1.25, Produces: "storage_lutinum", ProdBase: 20000, Group: "Resource buildings", Icon: "i-tank", Desc: "Increases the lutinum storage capacity of the planet."},
		{Code: "water_storage", Name: "Water Tanks", Kind: KindBuilding, Cost: Resources{Metal: 100}, CostFactor: 1.5, TimeSecs: 30, TimeFactor: 1.25, Produces: "storage_water", ProdBase: 20000, Group: "Resource buildings", Icon: "i-tank", Desc: "Increases the water storage capacity of the planet."},
		{Code: "hydrogen_storage", Name: "Hydrogen Tanks", Kind: KindBuilding, Cost: Resources{Metal: 100}, CostFactor: 1.5, TimeSecs: 30, TimeFactor: 1.25, Produces: "storage_hydrogen", ProdBase: 20000, Group: "Resource buildings", Icon: "i-tank", Desc: "Increases the hydrogen storage capacity of the planet."},

		// ---- Buildings: military ----
		{Code: "shipyard", Name: "Shipyard", Kind: KindBuilding, Cost: Resources{Metal: 400, Lutinum: 200, Water: 100}, CostFactor: 1.6, TimeSecs: 120, TimeFactor: 1.3, Prereqs: map[string]int{"command_center": 1}, Group: "Military buildings", Icon: "i-ship", Desc: "Builds ships. Higher levels unlock bigger hulls."},
		{Code: "trading_post", Name: "Trading Post", Kind: KindBuilding, Cost: Resources{Metal: 2000, Lutinum: 2000}, CostFactor: 1.6, TimeSecs: 120, TimeFactor: 1.3, Prereqs: map[string]int{"shipyard": 1}, Group: "Main buildings", Icon: "i-fleet", Desc: "Connects you to the intergalactic trade network and opens offer slots."},
		{Code: "orbital_defense", Name: "Orbital Defense Station", Kind: KindBuilding, Cost: Resources{Metal: 300, Lutinum: 200}, CostFactor: 1.6, TimeSecs: 90, TimeFactor: 1.3, Prereqs: map[string]int{"shipyard": 1}, Group: "Military buildings", Icon: "i-shield", Desc: "Orbital platform hosting your planetary defense turrets."},
		{Code: "planetary_shield", Name: "Planetary Shield", Kind: KindBuilding, Cost: Resources{Metal: 500, Lutinum: 300, Water: 200}, CostFactor: 1.6, TimeSecs: 120, TimeFactor: 1.3, Prereqs: map[string]int{"orbital_defense": 1}, Group: "Military buildings", Icon: "i-shield", Desc: "A shield that protects the whole planet. Requires an Orbital Defense Station."},

		// ---- Research ----
		{Code: "energy_tech", Name: "Energy Research", Kind: KindResearch, Cost: Resources{Metal: 0, Lutinum: 400, Water: 200}, CostFactor: 2.0, TimeSecs: 120, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}},
		{Code: "combustion_drive", Name: "Combustion Drive", Kind: KindResearch, Cost: Resources{Metal: 400, Water: 400}, CostFactor: 2.0, TimeSecs: 200, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "energy_tech": 1}},
		{Code: "ion_drive", Name: "Ion Drive", Kind: KindResearch, Cost: Resources{Metal: 2000, Lutinum: 4000, Water: 1000}, CostFactor: 2.0, TimeSecs: 400, TimeFactor: 2.0, Prereqs: map[string]int{"combustion_drive": 3, "energy_tech": 2}},
		{Code: "spacefold_drive", Name: "Space-Folding Drive", Kind: KindResearch, Cost: Resources{Metal: 10000, Lutinum: 20000, Water: 6000}, CostFactor: 2.0, TimeSecs: 800, TimeFactor: 2.0, Prereqs: map[string]int{"ion_drive": 4, "energy_tech": 4}},
		{Code: "weapon_tech", Name: "Weapons Research", Kind: KindResearch, Cost: Resources{Metal: 800, Lutinum: 200}, CostFactor: 2.0, TimeSecs: 200, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}},
		{Code: "shield_tech", Name: "Defense Research", Kind: KindResearch, Cost: Resources{Metal: 200, Lutinum: 600}, CostFactor: 2.0, TimeSecs: 200, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "energy_tech": 1}},
		{Code: "cargo_tech", Name: "Cargo Research", Kind: KindResearch, Cost: Resources{Metal: 1000, Lutinum: 500}, CostFactor: 2.0, TimeSecs: 250, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "combustion_drive": 1}},
		{Code: "espionage_tech", Name: "Espionage Research", Kind: KindResearch, Cost: Resources{Metal: 200, Lutinum: 1000}, CostFactor: 2.0, TimeSecs: 200, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}},
		{Code: "ionization", Name: "Ionization", Kind: KindResearch, Cost: Resources{Metal: 1000, Lutinum: 400, Water: 200}, CostFactor: 2.0, TimeSecs: 200, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "energy_tech": 1}},
		{Code: "explosive_projectiles", Name: "Explosive Projectiles", Kind: KindResearch, Cost: Resources{Metal: 1000, Lutinum: 600, Water: 200}, CostFactor: 2.0, TimeSecs: 220, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "energy_tech": 1}},

		// ---- Ships ----
		{Code: "spy_probe", Name: "Spy Probe", Kind: KindShip, Cost: Resources{Lutinum: 50, Hydrogen: 10}, TimeSecs: 8, Attack: 1, Shield: 0, Hull: 100, Speed: 100000000, Fuel: 1, Prereqs: map[string]int{"shipyard": 1}},
		{Code: "handelsschiff", Name: "Trade Ship", Kind: KindShip, Cost: Resources{Metal: 2000, Lutinum: 2000, Hydrogen: 200}, TimeSecs: 120, Attack: 5, Shield: 10, Hull: 1200, Cargo: 10000, Speed: 5000, Fuel: 20, Prereqs: map[string]int{"shipyard": 2, "combustion_drive": 2}},
		{Code: "cougar", Name: "Cougar", Kind: KindShip, Cost: Resources{Metal: 3000, Lutinum: 1000, Hydrogen: 100}, TimeSecs: 90, Attack: 50, Shield: 10, Hull: 400, Cargo: 50, Speed: 12500, Fuel: 20, Prereqs: map[string]int{"shipyard": 1, "combustion_drive": 2}},
		{Code: "longeagle_x", Name: "Longeagle X", Kind: KindShip, Cost: Resources{Metal: 45000, Lutinum: 15000, Hydrogen: 1000}, TimeSecs: 300, Attack: 1000, Shield: 200, Hull: 6000, Cargo: 1500, Speed: 10000, Fuel: 500, Prereqs: map[string]int{"shipyard": 4, "spacefold_drive": 2}},
		{Code: "colony_ship", Name: "Colony Ship", Kind: KindShip, Cost: Resources{Metal: 10000, Lutinum: 20000, Hydrogen: 2000}, TimeSecs: 400, Attack: 50, Shield: 100, Hull: 3000, Cargo: 7500, Speed: 2500, Fuel: 1000, Prereqs: map[string]int{"shipyard": 4, "ion_drive": 3}},
		{Code: "recycler", Name: "Recycler", Kind: KindShip, Cost: Resources{Metal: 10000, Lutinum: 6000, Hydrogen: 2000}, TimeSecs: 200, Attack: 1, Shield: 10, Hull: 1600, Cargo: 20000, Speed: 2000, Fuel: 300, Prereqs: map[string]int{"shipyard": 4, "combustion_drive": 2}},

		// ---- Defense (GalaxyWars Uni5 roster; see 03-combat.md for the mapping) ----
		{Code: "light_laser", Name: "Light Laser Turret", Kind: KindDefense, Cost: Resources{Metal: 600}, TimeSecs: 90, Attack: 350, Shield: 150, Hull: 500, Prereqs: map[string]int{"orbital_defense": 1}},
		{Code: "heavy_laser", Name: "Laser Turret", Kind: KindDefense, Cost: Resources{Metal: 2000, Lutinum: 200}, TimeSecs: 909, Attack: 1000, Shield: 700, Hull: 1700, Prereqs: map[string]int{"orbital_defense": 1, "combustion_drive": 1}},
		{Code: "emp_launcher", Name: "EMP Launcher", Kind: KindDefense, Cost: Resources{Metal: 200, Lutinum: 5000}, TimeSecs: 2272, Attack: 1800, Shield: 1000, Hull: 2800, Prereqs: map[string]int{"orbital_defense": 1}},
		{Code: "plasma_turret", Name: "Plasma Turret", Kind: KindDefense, Cost: Resources{Metal: 2000, Lutinum: 2000, Water: 50}, TimeSecs: 77, Attack: 700, Shield: 1400, Hull: 2100, Prereqs: map[string]int{"orbital_defense": 1, "ionization": 4}},
		{Code: "rocket_launcher", Name: "Rocket Launchers", Kind: KindDefense, Cost: Resources{Metal: 15000, Lutinum: 2000, Water: 2000}, TimeSecs: 909, Attack: 4500, Shield: 8100, Hull: 12600, Prereqs: map[string]int{"orbital_defense": 1, "explosive_projectiles": 10}},
	}
	m := make(map[string]Def, len(defs))
	for _, d := range defs {
		m[d.Code] = d
	}
	return m
}()

// Get returns the definition for a code and whether it exists.
func Get(code string) (Def, bool) {
	d, ok := Catalog[code]
	return d, ok
}

// DefsOf returns every definition of a kind, in stable catalog order.
func DefsOf(kind Kind) []Def {
	// Iterate the catalog deterministically by slicing known order is not
	// guaranteed by map, so callers that need order should sort by Name.
	out := make([]Def, 0, len(Catalog))
	for _, d := range Catalog {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

// LevelCost returns the resource cost of building/researching the next level,
// where nextLevel is the level being purchased (the existing level + 1).
func (d Def) LevelCost(nextLevel int) Resources {
	mult := math.Pow(d.CostFactor, float64(nextLevel-1))
	return Resources{
		Metal:    d.Cost.Metal * mult,
		Lutinum:  d.Cost.Lutinum * mult,
		Water:    d.Cost.Water * mult,
		Hydrogen: d.Cost.Hydrogen * mult,
	}
}

// LevelTime returns the base (unmultiplied) duration in seconds for the next
// level of a building/research.
func (d Def) LevelTime(nextLevel int) float64 {
	return d.TimeSecs * math.Pow(d.TimeFactor, float64(nextLevel-1))
}
