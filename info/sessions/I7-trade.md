# I7 — Player trading / marketplace

**Kind:** build · **Depends on:** E2 (spec `07-trade.md`)
**Commit:** `feat: player trade (offers, accept, storage, history)`

## Goal

A player-to-player marketplace: public and directed offers, accept/revoke/expire,
a trade storage holding delivered goods, and an offer history. Scope it to the
MVP described at the end of `07-trade.md`; fees/rates can stay configurable
constants.

## Design

- **Tables** (migration `0004_trade.sql`):
  - `trade_offers (id, universe_id, creator_id, target_id NULL, offer_metal,
    offer_lutinum, offer_water, offer_hydrogen, want_metal, want_lutinum,
    want_water, want_hydrogen, status, expires_at, created_at)`
    status ∈ `OPEN|ACCEPTED|REVOKED|EXPIRED`.
  - `trade_history (id, offer_id, actor_id, action, at)` (audit).
  - Trade storage: either columns on `planets` or a dedicated
    `planet_trade_storage (planet_id, metal, lutinum, water, hydrogen)` — keep it
    separate so the protected floor / storage caps do not silently apply.
- **Delivery:** accepting an offer should resolve atomically: debit the accepter's
  resources into storage for the creator and vice versa. If two planets are
  involved, this likely needs a fleet leg; **simplest MVP**: instant exchange
  between two planets in the same universe, validated against caps.
- **Store:** `internal/store/trade_store.go`.
- **Web:** `/trade` (marketplace), `/trade/new`, `/trade/own`, `/trade/history`,
  `/trade/storage`; nav under PLANET. Add the **Trading Post** building to
  `internal/game/catalog.go` as the prerequisite (see `01-catalog.md`).

## Definition of done

- Two accounts can post, list, accept and revoke offers; resources move correctly
  and storage/history pages reflect it; expiry is enforced (scheduler or lazy
  check on read).
- `make migrate` idempotent.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean,
  with tests for the exchange math + insufficient-resources path.

## Notes

- Reuse `game.Resources` and the existing `ErrInsufficientResources` flow.
- Decide explicitly whether trade ignores the protected floor (the reference has
  a separate storage) and document it in the spec.
