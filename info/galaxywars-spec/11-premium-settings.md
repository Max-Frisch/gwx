# GWX ↔ GalaxyWars spec — 11 Premium, Settings, Vacation & Sanctions

Account/planet settings, the premium shop and the vacation/sanction surfaces,
mined from `HAR-DTA` (`info/recordings/Uni5/`). Reference client
`GalaxyWars v3.16.19-beta`, universe `uni5`, account `12:147:1` (`[M]`, player
`Redock`). Two areas:

- **Settings** (`/settings/*`) — five tabs: `Game | Planets | Vacation mode |
  Account | Sanctions`.
- **Premium** (`/premium/*`) — three tabs: `Packages | Buy Galaxium | Account`
  (real-money shop, currency **Galaxium**).

A feasibility tag follows each block: **[build now]** (fits gwx's existing data),
**[needs mechanic]**, or **[out of scope]**.

## Settings → Game — `/settings/game`

Four independent cards, each its own POST (all re-render **200**, no redirect).

### Fleets — form `fleet_settings` [needs mechanic]

| Field | Type | Observed |
|---|---|---|
| `fleet_settings[spyShortcutCount]` | text (required) | quick-dispatch probe count, default `1` |
| `fleet_settings[spyLimitOnSpy]` | text (required) | max recon probes, default `200` |
| `fleet_settings[raidOrder][]` | drag-sort list (hidden inputs) | `Iron, Lutinum, Hydrogen, Water` — raid loot priority |

POST body (form-urlencoded) captured twice — only the raid order differs:

```
fleet_settings[spyShortcutCount]=2
fleet_settings[spyLimitOnSpy]=200
fleet_settings[raidOrder][]=iron&…=lutinum&…=hydrogen&…=water
fleet_settings[send]=
fleet_settings[_token]=<csrf>
```

gwx has espionage fleets and loot, so these knobs map to its existing
`fleets`/combat payloads, but the *settings plumbing* (per-user prefs) is new.

### Fleet label — `/settings/game/fleet-label`

Just a **Manage** link to a sub-page (not captured).

### Tactical retreat [needs mechanic]

Read-only card: *"Tactical retreat is currently inactive."* + a help link. The
mechanic (auto-retreat a stationed fleet under attack) is referenced elsewhere
(system messages, `10-…`) but not configurable here.

### Interface — form `interface_settings` [build now]

| Field | Type | Options / default |
|---|---|---|
| `interface_settings[navigationPosition]` | select | `Left` / `Right` (**Right**) |
| `interface_settings[desktopSize]` | select | `Default` / `Small` |
| `interface_settings[mobileSize]` | select | `Default` / `Small` |
| `interface_settings[messagesPerPage]` | text (required) | `10` |
| `interface_settings[locale]` | select | `Deutsch` / `English` (**English**) |

Purely client preferences — the same POST stash pattern as the fleet settings
(single fieldset, `interface_settings[send]`, `interface_settings[_token]`).

## Settings → Planets — `/settings/planet`

A drag-sortable planet list (`data-controller="ajax-sort"`, reorder POSTs to
`/settings/planet/sort`, handle class `.grab`). Each planet row shows
`<coord> - <name>` plus, for every **non-home** planet:

- **Make home planet** → `/settings/planet/<planet-uuid>/change-home` (home icon)
- **Abandon planet** → `/settings/planet/<planet-uuid>/leave` (× icon)

Footer: *"N free planets remaining"* (here `7`; the account owns 3 planets).
The home planet row has no actions. **No rename form is present in this
capture** — planet naming happens at colonisation time / elsewhere, not from
Settings. [change-home/abandon: **build now**]

## Settings → Vacation mode — `/settings/vacation/`

Not a form here but a **status/description card** (the capture is blocked). The
copy pins the mechanic precisely [needs mechanic]:

- Can **only start when no own fleets are in flight** (a red note appears
  otherwise: *"Some of your fleets are still in flight. Once they have landed,
  you can switch to vacation mode."*).
- While active: **construction queues (buildings/research/ships/defense) are
  paused** and **resource production is frozen**.
- **Trade offers stay active** and trading-post storage continues to be credited.
- **Protection from new attacks and reconnaissance**, but **already-launched
  fleets still arrive**.
- **Minimum 12 hours**; **auto-ends after 21 days** if you don't return.

## Settings → Account — `/settings/account`

Header line: *"The account was created on 22.09.2026."* Then three POST forms
(no captured responses) plus four action buttons. **All [build now]** — gwx
already has PBKDF2 hashing + opaque sessions (`store.AuthStore`).

| Form | Fields | Notes |
|---|---|---|
| `change_username` | `name` (required), `currentPassword` (required) | rename player |
| `change_mail` | `mail` (required), `currentPassword` (required) | change email |
| `change_password` | `currentPassword`, `passwordFirst`, `passwordSecond` (all required) | **confirm field** = `passwordSecond` |

Each form carries its own `[send]` + `[_token]` CSRF. The **current password is
required for every** identity change. No min-length constraint is rendered in
this capture (Symfony validates server-side).

Action buttons (sub-routes, DTOs not captured):

- **Reset account** → `/settings/account/reset`
- **Delete account** → `/settings/account/delete`
- **External applications** → `/settings/account/external-application`
- **Change profile picture** → `/settings/account/avatar`

## Settings → Sanctions — `/settings/sanction`

Empty table `From | Until | Sanction | Category` (this account has none). No
form — sanctions are imposed by operators. **[out of scope]** (no admin tooling
in gwx).

## Premium

Currency: **Galaxium** (account balance shown on every premium page). Sub-nav:
`Packages | Buy Galaxium | Account`. **All [out of scope]** — real money, payment
provider and operator fulfilment.

### Packages — `/premium/packages`

`Galaxium: 0` + table `Package | Price | Status | (action)`. One package:

| Package | Price (Galaxium) | Status |
|---|---|---|
| Ad-free | 2.000 | Inactive (`/premium/packages/ad_free/buy`) |

### Buy Galaxium — `/premium/galaxium`

| Galaxium | Price |
|---|---|
| 5.000 | 4,99 € |
| 11.000 | 9,99 € |
| 22.000 | 19,99 € |
| 33.000 | 29,99 € |
| 60.000 | 49,99 € |
| 130.000 | 99,99 € |

Each row links to a **PayPal** checkout
(`/premium/galaxium/checkout/paypal-detail/<uuid>`, opens `target="_blank"`).
A warning banner announces the **universe end** (`01.11. at 20:00`).

### Account — `/premium/account`

Ledger table `Date | Galaxium | Note` (empty here) + a **Payments** button →
`/premium/account/transactions`.

## Endpoint summary

| Method | Route | Purpose |
|---|---|---|
| GET | `/settings/game` | fleets / label / retreat / interface |
| POST | `/settings/game` | `fleet_settings`, `interface_settings` (200 re-render) |
| GET | `/settings/game/fleet-label` | manage fleet labels |
| GET | `/settings/planet` | planet list |
| POST | `/settings/planet/sort` | reorder planets |
| GET | `/settings/planet/<uuid>/change-home` | set main planet |
| GET | `/settings/planet/<uuid>/leave` | abandon planet |
| GET | `/settings/vacation/` | vacation status / start |
| GET | `/settings/account` | account forms |
| POST | `/settings/account` | `change_username` / `change_mail` / `change_password` |
| GET | `/settings/account/{reset,delete,external-application,avatar}` | account maintenance |
| GET | `/settings/sanction` | sanction table (read-only) |
| GET | `/premium/packages` | buy perk packages |
| GET | `/premium/galaxium` | buy Galaxium (PayPal) |
| GET | `/premium/account` | Galaxium ledger |
| GET | `/premium/account/transactions` | payment history |

## Differences vs gwx (what implementing this needs)

gwx has **no settings page** at all. `BACKLOG.md` already lists *Account
settings (change username / email / password)* as **build-now**.

1. **Account settings [build now]** — a `/settings` page with three POST
   handlers over `store.AuthStore`: rename (`users.username`), change email,
   change password (PBKDF2 re-hash; require `currentPassword`; `passwordFirst ==
   passwordSecond`). Add **logout-everywhere** on password change and keep
   sessions opaque (already the case). Skip reset/delete/avatar/external unless
   wanted.
2. **Interface prefs [build now]** — a `user_settings` / `users.prefs` JSONB
   column for `navigationPosition`, sizes, `messagesPerPage`, `locale`; read
   them in `base.html` (nav side) and the messages pagination
   (`10-…`). Cheap and self-contained.
3. **Fleets prefs [needs mechanic]** — `spyShortcutCount` / `spyLimitOnSpy` /
   `raidOrder` only make sense once the quick-dispatch spy and raid-order loot
   pick exist; store the prefs now, wire them when those UIs land.
4. **Planets tab [build now]** — reuse the existing planet list; add
   `change-home` (flip `[M]`/main flag) and `leave`/abandon (needs a
   *last-planet* guard); the free-planet counter derives from the colony cap.
5. **Vacation mode [needs mechanic]** — a `vacation_since` timestamp on the
   account: pause all `queues` scheduling, freeze production, block new
   attacks/spy on the player's planets, keep active trade offers; enforce
   min-12 h / max-21-day windows. This is a real scheduler feature (see
   **I9-premium-vacation**, optional).
6. **Sanctions [out of scope]** — no admin/operator tooling in gwx.
7. **Premium [out of scope]** — no payments; record only. The one perk with a
   gameplay equivalent (*Ad-free*) is meaningless for gwx.

## Reconciliation with `BACKLOG.md`

- BACKLOG *"Account settings — change username / email / password (auth store
  already does hashing + sessions)"* is confirmed and now has exact field names
  (`currentPassword`, `passwordFirst`, `passwordSecond`).
- BACKLOG *"Settings → Game"* content list is confirmed field-by-field
  (`spyShortcutCount`, `spyLimitOnSpy`, `raidOrder`, fleet labels, tactical
  retreat, interface/messages-per-page) — the interface subset is build-now, the
  fleet subset is mechanic-gated.
- BACKLOG *"Premium, Vacation mode, Sanctions"* are documented above; Vacation is
  the only one with a plausible gwx port, and it needs scheduler support.
