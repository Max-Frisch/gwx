# GWX ↔ GalaxyWars spec — 05 UI flows & navigation

The complete navigation map and page inventory for the Uni5 walk, consolidated
in **E8** from the routes captured across `HAR-CRS`, `HAR-DTA` and `HAR-LOGIN`.
Every route below is present in the captures — there are **no "(not captured)"
gaps** left. Deeper detail lives in the numbered sections (`01`–`12`).

Reference client `GalaxyWars v3.16.19-beta`, universe `uni5`, account
`12:147:1` (`[M]`, player `Redock`).

## Shell (every in-game page)

- **Left sidebar**, grouped, with an active state on the current item (below).
- **Top resource strip** — Iron / Lutinum / Water / Hydrogen: current value,
  `/h` production and a colour-banded storage bar
  (`{green:50, yellow:75, orange:98, red:100}`).
- **Planet switcher** — prev/next arrows + a `<select>` of owned coords (home
  tagged `[M]`); the selected coords are embedded in the path (`/app/<c>/…`).
- **Sub-navigation** — a second bar on multi-tab pages (Settings, Premium,
  Highscore, Statistics, Messages, Technology).
- **Footer** — Help / Game rules / Support / Status / Legal notice / version.
- **Live refresh** — `POST /app/push-stats` (2-byte JSON) polls header
  resource/queue state; Symfony UX **LiveComponents** re-render bounded widgets
  (`POST /app/_components/<coord>/<Component>`).

## Navigation map (captured)

```
PLANET     Overview      /planet
           Construction  /building
           Research      /research
           Ships/Defense /ship   (+ /defense)
           Trade         /trading
           Technology    /technology-tree/{building,research,ship,defense}
UNIVERSE   Fleets        /fleet/attack
           Galaxy        /galaxy[/<g>/<s>]
           Events        /event
COMMUNITY  Messages      /message
           Search        /search
           Alliance      /alliance/search
           Highscore     /highscore/player
ACCOUNT    Statistics    /statistic/battle-balance
           Premium       /premium/packages
           Settings      /settings/game
           Logout        /logout
```

All routes are relative to `/app/<galaxy>:<system>:<position>` unless noted.

## Captured surfaces

| Surface | Key endpoints | Spec | Notes to copy |
|---|---|---|---|
| **Overview** | `/planet` | [12](12-onboarding.md) | Messages badges, fleet activity, building/research order widgets, Ships/Defenses (ckk), trading storage, Points block |
| **Construction** | `/building`, `/building/append/<ident>`, `/building/remove/<id>` | [01](01-catalog.md), [06](06-queue-management.md) | Grouped cards, level badge, cost pills, duration, affordability, upgrade link |
| **Research** | `/research`, `/research/append/<ident>`, `/research/remove/<id>` | [01](01-catalog.md) | Grouped by Drive / Weapons / Ship research; per-level effect text |
| **Ships/Defense** | `/ship` (GET/POST), `/defense` | [01](01-catalog.md), [03](03-combat.md) | Card per unit: cost, time, quantity input, `max. N`; batch queue form |
| **Technology** | `/technology-tree/{building,research,ship,defense}[/<ident>]` | [01](01-catalog.md) | Info + prereqs + points/level + **adaptive level-calculator** (cost/time/effect table) |
| **Trade** | `/trading`, `/trading/create`, `/trading/offer/<uuid>/detail`, `/trading/own[/history]`, `/trading/storage`, `/trading/information[/history]` | [07](07-trade.md) | Marketplace, offers, rates/fees, storage, extreme-rate log |
| **Fleets** | `/fleet/{attack,spy,own,expedition}`, `/fleet/send`, `/fleet/target-info`, `/fleet/battle-simulator`, `/fleet/target-list/*` | [04](04-fleet.md) | Mission tabs, ship grid, raid order, target lists, simulator |
| **Galaxy** | `/galaxy/<g>/<s>` | [04](04-fleet.md) | Per-planet tiles: name, points, owner, own/other styling, fleet/target shortcuts, empty slots |
| **Events** | `/event`, `/event/<uuid>` | [10](10-messages-events-search.md) | Universe-wide scheduled events (title, period, description, detail) |
| **Messages** | `/message/conversation[/detail/<uuid>]`, `/message/report/{attack,spy,defense,archive}`, `/message/system/*` | [10](10-messages-events-search.md) | Conversations + battle/system report folders with delete/read/clear |
| **Search** | `/search?ingame_search[term]=`, `/player/<uuid>` | [10](10-messages-events-search.md) | Player + alliance results; profile with planets and actions |
| **Alliance** | `/alliance/{search,show/<tag>,apply/<tag>,applications,create}`, `/alliance/internal/*` | [08](08-alliance.md) | Public search/profile/apply/create; internal dashboard, members, board, management |
| **Highscore** | `/highscore/{player,alliance,planet,battle-reports}` | [09](09-rankings-statistics.md) | Four tabs; 100/page; sortable; extern battle-report showcase |
| **Statistics** | `/statistic/{battle-balance,farming,resources}` | [09](09-rankings-statistics.md) | Destroyed/lost, plundered/lost, resources ledger, plundered chart |
| **Premium** | `/premium/{packages,galaxium,account}` | [11](11-premium-settings.md) | Galaxium shop (out of scope for gwx) |
| **Settings** | `/settings/{game,planet,vacation,account,sanction}` | [11](11-premium-settings.md) | Fleets/interface prefs, planets, vacation, account, sanctions |

## Reskinned reference (gwx screenshots)

`info/` also holds screenshots of the **retro GalaxyWars skin** gwx now mirrors:
dark-navy shell, starfield background, grouped left sidebar with inline SVG
icons, top resource strip, centered page tabs, steel-blue accents. gwx's own
navigation (`internal/web/templates/base.html`) uses the same
PLANET / UNIVERSE / COMMUNITY / ACCOUNT grouping but only implements Overview,
Construction, Research, Ships/Defense (Fleets, Galaxy, Highscore, Reports so
far) — see the implementation sessions `I1`–`I9` for the rest.

## LiveComponent inventory (Symfony UX)

| Component | Host | Spec |
|---|---|---|
| `Technology{Building,Research,Ship,Defense}/calculate` | Technology tree | [01](01-catalog.md) |
| `OfferOverview` | `/trading` marketplace | [07](07-trade.md) |
| `Charts:StolenResourcesLineChart` | `/statistic/farming` | [09](09-rankings-statistics.md) |

gwx is server-rendered (`html/template`), so each component maps to a normal GET
page or a small JSON endpoint — no LiveComponent runtime is needed.

## Help / rules (external)

Footer links point off-site: `the reference help site`, `the reference site/rules`,
`the reference site/support`, `the reference status site`, `the reference site/imprint`. gwx can
add static Help/rules templates (BACKLOG "Ready to build now" item 5).
