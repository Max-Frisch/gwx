package store

import (
	"strings"
	"testing"
)

func TestMessageFilterWhere(t *testing.T) {
	cases := []struct {
		name     string
		f        MessageFilter
		want     []string
		wantArgs int
	}{
		{"default inbox", MessageFilter{}, []string{"user_id = $1", "archived_at IS NULL"}, 1},
		{"category", MessageFilter{Category: "report"}, []string{"category = $2", "archived_at IS NULL"}, 2},
		{"unread", MessageFilter{UnreadOnly: true}, []string{"NOT is_read", "archived_at IS NULL"}, 1},
		{"archive", MessageFilter{Archived: true, Category: "system"}, []string{"category = $2", "archived_at IS NOT NULL"}, 2},
		{"bogus category ignored", MessageFilter{Category: "bogus"}, []string{"archived_at IS NULL"}, 1},
	}
	for _, c := range cases {
		where, args := c.f.where(42)
		if len(args) != c.wantArgs {
			t.Errorf("%s: got %d args, want %d", c.name, len(args), c.wantArgs)
		}
		for _, want := range c.want {
			if !strings.Contains(where, want) {
				t.Errorf("%s: where %q missing %q", c.name, where, want)
			}
		}
	}
}
