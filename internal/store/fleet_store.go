package store

import (
	"context"
	"database/sql"
	"errors"

	"gwx/internal/game"
)

// FleetStore dispatches, lists and recalls fleets.
type FleetStore struct {
	db *sql.DB
}

// NewFleetStore constructs a FleetStore.
func NewFleetStore(db *sql.DB) *FleetStore { return &FleetStore{db: db} }

// DispatchParams describes an outgoing fleet.
type DispatchParams struct {
	UniverseCode string
	UserID       int64
	OriginID     int64
	Mission      string
	TargetG      int
	TargetS      int
	TargetP      int
	SpeedPct     float64
	Ships        map[string]int64
	Cargo        game.Resources
}

// Dispatch validates, spends fuel/cargo, removes the ships from the origin and
// creates the fleet. Returns the new fleet id.
func (s *FleetStore) Dispatch(ctx context.Context, p DispatchParams) (int64, error) {
	if p.SpeedPct <= 0 || p.SpeedPct > 1 {
		p.SpeedPct = 1
	}
	if len(p.Ships) == 0 {
		return 0, ErrInsufficientShips
	}
	for code := range p.Ships {
		def, ok := game.Get(code)
		if !ok || def.Kind != game.KindShip {
			return 0, ErrUnknownCode
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var (
		universeUUID            string
		gameSpeed, fleetSpeed   float64
		maxGalaxies, maxSystems int
	)
	err = tx.QueryRowContext(ctx, `
		SELECT id::text, game_speed, fleet_speed, max_galaxies, max_systems
		FROM universes WHERE code_name = $1
	`, p.UniverseCode).Scan(&universeUUID, &gameSpeed, &fleetSpeed, &maxGalaxies, &maxSystems)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if p.TargetG < 1 || p.TargetG > maxGalaxies || p.TargetS < 1 || p.TargetS > maxSystems ||
		p.TargetP < 1 || p.TargetP > 15 {
		return 0, ErrInvalidCoordinates
	}

	var (
		ownerID    sql.NullInt64
		oG, oS, oP int
	)
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, galaxy, system, position FROM planets WHERE id = $1 FOR UPDATE
	`, p.OriginID).Scan(&ownerID, &oG, &oS, &oP)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if !ownerID.Valid || ownerID.Int64 != p.UserID {
		return 0, ErrNotOwner
	}

	techs, err := loadTechsTx(ctx, tx, p.UserID)
	if err != nil {
		return 0, err
	}

	// Resolve target.
	var targetID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM planets
		WHERE universe_id = $1 AND galaxy = $2 AND system = $3 AND position = $4
	`, universeUUID, p.TargetG, p.TargetS, p.TargetP).Scan(&targetID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	switch p.Mission {
	case "COLONIZE":
		if targetID.Valid {
			return 0, ErrTargetOccupied
		}
	case "DEPLOY":
		if !targetID.Valid {
			return 0, ErrNoTarget
		}
		var targetOwner sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT user_id FROM planets WHERE id = $1`, targetID.Int64).Scan(&targetOwner); err != nil {
			return 0, err
		}
		if !targetOwner.Valid || targetOwner.Int64 != p.UserID {
			return 0, ErrNotOwner
		}
	case "TRANSPORT", "ATTACK", "ESPIONAGE":
		if !targetID.Valid {
			return 0, ErrNoTarget
		}
	default:
		return 0, ErrUnknownCode
	}

	// Verify and remove the ships.
	if err := removeShipsTx(ctx, tx, p.OriginID, p.Ships); err != nil {
		return 0, err
	}

	// Fuel.
	distance := game.CalculateDistance(oG, oS, oP, p.TargetG, p.TargetS, p.TargetP)
	oneWayFuel := game.FuelCost(p.Ships, distance, p.SpeedPct)
	fuel := oneWayFuel
	if p.Mission != "DEPLOY" && p.Mission != "COLONIZE" {
		fuel = oneWayFuel * 2
	}

	// Cargo capacity check.
	cap := game.CargoCapacity(p.Ships, techs["cargo_tech"])
	cargoTotal := p.Cargo.Metal + p.Cargo.Lutinum + p.Cargo.Water + p.Cargo.Hydrogen
	if cargoTotal > float64(cap) {
		return 0, ErrInsufficientResources
	}

	// Spend fuel + load cargo from the origin's stored resources.
	have, err := accrueTx(ctx, tx, p.OriginID)
	if err != nil {
		return 0, err
	}
	spend := game.Resources{
		Metal:    p.Cargo.Metal,
		Lutinum:  p.Cargo.Lutinum,
		Water:    p.Cargo.Water,
		Hydrogen: p.Cargo.Hydrogen + float64(fuel),
	}
	if !affordable(have, spend) {
		return 0, ErrInsufficientFuel
	}
	if err := deductTx(ctx, tx, p.OriginID, spend); err != nil {
		return 0, err
	}

	// Flight time.
	maxSpeed := game.FleetMaxSpeed(p.Ships, techs)
	dur := game.FlightDuration(distance, maxSpeed, p.SpeedPct, fleetSpeed)

	var fleetID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO fleets (
			universe_id, user_id, mission, phase, origin_id, target_id,
			origin_galaxy, origin_system, origin_pos,
			target_galaxy, target_system, target_pos,
			start_at, arrival_at, speed_pct, hydrogen_cost,
			cargo_metal, cargo_lutinum, cargo_water, cargo_hydrogen
		) VALUES (
			$1, $2, $3, 'OUTBOUND', $4, $5,
			$6, $7, $8, $9, $10, $11,
			NOW(), NOW() + ($12 * INTERVAL '1 second'), $13, $14, $15, $16, $17, $18
		) RETURNING id
	`, universeUUID, p.UserID, p.Mission, p.OriginID, targetID,
		oG, oS, oP, p.TargetG, p.TargetS, p.TargetP,
		dur, p.SpeedPct, fuel,
		int64(p.Cargo.Metal), int64(p.Cargo.Lutinum), int64(p.Cargo.Water), int64(p.Cargo.Hydrogen),
	).Scan(&fleetID)
	if err != nil {
		return 0, err
	}
	for code, n := range p.Ships {
		if n > 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO fleet_units (fleet_id, code, count) VALUES ($1, $2, $3)`,
				fleetID, code, n); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return fleetID, nil
}

func removeShipsTx(ctx context.Context, tx *sql.Tx, planetID int64, ships map[string]int64) error {
	for code, n := range ships {
		if n <= 0 {
			continue
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE planet_items SET amount = amount - $3
			WHERE planet_id = $1 AND kind = 'ship' AND code = $2 AND amount >= $3
		`, planetID, code, n)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return ErrInsufficientShips
		}
	}
	return nil
}

const fleetCols = `
	f.id, f.mission, f.phase, f.origin_id, f.target_id,
	f.origin_galaxy, f.origin_system, f.origin_pos,
	f.target_galaxy, f.target_system, f.target_pos,
	f.start_at, f.arrival_at, f.return_at, f.speed_pct, f.hydrogen_cost,
	f.cargo_metal, f.cargo_lutinum, f.cargo_water, f.cargo_hydrogen`

func scanFleet(row interface{ Scan(...any) error }) (*Fleet, error) {
	var f Fleet
	err := row.Scan(&f.ID, &f.Mission, &f.Phase, &f.OriginID, &f.TargetID,
		&f.OriginG, &f.OriginS, &f.OriginP,
		&f.TargetG, &f.TargetS, &f.TargetP,
		&f.StartAt, &f.ArrivalAt, &f.ReturnAt, &f.SpeedPct, &f.HydrogenCost,
		&f.CargoMetal, &f.CargoLutinum, &f.CargoWater, &f.CargoHydrogen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (s *FleetStore) attachUnits(ctx context.Context, f *Fleet) error {
	rows, err := s.db.QueryContext(ctx, `SELECT code, count FROM fleet_units WHERE fleet_id = $1`, f.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	f.Units = map[string]int64{}
	for rows.Next() {
		var code string
		var n int64
		if err := rows.Scan(&code, &n); err != nil {
			return err
		}
		f.Units[code] = n
	}
	return rows.Err()
}

// ListActive returns a user's in-flight fleets.
func (s *FleetStore) ListActive(ctx context.Context, userID int64) ([]Fleet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+fleetCols+` FROM fleets f
		WHERE f.user_id = $1 AND f.phase IN ('OUTBOUND','RETURNING')
		ORDER BY f.arrival_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Fleet
	for rows.Next() {
		f, err := scanFleet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.attachUnits(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Incoming returns hostile fleets headed to a planet.
func (s *FleetStore) Incoming(ctx context.Context, planetID int64) ([]Fleet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+fleetCols+` FROM fleets f
		WHERE f.target_id = $1 AND f.phase = 'OUTBOUND'
		ORDER BY f.arrival_at
	`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Fleet
	for rows.Next() {
		f, err := scanFleet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// IncomingForUser returns hostile fleets headed to any of a user's planets.
func (s *FleetStore) IncomingForUser(ctx context.Context, userID int64) ([]Fleet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+fleetCols+` FROM fleets f
		JOIN planets p ON p.id = f.target_id
		WHERE p.user_id = $1 AND f.phase = 'OUTBOUND'
		ORDER BY f.arrival_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Fleet
	for rows.Next() {
		f, err := scanFleet(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

// Recall turns an outbound fleet around mid-flight.
func (s *FleetStore) Recall(ctx context.Context, fleetID, userID int64) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE fleets SET phase = 'RETURNING',
			arrival_at = NOW() + (NOW() - start_at)
		WHERE id = $1 AND user_id = $2 AND phase = 'OUTBOUND'
	`, fleetID, userID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrCannotRecall
	}
	return nil
}
