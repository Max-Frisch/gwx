package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"gwx/internal/game"
)

// BuilderStore owns the Colony Builder: an account-wide target-level template
// the durable scheduler drives every owned planet toward. It is a normal `users`
// row's columns, so no separate table is needed.
type BuilderStore struct {
	db *sql.DB
}

// NewBuilderStore constructs a BuilderStore.
func NewBuilderStore(db *sql.DB) *BuilderStore { return &BuilderStore{db: db} }

// Builder is one due builder account: its id, universe and template.
type Builder struct {
	UserID   int64
	Universe string
	Template game.BuildTemplate
}

// LoadTemplate returns the account's build template and whether the builder is
// enabled.
func (s *BuilderStore) LoadTemplate(ctx context.Context, userID int64) (game.BuildTemplate, bool, error) {
	var (
		raw     []byte
		enabled bool
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT build_template, builder_enabled FROM users WHERE id = $1`, userID).Scan(&raw, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, ErrNotFound
	}
	if err != nil {
		return nil, false, err
	}
	tpl := game.BuildTemplate{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &tpl)
	}
	return tpl, enabled, nil
}

// SaveTemplate persists the account's template and enabled flag, and marks the
// account due immediately so the change takes effect on the next tick.
func (s *BuilderStore) SaveTemplate(ctx context.Context, userID int64, tpl game.BuildTemplate, enabled bool) error {
	if tpl == nil {
		tpl = game.BuildTemplate{}
	}
	raw, err := json.Marshal(tpl)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE users SET build_template = $2, builder_enabled = $3, builder_next_check_at = NOW()
		WHERE id = $1
	`, userID, raw, enabled)
	return err
}

// SetEnabled flips the builder on or off for an account, marking it due when it
// is switched on.
func (s *BuilderStore) SetEnabled(ctx context.Context, userID int64, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET builder_enabled = $2,
			builder_next_check_at = CASE WHEN $2 THEN NOW() ELSE builder_next_check_at END
		WHERE id = $1
	`, userID, enabled)
	return err
}

// ClaimDueBuilders leases up to limit enabled, non-NPC accounts whose next check
// is due. Each claim is its own transaction using FOR UPDATE SKIP LOCKED, so
// concurrent schedulers never process the same account. Claiming parks the
// account for the interval so the phase runs on a cadence rather than every
// tick.
func (s *BuilderStore) ClaimDueBuilders(ctx context.Context, limit int, interval time.Duration) ([]Builder, error) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	var out []Builder
	for len(out) < limit {
		b, ok, err := s.claimOne(ctx, interval)
		if err != nil {
			return out, err
		}
		if !ok {
			break
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *BuilderStore) claimOne(ctx context.Context, interval time.Duration) (Builder, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Builder{}, false, err
	}
	defer tx.Rollback()

	var (
		b   Builder
		raw []byte
	)
	err = tx.QueryRowContext(ctx, `
		SELECT u.id, un.code_name, u.build_template
		FROM users u
		JOIN universes un ON un.id = u.universe_id
		WHERE u.builder_enabled AND NOT u.is_npc
		  AND (u.builder_next_check_at IS NULL OR u.builder_next_check_at <= NOW())
		ORDER BY u.builder_next_check_at NULLS FIRST
		FOR UPDATE OF u SKIP LOCKED
		LIMIT 1
	`).Scan(&b.UserID, &b.Universe, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Builder{}, false, nil
	}
	if err != nil {
		return Builder{}, false, err
	}
	b.Template = game.BuildTemplate{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &b.Template)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET builder_next_check_at = NOW() + make_interval(secs => $2) WHERE id = $1
	`, b.UserID, interval.Seconds()); err != nil {
		return Builder{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Builder{}, false, err
	}
	return b, true, nil
}
