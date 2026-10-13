package game

import (
	"math/rand"
	"time"
)

// NPCStrategy selects the emphasis of an NPC's decisions. A strategy never
// unlocks anything a player could not do — it only reweights the choices the
// NPC makes between legal actions.
type NPCStrategy string

const (
	NPCBalanced   NPCStrategy = "balanced"
	NPCBuilder    NPCStrategy = "builder"
	NPCAggressive NPCStrategy = "aggressive"
	NPCPassive    NPCStrategy = "passive"
)

// NPCProfile is the set of tunables that make one NPC behave differently from
// another. It is stored as JSON in `users.npc_profile`. Zero values are safe:
// Normalized() fills in sensible defaults.
type NPCProfile struct {
	Strategy   NPCStrategy `json:"strategy"`
	Difficulty string      `json:"difficulty"` // easy | normal | hard
	Phase      int         `json:"phase"`      // 0..3, development tier derived from structs/techs
	ActiveFrom int         `json:"activeFrom"` // 0..23, server-clock hour the window opens
	ActiveTo   int         `json:"activeTo"`   // 1..24 exclusive; may wrap past midnight; == from = always on
}

// DefaultNPCProfile is a balanced, always-on NPC.
func DefaultNPCProfile() NPCProfile {
	return NPCProfile{
		Strategy:   NPCBalanced,
		Difficulty: "normal",
		Phase:      0,
		ActiveFrom: 0,
		ActiveTo:   24,
	}
}

// Normalized returns the profile with every field clamped to a valid range and
// missing values replaced by defaults.
func (p NPCProfile) Normalized() NPCProfile {
	switch p.Strategy {
	case NPCBalanced, NPCBuilder, NPCAggressive, NPCPassive:
	default:
		p.Strategy = NPCBalanced
	}
	switch p.Difficulty {
	case "easy", "normal", "hard":
	default:
		p.Difficulty = "normal"
	}
	if p.Phase < 0 {
		p.Phase = 0
	}
	if p.Phase > 3 {
		p.Phase = 3
	}
	if p.ActiveFrom < 0 || p.ActiveFrom > 23 {
		p.ActiveFrom = 0
	}
	if p.ActiveTo < 1 || p.ActiveTo > 24 {
		p.ActiveTo = 24
	}
	// Equal endpoints mean "always on"; otherwise 20 -> 3 is a valid overnight
	// window that wraps past midnight.
	if p.ActiveFrom == p.ActiveTo {
		p.ActiveFrom, p.ActiveTo = 0, 24
	}
	return p
}

// RandomNPCProfile rolls a plausible profile, used when seeding a universe.
func RandomNPCProfile(rng *rand.Rand) NPCProfile {
	strategies := []NPCStrategy{NPCBalanced, NPCBuilder, NPCAggressive, NPCPassive}
	difficulties := []string{"easy", "normal", "hard"}
	p := NPCProfile{
		Strategy:   strategies[rng.Intn(len(strategies))],
		Difficulty: difficulties[rng.Intn(len(difficulties))],
	}
	// Most NPCs are around the clock; some keep shorter, shifted windows and a
	// few keep an overnight window that wraps past midnight.
	switch {
	case rng.Float64() < 0.5:
		from := rng.Intn(18)
		span := 4 + rng.Intn(8) // 4..11 hours
		p.ActiveFrom = from
		p.ActiveTo = from + span
		if p.ActiveTo > 24 {
			p.ActiveTo = 24
		}
	case rng.Float64() < 0.4:
		p.ActiveFrom = 18 + rng.Intn(6) // 18..23
		p.ActiveTo = 4 + rng.Intn(6)    // 4..9
	}
	return p.Normalized()
}

// DevelopmentTier derives an NPC's coarse development tier (0..3) from the
// buildings and tech it already owns. It is a cheap score, not a database read:
// the caller passes the structures/techs it has already loaded. Tier 0 is a
// bare colony, 1 an economy that is online, 2 a military that has come online,
// 3 an advanced, late-round power.
func DevelopmentTier(structs, techs map[string]int) int {
	score := structs["iron_mine"] + structs["lutinum_mine"] + structs["water_extractor"]
	score += structs["command_center"] + structs["research_center"]
	score += structs["chemical_factory"] + structs["shipyard"] + structs["orbital_defense"]
	for _, level := range techs {
		score += level
	}
	switch {
	case score >= 60:
		return 3
	case score >= 30:
		return 2
	case score >= 10:
		return 1
	default:
		return 0
	}
}

// npcPhaseBuildingWeight biases a building choice by the NPC's development
// tier: early NPCs build their economy, late ones build military and advanced
// structures. Multipliers stay modest so strategy still shows through.
func npcPhaseBuildingWeight(phase int, code string) float64 {
	d, ok := Catalog[code]
	if !ok {
		return 1
	}
	early := d.Group == "Resource buildings" || code == "command_center" || code == "research_center"
	late := d.Group == "Military buildings" || code == "advanced_chemical_factory"
	switch {
	case phase <= 1:
		if late {
			return 0.8
		}
		if early {
			return 1.3
		}
	case phase >= 2:
		if late {
			return 1.3
		}
		if early {
			return 0.8
		}
	}
	return 1
}

// npcPhaseUnitWeight biases unit production by development tier: early NPCs
// favour probes, recyclers and traders; late ones favour warships/defense.
func npcPhaseUnitWeight(phase int, def Def) float64 {
	utility := def.Code == "spy_probe" || def.Code == "recycler" || def.Code == "colony_ship"
	trader := npcTraders[def.Code]
	warship := npcWarships[def.Code] || def.Kind == KindDefense
	switch {
	case phase <= 1:
		if utility || trader {
			return 1.3
		}
		if warship {
			return 0.85
		}
	case phase >= 2:
		if warship {
			return 1.3
		}
		if utility || trader {
			return 0.85
		}
	}
	return 1
}

// npcPhaseResearchWeight biases research by development tier: early NPCs chase
// drives and utility techs, late ones weapons and armour.
func npcPhaseResearchWeight(phase int, code string) float64 {
	early := map[string]bool{
		"combustion_drive": true, "ion_drive": true,
		"increased_cargo_capacity": true, "recycling_technology": true,
	}
	late := map[string]bool{
		"ionization": true, "energy_focusing": true, "explosive_projectiles": true,
		"advanced_ship_armor": true, "space_curvature_drive": true,
		"space_folding_drive": true,
	}
	switch {
	case phase <= 1:
		if early[code] {
			return 1.3
		}
		if late[code] {
			return 0.85
		}
	case phase >= 2:
		if late[code] {
			return 1.3
		}
		if early[code] {
			return 0.85
		}
	}
	return 1
}

// npcBaseBuildingWeight is the base desirability of building a level of each
// building. It is a hand-authored probability tree in the spirit of the
// reference: weighted-random rather than a fixed build order, so builds vary.
var npcBaseBuildingWeight = map[string]float64{
	"iron_mine":                 10,
	"lutinum_mine":              9,
	"water_extractor":           8,
	"command_center":            7,
	"chemical_factory":          6,
	"research_center":           5,
	"shipyard":                  5,
	"orbital_defense":           4,
	"iron_storage":              3,
	"lutinum_storage":           3,
	"water_storage":             2,
	"hydrogen_storage":          2,
	"fusion_reactor":            1,
	"advanced_chemical_factory": 1,
	"planetary_shield":          1,
	"trading_post":              1,
}

// npcStrategyWeight reweights a building by the NPC's strategy.
func npcStrategyWeight(code string, strategy NPCStrategy) float64 {
	d, ok := Catalog[code]
	if !ok {
		return 0
	}
	resource := d.Group == "Resource buildings"
	military := d.Group == "Military buildings"
	switch strategy {
	case NPCBuilder:
		if resource {
			return 1.5
		}
		if military {
			return 0.6
		}
	case NPCAggressive:
		if military {
			return 1.6
		}
		if resource {
			return 0.8
		}
	case NPCPassive:
		if military {
			return 1.4
		}
		if code == "shipyard" {
			return 0.5
		}
	}
	return 1
}

// npcCandidate is one weighted choice in an NPC probability tree.
type npcCandidate struct {
	code   string
	weight float64
}

// weightedShuffle orders candidates by repeated weighted picks, so every legal
// option can come up first but likelier ones usually do. It is the one shuffle
// behind both the building and the unit probability trees.
func weightedShuffle(pool []npcCandidate, rng *rand.Rand) []string {
	out := make([]string, 0, len(pool))
	for len(pool) > 0 {
		total := 0.0
		for _, c := range pool {
			total += c.weight
		}
		pick := rng.Float64() * total
		idx := len(pool) - 1
		for i, c := range pool {
			pick -= c.weight
			if pick <= 0 {
				idx = i
				break
			}
		}
		out = append(out, pool[idx].code)
		pool = append(pool[:idx], pool[idx+1:]...)
	}
	return out
}

// RankedBuildings returns every building the NPC may legally start on now,
// ordered by a weighted shuffle. The caller attempts them in order until one
// succeeds, so soft failures (no fields, not enough resources) simply fall
// through to the next candidate. Buildings already at a higher level are
// damped, which spreads levels out instead of over-upgrading a favourite.
func RankedBuildings(structs, techs map[string]int, profile NPCProfile, rng *rand.Rand) []string {
	profile = profile.Normalized()
	pool := make([]npcCandidate, 0, len(Catalog))
	for code, d := range Catalog {
		if d.Kind != KindBuilding {
			continue
		}
		if !PrereqsMet(d, structs, techs) {
			continue
		}
		w := npcBaseBuildingWeight[code]
		if w <= 0 {
			w = 1
		}
		w *= npcStrategyWeight(code, profile.Strategy)
		w *= npcPhaseBuildingWeight(profile.Phase, code)
		w *= 1.0 / (1.0 + 0.12*float64(structs[code]))
		if w <= 0 {
			continue
		}
		pool = append(pool, npcCandidate{code: code, weight: w})
	}
	return weightedShuffle(pool, rng)
}

// npcBaseUnitWeight is the base desirability of producing each ship/defense
// unit. Like the building tree it is a hand-authored probability tree: which
// hulls an NPC favours, not a fixed production order. Utilities (probes,
// recyclers, colony ships) are cheap-but-niche; combat hulls vary.
var npcBaseUnitWeight = map[string]float64{
	"spy_probe":    4,
	"jackal":       5,
	"small_trader": 3,
	"renegade":     5,
	"raider":       4,
	"recycler":     2,
	"falcon":       3,
	"colony_ship":  2,
	"tjuger":       2,
	"cougar":       2,
	"longeagle_v":  1,
	"large_trader": 1,
	"noah":         1,
	"longeagle_x":  1,

	"light_laser":     5,
	"heavy_laser":     4,
	"emp_launcher":    3,
	"plasma_turret":   2,
	"rocket_launcher": 1,
}

// npcWarships and npcTraders classify the ship roster so a strategy can favour
// the hulls it cares about. Everything else (probes, recyclers, colony ships)
// is utility.
var (
	npcWarships = map[string]bool{
		"jackal": true, "renegade": true, "raider": true, "falcon": true,
		"tjuger": true, "cougar": true, "longeagle_v": true, "longeagle_x": true,
	}
	npcTraders = map[string]bool{
		"small_trader": true, "large_trader": true, "noah": true,
	}
)

// npcUnitStrategyWeight reweights a ship/defense unit by the NPC's strategy.
func npcUnitStrategyWeight(def Def, strategy NPCStrategy) float64 {
	switch strategy {
	case NPCBuilder:
		if npcTraders[def.Code] || def.Code == "colony_ship" {
			return 1.7
		}
		if npcWarships[def.Code] {
			return 0.6
		}
		if def.Code == "recycler" {
			return 1.3
		}
	case NPCAggressive:
		if npcWarships[def.Code] {
			return 1.8
		}
		if def.Kind == KindDefense {
			return 0.6
		}
		if def.Code == "spy_probe" {
			return 1.3
		}
	case NPCPassive:
		if def.Kind == KindDefense {
			return 1.8
		}
		if npcWarships[def.Code] {
			return 0.6
		}
	}
	return 1
}

// RankedUnits returns every ship/defense unit the NPC may legally produce now,
// ordered by a weighted shuffle. The same attempt-in-order-until-success
// contract as RankedBuildings applies: soft failures fall through to the next
// candidate.
func RankedUnits(structs, techs map[string]int, profile NPCProfile, rng *rand.Rand) []string {
	profile = profile.Normalized()
	pool := make([]npcCandidate, 0, len(Catalog))
	for code, d := range Catalog {
		if d.Kind != KindShip && d.Kind != KindDefense {
			continue
		}
		if !PrereqsMet(d, structs, techs) {
			continue
		}
		w := npcBaseUnitWeight[code]
		if w <= 0 {
			w = 1
		}
		w *= npcUnitStrategyWeight(d, profile.Strategy)
		w *= npcPhaseUnitWeight(profile.Phase, d)
		if w <= 0 {
			continue
		}
		pool = append(pool, npcCandidate{code: code, weight: w})
	}
	return weightedShuffle(pool, rng)
}

// npcBaseResearchWeight is the base desirability of researching each tech. Like
// the building and unit trees it is a hand-authored probability tree: which
// routes an NPC favours, not a fixed research order. It is what lets an NPC
// progress past the tech gates that otherwise pin it to probes and light guns.
var npcBaseResearchWeight = map[string]float64{
	"combustion_drive":         10,
	"ion_drive":                9,
	"space_curvature_drive":    5,
	"space_folding_drive":      2,
	"spy_technology":           5,
	"ionization":               6,
	"energy_focusing":          5,
	"explosive_projectiles":    4,
	"advanced_ship_armor":      4,
	"increased_cargo_capacity": 3,
	"recycling_technology":     3,
}

// npcResearchStrategyWeight reweights a tech by the NPC's strategy.
func npcResearchStrategyWeight(code string, strategy NPCStrategy) float64 {
	drives := map[string]bool{
		"combustion_drive": true, "ion_drive": true,
		"space_curvature_drive": true, "space_folding_drive": true,
	}
	weapons := map[string]bool{
		"ionization": true, "energy_focusing": true, "explosive_projectiles": true,
	}
	switch strategy {
	case NPCBuilder:
		if drives[code] || code == "increased_cargo_capacity" || code == "recycling_technology" {
			return 1.5
		}
		if weapons[code] {
			return 0.7
		}
	case NPCAggressive:
		if weapons[code] {
			return 1.6
		}
		if code == "increased_cargo_capacity" || code == "recycling_technology" {
			return 0.8
		}
	case NPCPassive:
		if code == "advanced_ship_armor" {
			return 1.6
		}
		if code == "spy_technology" {
			return 1.3
		}
		if weapons[code] {
			return 0.9
		}
	}
	return 1
}

// RankedResearch returns every tech the NPC may legally start now, ordered by a
// weighted shuffle. The same attempt-in-order-until-success contract as
// RankedBuildings applies: soft failures fall through to the next candidate.
// Techs are account-wide but gated on a planet's Research Center, so the caller
// evaluates them against one planet's structures at a time.
func RankedResearch(structs, techs map[string]int, profile NPCProfile, rng *rand.Rand) []string {
	profile = profile.Normalized()
	pool := make([]npcCandidate, 0, len(Catalog))
	for code, d := range Catalog {
		if d.Kind != KindResearch {
			continue
		}
		if !PrereqsMet(d, structs, techs) {
			continue
		}
		w := npcBaseResearchWeight[code]
		if w <= 0 {
			w = 1
		}
		w *= npcResearchStrategyWeight(code, profile.Strategy)
		w *= npcPhaseResearchWeight(profile.Phase, code)
		w *= 1.0 / (1.0 + 0.12*float64(techs[code]))
		if w <= 0 {
			continue
		}
		pool = append(pool, npcCandidate{code: code, weight: w})
	}
	return weightedShuffle(pool, rng)
}

// NPCUnitBatch picks how many of one unit to produce in a single order. Cheap
// hulls are built in handfuls, capitals one or two at a time, so a batch is
// always something the NPC might plausibly afford. Aggressive NPCs push a
// little harder.
func NPCUnitBatch(def Def, profile NPCProfile, rng *rand.Rand) int64 {
	if def.Code == "colony_ship" {
		return 1
	}
	cost := def.Cost.Metal + def.Cost.Lutinum + def.Cost.Water + def.Cost.Hydrogen
	var qty int64
	switch {
	case cost <= 1000:
		qty = int64(5 + rng.Intn(6)) // 5..10
	case cost <= 10000:
		qty = int64(2 + rng.Intn(4)) // 2..5
	default:
		qty = 1
	}
	if profile.Normalized().Strategy == NPCAggressive {
		qty++
	}
	return qty
}

// clamp01 clamps a probability-like factor into [0,1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Difficulty scales reaction odds and decision cadence. The multipliers are
// shared so the two stay in lock-step: hard NPCs act more decisively and think a
// little sooner, easy ones more sluggishly and a little later.
const (
	diffScaleEasy = 0.7
	diffScaleHard = 1.3
)

// difficultyScale nudges every reaction probability: hard NPCs act more
// decisively, easy ones more sluggishly. It never exceeds a valid probability
// once multiplied and clamped. The same factor also compresses/expands the
// decision cadence in NextDecisionDelay.
func (p NPCProfile) difficultyScale() float64 {
	switch p.Difficulty {
	case "easy":
		return diffScaleEasy
	case "hard":
		return diffScaleHard
	default:
		return 1.0
	}
}

// MinDifficultyScale is the smallest reaction/cadence scale (easy). The startup
// capacity check uses it to estimate the fastest possible due rate.
func MinDifficultyScale() float64 { return diffScaleEasy }

// SpyChance is the per-inbound-attack probability that the NPC spends a probe
// to scout the attacker. Aggressive NPCs look; builders usually shrug.
func (p NPCProfile) SpyChance() float64 {
	base := 0.35
	switch p.Strategy {
	case NPCAggressive:
		base = 0.55
	case NPCPassive:
		base = 0.40
	case NPCBuilder:
		base = 0.20
	}
	return clamp01(base * p.difficultyScale())
}

// FleetSaveChance is the probability that, on an inbound attack, the NPC
// deliberately lifts its fleet off the planet ("fleet in the air"). It is
// deliberately low: saves are rare and sloppy, not a perfect defence.
func (p NPCProfile) FleetSaveChance() float64 {
	base := 0.20
	switch p.Strategy {
	case NPCAggressive:
		base = 0.10
	case NPCPassive:
		base = 0.35
	case NPCBuilder:
		base = 0.25
	}
	return clamp01(base * p.difficultyScale())
}

// ColonizeChance is the probability that an NPC holding a colony ship commits
// it on this decision. Builders expand most eagerly.
func (p NPCProfile) ColonizeChance() float64 {
	base := 0.25
	switch p.Strategy {
	case NPCBuilder:
		base = 0.40
	case NPCAggressive:
		base = 0.20
	case NPCPassive:
		base = 0.10
	}
	return clamp01(base * p.difficultyScale())
}

// npcDecisionFloor is the shortest real-time gap between two NPC decisions. A
// whole population sharing a floor this long stays well inside one tick's
// npcBatch, even after the game-speed scaling below compresses every delay
// onto it.
const npcDecisionFloor = 30 * time.Second

// NPCDecisionFloorSeconds exposes the real-time decision floor in seconds so
// startup code can estimate the worst-case NPC due rate against NPC_BATCH.
func NPCDecisionFloorSeconds() float64 { return npcDecisionFloor.Seconds() }

// npcDecisionCapMinutes caps the jittered, game-time delay so an NPC never
// idles for an unbounded stretch (the old tail ran to ~2h30m).
const npcDecisionCapMinutes = 60

// NextDecisionDelay returns how long until the NPC's next decision. The cadence
// is deliberately uneven ("here and there"): usually a handful of minutes, now
// and then a longer pause — never a tight 24/7 loop. The shape is measured in
// game-time and divided by gameSpeed, so NPCs keep pace in fast universes; a
// real-time floor keeps a large population from flooding the scheduler.
//
// Difficulty also shows up here: the game-time pause and the floor are divided
// by difficultyScale, so hard NPCs decide a little sooner and easy ones later.
// Normal is unchanged (scale 1).
func NextDecisionDelay(profile NPCProfile, gameSpeed float64, rng *rand.Rand) time.Duration {
	if gameSpeed <= 0 {
		gameSpeed = 1
	}
	mins := float64(4 + rng.Intn(26)) // 4..29 game-minutes
	if rng.Float64() < 0.25 {
		mins += float64(30 + rng.Intn(90)) // occasionally a longer pause
	}
	mins /= profile.difficultyScale()
	if mins > npcDecisionCapMinutes {
		mins = npcDecisionCapMinutes
	}
	d := time.Duration(mins / gameSpeed * float64(time.Minute))
	floor := time.Duration(float64(npcDecisionFloor) / profile.difficultyScale())
	if d < floor {
		d = floor
	}
	return d
}

// NPCAwake reports whether the NPC is inside its server-clock online window at
// t. A window whose endpoints are equal (normalized to 0..24) is always on; a
// window where ActiveFrom > ActiveTo wraps overnight. Pure clock math, no DB.
func NPCAwake(profile NPCProfile, t time.Time) bool {
	p := profile.Normalized()
	if p.ActiveFrom == 0 && p.ActiveTo == 24 {
		return true
	}
	hour := t.Hour()
	if p.ActiveFrom < p.ActiveTo {
		return hour >= p.ActiveFrom && hour < p.ActiveTo
	}
	return hour >= p.ActiveFrom || hour < p.ActiveTo
}

// NPCNextWindowOpen returns the next time the NPC's window opens (the
// closed→open transition) at or after t, plus a small deterministic jitter so a
// population that shares a window does not all wake in the same second. Pure
// clock math, no DB. If the window is already open, it returns the following
// day's opening.
func NPCNextWindowOpen(profile NPCProfile, t time.Time) time.Time {
	p := profile.Normalized()
	if p.ActiveFrom == 0 && p.ActiveTo == 24 {
		return t.Add(npcWindowJitter(p))
	}
	next := t.Truncate(time.Hour).Add(time.Hour)
	for i := 0; i < 25; i++ {
		if NPCAwake(p, next) && !NPCAwake(p, next.Add(-time.Hour)) {
			return next.Add(npcWindowJitter(p))
		}
		next = next.Add(time.Hour)
	}
	return t.Add(npcWindowJitter(p))
}

// npcWindowJitter derives a stable 0..119s offset from the profile, so two NPCs
// on the same window open at slightly different moments.
func npcWindowJitter(p NPCProfile) time.Duration {
	h := 2166136261
	mix := func(s string) {
		for i := 0; i < len(s); i++ {
			h ^= int(s[i])
			h *= 16777619
		}
	}
	mix(string(p.Strategy))
	mix(p.Difficulty)
	h ^= p.ActiveFrom*31 + p.ActiveTo
	h *= 16777619
	if h < 0 {
		h = -h
	}
	return time.Duration(h%120) * time.Second
}

// npcNameAdjectives and npcNameNouns are combined into readable NPC handles.
var (
	npcNameAdjectives = []string{
		"Silent", "Crimson", "Iron", "Void", "Broken", "Golden", "Wandering",
		"Ancient", "Hollow", "Frozen", "Burning", "Lost", "Distant", "Savage",
		"Quiet", "Cunning", "Eternal", "Rogue", "Nomad", "Obsidian",
	}
	npcNameNouns = []string{
		"Pilgrim", "Sentinel", "Marauder", "Scholar", "Artificer", "Warden",
		"Trader", "Reaver", "Cartographer", "Exile", "Herald", "Vanguard",
		"Prospector", "Sovereign", "Acolyte", "Drifter", "Architect", "Baron",
	}
)

// RandomNPCName returns a readable handle. Callers must still ensure it is
// unique within the universe.
func RandomNPCName(rng *rand.Rand) string {
	return npcNameAdjectives[rng.Intn(len(npcNameAdjectives))] + " " +
		npcNameNouns[rng.Intn(len(npcNameNouns))]
}
