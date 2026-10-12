# Session handoff — gwx ↔ GalaxyWars reference

Continuation notes for feeding HAR captures in a fresh session.

## Repo state

- Prior commit: `5b30fbe feat: initial gwx — a small GalaxyWars-style browser game`.
- This session added: UI reskin, Highscore page, capture runbook.
- Raw captures are private: `info/captures/` and `*.har` are git-ignored.

## Done this session

- **UI reskin** to match the uni5 screenshots: dark-navy shell, starfield
  background, left sidebar with grouped nav (PLANET / UNIVERSE / COMMUNITY /
  ACCOUNT) + inline SVG icons, top resource strip, centered page tabs,
  steel-blue accents. Files: `internal/web/static/app.css`,
  `internal/web/templates/base.html`.
- **Highscore / Hall of Fame**: Players + Planets tabs, sortable, economy points
  only. Files: `internal/game/points.go`, `internal/store/score_store.go`,
  `internal/web/templates/highscore.html`, `internal/web/data.go`,
  `internal/web/web.go`.
- **BACKLOG.md**: reskin + Highscore moved to Done; "content for later"
  annotated with the corrections below.
- **Capture runbook**: `info/capture-runbook.md`.

## Locked decisions

- Points are **economy only** = buildings + research, **no military points**
  (fog of war intact). `game.PointsDivisor = 1000` (1 pt / 1000 resources).
- `[M]` on planet coords = **Main Planet**, *not* a moon. GW has no moons.
- Sources: **uni5 HARs lead**. A mid-game account is enough because of the
  adaptive level selector (see below); the fresh **Uni6 (Oct 17)** capture is
  optional/confirmatory for spawn defaults + early flow.
- HARs stay private (git-ignored); the derived spec is committed under
  `info/galaxywars-spec/`.

## Key extraction insights

- **Adaptive level selector** (Command Center / Research Center / Shipyard) on
  item cards = formula probe. Cycling it 1→max yields `(level → cost/time/
  effect)`, enough to fit `base × factor^level` and the build-time multiplier.
- **Ship cards show base value + current value** → derive research bonus
  formulas, provided the Research page (current tech levels) is captured in the
  same HAR.
- **Shipyard queue UI**: per-item checkbox, **Delete selected**, **Delete all**
  with a confirmation popup → new spec section `06-queue-management.md`.
- Disable cache when capturing so the **JS bundles** land in the HAR; they
  usually hold the whole config + formula names.

## Next session — first steps (once HARs arrive)

1. Build `tools/har/harscan.py` (Python stdlib; **no jq** available) against the
   real HAR structure:
   - `index` → method/status/mime/size/hasBody/url
   - `dump-all --mime json --out dir/`, `dump --idx N`
   - `assets` (JS/CSS URLs), `grep PATTERN`, `slim` (strip images/base64)
2. Inventory endpoints; auto-detect level-parameterized responses by diffing
   across selector values; dump JSON catalog/config; grep JS bundles.
3. Emit `(item, level) → cost/time/effect` tables; fit formulas.
4. Produce `info/galaxywars-spec/`:
   - `01-catalog.md` — buildings/ships/defense/research: costs, factors, times,
     stats, prereqs, effects (incl. Recycling Tech's two jobs).
   - `02-economy-formulas.md` — production, energy, storage, protected floor,
     points; tech bonus formulas.
   - `03-combat.md` — attack/shield/hull, rapid-fire table, debris rate, loot.
   - `04-fleet.md` — speed/fuel/range/distance, mission behaviour.
   - `05-ui-flows.md` — trade/marketplace, settings, statistics, highscore
     tabs, messages, alliance, galaxy actions.
   - `06-queue-management.md` — batch delete semantics + confirm dialog.
5. Update `BACKLOG.md` "content for later" with real values.

## Reference: roster gaps to spec

- Techs: Space Curvature Drive, Ionization, Energy Focusing, Explosive
  Projectiles, Spy Technology, Advanced Ship Armor, Increased Cargo Capacity,
  Recycling Technology.
- Ships: Jackal, Renegade, Raider, Falcon, Tjuger, Longeagle V, Noah (ark),
  Small Trader, Large Trader.
- Defense: EMP Launcher, Rocket Launchers.
- Buildings: Trading Post, Advanced Chemical Factory; alt names Lutinum
  Refinery / Ship Factory / Water Tanks / Hydrogen Tanks.

## Handy commands

```bash
go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...
make up && make migrate && make run     # Postgres :5433, app :8090
```
