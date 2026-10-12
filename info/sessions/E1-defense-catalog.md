# E1 — Defense catalog & combat spec

**Kind:** extraction · **Source:** `HAR-DTA` · **Depends on:** none
**Commit:** `spec: extract GalaxyWars defense units from Uni5 HAR`

## Goal

Close the only gap in `info/galaxywars-spec/03-combat.md`: real costs, stats and
build times for the 5 defense units. `HAR-CRS` never visited `/defense`; `HAR-DTA`
does. The catalog parser (`extract_catalog.py` → `parse_meta`) already recognises
`defenseIdent`, so this is mostly plumbing + a `gen_spec.py` emitter.

## Inputs (verify with `index` first — indices are stable per file)

`HAR-DTA` (`info/recordings/Uni5/defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har`):

| idx | URL |
|---|---|
| 0, 109, 149, 189, 230 | `/app/12:147:1/defense` (index page: unit cards **with costs**) |
| 23  | `/app/12:147:1/technology-tree/defense/lightLaserTower` |
| 132 | `/app/12:147:1/technology-tree/defense/laserTower` |
| 172 | `/app/12:147:1/technology-tree/defense/empLauncher` |
| 212 | `/app/12:147:1/technology-tree/defense/plasmaTower` |
| 253 | `/app/12:147:1/technology-tree/defense/rocketDefenseCannons` |
| 39, 148, 188, 229, 269 | `POST /app/_components/12:147:1/TechnologyDefense/calculate` (one per unit) |

Known format facts:
- Techtree page carries name, description, `Requirements`, a `Base`/`Current`
  block (`Attack`, `Defense`, `ckk` — no cargo/speed for defense) and a
  `Level`/`Construction time` table (level = Orbital Defense Station level).
- The **`/defense` index page** carries the per-unit `pageCosts`
  (`Iron`/`Lutinum`/`Water`/`Hydrogen` + `Duration`) in `full-w-entry-wrapper`
  blocks; `extract_pages.py` already handles this (incl. `defense` in the ident regex).

## Steps

1. Index and dump:
   ```bash
   HAR="info/recordings/Uni5/defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har"
   python tools/har/harscan.py index "$HAR" > info/captures/derived/dta-index.txt
   python tools/har/harscan.py dump "$HAR" --idx 109 --out info/captures/derived/pages/defense-index.html
   for i in 23 132 172 212 253; do
     python tools/har/harscan.py dump "$HAR" --idx $i --out info/captures/derived/techtree/$i-defense.html
   done
   for i in 39 148 188 229 269; do
     python tools/har/harscan.py dump "$HAR" --idx $i --out info/captures/derived/livecomp/$i-calculate
   done
   ```
2. Run `python tools/har/extract_catalog.py` then `python tools/har/extract_pages.py`.
3. **Verify** `info/captures/derived/catalog.json` now has 5 entries with
   `"kind":"defense"` and a `calc` block. If `parse_meta` misses the stat rows or
   the kind, fix `tools/har/extract_catalog.py` (it already loops
   `building/research/ship/defenseIdent`).
4. Extend `tools/har/gen_spec.py`:
   - `01-catalog.md`: add a short **Defense** table (name, ident, base cost,
     requirements) via `by_kind("defense")`.
   - `03-combat.md`: **replace** the hard-coded "Defense (gap)" section with a
     real table shaped like the Ships table (ident, cost, Attack→Current,
     Defense→Current, ckk, construction time, requirements). Keep a note that
     defense has no cargo/speed/consumption.
5. Regenerate: `python tools/har/gen_spec.py`. Eyeball `01` and `03`.
6. Commit `info/galaxywars-spec/01-catalog.md`, `03-combat.md`, `tools/har/extract_catalog.py`,
   `tools/har/gen_spec.py` (and `catalog.json` stays uncommitted).

## Definition of done

- `03-combat.md` has no "(gap)" language and lists all 5 units with base cost,
  Attack/Defense (base→current for this account), `ckk`, and build time.
- `grep -c 'gap' info/galaxywars-spec/03-combat.md` → 0.
- `git status` shows only the intended files.

## Handoff

Feeds **I8-defense-roster** (align gwx's defense catalog with these real values).
Note the client idents differ from gwx codes; record the mapping in the spec.
