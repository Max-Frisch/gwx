package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MessageStore reads and writes the player inbox.
type MessageStore struct {
	db *sql.DB
}

// NewMessageStore constructs a MessageStore.
func NewMessageStore(db *sql.DB) *MessageStore { return &MessageStore{db: db} }

// MessageFilter narrows a message listing. The zero value shows the active
// (non-archived) inbox across all categories.
type MessageFilter struct {
	Category   string // "", "system" or "report"
	UnreadOnly bool
	Archived   bool // show the archive instead of the active inbox
}

// where builds the shared WHERE fragment plus its positional arguments.
func (f MessageFilter) where(userID int64) (string, []any) {
	conds := []string{"user_id = $1"}
	args := []any{userID}
	if f.Category == "system" || f.Category == "report" {
		args = append(args, f.Category)
		conds = append(conds, fmt.Sprintf("category = $%d", len(args)))
	}
	if f.UnreadOnly {
		conds = append(conds, "NOT is_read")
	}
	if f.Archived {
		conds = append(conds, "archived_at IS NOT NULL")
	} else {
		conds = append(conds, "archived_at IS NULL")
	}
	return strings.Join(conds, " AND "), args
}

// Insert stores a message. reportID <= 0 stores no report link.
func (s *MessageStore) Insert(ctx context.Context, universeUUID string, userID int64, category, kind, title, body string, reportID int64) error {
	var ref any
	if reportID > 0 {
		ref = reportID
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (universe_id, user_id, category, kind, title, body, report_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, universeUUID, userID, category, kind, title, body, ref)
	return err
}

// List returns one page of messages newest-first.
func (s *MessageStore) List(ctx context.Context, userID int64, page, perPage int, f MessageFilter) ([]Message, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	if page < 1 {
		page = 1
	}
	where, args := f.where(userID)
	args = append(args, perPage, (page-1)*perPage)
	q := fmt.Sprintf(`
		SELECT id, category, kind, title, body, report_id, is_read, created_at
		FROM messages WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Category, &m.Kind, &m.Title, &m.Body, &m.ReportID, &m.IsRead, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Count returns the number of messages matching the filter (for pagination).
func (s *MessageStore) Count(ctx context.Context, userID int64, f MessageFilter) (int, error) {
	where, args := f.where(userID)
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE `+where, args...).Scan(&n)
	return n, err
}

// UnreadCount returns the active unread badge count.
func (s *MessageStore) UnreadCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE user_id = $1 AND NOT is_read AND archived_at IS NULL`,
		userID).Scan(&n)
	return n, err
}

// MarkRead marks one owned message as read.
func (s *MessageStore) MarkRead(ctx context.Context, userID, messageID int64) error {
	return s.execOwned(ctx,
		`UPDATE messages SET is_read = TRUE WHERE id = $1 AND user_id = $2`, messageID, userID)
}

// Delete removes one owned message.
func (s *MessageStore) Delete(ctx context.Context, userID, messageID int64) error {
	return s.execOwned(ctx,
		`DELETE FROM messages WHERE id = $1 AND user_id = $2`, messageID, userID)
}

// Archive moves one owned message to the archive.
func (s *MessageStore) Archive(ctx context.Context, userID, messageID int64) error {
	return s.execOwned(ctx,
		`UPDATE messages SET archived_at = NOW(), is_read = TRUE WHERE id = $1 AND user_id = $2`, messageID, userID)
}

func (s *MessageStore) execOwned(ctx context.Context, q string, messageID, userID int64) error {
	res, err := s.db.ExecContext(ctx, q, messageID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
