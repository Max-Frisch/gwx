package store

import (
	"context"
	"database/sql"
	"errors"

	"gwx/internal/game"
)

// PlanetStore reads and writes planets and their economy.
type PlanetStore struct {
	db *sql.DB
}

// NewPlanetStore constructs a PlanetStore.
func NewPlanetStore(db *sql.DB) *PlanetStore { return &PlanetStore{db: db} }

const planetCols = `
	id, universe_id::text, user_id, name, galaxy, system, position, is_homeworld,
	diameter_km, fields_max, fields_used,
	metal, lutinum, water, hydrogen,
	metal_cap, lutinum_cap, water_cap, hydrogen_cap,
	metal_prod, lutinum_prod, water_prod, hydrogen_prod,
	energy_used, energy_prod,
	protected_metal, protected_lutinum, protected_water, protected_hydrogen,
	last_resource_calc_at`

func scanPlanet(row interface {
	Scan(...any) error
}) (*Planet, error) {
	var p Planet
	err := row.Scan(
		&p.ID, &p.UniverseID, &p.UserID, &p.Name, &p.Galaxy, &p.System, &p.Position, &p.IsHomeworld,
		&p.DiameterKM, &p.FieldsMax, &p.FieldsUsed,
		&p.Metal, &p.Lutinum, &p.Water, &p.Hydrogen,
		&p.MetalCap, &p.LutinumCap, &p.WaterCap, &p.HydrogenCap,
		&p.MetalProd, &p.LutinumProd, &p.WaterProd, &p.HydrogenProd,
		&p.EnergyUsed, &p.EnergyProd,
		&p.ProtectedMetal, &p.ProtectedLutinum, &p.ProtectedWater, &p.ProtectedHydrogen,
		&p.LastCalc,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Get returns a planet by id.
func (s *PlanetStore) Get(ctx context.Context, planetID int64) (*Planet, error) {
	return scanPlanet(s.db.QueryRowContext(ctx, `SELECT `+planetCols+` FROM planets WHERE id = $1`, planetID))
}

// HomeworldID returns the user's homeworld id (0 if none).
func (s *PlanetStore) HomeworldID(ctx context.Context, userID int64) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM planets WHERE user_id = $1 AND is_homeworld LIMIT 1`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// ListOwned returns all planets owned by a user.
func (s *PlanetStore) ListOwned(ctx context.Context, userID int64) ([]OwnedPlanet, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, galaxy, system, position, is_homeworld
		FROM planets WHERE user_id = $1 ORDER BY is_homeworld DESC, id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OwnedPlanet
	for rows.Next() {
		var p OwnedPlanet
		if err := rows.Scan(&p.ID, &p.Name, &p.Galaxy, &p.System, &p.Position, &p.IsHomeworld); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Structures returns building code -> level.
func (s *PlanetStore) Structures(ctx context.Context, planetID int64) (map[string]int, error) {
	return s.intMap(ctx, `SELECT code, amount FROM planet_items WHERE planet_id = $1 AND kind = 'building'`, planetID)
}

// Units returns ship/defense code -> count for a kind.
func (s *PlanetStore) Units(ctx context.Context, planetID int64, kind string) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT code, amount FROM planet_items WHERE planet_id = $1 AND kind = $2`, planetID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var code string
		var amt int64
		if err := rows.Scan(&code, &amt); err != nil {
			return nil, err
		}
		if amt > 0 {
			out[code] = amt
		}
	}
	return out, rows.Err()
}

// Techs returns research code -> level for an account.
func (s *PlanetStore) Techs(ctx context.Context, userID int64) (map[string]int, error) {
	return s.intMap(ctx, `SELECT code, level FROM user_techs WHERE user_id = $1`, userID)
}

func (s *PlanetStore) intMap(ctx context.Context, q string, arg int64) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, q, arg)
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

// Recompute derives the planet economy from current buildings + research and
// persists it. Safe to call often.
func (s *PlanetStore) Recompute(ctx context.Context, planetID int64) error {
	p, err := s.Get(ctx, planetID)
	if err != nil {
		return err
	}
	if p.UserID.Valid {
		return s.RecomputeWith(ctx, p)
	}
	return nil
}

// RecomputeWith persists a recomputed economy for an already-loaded planet.
func (s *PlanetStore) RecomputeWith(ctx context.Context, p *Planet) error {
	structs, err := s.Structures(ctx, p.ID)
	if err != nil {
		return err
	}
	var techs map[string]int
	if p.UserID.Valid {
		techs, err = s.Techs(ctx, p.UserID.Int64)
		if err != nil {
			return err
		}
	} else {
		techs = map[string]int{}
	}
	e := game.Recompute(structs, techs, s.resourceSpeed(ctx, p.UniverseID))
	_, err = s.db.ExecContext(ctx, `
		UPDATE planets SET
			metal_cap = $2, lutinum_cap = $3, water_cap = $4, hydrogen_cap = $5,
			metal_prod = $6, lutinum_prod = $7, water_prod = $8, hydrogen_prod = $9,
			energy_used = $10, energy_prod = $11,
			protected_metal = $12, protected_lutinum = $13, protected_water = $14, protected_hydrogen = $15
		WHERE id = $1
	`, p.ID, e.MetalCap, e.LutinumCap, e.WaterCap, e.HydrogenCap,
		e.MetalProd, e.LutinumProd, e.WaterProd, e.HydrogenProd,
		e.EnergyUsed, e.EnergyProd,
		e.ProtectedMetal, e.ProtectedLutinum, e.ProtectedWater, e.ProtectedHydrogen)
	return err
}

func (s *PlanetStore) resourceSpeed(ctx context.Context, universeUUID string) float64 {
	var v float64
	if err := s.db.QueryRowContext(ctx,
		`SELECT resource_speed FROM universes WHERE id = $1`, universeUUID).Scan(&v); err != nil {
		return 1
	}
	return v
}

// Scan returns the slots of one system. universeCode is the universes.code_name.
func (s *PlanetStore) Scan(ctx context.Context, universeCode string, galaxy, system int) ([]ScanRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.position, p.id, p.name, u.id, u.username, p.is_homeworld
		FROM planets p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.universe_id = (SELECT id FROM universes WHERE code_name = $1)
		  AND p.galaxy = $2 AND p.system = $3
		ORDER BY p.position
	`, universeCode, galaxy, system)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScanRow
	for rows.Next() {
		var r ScanRow
		if err := rows.Scan(&r.Position, &r.PlanetID, &r.Name, &r.OwnerID, &r.OwnerName, &r.IsHomeworld); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FindAt returns the planet id at exact coordinates within a universe, or 0.
func (s *PlanetStore) FindAt(ctx context.Context, universeCode string, g, sys, pos int) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM planets
		WHERE universe_id = (SELECT id FROM universes WHERE code_name = $1)
		  AND galaxy = $2 AND system = $3 AND position = $4
	`, universeCode, g, sys, pos).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}
