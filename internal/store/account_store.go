package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Vacation mode is a self-imposed pause: the account cannot start queues or
// send fleets, and its planets cannot be attacked or spied. Production and the
// build/research/unit queues freeze while it lasts. It has to last at least 12
// hours and ends automatically after 21 days.
const (
	VacationMinDuration = 12 * time.Hour
	VacationMaxDuration = 21 * 24 * time.Hour
)

// AccountFlags is the account-lifecycle state read from a user row.
type AccountFlags struct {
	Sanctioned    bool
	SanctionNote  string
	VacationSince sql.NullTime
	VacationUntil sql.NullTime
}

// VacationActive reports whether the account is on vacation at time t.
func (f AccountFlags) VacationActive(t time.Time) bool {
	return f.VacationSince.Valid && f.VacationUntil.Valid && t.Before(f.VacationUntil.Time)
}

// CanLeaveVacation reports whether the minimum vacation duration has elapsed.
func CanLeaveVacation(since, now time.Time) bool {
	return !now.Before(since.Add(VacationMinDuration))
}

// VacationPause is how long an in-progress queue should be pushed back when a
// vacation that started at `since` ends at `now` (capped at the maximum).
func VacationPause(since, now time.Time) time.Duration {
	d := now.Sub(since)
	if d < 0 {
		return 0
	}
	if d > VacationMaxDuration {
		return VacationMaxDuration
	}
	return d
}

// AccountFlags loads a user's lifecycle flags.
func (s *AuthStore) AccountFlags(ctx context.Context, userID int64) (AccountFlags, error) {
	return accountFlags(ctx, s.db, userID)
}

func accountFlags(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userID int64) (AccountFlags, error) {
	var f AccountFlags
	err := q.QueryRowContext(ctx, `
		SELECT sanctioned, COALESCE(sanction_reason, ''), vacation_since, vacation_until
		FROM users WHERE id = $1
	`, userID).Scan(&f.Sanctioned, &f.SanctionNote, &f.VacationSince, &f.VacationUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

func accountFlagsTx(ctx context.Context, tx *sql.Tx, userID int64) (AccountFlags, error) {
	return accountFlags(ctx, tx, userID)
}

// ensureQueueingAllowedTx blocks starting buildings/research/units for a
// sanctioned or vacationing account.
func ensureQueueingAllowedTx(ctx context.Context, tx *sql.Tx, userID int64) error {
	f, err := accountFlagsTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if f.Sanctioned {
		return ErrSanctioned
	}
	if f.VacationActive(time.Now()) {
		return ErrOnVacation
	}
	return nil
}

// ensureDispatchAllowedTx blocks sending any fleet for a sanctioned or
// vacationing account.
func ensureDispatchAllowedTx(ctx context.Context, tx *sql.Tx, userID int64) error {
	f, err := accountFlagsTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if f.Sanctioned {
		return ErrSanctioned
	}
	if f.VacationActive(time.Now()) {
		return ErrOnVacation
	}
	return nil
}

// ensureAttackableTx blocks attacking or spying on a planet whose owner is on
// vacation. Already-launched fleets are not re-checked (they still arrive).
func ensureAttackableTx(ctx context.Context, tx *sql.Tx, targetUserID int64) error {
	f, err := accountFlagsTx(ctx, tx, targetUserID)
	if err != nil {
		return err
	}
	if f.VacationActive(time.Now()) {
		return ErrTargetOnVacation
	}
	return nil
}

// StartVacation switches the account into vacation mode. It is refused while any
// own fleet is still in flight. Production is settled up to now, then frozen.
func (s *AuthStore) StartVacation(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var since sql.NullTime
	if err := tx.QueryRowContext(ctx,
		`SELECT vacation_since FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&since); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if since.Valid {
		return ErrAlreadyOnVacation
	}

	var inFlight int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM fleets WHERE user_id = $1 AND phase IN ('OUTBOUND','RETURNING')
	`, userID).Scan(&inFlight); err != nil {
		return err
	}
	if inFlight > 0 {
		return ErrFleetsInFlight
	}

	if err := settlePlanetsTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET vacation_since = NOW(),
		    vacation_until = NOW() + make_interval(secs => $2)
		WHERE id = $1
	`, userID, VacationMaxDuration.Seconds()); err != nil {
		return err
	}
	return tx.Commit()
}

// EndVacation leaves vacation mode manually. The minimum duration must have
// elapsed. Paused queues are pushed back by the time spent away.
func (s *AuthStore) EndVacation(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := endVacationTx(ctx, tx, userID, time.Now(), true); err != nil {
		return err
	}
	return tx.Commit()
}

func endVacationTx(ctx context.Context, tx *sql.Tx, userID int64, now time.Time, enforceMinimum bool) error {
	var since, until sql.NullTime
	if err := tx.QueryRowContext(ctx,
		`SELECT vacation_since, vacation_until FROM users WHERE id = $1 FOR UPDATE`, userID).
		Scan(&since, &until); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if !since.Valid {
		return ErrNotOnVacation
	}
	if enforceMinimum && !CanLeaveVacation(since.Time, now) {
		return ErrVacationMinimum
	}

	// Settle production while the vacation columns are still set so the
	// elapsed time inside the vacation window is frozen out.
	if err := settlePlanetsTx(ctx, tx, userID); err != nil {
		return err
	}
	if pause := VacationPause(since.Time, now); pause > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE queues
			SET start_at = start_at + make_interval(secs => $2),
			    finish_at = finish_at + make_interval(secs => $2)
			WHERE user_id = $1 AND status = 'IN_PROGRESS'
		`, userID, pause.Seconds()); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE users SET vacation_since = NULL, vacation_until = NULL WHERE id = $1`, userID)
	return err
}

// FinalizeExpiredVacations auto-ends every vacation whose 21-day window has
// elapsed. Called by the scheduler each tick.
func FinalizeExpiredVacations(ctx context.Context, db *sql.DB) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id FROM users
		WHERE vacation_since IS NOT NULL AND vacation_until IS NOT NULL AND vacation_until <= NOW()
	`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	done := 0
	for _, id := range ids {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return done, err
		}
		if err := endVacationTx(ctx, tx, id, time.Now(), false); err != nil {
			tx.Rollback()
			return done, err
		}
		if err := tx.Commit(); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

// settlePlanetsTx banks production for every planet the user owns. Called before
// entering or leaving vacation so the freeze windows are exact.
func settlePlanetsTx(ctx context.Context, tx *sql.Tx, userID int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM planets WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		var m float64
		if err := tx.QueryRowContext(ctx,
			`SELECT out_metal FROM update_planet_resources($1)`, id).Scan(&m); err != nil {
			return err
		}
	}
	return nil
}
