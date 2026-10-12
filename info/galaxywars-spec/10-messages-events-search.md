# GWX ↔ GalaxyWars spec — 10 Messages, Events & Search

The three "information" surfaces, mined from `HAR-DTA`
(`info/recordings/Uni5/`). Reference client `GalaxyWars v3.16.19-beta`, universe
`uni5`, planet `12:147:1` (`[M]`, player `Redock`, id
`e2c874f6-74f2-4e60-878b-408a0c4418c9`).

- **Messages** (`/message/*`) — player conversations + battle/system report folders.
- **Events** (`/event`) — the universe-wide **scheduled events** feed (not a
  personal notification feed).
- **Search** (`/search`) — player/alliance lookup, plus the **player profile**
  (`/player/<uuid>`) it links to.

## Messages

`GET /message` is a **302 → `/message/conversation`**. Every message page shares
a sub-navigation bar with four grouped dropdowns:

```
Conversations | Reports ▾              | Fleets ▾          | Events ▾
                Attack reports          | Own fleets        | Buildings
                Spy reports             | Foreign fleets    | Research
                Defense reports         |                   | Trade
                Archive                 |                   | Alliance
                                                             | System
                                                             | News
```

Route map: `/message/conversation`, `/message/conversation/detail/<uuid>`,
`/message/report/{attack,spy,defense}`, `/message/report/archive/`,
`/message/system/{own-fleet,other-fleet,building,research,trading,alliance,story,blog-post}`.
(`/message/system/trading`, `/system/alliance` and `/message/blog-post` are linked
but not in this capture.)

### Conversations

`/message/conversation` lists conversations as rows: avatar, **title** (the other
participant's name — links to the detail thread), relative **time**, and a
truncated **teaser** of the last message. Read/unread is signalled by the
`message-system unread` class. Compose is not a separate page: from a profile
**Write message** → `/message/conversation/new-direct/<player-uuid>`.

Conversation detail (`/message/conversation/detail/<uuid>`): a participants row
(each name links to `/player/<uuid>`), a reply form, then the thread.

Reply form `create_message` (POST, form-urlencoded):

```
create_message[message]=<textarea, required>
create_message[send]=
create_message[_token]=<csrf>
```

Each thread message: `.conversation-message` with `.message-author`, a relative
`.message-time` (`23.09 11:35`) and an HTML `.message` body.

### Report folders (`/message/report/*`)

Attack / Spy / Defense share the same `update_report_listing` **POST** form; the
rows are compact battle-report summaries:

- Header of a row: a numeric report id checkbox
  (`update_report_listing[reports][<id>]`), the planet coordinates (attacker →
  defender for *Attack*; reversed for *Defense*), and an external
  **Report** link `the reference site/battle-report/<uuid>?context=<coord>`.
- Body: a 3-column table — participant name (attacker red / defender green,
  linking to `/player/<uuid>`), a unit **count**, and **losses** (skull icon).
- *Defense* rows additionally show the **loot** taken (`<n> Water`, boxes icon).
- Footer controls: **Select all**; **Delete** selected
  (`update_report_listing[delete]`); **Mark as read**
  (`update_report_listing[readAll]`); **Clear** folder
  (`update_report_listing[deleteAll]`, `simple-confirm` dialog *"Are you sure you
  want to delete all messages in the folder?"*).

`/message/report/archive/` shows archived battle reports (empty here:
*"No archived battle reports found."*).

The full report itself is **externally hosted** (`the reference site/battle-report/`),
exactly like the Highscore *Battles* tab (`09-rankings-statistics.md`) — the
in-game folder only stores the one-line summary + link.

### System folders (`/message/system/*`)

Same `update_report_listing` form (checkbox per item, Delete / Mark as read /
Clear, `simple-confirm` on clear), paginated with `?page=N`. Items are plain
text with a relative timestamp:

| Folder | Example item |
|---|---|
| `own-fleet` | *"A fleet from planet 12:147:1 was stationed at 12:147:3. It brought the following resources: 1 Iron, 1 Lutinum, 1 Water and 2 Hydrogen."* (or *"…stationed there again after a tactical retreat."*) |
| `other-fleet` | foreign fleet arrival/return |
| `building` | *"Construction of Iron Mine (level 3) on 12:147:4 has been completed."* |
| `research` | *"Research of <tech> completed…"* |
| `system` | onboarding welcome text, maintenance notices |

There is **no message compose/threading** here — system folders are append-only
feeds.

## Events

`/event` is a **universe-wide scheduled events** feed (not personal
notifications). Each event card: `<h3>` title, an italic **Period**
(`27.07.2026 - 00:00:00 - 02.08.2026 - 23:59:00`), a description paragraph, and a
**Details** link → `/event/<uuid>`.

Observed events (recurring, seasonal):

| Event | Effect |
|---|---|
| **Hyper-Extraction** | +50 % resource production on all planets |
| **Zero-Point Drive** | fuel-free travel for all ships |
| **Stardust Vein** | exposed resource veins accessible to recyclers |
| **Molecular Recycling** | +50 % to recycling research / wreckage yield |
| **Winter Star** | expedition event — recover resources from a signal |

`/event/<uuid>` re-renders the single event with the same period/description and
a **More information** link to the public blog
(`the reference site/blog/…`).

## Search

`/search` is a **GET** form `ingame_search` with one required text field:

```
GET /search?ingame_search[term]=<q>&ingame_search[send]=
```

Results are rendered per match type, each in a table:

- **Alliances:** `Alliance (Name [tag]) | Points | Members | Average points` —
  name links to `/alliance/show/<tag>`. Unpopulated rows show `-`.
- **Players:** `Player | Planet points | Research points | Total points` — name
  links to `/player/<uuid>`.

So one term searches **both** alliances and players (e.g. `popplers` → the
`Popplers` alliance; `uiui` → player `uiuiui`, 2.510 / 827 / 3.337).
`/search` with no term renders just the form.

## Player profile

`GET /player/<uuid>` (container has `data-controller="spy"`):

- Header: avatar, player **name**, and a score line `#<rank> with <n> points`.
- Actions bar:
  - **Write message** → `/message/conversation/new-direct/<uuid>`
  - **Trade** (`fa-chart-bar`) → `/trading/create?target=<uuid>`
  - two disabled icon buttons (spy / target-list) — enabled only for non-self,
    in-range targets.
- **Planets** table: `Coordinate | Name | Points | (actions)`; coordinate links
  to `/planet/<coords>`, and each row has per-planet actions: probes
  (`/fleet/send-spy/<coord>`), fleet menu
  (`/fleet/attack?coordinate=<coord>`), target list
  (`/fleet/target-list/add/<coord>`), and (disabled) battle reports.

## Endpoint summary

| Method | Route | Purpose |
|---|---|---|
| GET | `/message` | 302 → `/message/conversation` |
| GET | `/message/conversation` | conversation list |
| GET | `/message/conversation/detail/<uuid>` | thread |
| POST | `/message/conversation/detail/<uuid>` | send reply (`create_message`) |
| GET | `/message/conversation/new-direct/<uuid>` | start conversation |
| GET | `/message/report/{attack,spy,defense}` | report folder |
| POST | `/message/report/{attack,spy,defense}` | `update_report_listing` (delete/readAll/deleteAll) |
| GET | `/message/report/archive/` | archived reports |
| GET | `/message/system/{own-fleet,other-fleet,building,research,trading,alliance,system}` | system feed |
| POST | `/message/system/*` | same `update_report_listing` |
| GET | `/message/blog-post` | news |
| GET | `/event` | universe events feed |
| GET | `/event/<uuid>` | event detail |
| GET | `/search` (`?ingame_search[term]=`) | player + alliance search |
| GET | `/player/<uuid>` | player profile |

## Mapping onto gwx (`reports`, `fleets`, `queues`)

gwx already has the raw material for a faithful subset:

- **Reports:** `reports` JSONB (`migrations/0001_init.sql:203`, `ReportStore`),
  one row per participant, `kind` ∈ `combat` / `espionage`
  (`internal/engine/event_engine.go:675`). The attack/spy/defense folders map
  1:1 onto `kind`, with the one-line summary derivable from the payload
  (`attackerLosses`/`defenderLosses`, `loot`).
- **System feed:** build/research completions come from `queues`/engine events;
  own-fleet/other-fleet from `fleets`. gwx currently shows these nowhere.
- **Search/profile:** `players`, `planets`, `ScoreStore` (rank/points) already
  exist; the profile is a thin page over them.
- **Conversations:** gwx has **no** player messaging table at all (new table).
- **Events:** gwx has **no** scheduler/universe-event concept — the reference
  events are operator-scheduled, not derived from game state.

## Differences vs gwx (gwx subset proposal)

Cheapest-first, each independent:

1. **Messages from existing reports (build now).** Add a `/messages` page that
   lists `reports` grouped by kind (*Attack / Spy / Defense*), reusing the
   combat payload for the summary row and a detail view. Add *Mark read* /
   *Delete* / *Clear* over a `read_at`/`deleted_at` column (or a small per-row
   flag). Internal detail pages replace the external `battle-report` link.
2. **System feed (build now).** A `system_events` (or reuse an engine log) table
   written on build/research completion and on fleet station/return; render the
   plain-text feed with pagination + the same delete/read controls. Backfillable
   from `queues` history if kept.
3. **Search + profile (build now).** `/search?term=` over `players` +
   `alliances` (needs **I6-alliances** for the alliance half), and a
   `/player/<uuid>` profile with trajectory row + planets table + actions
   (write-message/trade/spy/target-list — reuse existing fleet + trade pages).
   Names/coords everywhere become links.
4. **Conversations (needs mechanic).** New tables `conversations` +
   `conversation_participants` + `conversation_messages` (or a `messages` table
   with a thread id); participants row, reply form, unread flag, per-participant
   read state. Defer unless the messaging surface is wanted (**I2-messages**).
5. **Events (out of scope / optional).** Universe-wide scheduled events need an
   operator/admin scheduler gwx has no concept of. Record only; skip unless gwx
   gains an event system (**I3-events** documents the surface, implementation is
   optional).
6. **Nav.** Add *Messages*, *Events* and *Search* entries to the COMMUNITY
   sidebar group so the surfaces are reachable (gwx's current nav has none of
   them).

## Reconciliation with `BACKLOG.md`

- Confirms gwx's existing `reports` table is the right substrate for the
  report folders — only the listing/read/delete layer is missing.
- The reference **Search** returns players *and* alliances in one term; gwx's
  Highscore already exposes the columns — reuse them for both result tables.
- The **profile** page is the missing hub that the trade (`07-trade.md`) and
  fleet details cross-link to; it should land with **I1-search-settings**.
