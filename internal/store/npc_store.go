package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/lib/pq"

	"gwx/internal/game"
)

// NPCStore owns the accounts the game plays itself. An NPC is a normal `users`
// row flagged `is_npc`; the scheduler drives it through the very same stores a
// human player's requests go through (QueueStore, FleetStore).
type NPCStore struct {
	db  *sql.DB
	rng *rand.Rand
}

// NewNPCStore constructs an NPCStore.
func NewNPCStore(db *sql.DB) *NPCStore {
	return &NPCStore{db: db, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// NPC is one due NPC: its account id, the universe code and its profile.
type NPC struct {
	UserID   int64
	Universe string
	Profile  game.NPCProfile
}

// npcLease is how long a claimed NPC is parked while the brain is working. If
// the process dies mid-decision the lease expires and the NPC is retried.
const npcLease = 2 * time.Minute

// sentinelPassword is not a valid PBKDF2 hash, and login is additionally
// blocked on `is_npc`, so an NPC can never authenticate.
const sentinelPassword = "!npc-no-login"

// Create spawns one NPC account with a homeworld. Usernames are retried until a
// free slot/name/email is found.
func (s *NPCStore) Create(ctx context.Context, universeCode string, profile game.NPCProfile) (userID, planetID int64, err error) {
	profile = profile.Normalized()
	raw, err := json.Marshal(profile)
	if err != nil {
		return 0, 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	var (
		universeUUID            string
		maxGalaxies, maxSystems int
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

	for attempt := 0; attempt < 20; attempt++ {
		username := game.RandomNPCName(s.rng)
		email := "npc-" + itoa64(s.rng.Int63()) + "@npc.local"

		err = tx.QueryRowContext(ctx, `
			INSERT INTO users (
				universe_id, username, email, password_hash, is_npc,
				npc_profile, npc_next_decision_at
			) VALUES ($1, $2, $3, $4, TRUE, $5, NOW())
			RETURNING id
		`, universeUUID, username, email, sentinelPassword, raw).Scan(&userID)
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				continue // name or email taken, roll another
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
	return 0, 0, ErrUserExists
}

// ClaimDue leases up to limit due NPCs and returns them. Each claim is its own
// transaction using FOR UPDATE SKIP LOCKED, so concurrent schedulers never
// process the same NPC.
func (s *NPCStore) ClaimDue(ctx context.Context, limit int) ([]NPC, error) {
	var out []NPC
	for len(out) < limit {
		n, ok, err := s.claimOne(ctx)
		if err != nil {
			return out, err
		}
		if !ok {
			break
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *NPCStore) claimOne(ctx context.Context) (NPC, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return NPC{}, false, err
	}
	defer tx.Rollback()

	var (
		n   NPC
		raw []byte
	)
	err = tx.QueryRowContext(ctx, `
		SELECT u.id, un.code_name, u.npc_profile
		FROM users u
		JOIN universes un ON un.id = u.universe_id
		WHERE u.is_npc AND (u.npc_next_decision_at IS NULL OR u.npc_next_decision_at <= NOW())
		ORDER BY u.npc_next_decision_at NULLS FIRST
		FOR UPDATE OF u SKIP LOCKED
		LIMIT 1
	`).Scan(&n.UserID, &n.Universe, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return NPC{}, false, nil
	}
	if err != nil {
		return NPC{}, false, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &n.Profile)
	}
	n.Profile = n.Profile.Normalized()

	// Park the NPC for the lease window while the brain works.
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET npc_next_decision_at = NOW() + make_interval(secs => $2) WHERE id = $1
	`, n.UserID, npcLease.Seconds()); err != nil {
		return NPC{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return NPC{}, false, err
	}
	return n, true, nil
}

// Schedule sets when the NPC should next be allowed to decide.
func (s *NPCStore) Schedule(ctx context.Context, userID int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET npc_next_decision_at = $2 WHERE id = $1`, userID, at)
	return err
}

// Count returns how many NPCs live in a universe.
func (s *NPCStore) Count(ctx context.Context, universeCode string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users
		WHERE is_npc AND universe_id = (SELECT id FROM universes WHERE code_name = $1)
	`, universeCode).Scan(&n)
	return n, err
}

// EnsureCount seeds random NPCs until the universe holds at least n of them.
// Safe to call on every server start; it only ever adds.
func (s *NPCStore) EnsureCount(ctx context.Context, universeCode string, n int) (created int, err error) {
	if n <= 0 {
		return 0, nil
	}
	have, err := s.Count(ctx, universeCode)
	if err != nil {
		return 0, err
	}
	for have+created < n {
		if _, _, err := s.Create(ctx, universeCode, game.RandomNPCProfile(s.rng)); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// RandomEmptySlot returns a random unoccupied coordinate in the universe, for
// planting a colony. It samples a bounded number of coordinates and reports
// ok=false if it cannot find a free one (a full universe).
func (s *NPCStore) RandomEmptySlot(ctx context.Context, universeCode string) (g, sys, pos int, ok bool, err error) {
	var (
		uuid                    string
		maxGalaxies, maxSystems int
	)
	err = s.db.QueryRowContext(ctx, `
		SELECT id::text, max_galaxies, max_systems FROM universes WHERE code_name = $1
	`, universeCode).Scan(&uuid, &maxGalaxies, &maxSystems)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, 0, false, ErrNotFound
	}
	if err != nil {
		return 0, 0, 0, false, err
	}
	if maxGalaxies < 1 {
		maxGalaxies = 1
	}
	if maxSystems < 1 {
		maxSystems = 1
	}
	for attempt := 0; attempt < 40; attempt++ {
		g = 1 + s.rng.Intn(maxGalaxies)
		sys = 1 + s.rng.Intn(maxSystems)
		pos = 1 + s.rng.Intn(15)
		var exists bool
		if err := s.db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM planets
				WHERE universe_id = $1 AND galaxy = $2 AND system = $3 AND position = $4
			)
		`, uuid, g, sys, pos).Scan(&exists); err != nil {
			return 0, 0, 0, false, err
		}
		if !exists {
			return g, sys, pos, true, nil
		}
	}
	return 0, 0, 0, false, nil
}

// RandomTarget returns a random existing coordinate not owned by excludeUser,
// used for a sloppy "fleet in the air" save. It reports ok=false when the
// universe holds no other planet to send a fleet toward.
func (s *NPCStore) RandomTarget(ctx context.Context, universeCode string, excludeUser int64) (g, sys, pos int, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `
		SELECT galaxy, system, position FROM planets
		WHERE universe_id = (SELECT id FROM universes WHERE code_name = $1)
		  AND (user_id IS NULL OR user_id <> $2)
		ORDER BY random() LIMIT 1
	`, universeCode, excludeUser).Scan(&g, &sys, &pos)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, 0, false, err
	}
	return g, sys, pos, true, nil
}

// itoa64 renders a non-negative int64 without importing strconv (kept local so
// the store package's tiny helper surface stays consistent).
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
