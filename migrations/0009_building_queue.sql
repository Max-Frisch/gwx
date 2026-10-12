-- ============================================================================
-- gwx — I10 construction queue: multiple building orders per planet.
--
-- GalaxyWars lets a planet queue several building orders at once (the reference
-- panel holds 6). The old `uq_queue_building` partial unique index allowed only
-- one active building order per planet; drop it. The cap is now enforced in Go
-- (QueueStore.StartBuilding) while holding the planet row lock, and the slot
-- count lives on the universe so operators tune it per round.
--
-- Research and ship/defense keep their single-order indexes untouched.
-- ============================================================================

ALTER TABLE universes ADD COLUMN IF NOT EXISTS build_queue_slots SMALLINT NOT NULL DEFAULT 6;

DROP INDEX IF EXISTS uq_queue_building;
