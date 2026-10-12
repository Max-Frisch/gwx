# I4 — Statistics from battle reports

**Kind:** build · **Depends on:** E4 (spec `09-rankings-statistics.md`)
**Commit:** `feat: statistics (battle balance / farming / resources)`

## Goal

Aggregate the existing `reports` JSONB payloads into the reference statistics
pages. No new capture needed — the payloads already exist.

## Existing data shape

`internal/engine/event_engine.go` writes `reports` rows:

- **combat** (`resolveAttackTx`, ~line 675): payload
  `{ rounds, attackerLosses, defenderLosses, attackerSurvivors, target,
     loot:{metal,lutinum,water,hydrogen} }`
  (written once for attacker, once for defender over the same payload).
- **espionage** (`resolveEspionageTx`, ~line 567): payload
  `{ target, owner, resources, ships, defense, yourEspionageTech }`.

Note `attackerLosses`/`defenderLosses` are unit-count maps; convert to
**resource value destroyed** using `game.Catalog[code].Cost` so the numbers are
comparable to the reference's "destroyed resources".

## Design

- New store `internal/store/stats_store.go` (read-only SQL over `reports` for the
  user), exposing:
  - `BattleBalance(ctx,userID)` → attacked / defended counts,
    destroyed value (own vs enemy), win/loss/tie.
  - `Farming(ctx,userID)` → raids, total looted per resource, average haul.
  - `Resources(ctx,userID)` → plundered (from `loot` where the user is attacker)
    vs lost-to-plunder (where the user is defender). "Recycled" is deferred until
    debris exists.
- Aggregate either in SQL (jsonb operators) or in Go after `ReportStore.List`.
  Prefer **Go** for clarity with small limits; revisit if slow.
- Web: `GET /statistics` with tabs `?tab=battle|farming|resources`; template
  `statistics.html`; nav entry under ACCOUNT.

## Definition of done

- Run an attack with loot, then view statistics: destroyed, looted and lost
  values match the report.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean.
- Unit-test the payload→resource-value conversion in `internal/game` or a new
  `internal/store` helper.

## Notes

- Economy/points values only; **never** expose fleet composition on public pages
  (fog of war). Statistics here are the player's own.
