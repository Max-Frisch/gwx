-- ============================================================================
-- gwx — Colony Builder: automatic, template-driven construction.
--
-- A player defines an account-wide target level per building (build_template)
-- and flips builder_enabled on. The durable scheduler then fills every owned
-- planet's free construction slots toward that template, reusing the ordinary
-- QueueStore.StartBuilding path (cost/prereq/field/slot checks included).
--
-- build_template            — code -> target level, as JSON.
-- builder_enabled           — whether the scheduler should act for the account.
-- builder_next_check_at     — lease: when the account is next due. NULL means
--   "due now". The scheduler parks it forward while it works, so the phase is
--   restart- and multi-instance-safe (FOR UPDATE SKIP LOCKED).
-- ============================================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS build_template JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE users ADD COLUMN IF NOT EXISTS builder_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS builder_next_check_at TIMESTAMPTZ;

-- The scheduler polls only enabled, due accounts; a partial index keeps that
-- scan tiny.
CREATE INDEX IF NOT EXISTS idx_users_builder_due
    ON users (builder_next_check_at) WHERE builder_enabled;
