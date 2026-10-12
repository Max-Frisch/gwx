# I5 — Highscore expansion (Battles tab + columns)

**Kind:** build · **Depends on:** E4 (spec `09-rankings-statistics.md`); Alliances tab needs I6
**Commit:** `feat: highscore battles tab + sortable columns`

## Goal

Bring `internal/web/templates/highscore.html` + `HighscorePage` in line with the
reference tabs. gwx already has **Players** and **Planets** (economy points only).
Add the missing **Battles** tab, and complete columns/sorting/paging.

## Reference tabs (from E4)

`Players` (Planet points, Research points, Total points, Planets) ·
`Alliances` (tag, name, points, members) · `Planets` (name, owner, coords, points) ·
`Battles` (largest battles / battle reports).

## Work

- **Battles tab:** rank recent/largest combat `reports` (by destroyed resource
  value, reusing the I4 conversion) — but **public** battles only need the
  headline (date, coordinates, total destroyed); do not leak composition.
  Consider a dedicated `store.ScoreStore` query or a small `stats_store` method.
- **Sorting/paging:** match the reference query shape
  `?sort=<key>&direction=asc|desc&page=N` (currently gwx uses `?sort=`+`?tab=`).
  Keep backward-compatible defaults.
- **Alliances tab:** add it only if I6 has landed; otherwise render a disabled
  tab so the nav matches.
- Decisions to lock: points stay **economy only**; no military score anywhere.

## Files

- `internal/store/score_store.go` (queries + `BattleRow`).
- `internal/web/web.go` (`HighscorePage`), `internal/web/data.go`, `highscore.html`.

## Definition of done

- All four tabs render; Battles shows real data after a raid.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean.
- Confirm the fog-of-war rule: no fleet/defense composition is ever exposed.
