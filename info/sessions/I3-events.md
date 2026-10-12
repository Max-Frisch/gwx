# I3 — Events feed

**Kind:** build · **Depends on:** E5 (spec `10-messages-events-search.md`)
**Commit:** `feat: events feed (queues + fleets timeline)`

## Goal

Add an `/events` page that lists the player's upcoming and recent activity in one
timeline: running builds/research/unit orders (from `queues`) and fleet movements
(from `fleets`, both outgoing and incoming). This is **derived** data — no new
table needed.

## Design

- Read `queues` for the user (`internal/store/queue_store.go`, `Active`) and
  `fleets` both active and incoming (`internal/store/fleet_store.go`,
  `ListActive` / `Incoming`).
- Render a single chronological list with: type, what, where (coords), status
  (in progress / arriving / done-in), and a link to the relevant page
  (`/buildings/{id}`, `/research/{id}`, `/fleet/{id}`, overview).
- Optionally include the last N resolved items via `reports`/`messages` (only if
  I2 is implemented; otherwise skip).

## Files

- `internal/web/web.go`: `GET /events` (requireUser).
- `internal/web/data.go`: `eventsData` + `eventRow`.
- `internal/web/templates/events.html`; nav entry under UNIVERSE (replacing the
  missing `/event`).
- Reuse `newQueueView` / `fleetRow` where practical.

## Definition of done

- Queue an order and dispatch a fleet → both appear with the correct remaining
  time and link.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean.

## Notes

- No scheduler changes: this page only reads state; the durable engine stays the
  single source of resolution.
