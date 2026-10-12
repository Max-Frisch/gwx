# GWX ↔ GalaxyWars spec — 05 UI flows & navigation

Navigation (left sidebar; not all pages captured in this HAR):

```
PLANET     Overview   /app/<c>/planet
           Construction /building
           Research     /research
           Ships/Defense /ship   (+ /defense, not captured)
           Trade        /trading            (not captured)
           Technology   /technology-tree/{building,research,ship,defense}   (tabbed)
UNIVERSE   Fleets       /fleet/attack
           Galaxy       /galaxy[/g/s]
           Events       /event                (not captured)
COMMUNITY  Messages     /message              (not captured)
           Search       /search               (not captured)
           Alliance     /alliance/search      (not captured)
           Highscore    /highscore/player     (not captured)
ACCOUNT    Statistics   /statistic/battle-balance (not captured)
           Premium      /premium/packages     (not captured)
           Settings     /settings/game        (not captured)
```

## Captured surfaces

| Surface | Endpoints | What to copy |
|---|---|---|
| Construction | `/building`, `/building/append/<ident>`, `/building/remove/<id>` | Grouped cards (Main / Resource / Attack-Defense), level badge, cost pills, duration, red/green affordability vs stock, upgrade link. |
| Research | `/research`, `/research/append/<ident>`, `/research/remove/<id>` | Grouped by Drive / Weapons / Ship research; descriptions state the per-level effect. |
| Ships | `/ship` (GET/POST) | Card per ship: cost, time, quantity input, `max. N` shortcut. |
| Technology | `/technology-tree/{building,research,ship}/{ident}` | Info card + requirement list + point/level + **adaptive level-calculator** (Command-Center-at-level / from-to inputs) rendering a cost/time/effect table. |
| Galaxy | `/galaxy/<g>/<s>` | Per-planet tiles: name, points, owner, own/other styling, fleet + target-list shortcut buttons, empty slots. |
| Fleet | `/fleet/{attack,spy,own,expedition}?coordinate=…` | Mission tabs, ship grid, order block, raid order, target lists, battle simulator. |


## Not captured (follow-up HAR needed)

Trade/marketplace, Events, Messages, Search, Alliance, Highscore tabs, Statistics, Premium, Settings (all tabs), Help/rules, and the `/defense` unit page.

