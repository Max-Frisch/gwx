# gwx — backlog

Read first; one item per session. Keep this file short.

## Done (MVP)
- Standalone Go + Postgres service, stdlib `html/template` UI, no build step.
- Schema + `update_planet_resources()` (coupled water→hydrogen, row-locked).
- Durable scheduler: generic `queues` (building/research/ship/defense) + fleets,
  `FOR UPDATE SKIP LOCKED`, idempotent.
- Catalog: 13 buildings, 8 research, 6 ships, 8 defenses (original GW names).
- Economy: ground-truth per-level production tables (no energy gate), storages,
  protected floor.
- Fleet missions: Transport, Deploy, Attack, Espionage, Colonize; combat + loot
  (no rapid-fire) + return leg; combat/espionage reports.
- Auth (PBKDF2 + opaque sessions), registration → homeworld.
- Per-start universe config (speeds, bounds, optional `ENDS_AT`).
- Verified end-to-end: build → research → shipyard → raid → loot → return,
  colonize, and all pages render.
- **UI reskin** — dark-navy shell with starfield, left sidebar (grouped nav),
  top resource strip, centered page tabs, steel-blue accents. Matches the
  GalaxyWars reference screenshots (see `info/`). Pure CSS + `html/template`.
- **Highscore / Hall of Fame** — Players + Planets tabs, sortable. Economy
  points only (buildings + research) from `game.LevelPoints` /
  `store.ScoreStore`; no military points. Link under Community.
- **Uni5 follow-up HAR walk** — all previously-missing pages captured and mined
  (defense, trade, alliance, highscore/statistics, messages/events/search,
  premium/settings, login/overview). Spec now `info/galaxywars-spec/01`–`12`;
  implementation split into `I1`–`I9` in `info/sessions/`.
- **I1 search + settings** — `/search` (planets by name, players by username),
  `/player/{id}` profile (economy points + planets, no fleet info — fog of war),
  and `/settings` with Account (rename / email / password, current-password
  gated) and Game (nav position, locale, messages-per-page, spy knobs) tabs.
  New `users.settings` JSONB; password change revokes other sessions.
- **I2 messages** — `/messages` inbox: system notifications (building/research/
  ship/defense completions, own-fleet returns) and pointers to combat/espionage
  reports, filtered by kind, with read / archive / delete, an unread badge in
  the sidebar, and deep links to the matching `/reports#report-{id}` entry.
  New `messages` table; emitted from the engine where queues/fleets resolve and
  reports are written. (`10-messages-events-search.md`)
- **I3 events** — `/events` timeline (nav: UNIVERSE): in-progress building/
  research/unit queues across all planets, own fleets in flight and hostile
  incoming, plus recent resolved activity from `messages`. Purely derived from
  `queues`/`fleets`/`messages` — no new table (`I3-events.md`).
- **I4 statistics** — `/statistics` (nav: ACCOUNT): Battle balance (per-unit
  destroyed/lost resource value for ships + defense, plundered/lost resources,
  win/loss/tie), Farming balance (raids, loot totals, average haul) and Resources
  (plundered vs lost). Combat reports now record per-unit losses and the viewer's
  role; `StatsStore` aggregates the player's own JSONB payloads (`I4-statistics.md`).
- **I5 highscore expansion** — Highscore now has Players / Alliances / Planets /
  Battles tabs with server-side `?sort=&direction=&page=` (100/page, reference
  query shape). Battles is a gwx-native headline view built from `reports`
  (`ScoreStore.Battles`): date, attacker/defender, coordinates, total destroyed +
  plundered resource value. No fleet/defense composition is ever exposed
  (`I5-highscore-expansion.md`).
- **I6 alliances** — `/alliance` directory (paged search), public profile,
  create/apply/withdraw, and an internal member area (overview, members, board,
  applications, management). Fixed rank model `MEMBER`/`OFFICER`/`LEADER` with
  rights enforced transactionally in `AllianceStore`; open/closed joining,
  `min_points` gate, leader transfer on leave, founder close. `alliances` +
  `alliance_members` + `alliance_applications` + `alliance_posts` (migration
  `0003`). `ScoreStore.Alliances` powers the Highscore Alliances tab; `[TAG]`
  now shows next to usernames in galaxy/highscore/search/player.
- **I7 trade** — `/trade` marketplace (public + directed/private offers, offered/
  wanted filters), `/trade/new` (barter offer, escrowed goods + 2% up-front fee),
  accept with atomic settlement into a separate per-planet trade storage,
  revoke/expiry with escrow refund, `/trade/history` audit and `/trade/storage`
  with *credit all*. New **Trading Post** building gates offer creation and sets
  the per-planet offer slots (= level). `trade_offers` + `trade_history` +
  `trade_storage` (migration `0004`); `game.Settle` fee math (`07-trade.md`).

## Todo (ordered)
- [ ] **Rapid-fire combat** — per-unit rapid-fire table, behind a universe flag.
- [ ] **Alliance diplomacy** — NAP/war declarations between alliances (follow-up
      to I6; membership, ranks, applications and the shared board already land).
- [ ] **Debris + Recycler** — battle debris field at `debris_rate`, recycle mission.
- [ ] **Moon** — optional moon chance on large attacks; jump gate later.
      (Legacy/misread: reference GW has no moons; `[M]` means *main planet*.)
- [ ] **Seasonal lifecycle** — automatic archive/reset at `ENDS_AT`; round seeding.
- [ ] **JSON API** — thin `/api/v1` surface for tooling/bots.
- [ ] **Full-loop integration test** — register → … → abandon.
- [ ] **Anti-farm / noob protection** — optional points-ratio guard.

## Reference spec (derived from Uni5 HARs — `info/galaxywars-spec/`)

Real GalaxyWars v3.16.19-beta data extracted from the private Uni5 captures
(`tools/har/harscan.py` → `extract_catalog.py` → `extract_pages.py` →
`fit_formulas.py` → `gen_spec.py`). Full details in
`info/galaxywars-spec/01..12`; implementation queue in `info/sessions/`.
Headline facts worth folding into gwx:

- **Cost scaling:** `cost(L) = base × M(L)`, `M(1)=M(2)=1`, then
  `1,2,4,7,11.5,18,26.5,37,51,67.5,87,…` ≈ `(L³+15)/20` (near-cubic growth).
- **Storage:** `60_000·L² + 300_000`; **protected ("concealed") floor = 2%** of
  storage (`1_200·L² + 6_000`). NOTE: our README/backlog say 20% — GW is 2%.
- **Research effects:** drive techs = speed bonus + `−0.5%/level` fuel;
  utility techs (ionization/armor/cargo/spy/explosives/focusing) = **`10·√L %`**;
  recycling = **`1%/level`**.
- **Hydrogen:** Chemical Factory water→H₂ **5:1**; Advanced Chemical Factory
  **2:1**.
- **Points:** constant per item/level, economy-only (matches our model).
- **Speed of the source data:** Uni5 ran at **2× game speed / 1× fleet speed**.
  The captured production tables are treated as the **x1 base** (Uni5's 2× is
  context only). gwx has exactly **two** runtime modifiers — `GAME_SPEED`
  (build/research time **and** production) and `FLEET_SPEED` (`internal/config`),
  so the multiplier stays a clean knob rather than being baked in.
- **Defense (now captured — `03-combat.md`):** all five units have costs/stats.
  `lightLaserTower` 600 Iron (Atk 350/Def 150, ckk 0.06); `laserTower` 2.000
  Iron + 200 Lutinum (1.000/700, 0.22); `empLauncher` 200 Iron + 5.000 Lutinum
  (1.800/1.000, 0.35); `plasmaTower` 2.000 Iron + 2.000 Lutinum + 50 Water
  (700/1.400, 0.26); `rocketDefenseCannons` 15.000 Iron + 2.000 Lutinum + 2.000
  Water (4.500/8.100, 1.58). All require Orbital Defense Station 1.
- **Follow-up pages (now captured):** trade (`07`), alliance (`08`), highscore +
  statistics (`09`), messages/events/search (`10`), premium/settings (`11`),
  login/overview (`12`) — see the matching files for field-level detail.

## Content for later (from GalaxyWars reference screenshots in `info/`)

Not implemented; captured here so we can cherry-pick. Reference is `GalaxyWars
v3.16.19-beta`, a modern-dark skin (not the old grey bevel) despite the retro
feel. (Values below now backed by `info/galaxywars-spec/`.)

**Nav sections seen** — PLANET: Overview, Construction, Research, Ships/Defense,
**Trade**, **Technology**. UNIVERSE: Fleets, Galaxy, **Events**. COMMUNITY:
**Messages, Search, Alliance, Highscore**. ACCOUNT: **Statistics, Premium,
Settings**, Logout. (Bold = we don't have it.)

**Whole systems missing**
- **Player trading / marketplace**: public vs private offers, rates, fees,
  offer duration/expiry, "My offers", trade storage, trading post building,
  trading fleets, quick-add amount buttons, flagged/marked trades.
- **Alliance**: tags (e.g. `[ThF-]`), membership, diplomacy, shared board.
- **Highscore**: tabs for Players / Alliances / Planets / Battles; columns
  Planet points / Research points / Total points / Planets. *(Players + Planets +
  Battles done in I5; Alliances done in I6.)*
  NOTE: this does **not** conflict with fog-of-war. The game HAS points, but
  only **economy points (buildings + research)** — there are **no military
  points** (no fleet/defense score). That is the whole point: a 10k-point player
  can field 10× the ships of a 50k-point player who sank everything into
  infrastructure/research. So a leaderboard is fully consistent with the design.
- **Statistics**: Battle balance, Farming balance, Resources, Recycled
  resources (destroyed / plundered / gained / lost).
- **Messages** (inbox) and **Search** (player/planet lookup).
- **Events** feed.
- **Premium**, **Vacation mode**, **Sanctions**.
- **Settings → Game**: quick-dispatch probe count, max recon probes, raid order,
  fleet labels, tactical retreat, interface/messages-per-page options.
- **Settings → Account**: rename/email/password, reset/delete account, profile
  picture, external applications.
- **`[M]` tag** = **Main Planet** (= homeworld), *not* a moon. **GW has no
  moons** — drop the moon assumption (our own "Moon" todo below is legacy from
  that misread; keep it only if we deliberately add the feature).
- **Footer**: Help / Game rules / Support / Status / Legal notice / version.

**Roster gaps** (nice-to-have content) — real stats in `info/galaxywars-spec/01,03`
- Techs: Space Curvature Drive, Ionization, Energy Focusing, Explosive
  Projectiles, Spy Technology, Advanced Ship Armor, Increased Cargo Capacity,
  Recycling Technology. Effects now known: drives = speed + `−0.5%/lvl` fuel;
  utility techs = `10·√L %`; recycling = `1%/lvl` debris.
  Recycling Technology has *two* jobs: (1) unlock the Recycler ship
  (req: Ship Factory 1 + Combustion Drive 2), (2) raise the % of debris
  collected. Debris never appears on the planet — it is hauled directly
  during/after battle.
- Ships (costs/stats in `03-combat.md`): Jackal, Renegade, Raider, Falcon,
  Tjuger, Longeagle V, Noah (ark), Small Trader, Large Trader. Colonization
  Ship + Recycler + Spy Probe already covered. Our "Trade Ship"/"Colony Ship"
  map to their Trader / Colonization Ship.
- Defense: Light Laser Turret, Laser Turret, EMP Launcher, Plasma Turret,
  Rocket Launchers — costs/stats now captured in `03-combat.md` (see the
  "Defense" headline above), all gated by Orbital Defense Station 1.
- Buildings: Trading Post, Advanced Chemical Factory (water→H₂ 2:1 vs 5:1);
  alt names Lutinum Refinery / Ship Factory / Water Tanks / Hydrogen Tanks.

### Ready to build now (existing data / mechanics only — no live data)

These need no new subsystems and no external data; the tables/mechanics already
exist. Ordering is cheapest/highest-value first; the bracketed session is the
build task in `info/sessions/`:

1. **Highscore / Hall of Fame** — *done* (Players + Planets). Compute economy
   points from `planet_items` (building levels) + `user_techs` (research
   levels). Economy/research only, no military points.
2. **Account settings (`I1`)** — *done.* Change username / email / password
   (current password required; other sessions revoked), plus interface prefs
   (`navigationPosition`, `messagesPerPage`, `locale`, spy knobs) in
   `users.settings` (`11-premium-settings.md`).
3. **Search + player profile (`I1`)** — *done.* Find planets by name and players
   by username; `/player/{id}` hub with economy points + owned planets
   (`10-messages-events-search.md`).
4. **Statistics from `reports` (`I4`)** — *done.* `/statistics` aggregates per-unit
   destroyed/lost resource value, looted/lost resources and farming totals from
   the player's own combat reports. ("Recycled" needs debris first.)
5. **Defense roster (`I8`)** — the five units are catalog rows + prereqs and
   immediately build/fight; real costs/stats in `03-combat.md`.
6. **Messages from `reports` + system feed (`I2`)** — *done.* `/messages` lists
   combat/espionage reports by kind plus system notifications with read/archive/
   delete and an unread badge (`10-messages-events-search.md`).
7. **Highscore expansion (`I5`)** — *done.* Alliances tab is a disabled
   placeholder until I6 lands; Players headers are sortable with the reference
   `?sort=&direction=&page=` shape; *Battles* is a gwx-native headline view over
   `reports` (`ScoreStore.Battles`), no composition leaked.
8. **Static Help / Game rules pages + footer links** — pure templates.
9. **Roster additions that work with today's combat model** — new ships
   (Jackal, Renegade, Raider, Falcon, Tjuger, Longeagle V, Noah, Small/Large
   Trader) are just catalog rows + prereqs. Prereq-only techs (Ionization,
   Explosive Projectiles, Energy Focusing) can gate them.

**Bigger but still self-contained** (own tables, no external dependency):
Alliances (`I6`, *done* — diplomacy still open), Trade/market (`I7`, *done*),
Events feed (`I3`, *done* — derived, no table), Vacation mode (`I9`).

**Not standalone** (need a new mechanic first): debris + Recycling Technology,
Advanced Ship Armor hull bonus, extra cargo tech, Space Curvature Drive
movement effect, Premium, Sanctions.
