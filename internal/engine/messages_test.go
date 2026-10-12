package engine

import "testing"

func TestQueueMessage(t *testing.T) {
	cases := []struct {
		kind, code string
		level      int
		quantity   int64
		at         string
		wantKind   string
		wantTitle  string
	}{
		{"building", "iron_mine", 3, 0, "1:2:3", "building", "Construction of Iron Mine (level 3) on 1:2:3 has been completed."},
		{"research", "energy_tech", 2, 0, "1:2:3", "research", "Research of Energy Research (level 2) has been completed."},
		{"ship", "spy_probe", 0, 5, "1:2:3", "ship", "Production of 5 × Spy Probe on 1:2:3 has been completed."},
		{"defense", "defense_turret", 0, 4, "1:2:3", "defense", "Production of 4 × Defense Turret on 1:2:3 has been completed."},
		{"building", "unknown_code", 1, 0, "9:9:9", "building", "Construction of unknown_code (level 1) on 9:9:9 has been completed."},
	}
	for _, c := range cases {
		gotKind, gotTitle := queueMessage(c.kind, c.code, c.level, c.quantity, c.at)
		if gotKind != c.wantKind || gotTitle != c.wantTitle {
			t.Errorf("queueMessage(%q,%q,%d,%d,%q) = (%q,%q), want (%q,%q)",
				c.kind, c.code, c.level, c.quantity, c.at, gotKind, gotTitle, c.wantKind, c.wantTitle)
		}
	}
}
