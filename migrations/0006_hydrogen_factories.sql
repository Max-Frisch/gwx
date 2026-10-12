-- ============================================================================
-- gwx — remove OGame-style energy; support two hydrogen factories.
--
-- GalaxyWars has no electricity: nothing is ever switched off for lack of
-- energy. Water -> hydrogen is the only "fuel" coupling. The Chemical Factory
-- runs at 5:1 and the Advanced Chemical Factory at 2:1; both draw from the same
-- stored water pool (regardless of where that water came from) and their
-- outputs add up. If the pool cannot cover a tick's combined draw, both
-- factories throttle by the same fraction and stop entirely at zero water.
--
-- Hydrogen output rate lives in `hydrogen_prod`; the matching water draw lives
-- in the new `water_use` column (5 × H2_base + 2 × H2_advanced).
-- ============================================================================

-- OGame energy is not a GalaxyWars concept — drop it everywhere.
ALTER TABLE planets DROP COLUMN IF EXISTS energy_used;
ALTER TABLE planets DROP COLUMN IF EXISTS energy_prod;

ALTER TABLE planets ADD COLUMN IF NOT EXISTS water_use NUMERIC(16,4) NOT NULL DEFAULT 0;

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
    v_mprod NUMERIC; v_lprod NUMERIC; v_wprod NUMERIC; v_hprod NUMERIC; v_wuse NUMERIC;
    v_hydrogen_made NUMERIC;
    v_water_need NUMERIC;
BEGIN
    SELECT last_resource_calc_at, metal, lutinum, water, hydrogen,
           metal_cap, lutinum_cap, water_cap, hydrogen_cap,
           metal_prod, lutinum_prod, water_prod, hydrogen_prod, water_use
    INTO v_last, v_metal, v_lutinum, v_water, v_hydrogen,
         v_mcap, v_lcap, v_wcap, v_hcap,
         v_mprod, v_lprod, v_wprod, v_hprod, v_wuse
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

        -- Water accrues first, then the factories draw from the shared pool.
        v_water := v_water + (v_wprod/3600.0)*v_elapsed;

        v_hydrogen_made := (v_hprod/3600.0)*v_elapsed;
        v_water_need    := (v_wuse/3600.0)*v_elapsed;

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
    END IF;

    RETURN QUERY SELECT v_metal, v_lutinum, v_water, v_hydrogen, v_now;
END;
$$;
