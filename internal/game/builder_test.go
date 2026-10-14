package game

import "testing"

func TestExpandTemplateAddsPrerequisites(t *testing.T) {
	cases := []struct {
		name string
		in   BuildTemplate
		want map[string]int
	}{
		{
			name: "storage implies its producer",
			in:   BuildTemplate{"iron_storage": 5},
			want: map[string]int{"iron_storage": 5, "iron_mine": 5},
		},
		{
			name: "transitive chain via chemical factory",
			in:   BuildTemplate{"hydrogen_storage": 2},
			want: map[string]int{"hydrogen_storage": 2, "chemical_factory": 5, "water_extractor": 1},
		},
		{
			name: "deep chain for the planetary shield",
			in:   BuildTemplate{"planetary_shield": 1},
			want: map[string]int{
				"planetary_shield": 1, "shipyard": 1, "orbital_defense": 1,
				"fusion_reactor": 1, "chemical_factory": 5, "water_extractor": 1,
			},
		},
		{
			name: "explicit higher prereq is not lowered",
			in:   BuildTemplate{"iron_mine": 9, "iron_storage": 5},
			want: map[string]int{"iron_mine": 9, "iron_storage": 5},
		},
		{
			name: "unknown, non-building and non-positive targets are dropped",
			in:   BuildTemplate{"spy_probe": 3, "nope": 2, "iron_mine": 0, "iron_mine2": 4},
			want: map[string]int{},
		},
	}
	for _, c := range cases {
		got := ExpandTemplate(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for code, want := range c.want {
			if got[code] != want {
				t.Errorf("%s: target %s = %d, want %d", c.name, code, got[code], want)
			}
		}
	}
}

func TestBuildPlanPrereqGating(t *testing.T) {
	// Wanting an Iron Storage (needs Iron Mine 5) when the mine is level 0 must
	// plan the mine first and leave the storage out until it is buildable.
	plan := BuildPlan(map[string]int{}, map[string]int{}, BuildTemplate{"iron_storage": 1})
	if !contains(plan, "iron_mine") {
		t.Fatalf("plan %v should include the prerequisite iron_mine", plan)
	}
	if contains(plan, "iron_storage") {
		t.Fatalf("plan %v must not include an unbuildable iron_storage", plan)
	}

	// Once the prerequisite is met, the storage becomes the candidate.
	plan = BuildPlan(map[string]int{"iron_mine": 5}, map[string]int{}, BuildTemplate{"iron_storage": 1})
	if contains(plan, "iron_mine") || !contains(plan, "iron_storage") {
		t.Fatalf("plan %v should be the storage once its prereq is met", plan)
	}
}

func TestBuildPlanTargetReached(t *testing.T) {
	if plan := BuildPlan(map[string]int{}, map[string]int{}, BuildTemplate{}); len(plan) != 0 {
		t.Errorf("empty template planned %v, want nothing", plan)
	}
	if plan := BuildPlan(map[string]int{"iron_mine": 5}, map[string]int{}, BuildTemplate{"iron_mine": 5}); len(plan) != 0 {
		t.Errorf("met target planned %v, want nothing", plan)
	}
	if plan := BuildPlan(map[string]int{"iron_mine": 6}, map[string]int{}, BuildTemplate{"iron_mine": 5}); len(plan) != 0 {
		t.Errorf("over-met target planned %v, want nothing", plan)
	}
}

func TestBuildPlanOrderIsDeterministicAndEven(t *testing.T) {
	// iron_mine is 2/4, lutinum_mine is 0/4: the lower fraction goes first.
	plan := BuildPlan(map[string]int{"iron_mine": 2}, map[string]int{}, BuildTemplate{"iron_mine": 4, "lutinum_mine": 4})
	if len(plan) != 2 || plan[0] != "lutinum_mine" || plan[1] != "iron_mine" {
		t.Fatalf("plan = %v, want [lutinum_mine iron_mine]", plan)
	}
}

func TestBuilderProgress(t *testing.T) {
	tpl := BuildTemplate{"iron_storage": 5} // expands to iron_storage + iron_mine
	done, total := BuilderProgress(map[string]int{"iron_mine": 5}, tpl)
	if total != 2 || done != 1 {
		t.Errorf("progress = %d/%d, want 1/2", done, total)
	}
	done, total = BuilderProgress(map[string]int{"iron_mine": 5, "iron_storage": 5}, tpl)
	if total != 2 || done != 2 {
		t.Errorf("progress = %d/%d, want 2/2", done, total)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
