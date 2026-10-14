package integration

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// TestColonyBuilderBuildsTowardTemplate enables the builder with a small
// template and checks the scheduler fills the construction queue and lands the
// target levels automatically.
func TestColonyBuilderBuildsTowardTemplate(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "colonist")
	h.topUp(t, p)

	h.action(t, p.client, fmt.Sprintf("/builder/%d", p.home), url.Values{
		"enabled":               {"1"},
		"target_command_center": {"3"},
		"target_iron_mine":      {"3"},
		"target_lutinum_mine":   {"2"},
	})

	// The page renders the editor after saving.
	if code, body := h.get(t, p.client, fmt.Sprintf("/builder/%d", p.home)); code != 200 || !strings.Contains(body, "Colony Builder") {
		t.Fatalf("builder page -> %d, missing heading:\n%s", code, snippet([]byte(body)))
	}

	waitFor(t, "command center level 3", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items WHERE planet_id = $1 AND kind = 'building' AND code = 'command_center'
		`, p.home) >= 3
	})
	waitFor(t, "iron mine level 3", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items WHERE planet_id = $1 AND kind = 'building' AND code = 'iron_mine'
		`, p.home) >= 3
	})
	waitFor(t, "lutinum mine level 2", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items WHERE planet_id = $1 AND kind = 'building' AND code = 'lutinum_mine'
		`, p.home) >= 2
	})
}

// TestColonyBuilderAutoBuildsPrerequisites checks that a template naming only a
// deep building (Iron Storage, which needs Iron Mine 5) still drives the
// prerequisite automatically.
func TestColonyBuilderAutoBuildsPrerequisites(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "prereq")
	h.topUp(t, p)

	h.action(t, p.client, fmt.Sprintf("/builder/%d", p.home), url.Values{
		"enabled":             {"1"},
		"target_iron_storage": {"1"},
	})

	waitFor(t, "iron mine level 5 (auto prereq)", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items WHERE planet_id = $1 AND kind = 'building' AND code = 'iron_mine'
		`, p.home) >= 5
	})
	waitFor(t, "iron storage level 1", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items WHERE planet_id = $1 AND kind = 'building' AND code = 'iron_storage'
		`, p.home) >= 1
	})
}

// TestColonyBuilderAppliesToAllPlanets gives the account a second planet and
// checks the builder advances both.
func TestColonyBuilderAppliesToAllPlanets(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "empire")
	h.topUp(t, p)
	colony := h.addPlanet(t, p.id, "Colony B", 2, 50, 5)
	h.seed(t, colony, seedAmount, seedAmount, seedAmount, seedAmount)

	h.action(t, p.client, fmt.Sprintf("/builder/%d", p.home), url.Values{
		"enabled":               {"1"},
		"target_command_center": {"3"},
	})

	for _, planet := range []int64{p.home, colony} {
		want := planet // capture for the closure message
		waitFor(t, fmt.Sprintf("command center level 3 on planet %d", want), func() bool {
			return h.queryInt(t, `
				SELECT amount FROM planet_items WHERE planet_id = $1 AND kind = 'building' AND code = 'command_center'
			`, want) >= 3
		})
	}
}

// TestColonyBuilderQueueNow exercises the manual "queue now" action with the
// background builder disabled: the planet should still be queued once.
func TestColonyBuilderQueueNow(t *testing.T) {
	h := newHarness(t)
	p := h.register(t, "manual")
	h.topUp(t, p)

	// Save targets without enabling the builder.
	h.action(t, p.client, fmt.Sprintf("/builder/%d", p.home), url.Values{
		"target_iron_mine": {"2"},
	})
	if enabled := h.queryInt(t, `SELECT CASE WHEN builder_enabled THEN 1 ELSE 0 END FROM users WHERE id = $1`, p.id); enabled != 0 {
		t.Fatalf("builder should be disabled after saving without the checkbox")
	}

	h.action(t, p.client, fmt.Sprintf("/builder/%d/run", p.home), url.Values{})

	waitFor(t, "manual iron mine level 2", func() bool {
		return h.queryInt(t, `
			SELECT amount FROM planet_items WHERE planet_id = $1 AND kind = 'building' AND code = 'iron_mine'
		`, p.home) >= 2
	})
}

// addPlanet plants an extra colony for a player directly (test shortcut), the
// way the colonisation resolution would.
func (h *harness) addPlanet(t *testing.T, userID int64, name string, g, s, pos int) int64 {
	t.Helper()
	var id int64
	err := h.db.QueryRow(`
		INSERT INTO planets (
			universe_id, user_id, name, galaxy, system, position, is_homeworld,
			diameter_km, fields_max, fields_used,
			metal, lutinum, water, hydrogen,
			metal_cap, lutinum_cap, water_cap, hydrogen_cap,
			protected_metal, protected_lutinum, protected_water, protected_hydrogen
		) VALUES (
			(SELECT id FROM universes WHERE code_name = $1), $2, $3, $4, $5, $6, FALSE,
			12000, 120, 0,
			1000, 500, 500, 0,
			1000000000000, 1000000000000, 1000000000000, 1000000000000,
			0, 0, 0, 0
		) RETURNING id
	`, h.universe, userID, name, g, s, pos).Scan(&id)
	if err != nil {
		t.Fatalf("add planet: %v", err)
	}
	return id
}
