package game

import "sort"

// BuildTemplate maps a building code to the target level a player wants the
// Colony Builder to reach on every owned planet. Only buildings belong here;
// unknown codes, non-buildings and non-positive levels are ignored.
type BuildTemplate map[string]int

// ExpandTemplate returns the effective building targets: every code in the
// template plus, recursively, the prerequisite buildings and levels needed to
// reach them. This keeps a plan achievable even when the player omits a
// prerequisite (e.g. asking for an Iron Storage level 5 implies Iron Mine 5).
// Research prerequisites are dropped: they are account-wide and outside the
// builder's scope.
func ExpandTemplate(template BuildTemplate) BuildTemplate {
	out := BuildTemplate{}
	expanded := map[string]int{} // highest level already walked, for cycle safety

	var visit func(code string, level int)
	visit = func(code string, level int) {
		def, ok := Catalog[code]
		if !ok || def.Kind != KindBuilding {
			return
		}
		if level > out[code] {
			out[code] = level
		}
		if expanded[code] >= level {
			return
		}
		expanded[code] = level
		for pre, need := range def.Prereqs {
			pdef, ok := Catalog[pre]
			if !ok || pdef.Kind != KindBuilding {
				continue // tech prerequisites are not buildable content
			}
			visit(pre, need)
		}
	}
	for code, level := range template {
		if level > 0 {
			visit(code, level)
		}
	}
	return out
}

// BuildPlan returns, in priority order, the buildings that should be built next
// on one planet to advance toward the template. Each returned code has its
// prerequisites satisfied right now (so a caller may attempt them in order and
// let soft failures fall through), and appears only while its level is below the
// expanded target. Ordering is deterministic: the lowest fraction of the target
// first, so levels stay broadly even rather than over-upgrading one building.
//
// The caller is expected to feed a structures map that already reflects levels
// it has queued this pass (incrementing a code after a successful enqueue), so
// the plan never queues past a target.
func BuildPlan(structs, techs map[string]int, template BuildTemplate) []string {
	targets := ExpandTemplate(template)
	type cand struct {
		code   string
		cur    int
		target int
		ratio  float64
	}
	var cs []cand
	for code, target := range targets {
		cur := structs[code]
		if cur >= target {
			continue
		}
		def, ok := Catalog[code]
		if !ok || def.Kind != KindBuilding {
			continue
		}
		if !PrereqsMet(def, structs, techs) {
			continue
		}
		cs = append(cs, cand{code: code, cur: cur, target: target, ratio: float64(cur) / float64(target)})
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].ratio != cs[j].ratio {
			return cs[i].ratio < cs[j].ratio
		}
		if cs[i].cur != cs[j].cur {
			return cs[i].cur < cs[j].cur
		}
		return cs[i].code < cs[j].code
	})
	out := make([]string, len(cs))
	for i := range cs {
		out[i] = cs[i].code
	}
	return out
}

// BuilderProgress counts how many expanded targets are fully met versus total,
// for the UI's "x / y buildings at target" summary.
func BuilderProgress(structs map[string]int, template BuildTemplate) (done, total int) {
	targets := ExpandTemplate(template)
	for code, target := range targets {
		total++
		if structs[code] >= target {
			done++
		}
	}
	return done, total
}
