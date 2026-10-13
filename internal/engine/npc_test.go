package engine

import (
	"testing"
	"time"

	"gwx/internal/game"
)

func TestNPCGrowthAllowedWindows(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 1, 1, h, 0, 0, 0, time.UTC) }
	night := game.NPCProfile{ActiveFrom: 20, ActiveTo: 3}.Normalized()

	if npcGrowthAllowed(night, 1, at(12)) {
		t.Error("growth allowed outside the window at x1")
	}
	if !npcGrowthAllowed(night, 1, at(22)) {
		t.Error("growth blocked inside the window at x1")
	}
	// Above the window speed cap, windows are ignored.
	if !npcGrowthAllowed(night, npcWindowMaxGameSpeed+1, at(12)) {
		t.Error("growth should ignore the window at high game speed")
	}
}

func TestNPCScheduleAtPushesIntoWindow(t *testing.T) {
	night := game.NPCProfile{ActiveFrom: 20, ActiveTo: 3}.Normalized()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// A short delay would still land closed (12:30); push to the 20:00 opening.
	got := npcScheduleAt(night, 1, now, 30*time.Minute)
	open := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	if got.Before(open) || got.After(open.Add(2*time.Minute)) {
		t.Errorf("npcScheduleAt closed -> %v, want ~%v", got, open)
	}

	// A delay that lands inside the window is kept.
	if got := npcScheduleAt(night, 1, now, 9*time.Hour); !got.Equal(now.Add(9 * time.Hour)) {
		t.Errorf("npcScheduleAt inside window = %v, want %v", got, now.Add(9*time.Hour))
	}

	// At high speed the window is ignored entirely.
	if got := npcScheduleAt(night, 10, now, time.Minute); !got.Equal(now.Add(time.Minute)) {
		t.Errorf("npcScheduleAt high speed = %v, want %v", got, now.Add(time.Minute))
	}
}
