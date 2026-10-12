-- ============================================================================
-- gwx — I8 defense roster: prune gwx-specific placeholder emplacements.
--
-- The roster now matches GalaxyWars Uni5 exactly (5 units: Light Laser Turret,
-- Laser Turret, EMP Launcher, Plasma Turret, Rocket Launchers). The old
-- gwx-specific placeholders no longer exist in the catalog, so any built units
-- or in-flight production orders for them are dropped. `light_laser`,
-- `heavy_laser` and `plasma_turret` keep their codes and are retuned, so their
-- existing rows are intentionally left untouched.
-- ============================================================================

DELETE FROM queues
WHERE kind = 'defense'
  AND code IN ('defense_turret', 'ion_cannon', 'gauss_cannon', 'small_shield_dome', 'large_shield_dome');

DELETE FROM planet_items
WHERE kind = 'defense'
  AND code IN ('defense_turret', 'ion_cannon', 'gauss_cannon', 'small_shield_dome', 'large_shield_dome');
