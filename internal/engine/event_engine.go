// Package engine runs the durable game scheduler. Postgres is the source of
// truth: every tick it claims due rows with FOR UPDATE SKIP LOCKED, re-checks
// the condition inside the transaction and applies the effect idempotently.
// There is no cron and no in-memory state, so it is safe to restart at any time
// and to run more than one instance.
package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"gwx/internal/game"
)

// EventEngine is the durable scheduler.
type EventEngine struct {
	db *sql.DB
}

// NewEventEngine constructs an EventEngine.
func NewEventEngine(db *sql.DB) *EventEngine { return &EventEngine{db: db} }

// A newly colonized planet is seeded with this flat grant on top of whatever the
// colony ship carried (hydrogen is cargo-only; there is no base hydrogen).
const (
	colonyStartMetal = 1000
	colonyStartLut   = 500
	colonyStartWater = 500
)

// insertMessageTx appends one inbox entry inside the resolving transaction.
func insertMessageTx(ctx context.Context, tx *sql.Tx, universeID string, userID int64, category, kind, title, body string, reportID int64) error {
	var ref any
	if reportID > 0 {
		ref = reportID
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO messages (universe_id, user_id, category, kind, title, body, report_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, universeID, userID, category, kind, title, body, ref)
	return err
}

// planetCoordsTx renders "g:s:p" for a planet id.
func planetCoordsTx(ctx context.Context, tx *sql.Tx, planetID int64) (string, error) {
	var g, s, p int
	if err := tx.QueryRowContext(ctx,
		`SELECT galaxy, system, position FROM planets WHERE id = $1`, planetID).Scan(&g, &s, &p); err != nil {
		return "", err
	}
	return coords(g, s, p), nil
}

// queueMessage renders the system notification for a completed queue order.
// It is pure so it can be unit-tested without a database.
func queueMessage(kind, code string, level int, quantity int64, at string) (msgKind, title string) {
	name := code
	if d, ok := game.Get(code); ok {
		name = d.Name
	}
	switch kind {
	case "building":
		return "building", fmt.Sprintf("Construction of %s (level %d) on %s has been completed.", name, level, at)
	case "research":
		return "research", fmt.Sprintf("Research of %s (level %d) has been completed.", name, level)
	case "ship":
		return "ship", fmt.Sprintf("Production of %d × %s on %s has been completed.", quantity, name, at)
	case "defense":
		return "defense", fmt.Sprintf("Production of %d × %s on %s has been completed.", quantity, name, at)
	default:
		return kind, fmt.Sprintf("Order completed on %s.", at)
	}
}

// StartScheduler blocks, resolving due work roughly once per second.
func (e *EventEngine) StartScheduler(ctx context.Context) {
	log.Println("[ENGINE] durable scheduler online (1s poll)")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	e.processDue(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Println("[ENGINE] scheduler stopped")
			return
		case <-ticker.C:
			e.processDue(ctx)
		}
	}
}

func (e *EventEngine) processDue(ctx context.Context) {
	e.processQueues(ctx)
	e.processFleets(ctx)
}

func (e *EventEngine) processQueues(ctx context.Context) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT id FROM queues
		WHERE status = 'IN_PROGRESS' AND finish_at <= NOW()
		ORDER BY finish_at LIMIT 100
	`)
	if err != nil {
		log.Printf("[ENGINE] queue scan: %v", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := e.resolveQueue(ctx, id); err != nil {
			log.Printf("[ENGINE] resolve queue %d: %v", id, err)
		}
	}
}

func (e *EventEngine) resolveQueue(ctx context.Context, queueID int64) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		kind, code  string
		targetLevel sql.NullInt64
		quantity    sql.NullInt64
		planetID    int64
		userID      int64
		universeID  string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT kind, code, target_level, quantity, planet_id, user_id, universe_id::text
		FROM queues
		WHERE id = $1 AND status = 'IN_PROGRESS' AND finish_at <= NOW()
		FOR UPDATE SKIP LOCKED
	`, queueID).Scan(&kind, &code, &targetLevel, &quantity, &planetID, &userID, &universeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // claimed elsewhere or already done
	}
	if err != nil {
		return err
	}

	switch kind {
	case "building":
		if err := applyBuildingTx(ctx, tx, planetID, code, int(targetLevel.Int64)); err != nil {
			return err
		}
		if err := recomputePlanetTx(ctx, tx, planetID, universeID); err != nil {
			return err
		}
	case "research":
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_techs (user_id, code, level) VALUES ($1, $2, $3)
			ON CONFLICT (user_id, code) DO UPDATE SET level = EXCLUDED.level
		`, userID, code, int(targetLevel.Int64)); err != nil {
			return err
		}
		if err := recomputeUserPlanetsTx(ctx, tx, userID, universeID); err != nil {
			return err
		}
	case "ship", "defense":
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO planet_items (planet_id, kind, code, amount) VALUES ($1, $2, $3, $4)
			ON CONFLICT (planet_id, kind, code) DO UPDATE SET amount = planet_items.amount + EXCLUDED.amount
		`, planetID, kind, code, quantity.Int64); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE queues SET status = 'COMPLETED', quantity_done = COALESCE(quantity, 0) WHERE id = $1
	`, queueID); err != nil {
		return err
	}

	// Notify the player (best-effort: a missing coords lookup must not block).
	if at, err := planetCoordsTx(ctx, tx, planetID); err == nil {
		msgKind, title := queueMessage(kind, code, int(targetLevel.Int64), quantity.Int64, at)
		if err := insertMessageTx(ctx, tx, universeID, userID, "system", msgKind, title, "", 0); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func applyBuildingTx(ctx context.Context, tx *sql.Tx, planetID int64, code string, level int) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE planet_items SET amount = $3
		WHERE planet_id = $1 AND kind = 'building' AND code = $2
	`, planetID, code, level)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected > 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO planet_items (planet_id, kind, code, amount) VALUES ($1, 'building', $2, $3)
	`, planetID, code, level); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE planets SET fields_used = fields_used + 1 WHERE id = $1`, planetID)
	return err
}

// recomputePlanetTx re-derives and persists one planet's economy.
func recomputePlanetTx(ctx context.Context, tx *sql.Tx, planetID int64, universeID string) error {
	var userID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM planets WHERE id = $1`, planetID).Scan(&userID); err != nil {
		return err
	}
	structs, err := loadLevelsTx(ctx, tx,
		`SELECT code, amount FROM planet_items WHERE planet_id = $1 AND kind = 'building'`, planetID)
	if err != nil {
		return err
	}
	techs := map[string]int{}
	if userID.Valid {
		techs, err = loadLevelsTx(ctx, tx, `SELECT code, level FROM user_techs WHERE user_id = $1`, userID.Int64)
		if err != nil {
			return err
		}
	}
	speed := resourceSpeedTx(ctx, tx, universeID)
	e := game.Recompute(structs, techs, speed)
	_, err = tx.ExecContext(ctx, `
		UPDATE planets SET
			metal_cap = $2, lutinum_cap = $3, water_cap = $4, hydrogen_cap = $5,
			metal_prod = $6, lutinum_prod = $7, water_prod = $8, hydrogen_prod = $9,
			energy_used = $10, energy_prod = $11,
			protected_metal = $12, protected_lutinum = $13, protected_water = $14, protected_hydrogen = $15
		WHERE id = $1
	`, planetID, e.MetalCap, e.LutinumCap, e.WaterCap, e.HydrogenCap,
		e.MetalProd, e.LutinumProd, e.WaterProd, e.HydrogenProd,
		e.EnergyUsed, e.EnergyProd,
		e.ProtectedMetal, e.ProtectedLutinum, e.ProtectedWater, e.ProtectedHydrogen)
	return err
}

func recomputeUserPlanetsTx(ctx context.Context, tx *sql.Tx, userID int64, universeID string) error {
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
	for _, id := range ids {
		if err := recomputePlanetTx(ctx, tx, id, universeID); err != nil {
			return err
		}
	}
	return nil
}

func loadLevelsTx(ctx context.Context, tx *sql.Tx, q string, arg int64) (map[string]int, error) {
	rows, err := tx.QueryContext(ctx, q, arg)
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

func resourceSpeedTx(ctx context.Context, tx *sql.Tx, universeID string) float64 {
	var v float64
	if err := tx.QueryRowContext(ctx, `SELECT resource_speed FROM universes WHERE id = $1`, universeID).Scan(&v); err != nil {
		return 1
	}
	return v
}

func fleetSpeedTx(ctx context.Context, tx *sql.Tx, universeID string) float64 {
	var v float64
	if err := tx.QueryRowContext(ctx, `SELECT fleet_speed FROM universes WHERE id = $1`, universeID).Scan(&v); err != nil {
		return 1
	}
	return v
}

// ---------------------------------------------------------------------------
// Fleets
// ---------------------------------------------------------------------------

func (e *EventEngine) processFleets(ctx context.Context) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT id FROM fleets
		WHERE phase IN ('OUTBOUND','RETURNING') AND arrival_at <= NOW()
		ORDER BY arrival_at LIMIT 100
	`)
	if err != nil {
		log.Printf("[ENGINE] fleet scan: %v", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := e.resolveFleet(ctx, id); err != nil {
			log.Printf("[ENGINE] resolve fleet %d: %v", id, err)
		}
	}
}

func (e *EventEngine) resolveFleet(ctx context.Context, fleetID int64) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		userID     int64
		universeID string
		mission    string
		phase      string
		originID   int64
		targetID   sql.NullInt64
		tg, ts, tp int
		og, os, op int
		speedPct   float64
	)
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, universe_id::text, mission, phase, origin_id, target_id,
		       target_galaxy, target_system, target_pos,
		       origin_galaxy, origin_system, origin_pos, speed_pct
		FROM fleets
		WHERE id = $1 AND phase IN ('OUTBOUND','RETURNING') AND arrival_at <= NOW()
		FOR UPDATE SKIP LOCKED
	`, fleetID).Scan(&userID, &universeID, &mission, &phase, &originID, &targetID,
		&tg, &ts, &tp, &og, &os, &op, &speedPct)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	units, err := loadFleetUnitsTx(ctx, tx, fleetID)
	if err != nil {
		return err
	}
	techs, err := loadLevelsTx(ctx, tx, `SELECT code, level FROM user_techs WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}

	if phase == "RETURNING" {
		return e.finishReturnTx(ctx, tx, fleetID, userID, universeID, originID, units)
	}

	switch mission {
	case "COLONIZE":
		return e.resolveColonizeTx(ctx, tx, fleetID, userID, universeID, og, os, op, tg, ts, tp, speedPct, units)
	case "DEPLOY":
		return e.resolveDeployTx(ctx, tx, fleetID, userID, originID, targetID, units, og, os, op, tg, ts, tp, speedPct, universeID)
	case "TRANSPORT":
		return e.resolveTransportTx(ctx, tx, fleetID, targetID, units, og, os, op, tg, ts, tp, speedPct, universeID)
	case "ATTACK":
		return e.resolveAttackTx(ctx, tx, fleetID, userID, universeID, targetID, techs, units, og, os, op, tg, ts, tp, speedPct)
	case "ESPIONAGE":
		return e.resolveEspionageTx(ctx, tx, fleetID, userID, universeID, targetID, og, os, op, tg, ts, tp, speedPct, units)
	}
	return nil
}

func loadFleetUnitsTx(ctx context.Context, tx *sql.Tx, fleetID int64) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT code, count FROM fleet_units WHERE fleet_id = $1`, fleetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var code string
		var n int64
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		if n > 0 {
			out[code] = n
		}
	}
	return out, rows.Err()
}

func writeFleetUnitsTx(ctx context.Context, tx *sql.Tx, fleetID int64, units map[string]int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM fleet_units WHERE fleet_id = $1`, fleetID); err != nil {
		return err
	}
	for code, n := range units {
		if n > 0 {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO fleet_units (fleet_id, code, count) VALUES ($1, $2, $3)`,
				fleetID, code, n); err != nil {
				return err
			}
		}
	}
	return nil
}

// sendHome flips a fleet to RETURNING with a mirrored flight time.
func sendHomeTx(ctx context.Context, tx *sql.Tx, fleetID int64, units map[string]int64, techs map[string]int,
	og, os, op, tg, ts, tp int, speedPct, fleetSpeed float64) error {
	distance := game.CalculateDistance(og, os, op, tg, ts, tp)
	maxSpeed := game.FleetMaxSpeed(units, techs)
	dur := game.FlightDuration(distance, maxSpeed, speedPct, fleetSpeed)
	if _, err := tx.ExecContext(ctx, `
		UPDATE fleets SET phase = 'RETURNING', arrival_at = NOW() + ($2 * INTERVAL '1 second')
		WHERE id = $1
	`, fleetID, dur); err != nil {
		return err
	}
	return tx.Commit()
}

// finishReturnTx deposits the fleet's surviving ships and cargo back at origin.
func (e *EventEngine) finishReturnTx(ctx context.Context, tx *sql.Tx, fleetID, userID int64, universeID string, originID int64, units map[string]int64) error {
	var cm, cl, cw, ch int64
	if err := tx.QueryRowContext(ctx, `
		SELECT cargo_metal, cargo_lutinum, cargo_water, cargo_hydrogen FROM fleets WHERE id = $1
	`, fleetID).Scan(&cm, &cl, &cw, &ch); err != nil {
		return err
	}

	// Add ships back to the origin planet.
	for code, n := range units {
		if n <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO planet_items (planet_id, kind, code, amount) VALUES ($1, 'ship', $2, $3)
			ON CONFLICT (planet_id, kind, code) DO UPDATE SET amount = planet_items.amount + EXCLUDED.amount
		`, originID, code, n); err != nil {
			return err
		}
	}
	// Accrue then credit cargo (respecting caps).
	if _, err := accrueTx(ctx, tx, originID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE planets SET
			metal = LEAST(metal_cap, metal + $2),
			lutinum = LEAST(lutinum_cap, lutinum + $3),
			water = LEAST(water_cap, water + $4),
			hydrogen = LEAST(hydrogen_cap, hydrogen + $5)
		WHERE id = $1
	`, originID, cm, cl, cw, ch); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `UPDATE fleets SET phase = 'RESOLVED' WHERE id = $1`, fleetID)
	if err != nil {
		return err
	}
	if at, err := planetCoordsTx(ctx, tx, originID); err == nil {
		if err := insertMessageTx(ctx, tx, universeID, userID, "system", "own-fleet",
			"Your fleet has returned to "+at+".", "", 0); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (e *EventEngine) resolveColonizeTx(ctx context.Context, tx *sql.Tx, fleetID, userID int64, universeID string,
	og, os, op, tg, ts, tp int, speedPct float64, units map[string]int64) error {
	var cm, cl, cw, ch int64
	if err := tx.QueryRowContext(ctx, `
		SELECT cargo_metal, cargo_lutinum, cargo_water, cargo_hydrogen FROM fleets WHERE id = $1
	`, fleetID).Scan(&cm, &cl, &cw, &ch); err != nil {
		return err
	}

	var colonyID int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO planets (
			universe_id, user_id, name, galaxy, system, position, is_homeworld,
			diameter_km, fields_max, fields_used,
			metal, lutinum, water, hydrogen,
			metal_cap, lutinum_cap, water_cap, hydrogen_cap,
			protected_metal, protected_lutinum, protected_water, protected_hydrogen
		) VALUES (
			$1, $2, 'Colony', $3, $4, $5, FALSE,
			12000, 120, 0,
			$6, $7, $8, $9,
			20000, 20000, 20000, 20000,
			4000, 4000, 4000, 4000
		)
		ON CONFLICT (universe_id, galaxy, system, position) DO NOTHING
		RETURNING id
	`, universeID, userID, tg, ts, tp,
		float64(cm)+colonyStartMetal, float64(cl)+colonyStartLut,
		float64(cw)+colonyStartWater, float64(ch)).Scan(&colonyID)
	if errors.Is(err, sql.ErrNoRows) {
		// Slot taken meanwhile: send the colony ship home.
		techs, _ := loadLevelsTx(ctx, tx, `SELECT code, level FROM user_techs WHERE user_id = $1`, userID)
		if err := writeFleetUnitsTx(ctx, tx, fleetID, units); err != nil {
			return err
		}
		return sendHomeTx(ctx, tx, fleetID, units, techs, og, os, op, tg, ts, tp, speedPct, fleetSpeedTx(ctx, tx, universeID))
	}
	if err != nil {
		return err
	}

	// The colony ship is consumed.
	delete(units, "colony_ship")
	if _, err := tx.ExecContext(ctx, `UPDATE fleets SET phase = 'RESOLVED' WHERE id = $1`, fleetID); err != nil {
		return err
	}
	_ = colonyID
	return tx.Commit()
}

func (e *EventEngine) resolveDeployTx(ctx context.Context, tx *sql.Tx, fleetID, userID, originID int64, targetID sql.NullInt64,
	units map[string]int64, og, os, op, tg, ts, tp int, speedPct float64, universeID string) error {
	if targetID.Valid {
		var owner sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT user_id FROM planets WHERE id = $1`, targetID.Int64).Scan(&owner); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if owner.Valid && owner.Int64 == userID {
			for code, n := range units {
				if n <= 0 {
					continue
				}
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO planet_items (planet_id, kind, code, amount) VALUES ($1, 'ship', $2, $3)
					ON CONFLICT (planet_id, kind, code) DO UPDATE SET amount = planet_items.amount + EXCLUDED.amount
				`, targetID.Int64, code, n); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `UPDATE fleets SET phase = 'RESOLVED' WHERE id = $1`, fleetID); err != nil {
				return err
			}
			return tx.Commit()
		}
	}
	// Target vanished or changed hands: go home.
	techs, _ := loadLevelsTx(ctx, tx, `SELECT code, level FROM user_techs WHERE user_id = $1`, userID)
	if err := writeFleetUnitsTx(ctx, tx, fleetID, units); err != nil {
		return err
	}
	return sendHomeTx(ctx, tx, fleetID, units, techs, og, os, op, tg, ts, tp, speedPct, fleetSpeedTx(ctx, tx, universeID))
}

func (e *EventEngine) resolveTransportTx(ctx context.Context, tx *sql.Tx, fleetID int64, targetID sql.NullInt64,
	units map[string]int64, og, os, op, tg, ts, tp int, speedPct float64, universeID string) error {
	var cm, cl, cw, ch int64
	if err := tx.QueryRowContext(ctx, `
		SELECT cargo_metal, cargo_lutinum, cargo_water, cargo_hydrogen FROM fleets WHERE id = $1
	`, fleetID).Scan(&cm, &cl, &cw, &ch); err != nil {
		return err
	}
	if targetID.Valid {
		if _, err := accrueTx(ctx, tx, targetID.Int64); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE planets SET
				metal = LEAST(metal_cap, metal + $2),
				lutinum = LEAST(lutinum_cap, lutinum + $3),
				water = LEAST(water_cap, water + $4),
				hydrogen = LEAST(hydrogen_cap, hydrogen + $5)
			WHERE id = $1
		`, targetID.Int64, cm, cl, cw, ch); err != nil {
			return err
		}
	}
	// Empty the hold and head home.
	if _, err := tx.ExecContext(ctx, `
		UPDATE fleets SET cargo_metal = 0, cargo_lutinum = 0, cargo_water = 0, cargo_hydrogen = 0 WHERE id = $1
	`, fleetID); err != nil {
		return err
	}
	var fleetUser int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM fleets WHERE id = $1`, fleetID).Scan(&fleetUser); err != nil {
		return err
	}
	techs, _ := loadLevelsTx(ctx, tx, `SELECT code, level FROM user_techs WHERE user_id = $1`, fleetUser)
	return sendHomeTx(ctx, tx, fleetID, units, techs, og, os, op, tg, ts, tp, speedPct, fleetSpeedTx(ctx, tx, universeID))
}

func (e *EventEngine) resolveEspionageTx(ctx context.Context, tx *sql.Tx, fleetID, userID int64, universeID string,
	targetID sql.NullInt64, og, os, op, tg, ts, tp int, speedPct float64, units map[string]int64) error {
	techs, err := loadLevelsTx(ctx, tx, `SELECT code, level FROM user_techs WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	if targetID.Valid {
		ships, _ := loadItemsTx(ctx, tx, targetID.Int64, "ship")
		defense, _ := loadItemsTx(ctx, tx, targetID.Int64, "defense")
		res, _ := loadResourcesTx(ctx, tx, targetID.Int64)
		var ownerName string
		_ = tx.QueryRowContext(ctx, `
			SELECT COALESCE(u.username, 'Uninhabited') FROM planets p
			LEFT JOIN users u ON u.id = p.user_id WHERE p.id = $1
		`, targetID.Int64).Scan(&ownerName)

		payload := map[string]any{
			"target": coords(tg, ts, tp),
			"owner":  ownerName,
			"resources": map[string]int64{
				"metal": int64(res.Metal), "lutinum": int64(res.Lutinum),
				"water": int64(res.Water), "hydrogen": int64(res.Hydrogen),
			},
			"ships":             ships,
			"defense":           defense,
			"yourEspionageTech": techs["espionage_tech"],
		}
		raw, _ := json.Marshal(payload)
		var reportID int64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO reports (universe_id, user_id, kind, title, payload)
			VALUES ($1, $2, 'espionage', $3, $4) RETURNING id
		`, universeID, userID, "Espionage: "+coords(tg, ts, tp), raw).Scan(&reportID); err != nil {
			return err
		}
		if err := insertMessageTx(ctx, tx, universeID, userID, "report", "spy",
			"Espionage: "+coords(tg, ts, tp), "", reportID); err != nil {
			return err
		}
	}
	return sendHomeTx(ctx, tx, fleetID, units, techs, og, os, op, tg, ts, tp, speedPct, fleetSpeedTx(ctx, tx, universeID))
}

func (e *EventEngine) resolveAttackTx(ctx context.Context, tx *sql.Tx, fleetID, userID int64, universeID string,
	targetID sql.NullInt64, techs map[string]int, units map[string]int64, og, os, op, tg, ts, tp int, speedPct float64) error {
	fleetSpeed := fleetSpeedTx(ctx, tx, universeID)
	if !targetID.Valid {
		return sendHomeTx(ctx, tx, fleetID, units, techs, og, os, op, tg, ts, tp, speedPct, fleetSpeed)
	}

	defTechs := map[string]int{}
	var defOwner sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM planets WHERE id = $1`, targetID.Int64).Scan(&defOwner); err != nil {
		return err
	}
	if defOwner.Valid {
		defTechs, _ = loadLevelsTx(ctx, tx, `SELECT code, level FROM user_techs WHERE user_id = $1`, defOwner.Int64)
	}

	defShips, err := loadItemsTx(ctx, tx, targetID.Int64, "ship")
	if err != nil {
		return err
	}
	defDef, err := loadItemsTx(ctx, tx, targetID.Int64, "defense")
	if err != nil {
		return err
	}
	defender := map[string]int64{}
	for k, v := range defShips {
		defender[k] += v
	}
	for k, v := range defDef {
		defender[k] += v
	}

	result := game.Simulate(units, defender, techs["weapon_tech"], defTechs["shield_tech"], 6)

	// Persist defender losses (ships + defense) and attacker survivors.
	if err := persistSideLossesTx(ctx, tx, targetID.Int64, defShips, result.DefenderSurvivors); err != nil {
		return err
	}
	if err := persistSideLossesTx(ctx, tx, targetID.Int64, defDef, result.DefenderSurvivors); err != nil {
		return err
	}
	if err := writeFleetUnitsTx(ctx, tx, fleetID, result.AttackerSurvivors); err != nil {
		return err
	}

	// Plunder: fill the hold from the unprotected remainder.
	loot := game.Resources{}
	if len(result.AttackerSurvivors) > 0 {
		capacity := game.CargoCapacity(result.AttackerSurvivors, techs["cargo_tech"])
		res, err := loadResourcesTx(ctx, tx, targetID.Int64)
		if err != nil {
			return err
		}
		var pM, pL, pW, pH int64
		if err := tx.QueryRowContext(ctx, `
			SELECT protected_metal, protected_lutinum, protected_water, protected_hydrogen
			FROM planets WHERE id = $1
		`, targetID.Int64).Scan(&pM, &pL, &pW, &pH); err != nil {
			return err
		}
		remaining := capacity
		take := func(have, protected int64) int64 {
			avail := have - protected
			if avail <= 0 {
				return 0
			}
			if avail > remaining {
				avail = remaining
			}
			remaining -= avail
			return avail
		}
		loot.Metal = float64(take(int64(res.Metal), pM))
		loot.Lutinum = float64(take(int64(res.Lutinum), pL))
		loot.Water = float64(take(int64(res.Water), pW))
		loot.Hydrogen = float64(take(int64(res.Hydrogen), pH))

		if _, err := tx.ExecContext(ctx, `
			UPDATE planets SET metal = metal - $2, lutinum = lutinum - $3, water = water - $4, hydrogen = hydrogen - $5
			WHERE id = $1
		`, targetID.Int64, loot.Metal, loot.Lutinum, loot.Water, loot.Hydrogen); err != nil {
			return err
		}
	}

	// Report. Each participant gets their own copy of the payload, tagged with
	// their role so statistics can read destroyed/lost from the right side.
	payload := map[string]any{
		"target":            coords(tg, ts, tp),
		"rounds":            result.Rounds,
		"attackerLosses":    result.AttackerLosses,
		"defenderLosses":    result.DefenderLosses,
		"attackerSurvivors": result.AttackerSurvivors,
		"loot": map[string]int64{
			"metal": int64(loot.Metal), "lutinum": int64(loot.Lutinum),
			"water": int64(loot.Water), "hydrogen": int64(loot.Hydrogen),
		},
	}
	payload["role"] = "attacker"
	rawAttacker, _ := json.Marshal(payload)
	var attackReportID int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO reports (universe_id, user_id, kind, title, payload)
		VALUES ($1, $2, 'combat', $3, $4) RETURNING id
	`, universeID, userID, "Attack on "+coords(tg, ts, tp), rawAttacker).Scan(&attackReportID); err != nil {
		return err
	}
	if err := insertMessageTx(ctx, tx, universeID, userID, "report", "attack",
		"Attack on "+coords(tg, ts, tp), "", attackReportID); err != nil {
		return err
	}
	if defOwner.Valid {
		payload["role"] = "defender"
		rawDefender, _ := json.Marshal(payload)
		var defReportID int64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO reports (universe_id, user_id, kind, title, payload)
			VALUES ($1, $2, 'combat', $3, $4) RETURNING id
		`, universeID, defOwner.Int64, "Defense at "+coords(tg, ts, tp), rawDefender).Scan(&defReportID); err != nil {
			return err
		}
		if err := insertMessageTx(ctx, tx, universeID, defOwner.Int64, "report", "defense",
			"Defense at "+coords(tg, ts, tp), "", defReportID); err != nil {
			return err
		}
	}

	// Carry loot home (or resolve if the whole fleet died).
	if len(result.AttackerSurvivors) == 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE fleets SET phase = 'RESOLVED' WHERE id = $1`, fleetID); err != nil {
			return err
		}
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE fleets SET cargo_metal = $2, cargo_lutinum = $3, cargo_water = $4, cargo_hydrogen = $5 WHERE id = $1
	`, fleetID, int64(loot.Metal), int64(loot.Lutinum), int64(loot.Water), int64(loot.Hydrogen)); err != nil {
		return err
	}
	return sendHomeTx(ctx, tx, fleetID, result.AttackerSurvivors, techs, og, os, op, tg, ts, tp, speedPct, fleetSpeed)
}

func persistSideLossesTx(ctx context.Context, tx *sql.Tx, planetID int64, before, survivors map[string]int64) error {
	for code, have := range before {
		if have <= 0 {
			continue
		}
		left := survivors[code]
		kind := "ship"
		if _, ok := game.Catalog[code]; ok && game.Catalog[code].Kind == game.KindDefense {
			kind = "defense"
		}
		if left <= 0 {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM planet_items WHERE planet_id = $1 AND kind = $2 AND code = $3`, planetID, kind, code); err != nil {
				return err
			}
			continue
		}
		if left != have {
			if _, err := tx.ExecContext(ctx,
				`UPDATE planet_items SET amount = $4 WHERE planet_id = $1 AND kind = $2 AND code = $3`,
				planetID, kind, code, left); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadItemsTx(ctx context.Context, tx *sql.Tx, planetID int64, kind string) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT code, amount FROM planet_items WHERE planet_id = $1 AND kind = $2 AND amount > 0`, planetID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var code string
		var n int64
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

func loadResourcesTx(ctx context.Context, tx *sql.Tx, planetID int64) (game.Resources, error) {
	var m, l, w, h float64
	err := tx.QueryRowContext(ctx,
		`SELECT out_metal, out_lutinum, out_water, out_hydrogen FROM update_planet_resources($1)`,
		planetID).Scan(&m, &l, &w, &h)
	return game.Resources{Metal: m, Lutinum: l, Water: w, Hydrogen: h}, err
}

func accrueTx(ctx context.Context, tx *sql.Tx, planetID int64) (game.Resources, error) {
	return loadResourcesTx(ctx, tx, planetID)
}

func coords(g, s, p int) string {
	return itoa(g) + ":" + itoa(s) + ":" + itoa(p)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
