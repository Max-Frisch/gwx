# I6 — Alliances

**Kind:** build · **Depends on:** E3 (spec `08-alliance.md`)
**Commit:** `feat: alliances (membership, ranks, applications, board)`

## Goal

A self-contained alliance subsystem: create/join, membership, a simple rank/rights
model, an application flow, and one shared board. Start with the minimal gwx
subset in `08-alliance.md`; do not chase the full reference feature set in one go.

## Scope (recommended MVP)

- **Tables** (migration `0003_alliances.sql`):
  - `alliances (id, universe_id, tag UNIQUE per universe, name, description,
    founder_id, is_open, min_points, created_at)`
  - `alliance_members (alliance_id, user_id, rank, joined_at)`
  - `alliance_applications (id, alliance_id, user_id, message, status, created_at)`
  - `alliance_posts (id, alliance_id, author_id, title, body, created_at)`
  - `alliance_ranks` or a fixed rank enum (`MEMBER`/`OFFICER`/`LEADER`) with a
    small rights table.
- **Store:** `internal/store/alliance_store.go`.
- **Web:** `/alliance` (overview/apply/search), `/alliance/{id}`, internal
  dashboard (members, applications, board, settings). Nav under COMMUNITY.
  Update the highscore Alliances tab (I5) once this lands.
- **Display:** show `[TAG]` next to usernames in galaxy/highscore (reference does).

## Definition of done

- Two accounts can create/apply/accept, see each other in members and post to the
  board; the `[TAG]` shows in galaxy + highscore.
- `make migrate` idempotent on an existing DB.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean,
  with store tests for the membership/rights rules.

## Notes

- Rights checks belong in the store (transactional), not only the handler.
- Keep diplomacy (NAP/war) for a follow-up; the BACKLOG lists it separately.
