-- ============================================================================
-- gwx — research queue: multiple research orders per account.
--
-- Mirrors the construction queue (0009): an account may chain several research
-- orders at once (the reference panel holds 4). The old `uq_queue_research`
-- partial unique index allowed only one active research order per account; drop
-- it. The cap is now enforced in Go (QueueStore.StartResearch) while holding the
-- user row lock, and the slot count lives on the universe so operators tune it
-- per round.
--
-- Ship/defense keep their single-order index untouched.
-- ============================================================================

ALTER TABLE universes ADD COLUMN IF NOT EXISTS research_queue_slots SMALLINT NOT NULL DEFAULT 4;

DROP INDEX IF EXISTS uq_queue_research;
