-- ============================================================================
-- gwx — PRODUCTION MODEL correction (no energy).
--
-- GalaxyWars has no energy/electricity: extraction buildings always run at 100%.
-- The Chemical Factory converts water into hydrogen at the reference 5:1 ratio
-- (consumption = 5 x hydrogen produced), and hydrogen output stops when water
-- runs dry. Replaces the old 1:1 coupling from 0001_init.sql.
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

        -- Coupled water -> hydrogen conversion at the reference 5:1 ratio.
        v_water := v_water + (v_wprod/3600.0)*v_elapsed;
        v_hydrogen_made := LEAST((v_hprod/3600.0)*v_elapsed, v_water/5.0);
        v_water := v_water - v_hydrogen_made*5.0;
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
