-- ============================================================================
-- gwx — PLAYER TRADING (I7). A player-to-player marketplace: barter offers with
-- escrowed goods, an atomic settlement into per-planet trade storage, an audit
-- history and lazy expiry. Idempotent so it also lands on existing databases.
-- ============================================================================

-- One offer: the creator gives the offer_* basket and wants the want_* basket.
-- target_id NULL = public offer; otherwise it is directed at one player.
-- The given basket is escrowed (debited from planet_id) while the offer is open.
CREATE TABLE IF NOT EXISTS trade_offers (
    id             BIGSERIAL PRIMARY KEY,
    universe_id    UUID   NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    creator_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    planet_id      BIGINT NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    target_id      BIGINT REFERENCES users(id) ON DELETE CASCADE,

    offer_metal    NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (offer_metal >= 0),
    offer_lutinum  NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (offer_lutinum >= 0),
    offer_water    NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (offer_water >= 0),
    offer_hydrogen NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (offer_hydrogen >= 0),
    want_metal     NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (want_metal >= 0),
    want_lutinum   NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (want_lutinum >= 0),
    want_water     NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (want_water >= 0),
    want_hydrogen  NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (want_hydrogen >= 0),

    status         VARCHAR(16) NOT NULL DEFAULT 'OPEN'
                   CHECK (status IN ('OPEN','ACCEPTED','REVOKED','EXPIRED')),
    expires_at     TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_trade_offers_open
    ON trade_offers (universe_id, created_at DESC) WHERE status = 'OPEN';
CREATE INDEX IF NOT EXISTS idx_trade_offers_planet
    ON trade_offers (planet_id) WHERE status = 'OPEN';
CREATE INDEX IF NOT EXISTS idx_trade_offers_target
    ON trade_offers (target_id) WHERE target_id IS NOT NULL AND status = 'OPEN';
CREATE INDEX IF NOT EXISTS idx_trade_offers_due
    ON trade_offers (expires_at) WHERE status = 'OPEN';

-- Audit trail: one row per lifecycle action (create / accept / revoke / expire).
CREATE TABLE IF NOT EXISTS trade_history (
    id              BIGSERIAL PRIMARY KEY,
    offer_id        BIGINT NOT NULL REFERENCES trade_offers(id) ON DELETE CASCADE,
    actor_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    counterparty_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action          VARCHAR(16) NOT NULL CHECK (action IN ('CREATE','ACCEPT','REVOKE','EXPIRE')),
    at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_trade_history_actor ON trade_history (actor_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_trade_history_counterparty ON trade_history (counterparty_id, at DESC);

-- Delivered goods sit here until collected ("Credit all") or until they expire
-- and are forfeited. Kept separate from the planet's stock so storage caps and
-- the protected floor never silently apply to a trade delivery.
CREATE TABLE IF NOT EXISTS trade_storage (
    id        BIGSERIAL PRIMARY KEY,
    planet_id BIGINT NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    metal     NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (metal >= 0),
    lutinum   NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (lutinum >= 0),
    water     NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (water >= 0),
    hydrogen  NUMERIC(24,4) NOT NULL DEFAULT 0 CHECK (hydrogen >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_trade_storage_planet ON trade_storage (planet_id, expires_at);
