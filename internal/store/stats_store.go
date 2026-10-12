package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"gwx/internal/game"
)

// statsReportLimit caps how many recent combat reports feed the aggregates.
const statsReportLimit = 2000

// StatsStore aggregates a player's own battle reports into the Statistics
// pages. It only ever reads the caller's own reports, so no other player's
// fleet composition is exposed.
type StatsStore struct {
	db *sql.DB
}

// NewStatsStore constructs a StatsStore.
func NewStatsStore(db *sql.DB) *StatsStore { return &StatsStore{db: db} }

// combatReports loads the player's most recent combat payloads. Reports written
// before the per-unit/role payload change (or otherwise malformed) are skipped.
func (s *StatsStore) combatReports(ctx context.Context, userID int64) ([]game.CombatReport, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT payload FROM reports
		WHERE user_id = $1 AND kind = 'combat'
		ORDER BY created_at DESC LIMIT $2
	`, userID, statsReportLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []game.CombatReport
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var rep game.CombatReport
		if err := json.Unmarshal(raw, &rep); err != nil {
			continue
		}
		if rep.Role != "attacker" && rep.Role != "defender" {
			continue
		}
		out = append(out, rep)
	}
	return out, rows.Err()
}

// BattleBalance returns the player's destroyed/lost battle summary.
func (s *StatsStore) BattleBalance(ctx context.Context, userID int64) (game.BattleBalance, error) {
	reports, err := s.combatReports(ctx, userID)
	if err != nil {
		return game.BattleBalance{}, err
	}
	return game.AggregateBattle(reports), nil
}

// Farming returns the player's raid/loot summary.
func (s *StatsStore) Farming(ctx context.Context, userID int64) (game.Farming, error) {
	reports, err := s.combatReports(ctx, userID)
	if err != nil {
		return game.Farming{}, err
	}
	return game.AggregateFarming(reports), nil
}

// Resources returns the player's plundered-versus-lost resource ledger.
func (s *StatsStore) Resources(ctx context.Context, userID int64) (game.ResourceBalance, error) {
	reports, err := s.combatReports(ctx, userID)
	if err != nil {
		return game.ResourceBalance{}, err
	}
	return game.AggregateResources(reports), nil
}
