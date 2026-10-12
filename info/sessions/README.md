# gwx — session playbook (split extraction & implementation)

This folder splits the GalaxyWars reference work into small, self-contained
sessions. **Each file is a prompt you can run in a fresh opencode session**,
e.g. start a clean session and say:

> execute `info/sessions/E1-defense-catalog.md`

## Why split it

The Uni5 captures are **232 MB + 160 MB + 3 MB**. Mining them *and* building
the feature in one session blows the context budget and mixes concerns. Every
session below:

- reads **at most one HAR** (or one page group),
- writes **one artifact** (one spec file, or one Go feature),
- makes **one commit** at the end.

## Ground rules

- Raw HARs live in `info/recordings/Uni5/` and stay **git-ignored**. Never commit them.
- Intermediate dumps go to `info/captures/derived/` (git-ignored); they persist on
  disk between sessions, so later sessions can reuse earlier dumps.
- Only these are committed on the extraction side:
  `info/galaxywars-spec/*.md`, `tools/har/*`, `BACKLOG.md`, `HANDOFF.md`.
- One commit per session (message suggested in each file).
- Append one line to `info/sessions/LOG.md` when a session is done.

## Source HARs

| Shorthand | File (in `info/recordings/Uni5/`) | Size | Covers |
|---|---|---|---|
| `HAR-CRS`   | `construction_research_ships_defense_fleets_fleetmissions_galaxy.har` | 232 MB | building/research/ship techtrees + calcs, fleet missions, galaxy, target lists |
| `HAR-DTA`   | `defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har` | 160 MB | `/defense` + defense techtree/calc, trade, alliance, highscore, statistics, premium, settings, search, events, messages |
| `HAR-LOGIN` | `Login_overview.har` | 3 MB | login → dashboard → overview onboarding flow |

Reference client: `GalaxyWars v3.16.19-beta`, universe `uni5`, account planet `12:147:1`.

## Common extraction recipe

```bash
HAR="info/recordings/Uni5/<file>.har"

# a) list entries (idx, method, status, mime, size, url)
python tools/har/harscan.py index "$HAR" > info/captures/derived/<name>-index.txt

# b) dump a single response body (use --body request for POST bodies)
python tools/har/harscan.py dump "$HAR" --idx <N> --out <path>

# c) dump all bodies of a mime, optionally filtered by a URL regex
python tools/har/harscan.py dump-all "$HAR" --mime text/html \
    --pattern '/trading' --out info/captures/derived/pages/trade

# d) search entry bodies for a config/formula string
python tools/har/harscan.py grep "<pattern>" "$HAR" --mime application/json --context 80

# e) JS bundles often hold the whole client config
python tools/har/harscan.py assets "$HAR" --kind js
python tools/har/harscan.py dump "$HAR" --idx <bundleIdx> --out info/captures/derived/js/<name>.js
```

Notes:
- LiveComponent calculators respond with mime `application/vnd.live-component+html`
  and are decoded to HTML by `harscan`.
- On Windows `python` is native: write temp files under the workspace or
  `C:\Users\Max\AppData\Local\Temp\opencode`, **not** `/tmp` (MSYS `/tmp` ≠ native `/tmp`).
- Existing pipeline: `harscan` → `extract_catalog.py` → `extract_pages.py` →
  `fit_formulas.py` → `gen_spec.py` produces `info/galaxywars-spec/*.md`.

## Session order

Extraction sessions are **independent** of each other. Implementation sessions
depend on the matching extraction output and are roughly ordered by difficulty.
**E8 consolidates** the spec (regenerate + nav/BACKLOG) — run it *after* E1–E7.

```
E1 defense ─┐
E2 trade    │
E3 alliance │
E4 rank/stat├─► E8 consolidate ─► I1..I9
E5 msg/evt  │
E6 premium  │
E7 onboarding┘
```

| # | Session | Kind | Source | Depends on |
|---|---|---|---|---|
| E1 | [defense-catalog](E1-defense-catalog.md) | extract | HAR-DTA | — |
| E2 | [trade](E2-trade.md) | extract | HAR-DTA | — |
| E3 | [alliance](E3-alliance.md) | extract | HAR-DTA | — |
| E4 | [rankings-statistics](E4-rankings-statistics.md) | extract | HAR-DTA | — |
| E5 | [messages-events-search](E5-messages-events-search.md) | extract | HAR-DTA | — |
| E6 | [premium-settings](E6-premium-settings.md) | extract | HAR-DTA | — |
| E7 | [onboarding](E7-onboarding.md) | extract | HAR-LOGIN | — |
| E8 | [spec-consolidation](E8-spec-consolidation.md) | extract | CRS+DTA+LOGIN | E1–E7 |
| I1 | [search-settings](I1-search-settings.md) | build | — | E4/E5 |
| I2 | [messages](I2-messages.md) | build | — | E5 |
| I3 | [events](I3-events.md) | build | — | E5 |
| I4 | [statistics](I4-statistics.md) | build | — | E4 |
| I5 | [highscore-expansion](I5-highscore-expansion.md) | build | — | E4 |
| I6 | [alliances](I6-alliances.md) | build | — | E3 |
| I7 | [trade](I7-trade.md) | build | — | E2 |
| I8 | [defense-roster](I8-defense-roster.md) | build | — | E1 |
| I9 | [premium-vacation](I9-premium-vacation.md) | build (opt) | — | E6 |

## Status

- [x] E1  - [x] E2  - [x] E3  - [x] E4  - [x] E5  - [x] E6  - [x] E7  - [x] E8
- [x] I1  - [x] I2  - [x] I3  - [ ] I4  - [ ] I5  - [ ] I6  - [ ] I7  - [ ] I8  - [ ] I9
