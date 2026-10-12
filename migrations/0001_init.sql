-- ============================================================================
-- gwx — a deliberately small GalaxyWars-style engine.
-- Postgres 16+. One catalog-keyed schema, a durable event scheduler, and
-- on-demand resource accrual (no cron).
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- UNIVERSES
-- ============================================================================
CREATE TABLE IF NOT EXISTS universes (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code_name      VARCHAR(64) UNIQUE NOT NULL,
    display_name   VARCHAR(128) NOT NULL,
    game_speed     NUMERIC(10,2) NOT NULL DEFAULT 1.00,
    resource_speed NUMERIC(10,2) NOT NULL DEFAULT 1.00,
    fleet_speed    NUMERIC(10,2) NOT NULL DEFAULT 1.00,
    max_galaxies   SMALLINT NOT NULL DEFAULT 3,
    max_systems    SMALLINT NOT NULL DEFAULT 199,
    ends_at        TIMESTAMPTZ,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- USERS & SESSIONS
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    universe_id   UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    username      VARCHAR(64) NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    auth_role     VARCHAR(32) NOT NULL DEFAULT 'PLAYER',
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_universe_username UNIQUE (universe_id, username),
    CONSTRAINT uq_universe_email UNIQUE (universe_id, email)
);

-- Per-account UI/game preferences (interface settings etc.). Idempotent so it
-- also lands on databases created before the column existed.
ALTER TABLE users ADD COLUMN IF NOT EXISTS settings JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS sessions (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_agent   TEXT,
    ip           TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user   ON sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions (expires_at);

-- ============================================================================
-- PLANETS
-- ============================================================================
CREATE TABLE IF NOT EXISTS planets (
    id           BIGSERIAL PRIMARY KEY,
    universe_id  UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    user_id      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name         VARCHAR(64) NOT NULL DEFAULT 'Colony',
    galaxy       SMALLINT NOT NULL CHECK (galaxy BETWEEN 1 AND 9),
    system       SMALLINT NOT NULL CHECK (system BETWEEN 1 AND 999),
    position     SMALLINT NOT NULL CHECK (position BETWEEN 1 AND 15),
    is_homeworld BOOLEAN NOT NULL DEFAULT FALSE,
    diameter_km  INT NOT NULL DEFAULT 12800,
    fields_max   INT NOT NULL DEFAULT 120,
    fields_used  INT NOT NULL DEFAULT 0,

    -- Resource stock + the derived hourly rates / ceilings.
    metal                NUMERIC(24,4) NOT NULL DEFAULT 1000 CHECK (metal >= 0),
    lutinum              NUMERIC(24,4) NOT NULL DEFAULT 1000 CHECK (lutinum >= 0),
    water                NUMERIC(24,4) NOT NULL DEFAULT 500  CHECK (water >= 0),
    hydrogen             NUMERIC(24,4) NOT NULL DEFAULT 0    CHECK (hydrogen >= 0),
    metal_cap            BIGINT NOT NULL DEFAULT 20000,
    lutinum_cap          BIGINT NOT NULL DEFAULT 20000,
    water_cap            BIGINT NOT NULL DEFAULT 20000,
    hydrogen_cap         BIGINT NOT NULL DEFAULT 20000,
    metal_prod           NUMERIC(16,4) NOT NULL DEFAULT 0,
    lutinum_prod         NUMERIC(16,4) NOT NULL DEFAULT 0,
    water_prod           NUMERIC(16,4) NOT NULL DEFAULT 0,
    hydrogen_prod        NUMERIC(16,4) NOT NULL DEFAULT 0,
    energy_used          BIGINT NOT NULL DEFAULT 0,
    energy_prod          BIGINT NOT NULL DEFAULT 30,
    protected_metal      BIGINT NOT NULL DEFAULT 0,
    protected_lutinum    BIGINT NOT NULL DEFAULT 0,
    protected_water      BIGINT NOT NULL DEFAULT 0,
    protected_hydrogen   BIGINT NOT NULL DEFAULT 0,

    last_resource_calc_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_universe_coords UNIQUE (universe_id, galaxy, system, position)
);
CREATE INDEX IF NOT EXISTS idx_planets_user ON planets(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_planets_scan ON planets(universe_id, galaxy, system);

-- ============================================================================
-- PLANET ITEMS: buildings, ships, defense all keyed by catalog code.
-- amount = level for buildings, count for ships/defense.
-- ============================================================================
CREATE TABLE IF NOT EXISTS planet_items (
    planet_id BIGINT NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    kind      VARCHAR(16) NOT NULL CHECK (kind IN ('building','ship','defense')),
    code      VARCHAR(40) NOT NULL,
    amount    BIGINT NOT NULL DEFAULT 0 CHECK (amount >= 0),
    PRIMARY KEY (planet_id, kind, code)
);

-- ============================================================================
-- RESEARCH: account-wide levels.
-- ============================================================================
CREATE TABLE IF NOT EXISTS user_techs (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code    VARCHAR(40) NOT NULL,
    level   INT NOT NULL DEFAULT 0 CHECK (level >= 0),
    PRIMARY KEY (user_id, code)
);

-- ============================================================================
-- QUEUES: one generic table drives construction, research and ship/defense
-- production. kind + nullable target_level/quantity cover all three shapes.
-- ============================================================================
CREATE TABLE IF NOT EXISTS queues (
    id            BIGSERIAL PRIMARY KEY,
    universe_id   UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    planet_id     BIGINT NOT NULL REFERENCES planets(id) ON DELETE CASCADE,
    kind          VARCHAR(16) NOT NULL CHECK (kind IN ('building','research','ship','defense')),
    code          VARCHAR(40) NOT NULL,
    target_level  INT,     -- building/research
    quantity      BIGINT,  -- ship/defense
    quantity_done BIGINT NOT NULL DEFAULT 0,
    start_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finish_at     TIMESTAMPTZ NOT NULL,
    status        VARCHAR(16) NOT NULL DEFAULT 'IN_PROGRESS'
                  CHECK (status IN ('IN_PROGRESS','COMPLETED','CANCELLED')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_queue_timeline CHECK (start_at < finish_at)
);

-- One active build per planet, one active research per account, one active
-- ship/defense order per planet. Concurrent inserts collide -> ErrQueueBusy.
CREATE UNIQUE INDEX IF NOT EXISTS uq_queue_building ON queues(planet_id)
    WHERE kind = 'building' AND status = 'IN_PROGRESS';
CREATE UNIQUE INDEX IF NOT EXISTS uq_queue_research ON queues(user_id)
    WHERE kind = 'research' AND status = 'IN_PROGRESS';
CREATE UNIQUE INDEX IF NOT EXISTS uq_queue_shipyard ON queues(planet_id)
    WHERE kind IN ('ship','defense') AND status = 'IN_PROGRESS';
CREATE INDEX IF NOT EXISTS idx_queue_due ON queues(finish_at) WHERE status = 'IN_PROGRESS';

-- ============================================================================
-- FLEETS
-- ============================================================================
CREATE TABLE IF NOT EXISTS fleets (
    id           BIGSERIAL PRIMARY KEY,
    universe_id  UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mission      VARCHAR(16) NOT NULL CHECK (mission IN ('COLONIZE','TRANSPORT','DEPLOY','ATTACK','ESPIONAGE')),
    phase        VARCHAR(16) NOT NULL DEFAULT 'OUTBOUND'
                 CHECK (phase IN ('OUTBOUND','RETURNING','RESOLVED','CANCELLED')),

    origin_id    BIGINT NOT NULL REFERENCES planets(id) ON DELETE RESTRICT,
    target_id    BIGINT REFERENCES planets(id) ON DELETE RESTRICT,

    origin_galaxy SMALLINT NOT NULL,
    origin_system SMALLINT NOT NULL,
    origin_pos    SMALLINT NOT NULL,
    target_galaxy SMALLINT NOT NULL,
    target_system SMALLINT NOT NULL,
    target_pos    SMALLINT NOT NULL,

    start_at     TIMESTAMPTZ NOT NULL,
    arrival_at   TIMESTAMPTZ NOT NULL,
    return_at    TIMESTAMPTZ,

    speed_pct    NUMERIC(4,2) NOT NULL DEFAULT 1.00 CHECK (speed_pct BETWEEN 0.10 AND 1.00),
    hydrogen_cost BIGINT NOT NULL DEFAULT 0,

    cargo_metal    BIGINT NOT NULL DEFAULT 0 CHECK (cargo_metal >= 0),
    cargo_lutinum  BIGINT NOT NULL DEFAULT 0 CHECK (cargo_lutinum >= 0),
    cargo_water    BIGINT NOT NULL DEFAULT 0 CHECK (cargo_water >= 0),
    cargo_hydrogen BIGINT NOT NULL DEFAULT 0 CHECK (cargo_hydrogen >= 0),

    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_fleet_timeline CHECK (start_at < arrival_at)
);
CREATE INDEX IF NOT EXISTS idx_fleet_due ON fleets(arrival_at)
    WHERE phase IN ('OUTBOUND','RETURNING');
CREATE INDEX IF NOT EXISTS idx_fleet_user ON fleets(user_id)
    WHERE phase IN ('OUTBOUND','RETURNING');

CREATE TABLE IF NOT EXISTS fleet_units (
    fleet_id BIGINT NOT NULL REFERENCES fleets(id) ON DELETE CASCADE,
    code     VARCHAR(40) NOT NULL,
    count    BIGINT NOT NULL CHECK (count > 0),
    PRIMARY KEY (fleet_id, code)
);

-- ============================================================================
-- REPORTS (combat + espionage), stored as JSONB payloads.
-- ============================================================================
CREATE TABLE IF NOT EXISTS reports (
    id           BIGSERIAL PRIMARY KEY,
    universe_id  UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind         VARCHAR(16) NOT NULL CHECK (kind IN ('combat','espionage')),
    title        VARCHAR(160) NOT NULL,
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_reports_user ON reports(user_id, created_at DESC);

-- ============================================================================
-- COORDINATE LOCKS: a freshly abandoned slot stays reserved briefly.
-- ============================================================================
CREATE TABLE IF NOT EXISTS coordinate_locks (
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    galaxy      SMALLINT NOT NULL,
    system      SMALLINT NOT NULL,
    position    SMALLINT NOT NULL,
    locked_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (universe_id, galaxy, system, position)
);

-- ============================================================================
-- ON-DEMAND RESOURCE ACCRUAL (zero-cron).
-- Water is consumed 1:1 to synthesise hydrogen: the chemical factory only runs
-- as fast as water is available. Row-locks the planet to stay race-free.
-- ============================================================================
CREATE OR REPLACE FUNCTION update_planet_resources(p_planet_id BIGINT)
RETURNS TABLE (
    out_metal NUMERIC(24,4),
    out_lutinum NUMERIC(24,4),
    out_water NUMERIC(24,4),
    out_hydrogen NUMERIC(24,4),
    out_last_calc TIMESTAMPTZ
)
LANGUAGE plpgsql AS $$
DECLARE
    v_now TIMESTAMPTZ := NOW();
    v_last TIMESTAMPTZ;
    v_elapsed NUMERIC;
    v_metal NUMERIC; v_lutinum NUMERIC; v_water NUMERIC; v_hydrogen NUMERIC;
    v_mcap BIGINT; v_lcap BIGINT; v_wcap BIGINT; v_hcap BIGINT;
    v_mprod NUMERIC; v_lprod NUMERIC; v_wprod NUMERIC; v_hprod NUMERIC;
    v_hydrogen_made NUMERIC;
BEGIN
    SELECT last_resource_calc_at, metal, lutinum, water, hydrogen,
           metal_cap, lutinum_cap, water_cap, hydrogen_cap,
           metal_prod, lutinum_prod, water_prod, hydrogen_prod
    INTO v_last, v_metal, v_lutinum, v_water, v_hydrogen,
         v_mcap, v_lcap, v_wcap, v_hcap,
         v_mprod, v_lprod, v_wprod, v_hprod
    FROM planets
    WHERE id = p_planet_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'planet % not found', p_planet_id;
    END IF;

    v_elapsed := EXTRACT(EPOCH FROM (v_now - v_last));

    IF v_elapsed > 0 THEN
        v_metal   := LEAST(v_mcap, v_metal   + (v_mprod/3600.0)*v_elapsed);
        v_lutinum := LEAST(v_lcap, v_lutinum + (v_lprod/3600.0)*v_elapsed);

        -- Coupled water -> hydrogen conversion.
        v_water := v_water + (v_wprod/3600.0)*v_elapsed;
        v_hydrogen_made := LEAST((v_hprod/3600.0)*v_elapsed, v_water);
        v_water := v_water - v_hydrogen_made;
        v_water := LEAST(v_wcap, v_water);
        v_hydrogen := LEAST(v_hcap, v_hydrogen + v_hydrogen_made);

        UPDATE planets
        SET metal = v_metal, lutinum = v_lutinum, water = v_water, hydrogen = v_hydrogen,
            last_resource_calc_at = v_now
        WHERE id = p_planet_id;
    END IF;

    RETURN QUERY SELECT v_metal, v_lutinum, v_water, v_hydrogen, v_now;
END;
$$;
