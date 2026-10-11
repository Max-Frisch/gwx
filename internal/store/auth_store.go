package store

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"time"

	"github.com/lib/pq"
)

// AuthStore handles users and opaque sessions.
type AuthStore struct {
	db *sql.DB
}

// NewAuthStore constructs an AuthStore.
func NewAuthStore(db *sql.DB) *AuthStore { return &AuthStore{db: db} }

const (
	homeworldFields      = 120
	homeworldStartMetal  = 1000
	homeworldStartLut    = 1000
	homeworldStartWater  = 500
	homeworldMinPosition = 4
	homeworldMaxPosition = 12
	homeworldSlotTries   = 50
)

// CreateUser registers a user in the universe and provisions a starter
// homeworld with a level-1 command center.
func (s *AuthStore) CreateUser(ctx context.Context, universeCode, username, email, passwordHash string) (userID, planetID int64, err error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	var (
		universeUUID string
		maxGalaxies  int
		maxSystems   int
	)
	err = tx.QueryRowContext(ctx, `
		SELECT id, max_galaxies, max_systems FROM universes WHERE code_name = $1
	`, universeCode).Scan(&universeUUID, &maxGalaxies, &maxSystems)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	if err != nil {
		return 0, 0, err
	}
	if maxGalaxies < 1 {
		maxGalaxies = 1
	}
	if maxSystems < 1 {
		maxSystems = 1
	}

	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (universe_id, username, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, universeUUID, username, email, passwordHash).Scan(&userID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return 0, 0, ErrUserExists
		}
		return 0, 0, err
	}

	planetID, err = createHomeworld(ctx, tx, universeUUID, userID, username, maxGalaxies, maxSystems)
	if err != nil {
		return 0, 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO planet_items (planet_id, kind, code, amount)
		VALUES ($1, 'building', 'command_center', 1)
	`, planetID); err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return userID, planetID, nil
}

func createHomeworld(ctx context.Context, tx *sql.Tx, universeUUID string, userID int64, username string, maxGalaxies, maxSystems int) (int64, error) {
	name := username + "'s Homeworld"
	if len(name) > 64 {
		name = name[:64]
	}
	for attempt := 0; attempt < homeworldSlotTries; attempt++ {
		g := 1 + rand.Intn(maxGalaxies)
		sys := 1 + rand.Intn(maxSystems)
		p := homeworldMinPosition + rand.Intn(homeworldMaxPosition-homeworldMinPosition+1)

		var planetID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO planets (
				universe_id, user_id, name, galaxy, system, position, is_homeworld,
				diameter_km, fields_max, fields_used,
				metal, lutinum, water, hydrogen,
				metal_cap, lutinum_cap, water_cap, hydrogen_cap,
				metal_prod, lutinum_prod, water_prod, hydrogen_prod,
				energy_prod, protected_metal, protected_lutinum, protected_water, protected_hydrogen
			) VALUES (
				$1, $2, $3, $4, $5, $6, TRUE,
				12800, $7, 1,
				$8, $9, $10, 0,
				20000, 20000, 20000, 20000,
				0, 0, 0, 0,
				30, 4000, 4000, 4000, 4000
			)
			ON CONFLICT (universe_id, galaxy, system, position) DO NOTHING
			RETURNING id
		`, universeUUID, userID, name, g, sys, p, homeworldFields,
			homeworldStartMetal, homeworldStartLut, homeworldStartWater).Scan(&planetID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		return planetID, nil
	}
	return 0, ErrNotFound
}

// UserAuth returns the id and password hash for a login (username OR email).
func (s *AuthStore) UserAuth(ctx context.Context, login string) (int64, string, error) {
	var id int64
	var hash string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, password_hash FROM users WHERE username = $1 OR email = $1 LIMIT 1
	`, login).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		return 0, "", ErrInvalidCredentials
	}
	return id, hash, err
}

// Username returns a user's display name.
func (s *AuthStore) Username(ctx context.Context, userID int64) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT username FROM users WHERE id = $1`, userID).Scan(&name)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return name, err
}

// CreateSession stores a new session token hash.
func (s *AuthStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, userAgent, ip string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
	`, userID, tokenHash, expiresAt, userAgent, ip)
	return err
}

// SessionUser resolves a non-expired session to its user id.
func (s *AuthStore) SessionUser(ctx context.Context, tokenHash string) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, `
		UPDATE sessions SET last_seen_at = NOW()
		WHERE token_hash = $1 AND expires_at > NOW()
		RETURNING user_id
	`, tokenHash).Scan(&userID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return userID, err
}

// DeleteSession revokes one session.
func (s *AuthStore) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}
