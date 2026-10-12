package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"gwx/internal/game"
)

// battlesReportLimit caps how many recent combat reports feed the Battles tab.
const battlesReportLimit = 2000

// ScoreStore computes economy leaderboards. Points are buildings + research
// only; the game has no military points, so fleet/defense strength never shows.
type ScoreStore struct {
	db *sql.DB
}

// NewScoreStore constructs a ScoreStore.
func NewScoreStore(db *sql.DB) *ScoreStore { return &ScoreStore{db: db} }

// PlayerScore is one row of the players leaderboard.
type PlayerScore struct {
	UserID         int64
	Username       string
	PlanetPoints   float64
	ResearchPoints float64
	Planets        int
}

// Total is the combined economy score.
func (p PlayerScore) Total() float64 { return p.PlanetPoints + p.ResearchPoints }

// PlanetScore is one row of the planets leaderboard.
type PlanetScore struct {
	PlanetID  int64
	Name      string
	Galaxy    int
	System    int
	Position  int
	OwnerID   int64
	OwnerName string
	Points    float64
}

// Players returns economy points for every account in the universe.
func (s *ScoreStore) Players(ctx context.Context, universeCode string) ([]PlayerScore, error) {
	scores := map[int64]*PlayerScore{}
	get := func(id int64) *PlayerScore {
		ps, ok := scores[id]
		if !ok {
			ps = &PlayerScore{UserID: id}
			scores[id] = ps
		}
		return ps
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username FROM users
		WHERE universe_id = (SELECT id FROM universes WHERE code_name = $1)`, universeCode)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		get(id).Username = name
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	brows, err := s.db.QueryContext(ctx, `
		SELECT p.user_id, pi.code, SUM(pi.amount)::bigint
		FROM planet_items pi
		JOIN planets p ON p.id = pi.planet_id
		WHERE pi.kind = 'building' AND p.user_id IS NOT NULL
		  AND p.universe_id = (SELECT id FROM universes WHERE code_name = $1)
		GROUP BY p.user_id, pi.code`, universeCode)
	if err != nil {
		return nil, err
	}
	for brows.Next() {
		var uid, amount int64
		var code string
		if err := brows.Scan(&uid, &code, &amount); err != nil {
			brows.Close()
			return nil, err
		}
		if d, ok := game.Catalog[code]; ok {
			get(uid).PlanetPoints += d.LevelPoints(int(amount))
		}
	}
	brows.Close()
	if err := brows.Err(); err != nil {
		return nil, err
	}

	rrows, err := s.db.QueryContext(ctx, `
		SELECT ut.user_id, ut.code, ut.level
		FROM user_techs ut
		JOIN users u ON u.id = ut.user_id
		WHERE u.universe_id = (SELECT id FROM universes WHERE code_name = $1)`, universeCode)
	if err != nil {
		return nil, err
	}
	for rrows.Next() {
		var uid int64
		var code string
		var level int
		if err := rrows.Scan(&uid, &code, &level); err != nil {
			rrows.Close()
			return nil, err
		}
		if d, ok := game.Catalog[code]; ok {
			get(uid).ResearchPoints += d.LevelPoints(level)
		}
	}
	rrows.Close()
	if err := rrows.Err(); err != nil {
		return nil, err
	}

	prows, err := s.db.QueryContext(ctx, `
		SELECT user_id, COUNT(*)::int FROM planets
		WHERE user_id IS NOT NULL
		  AND universe_id = (SELECT id FROM universes WHERE code_name = $1)
		GROUP BY user_id`, universeCode)
	if err != nil {
		return nil, err
	}
	for prows.Next() {
		var uid int64
		var n int
		if err := prows.Scan(&uid, &n); err != nil {
			prows.Close()
			return nil, err
		}
		get(uid).Planets = n
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return nil, err
	}

	out := make([]PlayerScore, 0, len(scores))
	for _, ps := range scores {
		out = append(out, *ps)
	}
	return out, nil
}

// Planets returns economy points per planet in the universe.
func (s *ScoreStore) Planets(ctx context.Context, universeCode string) ([]PlanetScore, error) {
	scores := map[int64]*PlanetScore{}
	get := func(id int64) *PlanetScore {
		ps, ok := scores[id]
		if !ok {
			ps = &PlanetScore{PlanetID: id}
			scores[id] = ps
		}
		return ps
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.galaxy, p.system, p.position,
		       COALESCE(p.user_id, 0), COALESCE(u.username, '')
		FROM planets p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.universe_id = (SELECT id FROM universes WHERE code_name = $1)`, universeCode)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, ownerID int64
		var name, ownerName string
		var g, sys, pos int
		if err := rows.Scan(&id, &name, &g, &sys, &pos, &ownerID, &ownerName); err != nil {
			rows.Close()
			return nil, err
		}
		ps := get(id)
		ps.Name, ps.Galaxy, ps.System, ps.Position = name, g, sys, pos
		ps.OwnerID, ps.OwnerName = ownerID, ownerName
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	brows, err := s.db.QueryContext(ctx, `
		SELECT pi.planet_id, pi.code, pi.amount
		FROM planet_items pi
		JOIN planets p ON p.id = pi.planet_id
		WHERE pi.kind = 'building'
		  AND p.universe_id = (SELECT id FROM universes WHERE code_name = $1)`, universeCode)
	if err != nil {
		return nil, err
	}
	for brows.Next() {
		var pid, amount int64
		var code string
		if err := brows.Scan(&pid, &code, &amount); err != nil {
			brows.Close()
			return nil, err
		}
		if d, ok := game.Catalog[code]; ok {
			get(pid).Points += d.LevelPoints(int(amount))
		}
	}
	brows.Close()
	if err := brows.Err(); err != nil {
		return nil, err
	}

	out := make([]PlanetScore, 0, len(scores))
	for _, ps := range scores {
		out = append(out, *ps)
	}
	return out, nil
}

// BattleRow is one public battle headline. It never carries fleet or defense
// composition — only the total resource value destroyed/plundered, the two
// player names and the target coordinates. Fog of war holds.
type BattleRow struct {
	ID        int64
	When      time.Time
	Attacker  string
	Defender  string
	Coords    string
	Rounds    int
	Destroyed float64
	Plundered float64
}

// Battles returns recent combat headlines for the universe. Each battle is
// written as two combat reports (attacker + defender) inside one transaction,
// so they share created_at; the attacker copy defines the row and the defender
// copy supplies the defender's name (empty for attacks on unowned planets).
func (s *ScoreStore) Battles(ctx context.Context, universeCode string) ([]BattleRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, u.username, r.created_at, r.payload
		FROM reports r
		JOIN users u ON u.id = r.user_id
		WHERE r.kind = 'combat'
		  AND r.universe_id = (SELECT id FROM universes WHERE code_name = $1)
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2`, universeCode, battlesReportLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	defenders := map[string]string{}
	order := make([]string, 0)
	battles := map[string]BattleRow{}
	for rows.Next() {
		var id int64
		var username string
		var when time.Time
		var raw []byte
		if err := rows.Scan(&id, &username, &when, &raw); err != nil {
			return nil, err
		}
		var rep game.CombatReport
		if err := json.Unmarshal(raw, &rep); err != nil {
			continue
		}
		key := rep.Target + "|" + when.UTC().Format(time.RFC3339Nano)
		switch rep.Role {
		case "defender":
			defenders[key] = username
		case "attacker":
			if _, seen := battles[key]; seen {
				continue
			}
			battles[key] = BattleRow{
				ID: id, When: when, Attacker: username, Coords: rep.Target,
				Rounds: rep.Rounds, Destroyed: rep.DestroyedValue(), Plundered: rep.LootValue(),
			}
			order = append(order, key)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]BattleRow, 0, len(order))
	for _, key := range order {
		b := battles[key]
		b.Defender = defenders[key]
		out = append(out, b)
	}
	return out, nil
}
