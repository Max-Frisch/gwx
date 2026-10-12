package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gwx/internal/game"
)

// TradeStore owns the player marketplace: escrowed barter offers, an atomic
// settlement into per-planet trade storage, an audit history and lazy expiry.
type TradeStore struct {
	db *sql.DB
}

// NewTradeStore constructs a TradeStore.
func NewTradeStore(db *sql.DB) *TradeStore { return &TradeStore{db: db} }

// TradeOffer is one marketplace offer with its creator and planet context.
type TradeOffer struct {
	ID          int64
	CreatorID   int64
	CreatorName string
	PlanetID    int64
	PlanetName  string
	Galaxy      int
	System      int
	Position    int
	TargetID    sql.NullInt64
	TargetName  string
	Give        game.Resources
	Want        game.Resources
	Status      string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// IsPrivate reports whether the offer is directed at a single player.
func (o TradeOffer) IsPrivate() bool { return o.TargetID.Valid }

// TradeStorageEntry is one pending delivery sitting in a planet's trade storage.
type TradeStorageEntry struct {
	ID        int64
	PlanetID  int64
	Resources game.Resources
	ExpiresAt time.Time
}

// TradeHistoryEntry is one audit row, enriched with the offer's baskets.
type TradeHistoryEntry struct {
	OfferID          int64
	ActorID          int64
	ActorName        string
	CounterpartyID   int64
	CounterpartyName string
	Action           string
	At               time.Time
	Give             game.Resources
	Want             game.Resources
}

// tradeResCol maps a public resource key to its offer-table column prefix.
var tradeResCol = map[string]string{
	"iron":     "metal",
	"lutinum":  "lutinum",
	"water":    "water",
	"hydrogen": "hydrogen",
}

const tradeOfferCols = `
	o.id, o.creator_id, COALESCE(cu.username, ''), o.planet_id, COALESCE(p.name, ''),
	p.galaxy, p.system, p.position,
	o.target_id, COALESCE(tu.username, ''),
	o.offer_metal, o.offer_lutinum, o.offer_water, o.offer_hydrogen,
	o.want_metal, o.want_lutinum, o.want_water, o.want_hydrogen,
	o.status, o.expires_at, o.created_at`

const tradeOfferJoins = `
	JOIN users cu ON cu.id = o.creator_id
	JOIN planets p ON p.id = o.planet_id
	LEFT JOIN users tu ON tu.id = o.target_id`

func scanTradeOffer(row interface{ Scan(...any) error }) (*TradeOffer, error) {
	var o TradeOffer
	err := row.Scan(&o.ID, &o.CreatorID, &o.CreatorName, &o.PlanetID, &o.PlanetName,
		&o.Galaxy, &o.System, &o.Position,
		&o.TargetID, &o.TargetName,
		&o.Give.Metal, &o.Give.Lutinum, &o.Give.Water, &o.Give.Hydrogen,
		&o.Want.Metal, &o.Want.Lutinum, &o.Want.Water, &o.Want.Hydrogen,
		&o.Status, &o.ExpiresAt, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func addRes(a, b game.Resources) game.Resources {
	return game.Resources{
		Metal: a.Metal + b.Metal, Lutinum: a.Lutinum + b.Lutinum,
		Water: a.Water + b.Water, Hydrogen: a.Hydrogen + b.Hydrogen,
	}
}

func nullInt(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func insertTradeHistoryTx(ctx context.Context, tx *sql.Tx, offerID, actorID, counterpartyID int64, action string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO trade_history (offer_id, actor_id, counterparty_id, action)
		VALUES ($1, $2, $3, $4)
	`, offerID, actorID, nullInt(counterpartyID), action)
	return err
}

func insertTradeStorageTx(ctx context.Context, tx *sql.Tx, planetID int64, r game.Resources, expires time.Time) error {
	if r.IsZero() {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO trade_storage (planet_id, metal, lutinum, water, hydrogen, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, planetID, r.Metal, r.Lutinum, r.Water, r.Hydrogen, expires)
	return err
}

// CreateOffer escrows the given basket — plus the up-front offer fee — from the
// creator's planet and posts an OPEN offer. A private offer (targetID > 0) must
// name another account in the universe. The planet must host a Trading Post, and
// it govers the number of concurrent offers (slots = trading post level).
func (s *TradeStore) CreateOffer(ctx context.Context, universeCode string, creatorID, planetID, targetID int64, give, want game.Resources) (int64, error) {
	if give.IsZero() || want.IsZero() || !give.NonNegative() || !want.NonNegative() {
		return 0, ErrInvalidQuantity
	}
	if targetID == creatorID {
		return 0, ErrNoTarget
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var universeUUID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id::text FROM universes WHERE code_name = $1`, universeCode).Scan(&universeUUID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}

	var owner sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT user_id FROM planets WHERE id = $1 FOR UPDATE`, planetID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	if !owner.Valid || owner.Int64 != creatorID {
		return 0, ErrNotOwner
	}

	var postLevel int
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(amount, 0) FROM planet_items
		WHERE planet_id = $1 AND kind = 'building' AND code = 'trading_post'
	`, planetID).Scan(&postLevel)
	if postLevel < 1 {
		return 0, ErrNoTradingPost
	}

	if targetID > 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND universe_id = $2)`,
			targetID, universeUUID).Scan(&exists); err != nil {
			return 0, err
		}
		if !exists {
			return 0, ErrNoTarget
		}
	}

	var open int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM trade_offers
		WHERE planet_id = $1 AND status = 'OPEN' AND expires_at > NOW()
	`, planetID).Scan(&open); err != nil {
		return 0, err
	}
	if open >= postLevel {
		return 0, ErrTradeSlotsFull
	}

	have, err := accrueTx(ctx, tx, planetID)
	if err != nil {
		return 0, err
	}
	spend := addRes(give, game.FeeFloor(give, game.TradeOfferFee))
	if !affordable(have, spend) {
		return 0, ErrInsufficientResources
	}
	if err := deductTx(ctx, tx, planetID, spend); err != nil {
		return 0, err
	}

	life := game.TradePublicDuration
	if targetID > 0 {
		life = game.TradePrivateDuration
	}

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO trade_offers (
			universe_id, creator_id, planet_id, target_id,
			offer_metal, offer_lutinum, offer_water, offer_hydrogen,
			want_metal, want_lutinum, want_water, want_hydrogen,
			status, expires_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8, $9, $10, $11, $12,
			'OPEN', NOW() + ($13 * INTERVAL '1 second')
		) RETURNING id
	`, universeUUID, creatorID, planetID, nullInt(targetID),
		give.Metal, give.Lutinum, give.Water, give.Hydrogen,
		want.Metal, want.Lutinum, want.Water, want.Hydrogen,
		int64(life.Seconds())).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err := insertTradeHistoryTx(ctx, tx, id, creatorID, targetID, "CREATE"); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// GetOffer returns one offer with its creator/planet context.
func (s *TradeStore) GetOffer(ctx context.Context, id int64) (*TradeOffer, error) {
	return scanTradeOffer(s.db.QueryRowContext(ctx,
		`SELECT `+tradeOfferCols+` FROM trade_offers o `+tradeOfferJoins+` WHERE o.id = $1`, id))
}

// ListMarket returns visible OPEN offers for a viewer: public ones plus private
// offers directed at them, excluding their own. Optional offered/requested
// resource keys (iron/lutinum/water/hydrogen) narrow the list. Returns one page
// and the total match count.
func (s *TradeStore) ListMarket(ctx context.Context, universeCode string, viewerID int64, offered, requested string, page, perPage int) ([]TradeOffer, int, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 30
	}
	if page < 1 {
		page = 1
	}

	conds := []string{
		"o.universe_id = (SELECT id FROM universes WHERE code_name = $1)",
		"o.status = 'OPEN'",
		"o.expires_at > NOW()",
		"o.creator_id <> $2",
		"(o.target_id IS NULL OR o.target_id = $2)",
	}
	args := []any{universeCode, viewerID}
	if col, ok := tradeResCol[offered]; ok {
		args = append(args, 1)
		conds = append(conds, fmt.Sprintf("o.offer_%s > $%d", col, len(args)))
	}
	if col, ok := tradeResCol[requested]; ok {
		args = append(args, 1)
		conds = append(conds, fmt.Sprintf("o.want_%s > $%d", col, len(args)))
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM trade_offers o WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, perPage, (page-1)*perPage)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s
		FROM trade_offers o %s
		WHERE %s
		ORDER BY o.created_at DESC, o.id DESC
		LIMIT $%d OFFSET $%d
	`, tradeOfferCols, tradeOfferJoins, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out, err := scanTradeOffers(rows)
	return out, total, err
}

// MyOffers returns a creator's open offers, newest first.
func (s *TradeStore) MyOffers(ctx context.Context, creatorID int64) ([]TradeOffer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+tradeOfferCols+`
		FROM trade_offers o `+tradeOfferJoins+`
		WHERE o.creator_id = $1 AND o.status = 'OPEN'
		ORDER BY o.created_at DESC, o.id DESC
	`, creatorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTradeOffers(rows)
}

func scanTradeOffers(rows *sql.Rows) ([]TradeOffer, error) {
	var out []TradeOffer
	for rows.Next() {
		o, err := scanTradeOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

// Accept settles an offer atomically: the accepter's wanted basket is debited
// from accepterPlanetID, and the two net baskets are delivered into each side's
// trade storage. The escrowed give basket is consumed. Full accepts only.
func (s *TradeStore) Accept(ctx context.Context, offerID, accepterID, accepterPlanetID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		creatorID     int64
		creatorPlanet int64
		targetID      sql.NullInt64
		give, want    game.Resources
		status        string
		expiresAt     time.Time
	)
	err = tx.QueryRowContext(ctx, `
		SELECT creator_id, planet_id, target_id,
		       offer_metal, offer_lutinum, offer_water, offer_hydrogen,
		       want_metal, want_lutinum, want_water, want_hydrogen,
		       status, expires_at
		FROM trade_offers WHERE id = $1 FOR UPDATE
	`, offerID).Scan(&creatorID, &creatorPlanet, &targetID,
		&give.Metal, &give.Lutinum, &give.Water, &give.Hydrogen,
		&want.Metal, &want.Lutinum, &want.Water, &want.Hydrogen,
		&status, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "OPEN" || !expiresAt.After(time.Now()) {
		return ErrTradeNotOpen
	}
	if creatorID == accepterID {
		return ErrTradeOwn
	}
	if targetID.Valid && targetID.Int64 != accepterID {
		return ErrTradePrivate
	}

	var owner sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT user_id FROM planets WHERE id = $1 FOR UPDATE`, accepterPlanetID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if !owner.Valid || owner.Int64 != accepterID {
		return ErrNotOwner
	}

	have, err := accrueTx(ctx, tx, accepterPlanetID)
	if err != nil {
		return err
	}
	if !affordable(have, want) {
		return ErrInsufficientResources
	}
	if err := deductTx(ctx, tx, accepterPlanetID, want); err != nil {
		return err
	}

	ex := game.Settle(give, want)
	expires := time.Now().Add(game.TradeStorageDuration)
	if err := insertTradeStorageTx(ctx, tx, accepterPlanetID, ex.AccepterGets, expires); err != nil {
		return err
	}
	if err := insertTradeStorageTx(ctx, tx, creatorPlanet, ex.CreatorGets, expires); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE trade_offers SET status = 'ACCEPTED' WHERE id = $1`, offerID); err != nil {
		return err
	}
	if err := insertTradeHistoryTx(ctx, tx, offerID, accepterID, creatorID, "ACCEPT"); err != nil {
		return err
	}
	return tx.Commit()
}

// Revoke cancels the creator's own offer and refunds the escrowed basket to the
// planet. The up-front offer fee is retained.
func (s *TradeStore) Revoke(ctx context.Context, offerID, creatorID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var (
		ownerID  int64
		planetID int64
		give     game.Resources
		status   string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT creator_id, planet_id,
		       offer_metal, offer_lutinum, offer_water, offer_hydrogen, status
		FROM trade_offers WHERE id = $1 FOR UPDATE
	`, offerID).Scan(&ownerID, &planetID,
		&give.Metal, &give.Lutinum, &give.Water, &give.Hydrogen, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if ownerID != creatorID {
		return ErrNotOwner
	}
	if status != "OPEN" {
		return ErrTradeNotOpen
	}
	if err := creditTx(ctx, tx, planetID, give); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE trade_offers SET status = 'REVOKED' WHERE id = $1`, offerID); err != nil {
		return err
	}
	if err := insertTradeHistoryTx(ctx, tx, offerID, creatorID, 0, "REVOKE"); err != nil {
		return err
	}
	return tx.Commit()
}

// ExpireOverdue refunds and closes OPEN offers whose expiry has passed. It is
// called lazily before marketplace/own listings so no scheduler is required.
func (s *TradeStore) ExpireOverdue(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 100
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, creator_id, planet_id,
		       offer_metal, offer_lutinum, offer_water, offer_hydrogen
		FROM trade_offers
		WHERE status = 'OPEN' AND expires_at <= NOW()
		ORDER BY expires_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	if err != nil {
		return err
	}
	type due struct {
		id      int64
		creator int64
		planet  int64
		give    game.Resources
	}
	var dues []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.id, &d.creator, &d.planet,
			&d.give.Metal, &d.give.Lutinum, &d.give.Water, &d.give.Hydrogen); err != nil {
			rows.Close()
			return err
		}
		dues = append(dues, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, d := range dues {
		if err := creditTx(ctx, tx, d.planet, d.give); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE trade_offers SET status = 'EXPIRED' WHERE id = $1`, d.id); err != nil {
			return err
		}
		if err := insertTradeHistoryTx(ctx, tx, d.id, d.creator, 0, "EXPIRE"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// History lists the lifecycle rows a player took part in (as actor or
// counterparty), newest first.
func (s *TradeStore) History(ctx context.Context, userID int64, limit int) ([]TradeHistoryEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.offer_id, h.actor_id, COALESCE(au.username, ''),
		       COALESCE(h.counterparty_id, 0), COALESCE(cu.username, ''),
		       h.action, h.at,
		       o.offer_metal, o.offer_lutinum, o.offer_water, o.offer_hydrogen,
		       o.want_metal, o.want_lutinum, o.want_water, o.want_hydrogen
		FROM trade_history h
		JOIN trade_offers o ON o.id = h.offer_id
		LEFT JOIN users au ON au.id = h.actor_id
		LEFT JOIN users cu ON cu.id = h.counterparty_id
		WHERE h.actor_id = $1 OR h.counterparty_id = $1
		ORDER BY h.at DESC, h.id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TradeHistoryEntry
	for rows.Next() {
		var e TradeHistoryEntry
		if err := rows.Scan(&e.OfferID, &e.ActorID, &e.ActorName,
			&e.CounterpartyID, &e.CounterpartyName, &e.Action, &e.At,
			&e.Give.Metal, &e.Give.Lutinum, &e.Give.Water, &e.Give.Hydrogen,
			&e.Want.Metal, &e.Want.Lutinum, &e.Want.Water, &e.Want.Hydrogen); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Storage returns a planet's pending deliveries, forfeiting any that expired.
func (s *TradeStore) Storage(ctx context.Context, planetID int64) ([]TradeStorageEntry, error) {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM trade_storage WHERE planet_id = $1 AND expires_at <= NOW()`, planetID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, planet_id, metal, lutinum, water, hydrogen, expires_at
		FROM trade_storage WHERE planet_id = $1
		ORDER BY expires_at
	`, planetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TradeStorageEntry
	for rows.Next() {
		var e TradeStorageEntry
		if err := rows.Scan(&e.ID, &e.PlanetID, &e.Resources.Metal, &e.Resources.Lutinum,
			&e.Resources.Water, &e.Resources.Hydrogen, &e.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreditStorage sweeps a planet's trade storage (up to its storage caps) into
// the planet's stock and clears it. Expired entries are forfeited, not credited.
func (s *TradeStore) CreditStorage(ctx context.Context, userID, planetID int64) (game.Resources, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return game.Resources{}, err
	}
	defer tx.Rollback()

	var owner sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT user_id FROM planets WHERE id = $1 FOR UPDATE`, planetID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return game.Resources{}, ErrNotFound
		}
		return game.Resources{}, err
	}
	if !owner.Valid || owner.Int64 != userID {
		return game.Resources{}, ErrNotOwner
	}

	var sum game.Resources
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(metal),0), COALESCE(SUM(lutinum),0),
		       COALESCE(SUM(water),0), COALESCE(SUM(hydrogen),0)
		FROM trade_storage WHERE planet_id = $1 AND expires_at > NOW()
	`, planetID).Scan(&sum.Metal, &sum.Lutinum, &sum.Water, &sum.Hydrogen)

	if _, err := accrueTx(ctx, tx, planetID); err != nil {
		return game.Resources{}, err
	}
	if !sum.IsZero() {
		if err := creditTx(ctx, tx, planetID, sum); err != nil {
			return game.Resources{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM trade_storage WHERE planet_id = $1`, planetID); err != nil {
		return game.Resources{}, err
	}
	if err := tx.Commit(); err != nil {
		return game.Resources{}, err
	}
	return sum, nil
}
