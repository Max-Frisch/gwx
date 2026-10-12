-- ============================================================================
-- gwx — MESSAGES (I2). One flat table for the player inbox: system
-- notifications (building/research/fleet events) and pointers to combat /
-- espionage reports. Idempotent so it also lands on existing databases.
-- ============================================================================

CREATE TABLE IF NOT EXISTS messages (
    id          BIGSERIAL PRIMARY KEY,
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category    VARCHAR(16) NOT NULL CHECK (category IN ('system','report')),
    kind        VARCHAR(24) NOT NULL,
    title       VARCHAR(200) NOT NULL,
    body        TEXT NOT NULL DEFAULT '',
    report_id   BIGINT REFERENCES reports(id) ON DELETE SET NULL,
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    archived_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_user ON messages (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages (user_id)
    WHERE NOT is_read AND archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_messages_report ON messages (report_id)
    WHERE report_id IS NOT NULL;
