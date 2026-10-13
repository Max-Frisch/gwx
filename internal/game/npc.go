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
	Phase      int         `json:"phase"`      // 0..3, coarse round progress
	ActiveFrom int         `json:"activeFrom"` // 0..23, local "online" hour
	ActiveTo   int         `json:"activeTo"`   // 1..24, exclusive; 24 = always on
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
	if p.ActiveFrom < 0 || p.ActiveFrom > 23 {
		p.ActiveFrom = 0
	}
	if p.ActiveTo <= 0 || p.ActiveTo > 24 {
		p.ActiveTo = 24
	}
	if p.ActiveTo <= p.ActiveFrom {
		p.ActiveFrom, p.ActiveTo = 0, 24
	}
	return p
}

// ProductionMultiplier is the difficulty's resource-production factor. gwx has
// no separate "progress points" pool (that is an anti-stall abstraction for the
// reference's real resource economy); a mild production edge is enough to make
// difficulty visible while keeping NPCs on the same rules as players.
func (p NPCProfile) ProductionMultiplier() float64 {
	switch p.Difficulty {
	case "easy":
		return 0.9
	case "hard":
		return 1.15
	default:
		return 1.0
	}
}

// RandomNPCProfile rolls a plausible profile, used when seeding a universe.
func RandomNPCProfile(rng *rand.Rand) NPCProfile {
	strategies := []NPCStrategy{NPCBalanced, NPCBuilder, NPCAggressive, NPCPassive}
	difficulties := []string{"easy", "normal", "hard"}
	p := NPCProfile{
		Strategy:   strategies[rng.Intn(len(strategies))],
		Difficulty: difficulties[rng.Intn(len(difficulties))],
	}
	// Most NPCs are around the clock; some keep shorter, shifted windows.
	if rng.Float64() < 0.5 {
		from := rng.Intn(18)
		span := 4 + rng.Intn(8) // 4..11 hours
		p.ActiveFrom = from
		p.ActiveTo = from + span
		if p.ActiveTo > 24 {
			p.ActiveTo = 24
		}
	}
	return p.Normalized()
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

// RankedBuildings returns every building the NPC may legally start on now,
// ordered by a weighted shuffle. The caller attempts them in order until one
// succeeds, so soft failures (no fields, not enough resources) simply fall
// through to the next candidate. Buildings already at a higher level are
// damped, which spreads levels out instead of over-upgrading a favourite.
func RankedBuildings(structs, techs map[string]int, profile NPCProfile, rng *rand.Rand) []string {
	profile = profile.Normalized()
	type candidate struct {
		code   string
		weight float64
	}
	pool := make([]candidate, 0, len(Catalog))
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
		w *= 1.0 / (1.0 + 0.12*float64(structs[code]))
		if w <= 0 {
			continue
		}
		pool = append(pool, candidate{code: code, weight: w})
	}

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

// NextDecisionDelay returns how long until the NPC's next decision. The cadence
// is deliberately uneven ("here and there"): usually a handful of minutes, now
// and then a long pause — never a tight 24/7 loop.
func NextDecisionDelay(profile NPCProfile, rng *rand.Rand) time.Duration {
	mins := 4 + rng.Intn(26) // 4..29 minutes
	if rng.Float64() < 0.25 {
		mins += 30 + rng.Intn(90) // occasionally an hour or two
	}
	return time.Duration(mins) * time.Minute
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
