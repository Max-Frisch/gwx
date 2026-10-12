package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// AllianceStore owns alliances, membership, applications and the shared board.
// Rank/rights checks live here (transactional) rather than only in handlers.
type AllianceStore struct {
	db *sql.DB
}

// NewAllianceStore constructs an AllianceStore.
func NewAllianceStore(db *sql.DB) *AllianceStore { return &AllianceStore{db: db} }

// AllianceRank is the fixed per-member rank. The founder holds RankLeader.
type AllianceRank string

const (
	RankMember  AllianceRank = "MEMBER"
	RankOfficer AllianceRank = "OFFICER"
	RankLeader  AllianceRank = "LEADER"
)

// ParseRank normalises a stored rank string, defaulting to MEMBER.
func ParseRank(s string) AllianceRank {
	switch AllianceRank(s) {
	case RankMember, RankOfficer, RankLeader:
		return AllianceRank(s)
	default:
		return RankMember
	}
}

// CanManageApplications reports whether the rank may accept/reject applications.
func (r AllianceRank) CanManageApplications() bool { return r == RankOfficer || r == RankLeader }

// CanModerateBoard reports whether the rank may delete any board post.
func (r AllianceRank) CanModerateBoard() bool { return r == RankOfficer || r == RankLeader }

// CanManageAlliance reports whether the rank may change settings or close it.
func (r AllianceRank) CanManageAlliance() bool { return r == RankLeader }

// CanPost reports whether the rank may post to the board (any member may).
func (r AllianceRank) CanPost() bool {
	return r == RankMember || r == RankOfficer || r == RankLeader
}

// Membership is the caller's alliance and rank.
type Membership struct {
	AllianceID int64
	Rank       AllianceRank
}

// Create founds an alliance and installs the founder as LEADER in one
// transaction. The founder must not already belong to an alliance.
func (s *AllianceStore) Create(ctx context.Context, universeCode string, founderID int64, tag, name, description string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var universeUUID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM universes WHERE code_name = $1`, universeCode).Scan(&universeUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}

	if _, err := membershipTx(ctx, tx, founderID); err == nil {
		return 0, ErrAlreadyMember
	} else if !errors.Is(err, ErrNotFound) {
		return 0, err
	}

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO alliances (universe_id, tag, name, description, founder_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, universeUUID, tag, name, description, founderID).Scan(&id)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return 0, ErrAllianceTagTaken
		}
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO alliance_members (alliance_id, user_id, rank)
		VALUES ($1, $2, 'LEADER')
	`, id, founderID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM alliance_applications WHERE user_id = $1`, founderID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// Get returns one alliance with its founder's display name.
func (s *AllianceStore) Get(ctx context.Context, id int64) (*Alliance, error) {
	return scanAlliance(s.db.QueryRowContext(ctx, `
		SELECT a.id, a.tag, a.name, a.description, a.founder_id, COALESCE(u.username, ''),
		       a.is_open, a.min_points, a.created_at
		FROM alliances a
		LEFT JOIN users u ON u.id = a.founder_id
		WHERE a.id = $1
	`, id))
}

// GetByTag resolves an alliance by its universe-unique tag.
func (s *AllianceStore) GetByTag(ctx context.Context, universeCode, tag string) (*Alliance, error) {
	return scanAlliance(s.db.QueryRowContext(ctx, `
		SELECT a.id, a.tag, a.name, a.description, a.founder_id, COALESCE(u.username, ''),
		       a.is_open, a.min_points, a.created_at
		FROM alliances a
		LEFT JOIN users u ON u.id = a.founder_id
		WHERE a.tag = $1 AND a.universe_id = (SELECT id FROM universes WHERE code_name = $2)
	`, tag, universeCode))
}

func scanAlliance(row *sql.Row) (*Alliance, error) {
	var a Alliance
	err := row.Scan(&a.ID, &a.Tag, &a.Name, &a.Description, &a.FounderID, &a.FounderName,
		&a.IsOpen, &a.MinPoints, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// List returns one page of alliances, optionally filtered by a search term on
// tag/name, plus the total row count for pagination.
func (s *AllianceStore) List(ctx context.Context, universeCode, search string, page, perPage int) ([]Alliance, int, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	if page < 1 {
		page = 1
	}
	where := "a.universe_id = (SELECT id FROM universes WHERE code_name = $1)"
	args := []any{universeCode}
	if term := strings.TrimSpace(search); term != "" {
		args = append(args, "%"+term+"%")
		where += fmt.Sprintf(" AND (a.name ILIKE $%d OR a.tag ILIKE $%d)", len(args), len(args))
	}

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM alliances a WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT a.id, a.tag, a.name, a.description, a.founder_id, COALESCE(u.username, ''),
		       a.is_open, a.min_points, a.created_at
		FROM alliances a
		LEFT JOIN users u ON u.id = a.founder_id
		WHERE %s
		ORDER BY a.name
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Alliance
	for rows.Next() {
		var a Alliance
		if err := rows.Scan(&a.ID, &a.Tag, &a.Name, &a.Description, &a.FounderID, &a.FounderName,
			&a.IsOpen, &a.MinPoints, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// TagsByUser returns every member's alliance tag in the universe, for showing
// [TAG] next to usernames in galaxy/highscore/search.
func (s *AllianceStore) TagsByUser(ctx context.Context, universeCode string) (map[int64]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT am.user_id, a.tag
		FROM alliance_members am
		JOIN alliances a ON a.id = am.alliance_id
		WHERE a.universe_id = (SELECT id FROM universes WHERE code_name = $1)
	`, universeCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var uid int64
		var tag string
		if err := rows.Scan(&uid, &tag); err != nil {
			return nil, err
		}
		out[uid] = tag
	}
	return out, rows.Err()
}

// Membership returns the caller's alliance membership, or ErrNotFound.
func (s *AllianceStore) Membership(ctx context.Context, userID int64) (Membership, error) {
	return membershipTx(ctx, s.db, userID)
}

type queryer interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

func membershipTx(ctx context.Context, q queryer, userID int64) (Membership, error) {
	var m Membership
	var rank string
	err := q.QueryRowContext(ctx,
		`SELECT alliance_id, rank FROM alliance_members WHERE user_id = $1`, userID).Scan(&m.AllianceID, &rank)
	if errors.Is(err, sql.ErrNoRows) {
		return Membership{}, ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	m.Rank = ParseRank(rank)
	return m, nil
}

// Members lists an alliance's members, leader first then by join date.
func (s *AllianceStore) Members(ctx context.Context, allianceID int64) ([]AllianceMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT am.user_id, COALESCE(u.username, ''), am.rank, am.joined_at, (am.rank = 'LEADER')
		FROM alliance_members am
		LEFT JOIN users u ON u.id = am.user_id
		WHERE am.alliance_id = $1
		ORDER BY (am.rank = 'LEADER') DESC, am.joined_at
	`, allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AllianceMember
	for rows.Next() {
		var m AllianceMember
		var rank string
		if err := rows.Scan(&m.UserID, &m.Username, &rank, &m.JoinedAt, &m.IsFounder); err != nil {
			return nil, err
		}
		m.Rank = rank
		out = append(out, m)
	}
	return out, rows.Err()
}

// Join adds the caller directly to an open alliance.
func (s *AllianceStore) Join(ctx context.Context, userID, allianceID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var open bool
	if err := tx.QueryRowContext(ctx,
		`SELECT is_open FROM alliances WHERE id = $1 FOR UPDATE`, allianceID).Scan(&open); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if !open {
		return ErrAllianceClosed
	}
	if _, err := membershipTx(ctx, tx, userID); err == nil {
		return ErrAlreadyMember
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO alliance_members (alliance_id, user_id, rank) VALUES ($1, $2, 'MEMBER')
	`, allianceID, userID); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrAlreadyMember
		}
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM alliance_applications WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// Apply files (or refreshes) a pending application to a closed alliance.
func (s *AllianceStore) Apply(ctx context.Context, userID, allianceID int64, message string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var open bool
	if err := tx.QueryRowContext(ctx,
		`SELECT is_open FROM alliances WHERE id = $1 FOR UPDATE`, allianceID).Scan(&open); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err := membershipTx(ctx, tx, userID); err == nil {
		return ErrAlreadyMember
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	if open {
		return ErrAllianceClosed
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO alliance_applications (alliance_id, user_id, message, status)
		VALUES ($1, $2, $3, 'PENDING')
		ON CONFLICT (alliance_id, user_id) DO UPDATE
		SET message = EXCLUDED.message, status = 'PENDING', created_at = NOW()
	`, allianceID, userID, message); err != nil {
		return err
	}
	return tx.Commit()
}

// Withdraw marks the caller's own pending application withdrawn.
func (s *AllianceStore) Withdraw(ctx context.Context, userID, applicationID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE alliance_applications SET status = 'WITHDRAWN'
		WHERE id = $1 AND user_id = $2 AND status = 'PENDING'
	`, applicationID, userID)
	if err != nil {
		return err
	}
	return requireRow(res)
}

// MyApplications lists the caller's open (pending) applications.
func (s *AllianceStore) MyApplications(ctx context.Context, userID int64) ([]AllianceApplication, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ap.id, ap.alliance_id, a.tag, a.name, ap.user_id, COALESCE(u.username, ''),
		       ap.message, ap.status, ap.created_at
		FROM alliance_applications ap
		JOIN alliances a ON a.id = ap.alliance_id
		LEFT JOIN users u ON u.id = ap.user_id
		WHERE ap.user_id = $1 AND ap.status = 'PENDING'
		ORDER BY ap.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApplications(rows)
}

// PendingApplications lists an alliance's pending applications. The caller must
// hold a rank that may manage applications.
func (s *AllianceStore) PendingApplications(ctx context.Context, actorID, allianceID int64) ([]AllianceApplication, error) {
	rank, err := s.actorRank(ctx, allianceID, actorID)
	if err != nil {
		return nil, err
	}
	if !rank.CanManageApplications() {
		return nil, ErrForbidden
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ap.id, ap.alliance_id, a.tag, a.name, ap.user_id, COALESCE(u.username, ''),
		       ap.message, ap.status, ap.created_at
		FROM alliance_applications ap
		JOIN alliances a ON a.id = ap.alliance_id
		LEFT JOIN users u ON u.id = ap.user_id
		WHERE ap.alliance_id = $1 AND ap.status = 'PENDING'
		ORDER BY ap.created_at
	`, allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanApplications(rows)
}

func scanApplications(rows *sql.Rows) ([]AllianceApplication, error) {
	var out []AllianceApplication
	for rows.Next() {
		var a AllianceApplication
		if err := rows.Scan(&a.ID, &a.AllianceID, &a.Tag, &a.Name, &a.UserID, &a.Username,
			&a.Message, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Decide accepts or rejects an application. Accepting installs the applicant as
// a MEMBER. The caller must hold a rank that may manage applications.
func (s *AllianceStore) Decide(ctx context.Context, actorID, applicationID int64, accept bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var allianceID, applicantID int64
	var status string
	err = tx.QueryRowContext(ctx,
		`SELECT alliance_id, user_id, status FROM alliance_applications WHERE id = $1 FOR UPDATE`,
		applicationID).Scan(&allianceID, &applicantID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "PENDING" {
		return ErrNotFound
	}

	rank, err := actorRankTx(ctx, tx, allianceID, actorID)
	if err != nil {
		return err
	}
	if !rank.CanManageApplications() {
		return ErrForbidden
	}

	newStatus := "REJECTED"
	if accept {
		newStatus = "ACCEPTED"
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO alliance_members (alliance_id, user_id, rank)
			VALUES ($1, $2, 'MEMBER')
			ON CONFLICT (user_id) DO NOTHING
		`, allianceID, applicantID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE alliance_applications SET status = $2 WHERE id = $1`, applicationID, newStatus); err != nil {
		return err
	}
	return tx.Commit()
}

// Leave removes the caller. A sole leader dissolves the alliance; otherwise
// leadership transfers to the longest-serving remaining member.
func (s *AllianceStore) Leave(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var allianceID int64
	var rank string
	err = tx.QueryRowContext(ctx,
		`SELECT alliance_id, rank FROM alliance_members WHERE user_id = $1 FOR UPDATE`,
		userID).Scan(&allianceID, &rank)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotMember
	}
	if err != nil {
		return err
	}

	if ParseRank(rank) == RankLeader {
		var others int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM alliance_members WHERE alliance_id = $1 AND user_id <> $2`,
			allianceID, userID).Scan(&others); err != nil {
			return err
		}
		if others == 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM alliances WHERE id = $1`, allianceID); err != nil {
				return err
			}
			return tx.Commit()
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE alliance_members SET rank = 'LEADER'
			WHERE alliance_id = $1 AND user_id = (
				SELECT user_id FROM alliance_members
				WHERE alliance_id = $1 AND user_id <> $2
				ORDER BY joined_at LIMIT 1
			)
		`, allianceID, userID); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM alliance_members WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateSettings changes description/openness/min-points (leader only).
func (s *AllianceStore) UpdateSettings(ctx context.Context, actorID, allianceID int64, description string, isOpen bool, minPoints int) error {
	rank, err := s.actorRank(ctx, allianceID, actorID)
	if err != nil {
		return err
	}
	if !rank.CanManageAlliance() {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE alliances SET description = $2, is_open = $3, min_points = $4 WHERE id = $1
	`, allianceID, description, isOpen, minPoints)
	return err
}

// SetRank promotes or demotes a member (leader only; cannot demote the leader).
func (s *AllianceStore) SetRank(ctx context.Context, actorID, allianceID, targetUserID int64, rank AllianceRank) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	actor, err := actorRankTx(ctx, tx, allianceID, actorID)
	if err != nil {
		return err
	}
	if !actor.CanManageAlliance() {
		return ErrForbidden
	}
	var targetRank string
	err = tx.QueryRowContext(ctx,
		`SELECT rank FROM alliance_members WHERE alliance_id = $1 AND user_id = $2 FOR UPDATE`,
		allianceID, targetUserID).Scan(&targetRank)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotMember
	}
	if err != nil {
		return err
	}
	if ParseRank(targetRank) == RankLeader || rank == RankLeader {
		return ErrForbidden
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE alliance_members SET rank = $3 WHERE alliance_id = $1 AND user_id = $2
	`, allianceID, targetUserID, string(rank)); err != nil {
		return err
	}
	return tx.Commit()
}

// Close dissolves an alliance (leader only).
func (s *AllianceStore) Close(ctx context.Context, actorID, allianceID int64) error {
	rank, err := s.actorRank(ctx, allianceID, actorID)
	if err != nil {
		return err
	}
	if !rank.CanManageAlliance() {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM alliances WHERE id = $1`, allianceID)
	return err
}

// Posts lists an alliance's board, pinned first then newest first.
func (s *AllianceStore) Posts(ctx context.Context, allianceID int64) ([]AlliancePost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.author_id, COALESCE(u.username, ''), p.title, p.body, p.is_pinned, p.created_at
		FROM alliance_posts p
		LEFT JOIN users u ON u.id = p.author_id
		WHERE p.alliance_id = $1
		ORDER BY p.is_pinned DESC, p.created_at DESC
	`, allianceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlliancePost
	for rows.Next() {
		var p AlliancePost
		if err := rows.Scan(&p.ID, &p.AuthorID, &p.AuthorName, &p.Title, &p.Body, &p.IsPinned, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AddPost writes a board post. Any member may post.
func (s *AllianceStore) AddPost(ctx context.Context, actorID, allianceID int64, title, body string, pinned bool) error {
	rank, err := s.actorRank(ctx, allianceID, actorID)
	if err != nil {
		return err
	}
	if !rank.CanPost() {
		return ErrForbidden
	}
	if pinned && !rank.CanModerateBoard() {
		pinned = false
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO alliance_posts (alliance_id, author_id, title, body, is_pinned)
		VALUES ($1, $2, $3, $4, $5)
	`, allianceID, actorID, title, body, pinned)
	return err
}

// DeletePost removes a board post: the author or any board moderator may.
func (s *AllianceStore) DeletePost(ctx context.Context, actorID, postID int64) error {
	var allianceID, authorID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT alliance_id, author_id FROM alliance_posts WHERE id = $1`, postID).Scan(&allianceID, &authorID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if authorID != actorID {
		rank, err := s.actorRank(ctx, allianceID, actorID)
		if err != nil {
			return err
		}
		if !rank.CanModerateBoard() {
			return ErrForbidden
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM alliance_posts WHERE id = $1`, postID)
	if err != nil {
		return err
	}
	return requireRow(res)
}

func (s *AllianceStore) actorRank(ctx context.Context, allianceID, userID int64) (AllianceRank, error) {
	return actorRankTx(ctx, s.db, allianceID, userID)
}

func actorRankTx(ctx context.Context, q queryer, allianceID, userID int64) (AllianceRank, error) {
	var rank string
	err := q.QueryRowContext(ctx,
		`SELECT rank FROM alliance_members WHERE alliance_id = $1 AND user_id = $2`,
		allianceID, userID).Scan(&rank)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotMember
	}
	if err != nil {
		return "", err
	}
	return ParseRank(rank), nil
}

func requireRow(res sql.Result) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
