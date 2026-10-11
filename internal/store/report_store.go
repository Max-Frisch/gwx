package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// ReportStore persists and lists combat/espionage reports.
type ReportStore struct {
	db *sql.DB
}

// NewReportStore constructs a ReportStore.
func NewReportStore(db *sql.DB) *ReportStore { return &ReportStore{db: db} }

// Insert stores a report with an arbitrary JSON payload.
func (s *ReportStore) Insert(ctx context.Context, universeUUID string, userID int64, kind, title string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO reports (universe_id, user_id, kind, title, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, universeUUID, userID, kind, title, raw)
	return err
}

// List returns a user's most recent reports.
func (s *ReportStore) List(ctx context.Context, userID int64, limit int) ([]Report, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, title, payload, created_at
		FROM reports WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.Kind, &r.Title, &r.Payload, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Get returns one report owned by the user.
func (s *ReportStore) Get(ctx context.Context, userID, reportID int64) (*Report, error) {
	var r Report
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, title, payload, created_at
		FROM reports WHERE id = $1 AND user_id = $2
	`, reportID, userID).Scan(&r.ID, &r.Kind, &r.Title, &r.Payload, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
