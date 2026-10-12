# I2 — Messages / inbox

**Kind:** build · **Depends on:** E5 (spec `10-messages-events-search.md`)
**Commit:** `feat: message centre (system + report notifications)`

## Goal

Give the player an inbox: system notifications and pointers to combat/espionage
reports, plus the ability to mark read / delete / archive. This is the gwx-sized
subset of the reference message centre.

## Design (gwx-appropriate)

- One table (migration `0002_messages.sql`) rather than the reference's category
  tree. Columns: `id, universe_id, user_id, category, kind, title, body,
  report_id (nullable FK), is_read, created_at`.
  - `category` ∈ `system` (building/research/fleet events) | `report` (combat/espionage).
  - `kind` examples: `building`, `research`, `ship`, `own-fleet`, `other-fleet`,
    `system`, `attack`, `spy`, `defense`.
- `report_id` links a `report` message to the existing `reports` row so the page
  can deep-link to `/reports` (or render the payload inline).
- Emit messages from `internal/engine/event_engine.go` where we already write
  `reports` (search for `INSERT INTO reports`) and where queues/fleets resolve.
  Keep it a single helper, e.g. `insertMessageTx(...)`.

## Files

- `migrations/0002_messages.sql` (idempotent `CREATE TABLE IF NOT EXISTS` + indexes).
- `internal/store/message_store.go`: `Insert`, `List(ctx,userID,page,perPage,
  filter)`, `UnreadCount`, `MarkRead`, `Delete`, `Archive`.
- `internal/web/web.go`: `GET /messages`, `POST /messages/{id}/read`,
  `POST /messages/{id}/delete`. Show the unread count in the base template.
- `internal/web/data.go`: `messagesData` view struct.
- `internal/web/templates/messages.html`; nav entry under COMMUNITY.

## Definition of done

- `make migrate` applies cleanly on an existing DB (idempotent).
- Build an order / run an attack → a message appears; unread badge shows; opening
  it marks read; report messages link to the matching report.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean.

## Notes

- Do **not** build full player-to-player conversations (that is alliance/trade
  territory); a single `conversation` category may be added later.
- Keep pagination simple (limit/offset) and reuse `humanDur`/`until` formatting.
