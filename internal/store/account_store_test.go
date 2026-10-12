package store

import (
	"database/sql"
	"testing"
	"time"
)

func TestVacationActive(t *testing.T) {
	now := time.Now()
	active := AccountFlags{
		VacationSince: sql.NullTime{Time: now.Add(-time.Hour), Valid: true},
		VacationUntil: sql.NullTime{Time: now.Add(time.Hour), Valid: true},
	}
	if !active.VacationActive(now) {
		t.Error("expected active vacation")
	}
	if active.VacationActive(now.Add(2 * time.Hour)) {
		t.Error("vacation should be over")
	}
	none := AccountFlags{}
	if none.VacationActive(now) {
		t.Error("no vacation should not be active")
	}
}

func TestCanLeaveVacation(t *testing.T) {
	since := time.Now()
	if CanLeaveVacation(since, since.Add(11*time.Hour)) {
		t.Error("cannot leave before 12h")
	}
	if !CanLeaveVacation(since, since.Add(12*time.Hour)) {
		t.Error("can leave at 12h")
	}
	if !CanLeaveVacation(since, since.Add(30*time.Hour)) {
		t.Error("can leave after 12h")
	}
}

func TestVacationPause(t *testing.T) {
	since := time.Now()
	if got := VacationPause(since, since.Add(5*time.Hour)); got != 5*time.Hour {
		t.Errorf("pause = %v, want 5h", got)
	}
	if got := VacationPause(since, since.Add(-time.Hour)); got != 0 {
		t.Errorf("negative pause = %v, want 0", got)
	}
	if got := VacationPause(since, since.Add(VacationMaxDuration+48*time.Hour)); got != VacationMaxDuration {
		t.Errorf("pause capped = %v, want %v", got, VacationMaxDuration)
	}
}
