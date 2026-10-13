-- ============================================================================
-- gwx — ALLIANCE DIPLOMACY (follow-up to I6). NAP / war declarations between
-- two alliances with a proposal -> accept / reject / withdraw / end lifecycle.
-- War is unilateral (immediate); a NAP is bilateral (proposal -> accept).
-- Idempotent so it also lands on existing databases.
-- ============================================================================

CREATE TABLE IF NOT EXISTS alliance_diplomacy (
    id          BIGSERIAL PRIMARY KEY,
    universe_id UUID        NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    proposer_id BIGINT      NOT NULL REFERENCES alliances(id) ON DELETE CASCADE,
    target_id   BIGINT      NOT NULL REFERENCES alliances(id) ON DELETE CASCADE,
    kind        VARCHAR(8)  NOT NULL CHECK (kind IN ('WAR','NAP')),
    status      VARCHAR(12) NOT NULL DEFAULT 'PROPOSED'
                CHECK (status IN ('PROPOSED','ACTIVE','REJECTED','WITHDRAWN','ENDED')),
    message     TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at  TIMESTAMPTZ,
    CONSTRAINT ck_diplomacy_distinct CHECK (proposer_id <> target_id)
);
CREATE INDEX IF NOT EXISTS idx_diplomacy_proposer ON alliance_diplomacy (proposer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_diplomacy_target   ON alliance_diplomacy (target_id, created_at DESC);

-- At most one live (proposed or active) relationship per unordered alliance pair.
CREATE UNIQUE INDEX IF NOT EXISTS uq_diplomacy_pair_live
    ON alliance_diplomacy (LEAST(proposer_id, target_id), GREATEST(proposer_id, target_id))
    WHERE status IN ('PROPOSED', 'ACTIVE');
