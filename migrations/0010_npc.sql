-- ============================================================================
-- gwx — NPC players.
--
-- NPCs are ordinary `users` rows flagged `is_npc`, so every existing FK and
-- gameplay path (queues, fleets, reports, highscore, galaxy, search) keeps
-- working unchanged. They can never be logged into (login blocks the flag and
-- their password hash is a non-verifiable sentinel).
--
-- npc_profile  — behaviour tunables (strategy/difficulty/phase/activity) as JSON.
-- npc_next_decision_at — when the durable scheduler should next let the NPC
--   think. NULL means "due now" (freshly seeded). The scheduler leases an NPC by
--   pushing this forward, so it stays restart- and multi-instance-safe.
-- ============================================================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_npc BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS npc_profile JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE users ADD COLUMN IF NOT EXISTS npc_next_decision_at TIMESTAMPTZ;

-- The scheduler polls only due NPCs; a partial index keeps that scan tiny.
CREATE INDEX IF NOT EXISTS idx_users_npc_due
    ON users (npc_next_decision_at) WHERE is_npc;
