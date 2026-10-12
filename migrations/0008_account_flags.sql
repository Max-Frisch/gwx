-- ============================================================================
-- gwx — I9 account lifecycle: vacation mode + sanctions.
--
-- Vacation mode freezes resource production and pauses build/research/unit
-- queues for the duration. Production is frozen inside update_planet_resources
-- by discounting any elapsed time that overlaps the vacation window
-- [vacation_since, LEAST(NOW(), vacation_until)]. The scheduler pauses queues
-- (it skips vacationing users) and pushes their finish times back when the
-- vacation ends, so no build time is lost while away.
--
-- Vacation lasts at least 12h (manual leave gate, enforced in Go) and at most
-- 21 days (vacation_until cap, auto-ended by the scheduler).
-- ============================================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS vacation_since TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS vacation_until TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS sanctioned     BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS sanction_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_users_vacation
    ON users (vacation_until) WHERE vacation_since IS NOT NULL;

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
    v_owner BIGINT;
    v_vs TIMESTAMPTZ;
    v_vu TIMESTAMPTZ;
    v_elapsed NUMERIC;
    v_eff NUMERIC;
    v_metal NUMERIC; v_lutinum NUMERIC; v_water NUMERIC; v_hydrogen NUMERIC;
    v_mcap BIGINT; v_lcap BIGINT; v_wcap BIGINT; v_hcap BIGINT;
    v_mprod NUMERIC; v_lprod NUMERIC; v_wprod NUMERIC; v_hprod NUMERIC; v_wuse NUMERIC;
    v_hydrogen_made NUMERIC;
    v_water_need NUMERIC;
BEGIN
    SELECT last_resource_calc_at, metal, lutinum, water, hydrogen,
           metal_cap, lutinum_cap, water_cap, hydrogen_cap,
           metal_prod, lutinum_prod, water_prod, hydrogen_prod, water_use, user_id
    INTO v_last, v_metal, v_lutinum, v_water, v_hydrogen,
         v_mcap, v_lcap, v_wcap, v_hcap,
         v_mprod, v_lprod, v_wprod, v_hprod, v_wuse, v_owner
    FROM planets
    WHERE id = p_planet_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'planet % not found', p_planet_id;
    END IF;

    IF v_owner IS NOT NULL THEN
        SELECT vacation_since, vacation_until INTO v_vs, v_vu
        FROM users WHERE id = v_owner;
    END IF;

    v_elapsed := EXTRACT(EPOCH FROM (v_now - v_last));

    -- Discount any elapsed time that fell inside the vacation window. This
    -- freezes production whether the planet is queried during or only after the
    -- vacation ends.
    v_eff := v_elapsed;
    IF v_vs IS NOT NULL AND v_vu IS NOT NULL THEN
        v_eff := GREATEST(0, EXTRACT(EPOCH FROM (LEAST(v_now, v_vs) - v_last)))
               + GREATEST(0, EXTRACT(EPOCH FROM (v_now - GREATEST(v_last, LEAST(v_now, v_vu)))));
    END IF;

    IF v_eff > 0 THEN
        v_metal   := LEAST(v_mcap, v_metal   + (v_mprod/3600.0)*v_eff);
        v_lutinum := LEAST(v_lcap, v_lutinum + (v_lprod/3600.0)*v_eff);

        -- Water accrues first, then the factories draw from the shared pool.
        v_water := v_water + (v_wprod/3600.0)*v_eff;

        v_hydrogen_made := (v_hprod/3600.0)*v_eff;
        v_water_need    := (v_wuse/3600.0)*v_eff;

        -- Not enough water for a full tick: throttle both factories together
        -- (they run at 100% until the pool is drained, then stop).
        IF v_water_need > v_water THEN
            v_hydrogen_made := v_hydrogen_made * (v_water / v_water_need);
            v_water_need    := v_water;
        END IF;

        v_water := v_water - v_water_need;
        v_water := LEAST(v_wcap, v_water);
        v_hydrogen := LEAST(v_hcap, v_hydrogen + v_hydrogen_made);

        UPDATE planets
        SET metal = v_metal, lutinum = v_lutinum, water = v_water, hydrogen = v_hydrogen,
            last_resource_calc_at = v_now
        WHERE id = p_planet_id;
    ELSE
        -- Still advance the clock so a long vacation does not bank up when the
        -- player returns.
        UPDATE planets SET last_resource_calc_at = v_now WHERE id = p_planet_id;
    END IF;

    RETURN QUERY SELECT v_metal, v_lutinum, v_water, v_hydrogen, v_now;
END;
$$;
