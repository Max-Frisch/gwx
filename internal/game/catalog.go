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

// WeaponClass is the energy type of a combat unit's weapons. It selects which
// Weapons research boosts its attack: energy weapons scale with Energy
// Focusing, explosive weapons with Explosive Projectiles, each by the captured
// `10·√L %`. The empty class marks civilian hulls (or token guns) that gain no
// attack bonus.
//
// The Uni5 capture exposes no per-unit weapon class (the account had every
// Weapons research at level 0, so the ship cards show attack unchanged), so the
// assignment below is a gwx design decision inferred from each unit's
// description and its tech gate: laser/ion/plasma/EMP hulls are energy, while
// rocket/missile/bomb hulls are explosive. `rocket_launcher` (gated by Explosive
// Projectiles 10) and `plasma_turret` (gated by Ionization 4) anchor the two
// classes.
type WeaponClass string

const (
	WeaponNone      WeaponClass = ""
	WeaponEnergy    WeaponClass = "energy"
	WeaponExplosive WeaponClass = "explosive"
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
// price and TimeSeconds the level-1 duration; higher levels scale by the shared
// cost multiplier series and TimeFactor. For ships/defense Cost and TimeSeconds
// are per unit.
type Def struct {
	Code       string
	Name       string
	Kind       Kind
	Cost       Resources
	TimeSecs   float64
	TimeFactor float64

	// Combat stats (ships/defense).
	Attack float64
	Shield float64
	Hull   float64
	Cargo  int64
	Speed  int
	Fuel   int64
	Weapon WeaponClass

	// Production (buildings). Extraction output comes from the captured
	// per-level tables in production_tables.go.
	Produces string // "metal","lutinum","water","hydrogen"

	// Presentation: the category heading the card is grouped under, a blurb and
	// the SVG sprite symbol used as its tile.
	Group string
	Desc  string
	Icon  string

	// Prerequisites: code -> minimum level (building level or tech level).
	Prereqs map[string]int
}

// Catalog holds every definition keyed by code. Costs, prerequisites, stat and
// presentation strings are taken from the Uni5 capture
// (info/galaxywars-spec/01-catalog.md, 03-combat.md).
var Catalog = func() map[string]Def {
	defs := []Def{
		// ---- Buildings: main ----
		{Code: "command_center", Name: "Command Center", Kind: KindBuilding, Cost: Resources{Metal: 500, Lutinum: 200}, TimeSecs: 10000, TimeFactor: 1.25, Group: "Main buildings", Icon: "i-home", Desc: "The center of your planetary complex. Everything comes together here: resource distribution, planning, and administration. Upgrading this building reduces the construction time of all buildings."},
		{Code: "research_center", Name: "Research Center", Kind: KindBuilding, Cost: Resources{Metal: 300, Lutinum: 100}, TimeSecs: 75, TimeFactor: 1.3, Group: "Main buildings", Icon: "i-flask", Desc: "The technological center of your planet. New and improved technologies are researched here. The higher its level, the faster research completes."},
		{Code: "trading_post", Name: "Trading Post", Kind: KindBuilding, Cost: Resources{Metal: 2000, Lutinum: 2000}, TimeSecs: 95, TimeFactor: 1.3, Prereqs: map[string]int{"shipyard": 1}, Group: "Main buildings", Icon: "i-fleet", Desc: "Your connection to the intergalactic trade network. Trade with other imperators; each level opens one offer slot."},

		// ---- Buildings: resource production ----
		{Code: "iron_mine", Name: "Iron Mine", Kind: KindBuilding, Cost: Resources{Metal: 250, Lutinum: 20}, TimeSecs: 70, TimeFactor: 1.25, Produces: "metal", Group: "Resource buildings", Icon: "i-mine", Desc: "Extracts the basic material iron. The further it is upgraded, the more iron is mined."},
		{Code: "lutinum_mine", Name: "Lutinum Refinery", Kind: KindBuilding, Cost: Resources{Metal: 130, Lutinum: 20}, TimeSecs: 70, TimeFactor: 1.25, Produces: "lutinum", Group: "Resource buildings", Icon: "i-mine", Desc: "Extracts lutinum, a basic material for many fundamental and advanced technologies."},
		{Code: "water_extractor", Name: "Drilling Tower", Kind: KindBuilding, Cost: Resources{Metal: 100, Lutinum: 20}, TimeSecs: 70, TimeFactor: 1.25, Produces: "water", Group: "Resource buildings", Icon: "i-drill", Desc: "Extracts water using laser cutters and tractor beams to draw it from the depths of the planet."},
		{Code: "chemical_factory", Name: "Chemical Factory", Kind: KindBuilding, Cost: Resources{Metal: 350, Lutinum: 20}, TimeSecs: 70, TimeFactor: 1.25, Produces: "hydrogen", Prereqs: map[string]int{"water_extractor": 1}, Group: "Resource buildings", Icon: "i-factory", Desc: "Converts water into hydrogen at a ratio of 5:1. The further it is upgraded, the more water can be converted."},
		{Code: "advanced_chemical_factory", Name: "Advanced Chemical Factory", Kind: KindBuilding, Cost: Resources{Metal: 20000, Lutinum: 15000, Hydrogen: 1000}, TimeSecs: 9000, TimeFactor: 1.3, Produces: "hydrogen", Prereqs: map[string]int{"water_extractor": 10, "command_center": 6}, Group: "Resource buildings", Icon: "i-factory", Desc: "The evolution of the chemical factory. Converts water into hydrogen at a ratio of 2:1 through more efficient electrolysis."},

		// ---- Buildings: storages ----
		{Code: "iron_storage", Name: "Iron Storage", Kind: KindBuilding, Cost: Resources{Metal: 2000, Lutinum: 2000}, TimeSecs: 72000, TimeFactor: 1.5, Prereqs: map[string]int{"iron_mine": 5}, Group: "Resource buildings", Icon: "i-tank", Desc: "Stores your iron and contains a concealed area that is safe from attacks."},
		{Code: "lutinum_storage", Name: "Lutinum Storage", Kind: KindBuilding, Cost: Resources{Metal: 2000, Lutinum: 2000}, TimeSecs: 72000, TimeFactor: 1.5, Prereqs: map[string]int{"lutinum_mine": 5}, Group: "Resource buildings", Icon: "i-tank", Desc: "Stores your lutinum and contains a concealed area that is safe from attacks."},
		{Code: "water_storage", Name: "Water Tanks", Kind: KindBuilding, Cost: Resources{Metal: 2000}, TimeSecs: 72000, TimeFactor: 1.5, Prereqs: map[string]int{"water_extractor": 5}, Group: "Resource buildings", Icon: "i-tank", Desc: "Stores your water and contains a concealed area that is safe from attacks."},
		{Code: "hydrogen_storage", Name: "Hydrogen Tanks", Kind: KindBuilding, Cost: Resources{Metal: 2000, Lutinum: 2000}, TimeSecs: 72000, TimeFactor: 1.5, Prereqs: map[string]int{"chemical_factory": 5}, Group: "Resource buildings", Icon: "i-tank", Desc: "Stores your hydrogen and contains a concealed area that is safe from attacks."},

		// ---- Buildings: attack / defense ----
		{Code: "shipyard", Name: "Ship Factory", Kind: KindBuilding, Cost: Resources{Metal: 2000, Lutinum: 4000, Hydrogen: 20}, TimeSecs: 37000, TimeFactor: 1.3, Group: "Military buildings", Icon: "i-ship", Desc: "The heart of your space program. Higher levels shorten ship construction times."},
		{Code: "orbital_defense", Name: "Orbital Defense Station", Kind: KindBuilding, Cost: Resources{Metal: 400, Lutinum: 400, Hydrogen: 20}, TimeSecs: 4500, TimeFactor: 1.3, Group: "Military buildings", Icon: "i-shield", Desc: "A massive orbital defense post that hosts your planetary defense turrets."},
		{Code: "fusion_reactor", Name: "Fusion Reactor", Kind: KindBuilding, Cost: Resources{Metal: 10000, Water: 2000, Hydrogen: 10000}, TimeSecs: 11000, TimeFactor: 1.3, Prereqs: map[string]int{"chemical_factory": 5}, Group: "Military buildings", Icon: "i-atom", Desc: "Required to operate the planetary shield, providing the energy needed to maintain the sphere."},
		{Code: "planetary_shield", Name: "Planetary Shield", Kind: KindBuilding, Cost: Resources{Metal: 10000, Water: 2000, Hydrogen: 10000}, TimeSecs: 11000, TimeFactor: 1.3, Prereqs: map[string]int{"shipyard": 1, "orbital_defense": 1, "fusion_reactor": 1}, Group: "Military buildings", Icon: "i-shield", Desc: "The pinnacle of defense technology: a high-energy shield that protects the whole planet."},

		// ---- Research: drive research ----
		{Code: "combustion_drive", Name: "Combustion Drive", Kind: KindResearch, Cost: Resources{Metal: 1000}, TimeSecs: 5000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}, Group: "Drive research", Icon: "i-galaxy", Desc: "Mainly used in smaller ships. Each research level reduces flight costs by 0.5%."},
		{Code: "ion_drive", Name: "Ion Drive", Kind: KindResearch, Cost: Resources{Metal: 4000, Lutinum: 4000, Hydrogen: 2000}, TimeSecs: 20000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "combustion_drive": 1}, Group: "Drive research", Icon: "i-galaxy", Desc: "Accelerates particles to high speeds, increasing speed and reducing flight costs by 0.5% per level."},
		{Code: "space_curvature_drive", Name: "Space Curvature Drive", Kind: KindResearch, Cost: Resources{Hydrogen: 30000}, TimeSecs: 5000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "combustion_drive": 5, "ion_drive": 5}, Group: "Drive research", Icon: "i-galaxy", Desc: "Curves space itself to cover great distances very quickly. Each research level reduces flight costs by 0.5%."},
		{Code: "space_folding_drive", Name: "Space Folding Drive", Kind: KindResearch, Cost: Resources{Water: 20000, Hydrogen: 100000}, TimeSecs: 30000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "space_curvature_drive": 10}, Group: "Drive research", Icon: "i-galaxy", Desc: "Folds gravitational fields until space overlaps at a singular point. Each research level reduces flight costs by 0.5%."},

		// ---- Research: weapons research ----
		{Code: "ionization", Name: "Ionization", Kind: KindResearch, Cost: Resources{Metal: 9500, Lutinum: 5750, Hydrogen: 450}, TimeSecs: 52000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}, Group: "Weapons research", Icon: "i-atom", Desc: "The foundation of many energy-based weapons. Gives even smaller ships considerable firepower."},
		{Code: "energy_focusing", Name: "Energy Focusing", Kind: KindResearch, Cost: Resources{Metal: 17000, Lutinum: 13250, Hydrogen: 1240}, TimeSecs: 52000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "ionization": 1}, Group: "Weapons research", Icon: "i-atom", Desc: "Prism and particle guidance systems increase range and penetrating power, giving your fleet an advantage."},
		{Code: "explosive_projectiles", Name: "Explosive Projectiles", Kind: KindResearch, Cost: Resources{Metal: 4000}, TimeSecs: 5000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1, "ionization": 2, "energy_focusing": 1}, Group: "Weapons research", Icon: "i-atom", Desc: "New explosives and improved targeting allow explosive weapons to inflict devastating damage."},

		// ---- Research: ship research ----
		{Code: "spy_technology", Name: "Spy Technology", Kind: KindResearch, Cost: Resources{Metal: 400, Lutinum: 100}, TimeSecs: 52000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}, Group: "Ship research", Icon: "i-search", Desc: "Sensor and stealth technology, important for gathering information about the enemy."},
		{Code: "advanced_ship_armor", Name: "Advanced Ship Armor", Kind: KindResearch, Cost: Resources{Metal: 4000, Lutinum: 4000, Water: 4000, Hydrogen: 4000}, TimeSecs: 150000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}, Group: "Ship research", Icon: "i-shield", Desc: "New ship armor improves protection from enemy weapon systems without making the hull too massive."},
		{Code: "increased_cargo_capacity", Name: "Increased Cargo Capacity", Kind: KindResearch, Cost: Resources{Lutinum: 10000, Hydrogen: 200}, TimeSecs: 5000, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}, Group: "Ship research", Icon: "i-tank", Desc: "Subspace technology stores more resources in the same space, increasing your ships' transport capacity."},
		{Code: "recycling_technology", Name: "Recycling Technology", Kind: KindResearch, Cost: Resources{Metal: 10000, Lutinum: 8000, Water: 8000, Hydrogen: 3000}, TimeSecs: 9990, TimeFactor: 2.0, Prereqs: map[string]int{"research_center": 1}, Group: "Ship research", Icon: "i-drill", Desc: "Recyclers collect and process battle debris. Also unlocks the Recycler ship."},

		// ---- Ships (GalaxyWars Uni5 roster; see 03-combat.md) ----
		{Code: "spy_probe", Name: "Spy Probe", Kind: KindShip, Cost: Resources{Lutinum: 150}, TimeSecs: 50, Attack: 1, Shield: 1, Hull: 1, Cargo: 15, Speed: 9000000, Fuel: 1, Prereqs: map[string]int{"shipyard": 1}, Icon: "i-search", Desc: "Equipped with highly advanced spy technology, this probe serves as your eyes and ears in conflicts with other players."},
		{Code: "jackal", Name: "Jackal", Kind: KindShip, Weapon: WeaponExplosive, Cost: Resources{Metal: 250, Hydrogen: 75}, TimeSecs: 1200, Attack: 50, Shield: 100, Hull: 100, Cargo: 250, Speed: 800, Fuel: 10, Prereqs: map[string]int{"shipyard": 1, "combustion_drive": 1}, Icon: "i-ship", Desc: "A light spacecraft with weak armament, used as an escort ship. The first step toward interplanetary travel."},
		{Code: "small_trader", Name: "Small Trader", Kind: KindShip, Cost: Resources{Metal: 2000, Lutinum: 2000}, TimeSecs: 12500, Attack: 1, Shield: 50, Hull: 50, Cargo: 9000, Speed: 1900, Fuel: 10, Prereqs: map[string]int{"shipyard": 1, "combustion_drive": 1}, Icon: "i-fleet", Desc: "Forgoes weapon systems entirely in favor of cargo space. Its strong armor lets it serve as a bullet sponge for smaller fleets."},
		{Code: "renegade", Name: "Renegade", Kind: KindShip, Weapon: WeaponExplosive, Cost: Resources{Metal: 1000}, TimeSecs: 3000, Attack: 200, Shield: 200, Hull: 200, Cargo: 200, Speed: 1300, Fuel: 25, Prereqs: map[string]int{"shipyard": 1, "combustion_drive": 2}, Icon: "i-ship", Desc: "A completely redesigned version of the Jackal and a good all-rounder, well suited to small escort and attack missions."},
		{Code: "raider", Name: "Raider", Kind: KindShip, Weapon: WeaponEnergy, Cost: Resources{Metal: 500, Lutinum: 500}, TimeSecs: 5000, Attack: 150, Shield: 300, Hull: 300, Cargo: 900, Speed: 2000, Fuel: 40, Prereqs: map[string]int{"shipyard": 1, "ion_drive": 1}, Icon: "i-ship", Desc: "A fast and agile interceptor created with the development of the ion drive, excellently suited to combat in orbit of its home planet."},
		{Code: "recycler", Name: "Recycler", Kind: KindShip, Cost: Resources{Metal: 850, Lutinum: 850, Water: 100}, TimeSecs: 6200, Attack: 300, Shield: 100, Hull: 100, Cargo: 10000, Speed: 1600, Fuel: 30, Prereqs: map[string]int{"shipyard": 1, "combustion_drive": 2, "recycling_technology": 1}, Icon: "i-drill", Desc: "Recyclers offset the enormous losses suffered in space battles and become increasingly important as fleets grow."},
		{Code: "falcon", Name: "Falcon", Kind: KindShip, Weapon: WeaponEnergy, Cost: Resources{Lutinum: 2000}, TimeSecs: 36000, Attack: 1300, Shield: 150, Hull: 150, Cargo: 4000, Speed: 1000, Fuel: 100, Prereqs: map[string]int{"shipyard": 1, "ion_drive": 2, "spy_technology": 5}, Icon: "i-ship", Desc: "The strongest ship in the lower ship class, with high attack power but comparatively slow flight speed."},
		{Code: "colony_ship", Name: "Colonization Ship", Kind: KindShip, Cost: Resources{Metal: 10000, Lutinum: 15000, Water: 1000, Hydrogen: 15000}, TimeSecs: 720000, Attack: 1, Shield: 1000, Hull: 1000, Cargo: 100000, Speed: 15000, Fuel: 800, Prereqs: map[string]int{"shipyard": 1, "ion_drive": 4}, Icon: "i-home", Desc: "A mobile command center equipped with resources for building basic infrastructure on a new planet."},
		{Code: "tjuger", Name: "Tjuger", Kind: KindShip, Weapon: WeaponEnergy, Cost: Resources{Metal: 2000, Lutinum: 2000}, TimeSecs: 36000, Attack: 2000, Shield: 900, Hull: 900, Cargo: 2000, Speed: 3500, Fuel: 60, Prereqs: map[string]int{"shipyard": 1, "space_curvature_drive": 1}, Icon: "i-ship", Desc: "A purely experimental ship designed to attack larger combat ships in a squadron, with light lasers to keep small ships at bay."},
		{Code: "cougar", Name: "Cougar", Kind: KindShip, Weapon: WeaponEnergy, Cost: Resources{Metal: 4000, Lutinum: 5000}, TimeSecs: 36000, Attack: 2250, Shield: 6500, Hull: 6500, Cargo: 24000, Speed: 2100, Fuel: 90, Prereqs: map[string]int{"shipyard": 1, "space_curvature_drive": 1, "advanced_ship_armor": 1}, Icon: "i-ship", Desc: "A frigate with strong defensive capabilities, often used as a base for smaller ships, yet possessing devastating firepower."},
		{Code: "longeagle_v", Name: "Longeagle V", Kind: KindShip, Weapon: WeaponEnergy, Cost: Resources{Metal: 50000, Lutinum: 5000}, TimeSecs: 240000, Attack: 15000, Shield: 35000, Hull: 35000, Cargo: 34000, Speed: 2700, Fuel: 200, Prereqs: map[string]int{"shipyard": 1, "space_curvature_drive": 4}, Icon: "i-ship", Desc: "A cruiser without equal, equipped with defense lasers, plasma disruptors and phase weapons."},
		{Code: "large_trader", Name: "Large Trader", Kind: KindShip, Weapon: WeaponExplosive, Cost: Resources{Metal: 8000, Lutinum: 2000, Hydrogen: 5000}, TimeSecs: 20000, Attack: 15, Shield: 12000, Hull: 12000, Cargo: 50000, Speed: 4500, Fuel: 30, Prereqs: map[string]int{"shipyard": 1, "space_curvature_drive": 5}, Icon: "i-fleet", Desc: "Developed to transport large quantities of resources. Heavily armored and equipped with missile and short-range defense systems."},
		{Code: "noah", Name: "Noah", Kind: KindShip, Weapon: WeaponExplosive, Cost: Resources{Lutinum: 50000, Water: 2000}, TimeSecs: 30000, Attack: 1000, Shield: 24000, Hull: 24000, Cargo: 250000, Speed: 8000, Fuel: 20, Prereqs: map[string]int{"shipyard": 1, "space_folding_drive": 1}, Icon: "i-fleet", Desc: "The breakthrough in space folding research made it possible to move massive objects at will, improving upon the large trader."},
		{Code: "longeagle_x", Name: "Longeagle X", Kind: KindShip, Weapon: WeaponExplosive, Cost: Resources{Lutinum: 250000, Water: 25000}, TimeSecs: 1200000, Attack: 92000, Shield: 50000, Hull: 50000, Cargo: 150000, Speed: 6000, Fuel: 125, Prereqs: map[string]int{"shipyard": 1, "space_folding_drive": 2}, Icon: "i-ship", Desc: "A bomber without equal, boasting enormous firepower."},

		// ---- Defense (GalaxyWars Uni5 roster; see 03-combat.md) ----
		{Code: "light_laser", Name: "Light Laser Turret", Kind: KindDefense, Weapon: WeaponEnergy, Cost: Resources{Metal: 600}, TimeSecs: 500, Attack: 350, Shield: 150, Hull: 500, Prereqs: map[string]int{"orbital_defense": 1}, Icon: "i-shield", Desc: "A weak laser designed to defend against small, fast ships and spy probes. Built quickly, but offers only limited protection."},
		{Code: "heavy_laser", Name: "Laser Turret", Kind: KindDefense, Weapon: WeaponEnergy, Cost: Resources{Metal: 2000, Lutinum: 200}, TimeSecs: 5000, Attack: 1000, Shield: 700, Hull: 1700, Prereqs: map[string]int{"orbital_defense": 1, "combustion_drive": 1}, Icon: "i-shield", Desc: "An enhanced light laser turret using state-of-the-art ocular lenses; suitable for intercepting ships in larger numbers."},
		{Code: "emp_launcher", Name: "EMP Launcher", Kind: KindDefense, Weapon: WeaponEnergy, Cost: Resources{Metal: 200, Lutinum: 5000}, TimeSecs: 12500, Attack: 1800, Shield: 1000, Hull: 2800, Prereqs: map[string]int{"orbital_defense": 1}, Icon: "i-shield", Desc: "Directs wave-shaped electromagnetic pulses at enemy ships, destroying their molecular structure and reducing attack power."},
		{Code: "plasma_turret", Name: "Plasma Turret", Kind: KindDefense, Weapon: WeaponEnergy, Cost: Resources{Metal: 2000, Lutinum: 2000, Water: 50}, TimeSecs: 425, Attack: 700, Shield: 1400, Hull: 2100, Prereqs: map[string]int{"orbital_defense": 1, "ionization": 4}, Icon: "i-shield", Desc: "Fires volleys of high-energy matter over enormous distances, melting the hulls of incoming ships."},
		{Code: "rocket_launcher", Name: "Rocket Launchers", Kind: KindDefense, Weapon: WeaponExplosive, Cost: Resources{Metal: 15000, Lutinum: 2000, Water: 2000}, TimeSecs: 5000, Attack: 4500, Shield: 8100, Hull: 12600, Prereqs: map[string]int{"orbital_defense": 1, "explosive_projectiles": 10}, Icon: "i-shield", Desc: "Uses advanced explosive projectile technology to destroy enemy ships with immense firepower."},
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

// DefsOf returns every definition of a kind.
func DefsOf(kind Kind) []Def {
	out := make([]Def, 0, len(Catalog))
	for _, d := range Catalog {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

// costMultipliers is the exact shared cost series captured from Uni5: the price
// of level L is base × costMultipliers[L], with L1 and L2 both at the base
// price. Beyond the captured range it extrapolates with the fitted cubic
// (L³ + 15) / 20 rounded to the nearest half.
var costMultipliers = []float64{
	0, // index 0 unused
	1, 1, 2, 4, 7, 11.5, 18, 26.5, 37, 51,
	67.5, 87, 110.5, 138, 169.5, 205.5, 246.5, 292.5, 343.5, 401,
	464, 533, 609, 692, 782, 879.5, 985, 1098.5, 1220, 1351,
	1490.5, 1639, 1797.5, 1966, 2144.5, 2333.5, 2533.5, 2744.5, 2966.5, 3201,
}

// costMultiplier returns the price multiple for a level (0 for level 0).
func costMultiplier(level int) float64 {
	if level <= 0 {
		return 0
	}
	if level < len(costMultipliers) {
		return costMultipliers[level]
	}
	l := float64(level)
	return math.Round((l*l*l+15)/20*2) / 2
}

// LevelCost returns the resource cost of building/researching the next level,
// where nextLevel is the level being purchased (the existing level + 1).
func (d Def) LevelCost(nextLevel int) Resources {
	m := costMultiplier(nextLevel)
	return Resources{
		Metal:    d.Cost.Metal * m,
		Lutinum:  d.Cost.Lutinum * m,
		Water:    d.Cost.Water * m,
		Hydrogen: d.Cost.Hydrogen * m,
	}
}

// LevelTime returns the base (unmultiplied) duration in seconds for the next
// level of a building/research.
func (d Def) LevelTime(nextLevel int) float64 {
	if nextLevel <= 0 {
		return 0
	}
	return d.TimeSecs * math.Pow(d.TimeFactor, float64(nextLevel-1))
}
