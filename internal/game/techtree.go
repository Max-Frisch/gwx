package game

import "sort"

// levelOf resolves a prerequisite code to its current level, checking both the
// planet's building levels and the account's research levels.
func levelOf(code string, structures, techs map[string]int) int {
	if l, ok := structures[code]; ok {
		return l
	}
	return techs[code]
}

// PrereqsMet reports whether every prerequisite for def is satisfied.
func PrereqsMet(def Def, structures, techs map[string]int) bool {
	for code, need := range def.Prereqs {
		if levelOf(code, structures, techs) < need {
			return false
		}
	}
	return true
}

// MissingPrereqs returns a sorted, human-readable list of unmet prerequisites.
func MissingPrereqs(def Def, structures, techs map[string]int) []string {
	var out []string
	for code, need := range def.Prereqs {
		have := levelOf(code, structures, techs)
		if have < need {
			name := code
			if d, ok := Catalog[code]; ok {
				name = d.Name
			}
			out = append(out, name+" (needs level "+itoa(need)+", have "+itoa(have)+")")
		}
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
