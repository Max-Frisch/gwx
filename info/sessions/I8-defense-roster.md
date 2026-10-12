# I8 — Defense roster + `/defense` page

**Kind:** build · **Depends on:** E1 (spec `03-combat.md` defense table)
**Commit:** `feat: real defense stats + dedicated /defense page`

## Goal

Replace gwx's placeholder defense numbers with the real GalaxyWars values extracted
in E1, reconcile the roster, and split defense out of the shipyard into its own
`/defense` page (matching the reference nav).

## Reference units (idents → gwx code mapping)

| GW ident | Label | gwx code (proposed) |
|---|---|---|
| `lightLaserTower` | Light Laser Turret | `light_laser` (retune) |
| `laserTower` | Laser Turret | `heavy_laser` → rename/replace |
| `empLauncher` | EMP Launcher | `emp_launcher` (new) |
| `plasmaTower` | Plasma Turret | `plasma_turret` (retune) |
| `rocketDefenseCannons` | Rocket Launchers | `rocket_launcher` (new) |

gwx's existing `defense_turret`, `ion_cannon`, `gauss_cannon`,
`small/large_shield_dome` are gwx-specific; either keep them as variants or drop
them — **decide and record the decision** in the spec. Costs/stats come from
`03-combat.md`; build time scales with the Orbital Defense Station level.

## Work

- `internal/game/catalog.go`: update/create the `KindDefense` defs with real
  `Cost`, `Attack`, `Shield`(→`Defense`), `Hull`(→`ckk` semantics — investigate
  and document what `ckk` maps to in our combat model), `TimeSecs`, prereqs.
- `internal/game/combat.go` / `techtree.go`: ensure new prereqs and the defense
  fields plug into `Simulate`.
- Split the page: `GET /defense/{id}` (+ `POST /defense/{id}/build`) in
  `internal/web/web.go`, new `templates/defense.html`, and remove the defense
  section from `shipyard.html` (or keep a link across). Reuse `unitRows`.
- Nav: PLANET → Ships and PLANET → Defense as separate entries in `base.html`
  (the reference groups them as "Ships/Defense"; pick one and be consistent).

## Definition of done

- `/defense` lists the reference units with real costs/values; building them and
  defending against a raid uses the new stats.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean;
  add a `game` test asserting the defense prereqs/stats.
- Record the ident→code mapping + any dropped units in `03-combat.md`.
