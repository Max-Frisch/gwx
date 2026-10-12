-- ============================================================================
-- gwx — ALLIANCES (I6). Membership, a fixed rank model (MEMBER/OFFICER/LEADER),
-- an application flow and one shared board. Idempotent so it also lands on
-- existing databases.
-- ============================================================================

CREATE TABLE IF NOT EXISTS alliances (
    id          BIGSERIAL PRIMARY KEY,
    universe_id UUID NOT NULL REFERENCES universes(id) ON DELETE CASCADE,
    tag         VARCHAR(8)  NOT NULL,
    name        VARCHAR(64) NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    founder_id  BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_open     BOOLEAN     NOT NULL DEFAULT TRUE,
    min_points  INT         NOT NULL DEFAULT 0 CHECK (min_points >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_alliance_tag UNIQUE (universe_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_alliances_universe ON alliances (universe_id, created_at DESC);

-- One alliance per player: the unique user_id enforces it.
CREATE TABLE IF NOT EXISTS alliance_members (
    alliance_id BIGINT      NOT NULL REFERENCES alliances(id) ON DELETE CASCADE,
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rank        VARCHAR(16) NOT NULL DEFAULT 'MEMBER'
                CHECK (rank IN ('MEMBER','OFFICER','LEADER')),
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (alliance_id, user_id),
    CONSTRAINT uq_alliance_member_user UNIQUE (user_id)
);
CREATE INDEX IF NOT EXISTS idx_alliance_members_alliance ON alliance_members (alliance_id, joined_at);

-- One open application per player per alliance (re-applying after a rejection
-- upserts the same row).
CREATE TABLE IF NOT EXISTS alliance_applications (
    id          BIGSERIAL PRIMARY KEY,
    alliance_id BIGINT      NOT NULL REFERENCES alliances(id) ON DELETE CASCADE,
    user_id     BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message     TEXT        NOT NULL DEFAULT '',
    status      VARCHAR(16) NOT NULL DEFAULT 'PENDING'
                CHECK (status IN ('PENDING','ACCEPTED','REJECTED','WITHDRAWN')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_alliance_application UNIQUE (alliance_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_alliance_apps_pending ON alliance_applications (alliance_id)
    WHERE status = 'PENDING';

-- One flat shared board: a post is a title + body, optionally pinned.
CREATE TABLE IF NOT EXISTS alliance_posts (
    id          BIGSERIAL PRIMARY KEY,
    alliance_id BIGINT      NOT NULL REFERENCES alliances(id) ON DELETE CASCADE,
    author_id   BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       VARCHAR(120) NOT NULL DEFAULT '',
    body        TEXT        NOT NULL,
    is_pinned   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_alliance_posts ON alliance_posts (alliance_id, is_pinned DESC, created_at DESC);
