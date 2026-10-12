package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"gwx/internal/game"
)

// QueueStore starts and cancels construction, research and unit production.
type QueueStore struct {
	db *sql.DB
}

// NewQueueStore constructs a QueueStore.
func NewQueueStore(db *sql.DB) *QueueStore { return &QueueStore{db: db} }

type universeInfo struct {
	uuid      string
	gameSpeed float64
}

func (s *QueueStore) universeTx(ctx context.Context, tx *sql.Tx, code string) (universeInfo, error) {
	var u universeInfo
	err := tx.QueryRowContext(ctx,
		`SELECT id::text, game_speed FROM universes WHERE code_name = $1`, code).
		Scan(&u.uuid, &u.gameSpeed)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func accrueTx(ctx context.Context, tx *sql.Tx, planetID int64) (game.Resources, error) {
	var m, l, w, h float64
	err := tx.QueryRowContext(ctx,
		`SELECT out_metal, out_lutinum, out_water, out_hydrogen FROM update_planet_resources($1)`,
		planetID).Scan(&m, &l, &w, &h)
	return game.Resources{Metal: m, Lutinum: l, Water: w, Hydrogen: h}, err
}

func deductTx(ctx context.Context, tx *sql.Tx, planetID int64, c game.Resources) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE planets SET
			metal = metal - $2, lutinum = lutinum - $3,
			water = water - $4, hydrogen = hydrogen - $5
		WHERE id = $1
	`, planetID, c.Metal, c.Lutinum, c.Water, c.Hydrogen)
	return err
}

func creditTx(ctx context.Context, tx *sql.Tx, planetID int64, c game.Resources) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE planets SET
			metal = LEAST(metal_cap, metal + $2),
			lutinum = LEAST(lutinum_cap, lutinum + $3),
			water = LEAST(water_cap, water + $4),
			hydrogen = LEAST(hydrogen_cap, hydrogen + $5)
		WHERE id = $1
	`, planetID, c.Metal, c.Lutinum, c.Water, c.Hydrogen)
	return err
}

func affordable(have, cost game.Resources) bool {
	return have.Metal >= cost.Metal && have.Lutinum >= cost.Lutinum &&
		have.Water >= cost.Water && have.Hydrogen >= cost.Hydrogen
}

func loadStructuresTx(ctx context.Context, tx *sql.Tx, planetID int64) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT code, amount FROM planet_items WHERE planet_id = $1 AND kind = 'building'`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

func loadTechsTx(ctx context.Context, tx *sql.Tx, userID int64) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT code, level FROM user_techs WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

func mapQueueBusy(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrQueueBusy
	}
	return err
}

// StartBuilding queues the next level of a building.
func (s *QueueStore) StartBuilding(ctx context.Context, universeCode string, userID, planetID int64, code string) error {
	def, ok := game.Get(code)
	if !ok || def.Kind != game.KindBuilding {
		return ErrUnknownCode
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	u, err := s.universeTx(ctx, tx, universeCode)
	if err != nil {
		return err
	}
	if err := ensureQueueingAllowedTx(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := accrueTx(ctx, tx, planetID); err != nil {
		return err
	}

	structs, err := loadStructuresTx(ctx, tx, planetID)
	if err != nil {
		return err
	}
	techs, err := loadTechsTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !game.PrereqsMet(def, structs, techs) {
		return ErrPrerequisites
	}
	cur := structs[code]
	next := cur + 1
	cost := def.LevelCost(next)

	var fieldsMax, fieldsUsed int
	if err := tx.QueryRowContext(ctx,
		`SELECT fields_max, fields_used FROM planets WHERE id = $1`, planetID).Scan(&fieldsMax, &fieldsUsed); err != nil {
		return err
	}
	if cur == 0 && fieldsUsed >= fieldsMax {
		return ErrNoFields
	}

	have, err := accrueTx(ctx, tx, planetID)
	if err != nil {
		return err
	}
	if !affordable(have, cost) {
		return ErrInsufficientResources
	}
	if err := deductTx(ctx, tx, planetID, cost); err != nil {
		return err
	}

	dur := def.LevelTime(next) * game.BuildTimeMultiplier(structs["command_center"]) / u.gameSpeed
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO queues (universe_id, user_id, planet_id, kind, code, target_level, start_at, finish_at)
		VALUES ($1, $2, $3, 'building', $4, $5, NOW(), NOW() + ($6 * INTERVAL '1 second'))
	`, u.uuid, userID, planetID, code, next, dur); err != nil {
		return mapQueueBusy(err)
	}
	return tx.Commit()
}

// StartResearch queues the next level of a technology.
func (s *QueueStore) StartResearch(ctx context.Context, universeCode string, userID, planetID int64, code string) error {
	def, ok := game.Get(code)
	if !ok || def.Kind != game.KindResearch {
		return ErrUnknownCode
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	u, err := s.universeTx(ctx, tx, universeCode)
	if err != nil {
		return err
	}
	if err := ensureQueueingAllowedTx(ctx, tx, userID); err != nil {
		return err
	}
	structs, err := loadStructuresTx(ctx, tx, planetID)
	if err != nil {
		return err
	}
	techs, err := loadTechsTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !game.PrereqsMet(def, structs, techs) {
		return ErrPrerequisites
	}
	next := techs[code] + 1
	cost := def.LevelCost(next)

	have, err := accrueTx(ctx, tx, planetID)
	if err != nil {
		return err
	}
	if !affordable(have, cost) {
		return ErrInsufficientResources
	}
	if err := deductTx(ctx, tx, planetID, cost); err != nil {
		return err
	}

	dur := def.LevelTime(next) / u.gameSpeed
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO queues (universe_id, user_id, planet_id, kind, code, target_level, start_at, finish_at)
		VALUES ($1, $2, $3, 'research', $4, $5, NOW(), NOW() + ($6 * INTERVAL '1 second'))
	`, u.uuid, userID, planetID, code, next, dur); err != nil {
		return mapQueueBusy(err)
	}
	return tx.Commit()
}

// StartUnits queues a batch of ships or defenses.
func (s *QueueStore) StartUnits(ctx context.Context, universeCode string, userID, planetID int64, kind, code string, qty int64) error {
	if qty <= 0 {
		return ErrInvalidQuantity
	}
	def, ok := game.Get(code)
	if !ok || def.Kind != game.Kind(kind) || (def.Kind != game.KindShip && def.Kind != game.KindDefense) {
		return ErrUnknownCode
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	u, err := s.universeTx(ctx, tx, universeCode)
	if err != nil {
		return err
	}
	if err := ensureQueueingAllowedTx(ctx, tx, userID); err != nil {
		return err
	}
	structs, err := loadStructuresTx(ctx, tx, planetID)
	if err != nil {
		return err
	}
	techs, err := loadTechsTx(ctx, tx, userID)
	if err != nil {
		return err
	}
	if !game.PrereqsMet(def, structs, techs) {
		return ErrPrerequisites
	}
	cost := game.Resources{
		Metal:    def.Cost.Metal * float64(qty),
		Lutinum:  def.Cost.Lutinum * float64(qty),
		Water:    def.Cost.Water * float64(qty),
		Hydrogen: def.Cost.Hydrogen * float64(qty),
	}

	have, err := accrueTx(ctx, tx, planetID)
	if err != nil {
		return err
	}
	if !affordable(have, cost) {
		return ErrInsufficientResources
	}
	if err := deductTx(ctx, tx, planetID, cost); err != nil {
		return err
	}

	factoryLevel := structs[game.UnitFactoryCode(def.Kind)]
	dur := def.TimeSecs * game.UnitTimeMultiplier(factoryLevel) * float64(qty) / u.gameSpeed
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO queues (universe_id, user_id, planet_id, kind, code, quantity, start_at, finish_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW() + ($7 * INTERVAL '1 second'))
	`, u.uuid, userID, planetID, kind, code, qty, dur); err != nil {
		return mapQueueBusy(err)
	}
	return tx.Commit()
}

// Active returns in-progress queues for a planet and account.
func (s *QueueStore) Active(ctx context.Context, planetID, userID int64) ([]Queue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, planet_id, kind, code, target_level, quantity, quantity_done, start_at, finish_at, status
		FROM queues
		WHERE status = 'IN_PROGRESS'
		  AND ((kind IN ('building','ship','defense') AND planet_id = $1) OR (kind = 'research' AND user_id = $2))
		ORDER BY finish_at
	`, planetID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Queue
	for rows.Next() {
		var q Queue
		if err := rows.Scan(&q.ID, &q.PlanetID, &q.Kind, &q.Code, &q.TargetLevel, &q.Quantity,
			&q.QuantityDone, &q.StartAt, &q.FinishAt, &q.Status); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ActiveForUser returns every in-progress queue for an account, across all of
// its planets. Used by the events timeline.
func (s *QueueStore) ActiveForUser(ctx context.Context, userID int64) ([]Queue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, planet_id, kind, code, target_level, quantity, quantity_done, start_at, finish_at, status
		FROM queues
		WHERE status = 'IN_PROGRESS' AND user_id = $1
		ORDER BY finish_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Queue
	for rows.Next() {
		var q Queue
		if err := rows.Scan(&q.ID, &q.PlanetID, &q.Kind, &q.Code, &q.TargetLevel, &q.Quantity,
			&q.QuantityDone, &q.StartAt, &q.FinishAt, &q.Status); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// Cancel cancels a queued order and refunds its cost.
func (s *QueueStore) Cancel(ctx context.Context, queueID, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		planetID    int64
		kind, code  string
		targetLevel sql.NullInt64
		quantity    sql.NullInt64
		status      string
		owner       int64
	)
	err = tx.QueryRowContext(ctx, `
		SELECT planet_id, kind, code, target_level, quantity, status, user_id
		FROM queues WHERE id = $1 FOR UPDATE
	`, queueID).Scan(&planetID, &kind, &code, &targetLevel, &quantity, &status, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if owner != userID {
		return ErrNotOwner
	}
	if status != "IN_PROGRESS" {
		return ErrNotFound
	}

	def, ok := game.Get(code)
	if !ok {
		return ErrUnknownCode
	}
	var refund game.Resources
	switch kind {
	case "building", "research":
		refund = def.LevelCost(int(targetLevel.Int64))
	case "ship", "defense":
		refund = game.Resources{
			Metal:    def.Cost.Metal * float64(quantity.Int64),
			Lutinum:  def.Cost.Lutinum * float64(quantity.Int64),
			Water:    def.Cost.Water * float64(quantity.Int64),
			Hydrogen: def.Cost.Hydrogen * float64(quantity.Int64),
		}
	}

	if err := creditTx(ctx, tx, planetID, refund); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE queues SET status = 'CANCELLED' WHERE id = $1`, queueID); err != nil {
		return err
	}
	return tx.Commit()
}
