# GWX ↔ GalaxyWars spec — 09 Highscore & Statistics

Leaderboards and the player's own statistics pages, mined from `HAR-DTA`
(`info/recordings/Uni5/`). Reference client `GalaxyWars v3.16.19-beta`, universe
`uni5`, account `12:147:1` (`[M]`, player `Redock`, id
`e2c874f6-74f2-4e60-878b-408a0c4418c9`).

Two areas:

- **Highscore** (`/highscore/*`) — four tabs: **Players**, **Alliances**,
  **Planets**, **Battles**. The first three are score tables; *Battles* is a
  curated, **externally-hosted** battle-report showcase.
- **Statistics** (`/statistic/*`) — per-account boards: **Battle balance**,
  **Farming balance**, **Resources**.

**Key invariant (confirmed on every tab): there are no military/fleet-score
columns.** Score is economy points only — `Planet points + Research points =
Total points`. Fleet/defense show up only as *counts* on the Statistics pages,
never as points.

## Highscore tabs

Sub-navigation on all four tabs (links + active state):

```
Players | Alliances | Planets | Battles
```

Common table shape: a leading rank `#` column, then data; **100 rows per page**
(numbered pagination with `…` ellipsis; `?page=N`). Sortable headers carry
`fa-sort`/`fa-sort-down` icons and link with
`?sort=<key>&direction=<asc|desc>&page=1`. Player/planet/alliance names link to
their profile (`/player/<uuid>`, `/planet/<coords>`, `/alliance/show/<tag>`).

### Players — `/highscore/player`

| # | Name | Planet points | Research points | Total points | Planets |
|---|---|---|---|---|---|
| 1 | `capy` `[Fox-Wing]` | 96.030 | 22.922 | 118.952 | 10 |

- **Default sort:** `allPoints` **desc** (the *Total points* column shows the
  active `fa-sort-down`).
- **Sort keys:** `allPoints`, `planetPoints`, `researchPoints`, `planetCount`.
- Name cell = player link + separate alliance `[tag]` link.

### Alliances — `/highscore/alliance`

| # | Name | Members | Points | Points/player avg. |
|---|---|---|---|---|
| 1 | `Fuechse Wing` `[Fox-Wing]` | 14 | 613.805 | 43.843 |

- **Default sort:** `alliancePoints` desc.
- **Sort keys:** `alliancePoints`, `members`, `allianceAveragePoints`.
- `Points/player avg.` = `alliancePoints / members` (43.843 = 613.805 / 14).

### Planets — `/highscore/planet`

| # | Coordinate | Owner | Points |
|---|---|---|---|
| 1 | `10:73:8` | `capy` | 10.790 |

- **No sort** (no sortable header links) — fixed by planet points desc.
- Coordinate links to the planet page, Owner to the player profile.

### Battles — `/highscore/battle-reports`

Not a score table. It renders four blocks of **top 5** cards and links out to
the public site:

- **Top attacks**, **Top defenses**, **Top raids**, **Latest reports**.
- Each card: rank badge (top 3 tinted gold/silver/bronze via a left border
  colour), an **external** `Report` link (`the reference site/battlereport/
  <Slug>`), `Attacker` + alliance tag + **ckk** value (green, skull icon),
  `Defender` + tag + **ckk** value (red), and `Plundered resources` (e.g.
  `33.154 k`), or `-`.
- *Latest reports* uses `#` instead of a rank.
- Buttons/links: **Upload report** → `the reference site/battlereports`;
  *More reports* → `.../battlereports/universum-5/{top-attacker,top-defender,
  top-raider}` and `.../battlereports/last`.

The battle data is community-submitted and lives on `the reference site`, not in
the game DB — gwx has no equivalent to mirror here (see *Differences* below).

## Statistics

Sub-navigation: `Battle balance | Farming balance | Resources`.

### Battle balance — `/statistic/battle-balance`

Four tables, each `… | Destroyed | Lost` or `… | Plundered | Lost` /
`Gained | Lost`. Rows are the **full unit catalog** (zero rows included).

- **Ships** (`Destroyed | Lost`): Jackal, Recycler, Spy Probe, Renegade, Raider,
  Falcon, Colonization Ship, Tjuger, Cougar, Longeagle V, Small Trader,
  Large Trader, Noah, Longeagle X.
- **Defense** (`Destroyed | Lost`): Light Laser Turret, Laser Turret, EMP
  Launcher, Plasma Turret, Rocket Launchers.
- **Resources** (`Plundered | Lost`): Iron, Lutinum, Water, Hydrogen.
- **Recycled resources** (`Gained | Lost`): Iron, Lutinum, Water, Hydrogen.

`Destroyed` = enemy units you killed; `Lost` = your units destroyed. For
resources, `Plundered` = loot you took, `Lost` = loot taken from you. The
*Recycled* table is debris recovered/lost (needs a debris mechanic).

### Farming balance — `/statistic/farming`

A **beta** page (explicit *"figures may be inaccurate… will not remain free of
charge"* notice) whose body is a single LiveComponent, **"Plundered resources"**
— see the chart below.

### Resources — `/statistic/resources`

The account's resource ledger, in three sections:

**Resources in account**

| Block | Table |
|---|---|
| Total resources | one row: Iron, Lutinum, Water, Hydrogen |
| On planets | per planet (`12:147:1/3/4`) × resources + **Total** |
| Ship production | per planet × resources + Total |
| Defense production | per planet × resources + Total |
| Trading post storage | per planet × resources + Total |
| Own fleets | single row × resources |

**Resources invested since the beginning** — `Category × resources` with
`Building | Research | Ships | Defense | Fuel` rows and a `Total`.

**Account level** — a `Level N` progress bar (observed `Level 7`, 75 % wide).

## Chart LiveComponent (`Charts:StolenResourcesLineChart`)

Both *Farming balance* and (presumably) the same `live-4244680832-0` component
are rendered by `POST /app/_components/<coord>/Charts:StolenResourcesLineChart`
(`application/vnd.live-component`). The request is a single multipart field
`data` holding the props JSON (no CSRF token in the observed calls):

```json
{"props":{"playerSlug":"e2c874f6-…","battleEventType":null,
 "timeRange":"LAST_14_DAYS",
 "@attributes":{"id":"live-4244680832-0"},"@checksum":"…"},"updated":{}}
```

The response is the re-rendered component: two `<select>`s wired with
`data-model`, and a Chart.js `<canvas>` (`symfony--ux-chartjs--chart`,
`type: line`, `maintainAspectRatio:false`, y-axis `beginAtZero`).

| Select | Options |
|---|---|
| `battleEventType` | *(All)*, `PVP` *Player*, `NPC` *NPC*, `FREE_NATION` *Free nation*, `EVENT` *event* |
| `timeRange` | `LAST_60_MINUTES` *Last hour*, `LAST_24_HOURS` *Last 24 hours*, `LAST_14_DAYS` *Last 14 days* (default) |

One dataset per resource (colour = the resource-bar palette):

| Dataset | Dataset colour |
|---|---|
| Iron | `rgba(119, 140, 163, 1.0)` |
| Lutinum | `rgba(253, 150, 68, 1.0)` |
| Water | `rgba(45, 152, 218, 1.0)` |
| Hydrogen | `rgba(32, 191, 107, 1.0)` |

`data` is an **object keyed by label** (Chart.js pre-v4 object form),
`tension: 0.4`. Labels are date- or hour-bucketed by range: `LAST_14_DAYS` →
one bucket per day (`28.09.2026 … 12.10.2026`); `LAST_24_HOURS` → hourly
(`11.10 - 07 … 12.10 - 07`). All values were `0` for this account (no stolen
resources in-window).

## Endpoint summary

| Method | Route | Purpose |
|---|---|---|
| GET | `/highscore/player` | Players (sort/direction/page) |
| GET | `/highscore/alliance` | Alliances (sort/direction/page) |
| GET | `/highscore/planet` | Planets (page) |
| GET | `/highscore/battle-reports` | Battles (external links) |
| GET | `/statistic/battle-balance` | destroyed/lost + plundered/lost |
| GET | `/statistic/farming` | plundered chart (LiveComponent host) |
| GET | `/statistic/resources` | account resource ledger |
| POST | `/app/_components/<coord>/Charts:StolenResourcesLineChart` | chart data |

## Mapping onto gwx (`reports` JSONB)

gwx already stores combat payloads in `reports` (`migrations/0001_init.sql:203`,
`ReportStore`), one row per participant (attacker **and** defender get the same
`payload`). The combat payload (`internal/engine/event_engine.go:675`) is:

```
target, rounds,
attackerLosses / defenderLosses       (unit → count, destroyed)
attackerSurvivors                      (unit → count)
loot {metal,lutinum,water,hydrogen}    (plundered, attacker taken)
```

Espionage payloads are `kind='espionage'` and not used here.

| GalaxyWars metric | gwx source | Have? |
|---|---|---|
| Ships/Defense **Destroyed** | enemy-side loss map in your combat reports | ✅ (`attackerLosses`/`defenderLosses`) |
| Ships/Defense **Lost** | own-side loss map | ✅ |
| Resources **Plundered** | `loot` where you are the attacker | ✅ |
| Resources **Lost** | `loot` where you are the defender | ✅ |
| **Recycled** Gained/Lost | debris/recycler results | ❌ needs debris |
| Resources in account / per planet / fleet / production | `planets`, `planets` resources, `fleet_units`, production | ✅ |
| Resources invested since beginning | cumulative build/research/ship/defense spend (not stored per-category) | ⚠️ needs a ledger |
| Account level | economy points → level curve | ⚠️ define curve |
| Player/Alliance/Planet leaders | `ScoreStore.Players` / `Planets` (+ new alliance agg) | ✅ / ⚠️ alliances missing |

Naming note: GalaxyWars **Iron** = gwx **metal**.

## Reconciliation with `BACKLOG.md`

- BACKLOG *"Highscore / Hall of Fame — done (Players + Planets)"* matches the
  reference **Players** columns exactly (`Planet points / Research points /
  Total points / Planets`). gwx's `highscore.html` already has those columns —
  it is missing the **`[tag]`** alliance link, and the **Alliances** and
  **Battles** tabs.
- BACKLOG *"Statistics: Battle balance, Farming balance, Resources, Recycled
  (destroyed / plundered / gained / lost)"* maps 1:1 to the three `/statistic`
  pages; **Recycled** is debris-gated (unchanged).
- BACKLOG *"Statistics from `reports` — aggregate destroyed/plundered;
  Recycled needs debris first"* is confirmed and now backed by exact field
  names (above).
- BACKLOG design note *"no military points"* is confirmed: no Highscore tab
  exposes fleet/defense **points**; counts live only on Statistics.

## Differences vs gwx (what implementing this needs)

gwx has a Players+Planets Highscore and no Statistics. A faithful minimal port:

1. **Highscore Alliances tab** — a `ScoreStore.Alliances` aggregation over
   members (`SUM(member total points)`, `COUNT(members)`, avg), sorted by
   `points | members | average`; name links to the alliance profile (needs
   **I6-alliances**). Page size 100.
2. **Highscore Players** — add the `[tag]` link; make headers sortable with
   `sort`/`direction`/`page` params and `fa-sort` icons that gwx currently
   lacks (the reference sorts server-side).
3. **Battles tab** — gwx has no community battle-report site; either omit it or
   render a gwx-native "recent big battles" view from `reports`. Flag as
   optional/out-of-scope.
4. **Statistics from `reports`** — aggregate the combat JSONB per user:
   `Destroyed`/`Lost` per unit (from the four loss/survivor maps) and
   `Plundered`/`Lost` resources (from `loot` on your attacker/defender reports).
   Render the same unit-catalog tables (reuse `game.Catalog` + `03-combat.md`).
   *Recycled* stays empty until debris exists.
5. **Resources page** — sum `planets` resources (+ fleets + trade storage) with
   a per-planet breakdown and totals; the *invested since beginning* table
   needs a cumulative spend ledger gwx does not keep yet (defer or add a
   counter); *Account level* needs a points→level curve (economy points).
6. **Chart** — optional; a gwx line chart of plundered resources per day/hour
   over `LAST_60_MINUTES | LAST_24_HOURS | LAST_14_DAYS`, filtered by
   `battleEventType`, one dataset per resource in the reskinned palette. gwx is
   server-rendered (no LiveComponent), so ship the points in the page or add a
   small JSON endpoint.
