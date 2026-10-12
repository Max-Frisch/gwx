# E4 — Highscore & Statistics spec

**Kind:** extraction · **Source:** `HAR-DTA` · **Depends on:** none
**Commit:** `spec: extract GalaxyWars highscore + statistics from Uni5 HAR`
**Output:** `info/galaxywars-spec/09-rankings-statistics.md` (new)

## Goal

Document every highscore tab and statistics page: columns, sort parameters,
pagination, and the metric definitions (economy/research only — no military).
Also capture the chart LiveComponents.

## Inputs (`HAR-DTA`, indices stable per file)

| idx | URL | Note |
|---|---|---|
| 923 | `/highscore/player` | Players tab |
| 938, 953 | `/highscore/player?sort=planetPoints&direction={asc,desc}&page=1` | sort + paging params |
| 968 | `/highscore/alliance` | Alliances tab |
| 983 | `/highscore/planet` | Planets tab |
| 998 | `/highscore/battle-reports` | Battles tab |
| 1014 | `/statistic/battle-balance` | battle balance |
| 1030 | `/statistic/farming` | farming balance |
| 1048 | `/statistic/resources` | resources gained/stolen |
| ~ | `/app/_components/12:147:1/Charts:StolenResourcesLineChart` | chart payload (grep for `Charts`) |

## Steps

1. Dump pages and the chart component:
   ```bash
   HAR="info/recordings/Uni5/defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har"
   python tools/har/harscan.py index "$HAR" > info/captures/derived/dta-index.txt
   python tools/har/harscan.py dump-all "$HAR" --mime text/html --pattern '/highscore|/statistic' \
       --out info/captures/derived/pages/rankings
   python tools/har/harscan.py grep "StolenResourcesLineChart" "$HAR" --mime 'live-component' --context 200
   ```
2. Record for each highscore tab: columns, default sort, all sort keys, page
   size, whether player names/coords link to profiles, and the alliance tag
   shown. Confirm **no** military/fleet columns exist.
3. Record for each statistics page: the four metrics (destroyed / plundered /
   gained / lost), the time window/axis, totals, and how they map onto battle
   reports (gwx already stores combat JSONB in `reports`).
4. Write `info/galaxywars-spec/09-rankings-statistics.md`, and reconcile with the
   existing `highscore` section notes in `BACKLOG.md` (Players/Alliances/Planets/
   Battles; Planet points / Research points / Total points / Planets).
5. Commit only the new spec file.

## Definition of done

- Columns + sort parameters + paging documented for all 4 highscore tabs.
- All statistics pages and the chart component documented against gwx's
  `reports` table (which fields we already have vs. still need).
- No raw HAR content committed.

## Handoff

Feeds **I4-statistics** and **I5-highscore-expansion**.
