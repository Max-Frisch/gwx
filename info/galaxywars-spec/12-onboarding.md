# GWX ↔ GalaxyWars spec — 12 Login, dashboard & overview onboarding

The entry path and the planet overview, mined from `HAR-LOGIN`
(`info/recordings/Uni5/Login_overview.har`). Reference client
`GalaxyWars v3.16.19-beta`, universe `uni5`, fresh account on planet `12:147:1`
(`[M]`, player `Redock`). This is the **first** capture of a session, so it
documents what a player sees immediately after logging in.

## Login → overview flow

```
POST /login                  (mail + password)          → 302
GET  /app/dashboard                                     → 302
GET  /app/12:147:1/planet    (overview)                 → 200
        ↳ runtime / 544 / 105 / app .js, 544 / 105 .css, fonts, background
```

### Login POST

Captured request body (form-urlencoded), **59 bytes**:

```
mail=<email>&password=<password>
```

Notes:

- The field is **`mail`** (the email address), not `_username`; the password
  field is `password`.
- **No CSRF token** was present in the captured login POST (unlike every in-game
  form). The successful response is a **302** (empty body) that hands off to
  `/app/dashboard`.
- `/app/dashboard` then **302s** to the player's overview
  (`/{coord}/planet`) — it is a redirect-only bootstrap, not a page.

The account is identified in-game by its **email**, and the overview URL is
`/app/<galaxy>:<system>:<position>/planet` (coords, not an opaque id).

## Overview page (`/{coord}/planet`)

Top-of-page chrome (shared with every in-game page, see `05-ui-flows.md`):

- **Resource strip** (Iron / Lutinum / Water / Hydrogen) with current amount,
  `/h` production, and a storage fill bar whose colour is banded
  (`{green:50, yellow:75, orange:98, red:100}`).
- **Planet switcher**: prev/next arrows, a `<select>` of owned coords (home marked
  `[M]`), and the current avatar.
- **Sidebar** grouped PLANET / UNIVERSE / COMMUNITY / ACCOUNT with active state.
- **Footer**: Help / Game rules / Support / Status / Legal notice / version.

Body sections, in order:

| # | Section | Contents |
|---|---|---|
| 1 | **Messages** | badge counts linking to folders — `Defense reports (1)` (red/danger), `Own fleets (60)`, `Foreign fleets (110)`, `Buildings (12)`, plus the server time `06:02:05 - 12.10.2026` |
| 2 | **Fleet activity** | per-fleet slot cards; shows *"No fleet activity"* (disabled container) when idle |
| 3 | **Building orders** | collapsible (`settings-toggle` key `dashboardBuildingsCollapsed`); disabled when empty |
| 4 | **Research orders** | same, account-wide queue |
| 5 | *(ad slot)* | `adsbygoogle` responsive banner |
| 6 | **Ships** | `<header> (0,01 ckk)` + table of `unit → count` (here: Colonization Ship ×1) |
| 7 | **Defenses** | `<header> (0,00 ckk)` + table of `unit → count` |
| 8 | **Trading post storage** | link to `/trading/storage`; *"No resources in storage"* when empty |
| 9 | **Points** | `Number of planets`, `Points (planets)`, `Points (research)`, `Total points`, + **Empire overview** link (`/app/empire`) |

Key observations:

- The overview is a **summary dashboard**, not the resource/queue detail: queues
  appear as coarse "*Building orders* / *Research orders*" containers, and the
  header strip carries the live resources.
- **ckk** is a per-unit combat-power value shown as a category total (`Ships`,
  `Defenses`) — counts + ckk, never military *points* (consistent with
  `09-rankings-statistics.md`).
- **Points** are economy-only: `Points (planets) + Points (research) = Total
  points` (`2.628 + 902 = 3.530`).
- `[M]` = **Main Planet** (homeworld), per the locked decision; the switcher also
  lists the account's other coords without the tag.

## Fresh colony defaults (`12:147:5`)

A fourth capture (`extra_recording_new_colony.har`) follows the same account to a
**newly colonized planet** `12:147:5`, giving the spawn state the mid-game
homeworld can't show:

- **Buildings:** only the **Command Center at level 1** ("Stufe 1", next cost
  500 Iron / 200 Lutinum → *Upgrade to level 2*); every other building is level 0
  (Research Center "Stufe 0", cost 300 / 100).
- **Starting resources:** planet creation seeds **500 Iron / 500 Lutinum /
  500 Water / 0 Hydrogen**. Anything above that is **colony-ship cargo** (the
  sender loaded 20.000 Iron + 10.000 Lutinum): the observed stock was 20.519 /
  10.519 / 520 / 46, i.e. seed + cargo + a trickle of the (negligible, ignored)
  20/h base output; the 46 Hydrogen is cargo-only residue.
- **Storage:** `300.000` per resource with `6.000` unplunderable (the `L=0` term
  of the storage formula).
- **Ships / Defenses:** none; **Trading-post storage:** empty.
- The **Buildings / Resources** sub-navigation (`/building` ⇄ `/resource`) is
  present on both pages.

### gwx choice

gwx deliberately differs: a newly colonized planet is seeded with **1000 Iron /
500 Lutinum / 500 Water / 0 Hydrogen** on top of the colony ship's cargo
(`internal/engine/event_engine.go`, `colonyStart*`). The reference's flat 20/h
base output is intentionally **not** modelled.

## gwx today (for the gap checklist)

- **Routes:** `GET /login`, `POST /login`, `GET /register`, `POST /register`,
  `POST /logout`. A successful login 303s straight to
  `/overview/<homePlanetID>` (`internal/web/web.go:248`); there is no
  `/dashboard` hop. Landed users hit `GET /overview/{id}`
  (`internal/web/web.go:81`).
- **Login form** (`internal/web/templates/auth.html:5`): fields `login`
  (username **or** email) + `password`; no CSRF token.
- **Overview** (`internal/web/templates/overview.html`): planet name+coords,
  **Empire** card (fields/diameter), **Stockpile** card (four resources +
  protected floor), **Active queues** table
  (`type / item / finishes`), **Incoming fleets** table
  (`mission / from / arrives`). Nav + resource strip live in
  `internal/web/templates/base.html`.

## Gap checklist — gwx overview/onboarding vs reference

| Reference element | gwx status | Action |
|---|---|---|
| `POST /login → /app/dashboard → /{coord}/planet` hop | ⚠️ gwx 303s directly to `/overview/<id>` | Optional: add a `/dashboard` alias that redirects to the home planet; **not required** |
| Login by **email OR username**, field `mail` | ✅ gwx accepts username *or* email (`login` field) | none |
| CSRF on login POST | ⚠️ reference has none; gwx has none | leave as-is |
| Resource strip with live `/h` + storage bars | ✅ `base.html` strip | none |
| Planet switcher (coords, `[M]` tag) | ✅ `base.html` select; **`[M]` tag not shown** | add `[M]`/home marker |
| Sidebar grouped nav | ✅ PLANET/UNIVERSE/COMMUNITY/ACCOUNT | add the missing entries (Trade, Technology, Events, Messages, Search, Alliance, Statistics, Premium, Settings) as those land |
| **Messages** badge summary on overview | ❌ absent | add once **I2-messages** exists (defense/own/foreign/buildings counts) |
| **Fleet activity** per-fleet cards | ⚠️ gwx shows an *Incoming fleets* table only | rename/extend to a fleet-activity card; keep incoming highlighted |
| **Building / Research / Ship / Defense queue widgets** | ⚠️ one generic **Active queues** table | split into per-category "orders" containers (collapsible) |
| **Ships** section with running ckk total | ❌ absent (ships live only on `/shipyard`) | add a read-only Ships/Defenses summary card (counts + ckk) |
| **Defenses** section with ckk total | ❌ absent | same |
| **Trading post storage** summary | ❌ absent | add once **I7-trade** exists |
| **Points** block (planets/research/total) + Empire link | ⚠️ shown on `/highscore` only | surface the same numbers on the overview |
| Commit/redirect to homeworld on register | ✅ `POST /register` → home planet | none |

## Suggested gwx onboarding (minimal, no new tables)

The reference overview is a *dashboard over data gwx already has* — no new
storage is needed for the core parity pass:

1. **Overview already has** the planet card + stockpile; add two read-only
   summary cards, reusing existing stores:
   - **Ships / Defenses**: `fleet_units` (or stationed-ship counts) with a
     per-category total; add a `game.Ckk(unit)` helper if ckk is wanted
     (currently only points exist).
   - **Points**: `game.Points` / `LevelPoints` → `planetPoints`, `researchPoints`,
     `total` + a link to `/highscore`.
2. **Split the queue widget** into *Building orders* / *Research orders* /
   *Shipyard orders* containers driven by `queues` (kind), each collapsible.
   Keep the current `Active queues` table as the fallback.
3. **Fleet activity card**: keep the incoming-fleets table but retitle to match
   the reference and add outgoing/own-fleet rows from `fleets`.
4. **`[M]` marker**: render `[M]` next to the home planet in the switcher and
   planet header.
5. **Deferred until their subsystems exist**: Messages badge block (**I2**),
   Trading-post storage (**I7**). `/dashboard` alias optional.

## Reconciliation with `BACKLOG.md`

- Confirms gwx's auth (username-or-email + password, PBKDF2/sessions) is a
  faithful match; the only divergence is the extra `/app/dashboard` redirect hop,
  which carries no data.
- BACKLOG's *"verified end-to-end … all pages render"* is consistent — the
  overview just lacks the reference's dashboard-style summary cards (messages,
  ships/defense ckk, points, trading storage).
- No new persistent state is required for the onboarding parity pass; the extra
  cards are read-only projections over `planets`, `fleet_units`, `queues`,
  `user_techs` and `reports`.
