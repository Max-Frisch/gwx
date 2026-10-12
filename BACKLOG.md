# gwx — backlog

Read first; one item per session. Keep this file short.

## Done (MVP)
- Standalone Go + Postgres service, stdlib `html/template` UI, no build step.
- Schema + `update_planet_resources()` (coupled water→hydrogen, row-locked).
- Durable scheduler: generic `queues` (building/research/ship/defense) + fleets,
  `FOR UPDATE SKIP LOCKED`, idempotent.
- Catalog: 13 buildings, 8 research, 6 ships, 8 defenses (original GW names).
- Economy: production, energy gate with base supply, storages, protected floor.
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

## Todo (ordered)
- [ ] **Rapid-fire combat** — per-unit rapid-fire table, behind a universe flag.
- [ ] **Player trading** — Trade Ship + resource trade offers between players.
- [ ] **Alliances** — membership, diplomacy (NAP/war), shared board.
- [ ] **Debris + Recycler** — battle debris field at `debris_rate`, recycle mission.
- [ ] **Moon** — optional moon chance on large attacks; jump gate later.
      (Legacy/misread: reference GW has no moons; `[M]` means *main planet*.)
- [ ] **Seasonal lifecycle** — automatic archive/reset at `ENDS_AT`; round seeding.
- [ ] **Hall of Fame / stats** — remaining Highscore tabs (Alliances, Battles)
      and the Statistics pages (battle/farming/components). Economy/research
      points only, never fleet composition.
- [ ] **JSON API** — thin `/api/v1` surface for tooling/bots.
- [ ] **Full-loop integration test** — register → … → abandon.
- [ ] **Anti-farm / noob protection** — optional points-ratio guard.

## Content for later (from GalaxyWars reference screenshots in `info/`)

Not implemented; captured here so we can cherry-pick. Reference is `GalaxyWars
v3.16.19-beta`, a modern-dark skin (not the old grey bevel) despite the retro
feel.

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
  Planet points / Research points / Total points / Planets.
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

**Roster gaps** (nice-to-have content)
- Techs: Space Curvature Drive, Ionization, Energy Focusing, Explosive
  Projectiles, Spy Technology, Advanced Ship Armor, Increased Cargo Capacity,
  Recycling Technology. (Each has a real function; details to be specced later
  from live data. E.g. Recycling Technology has *two* jobs: (1) unlock the
  Recycler ship as a tech prerequisite, (2) raise the % of debris collected.
  Debris never appears on the planet — it is hauled directly during/after
  battle.)
- Ships: Jackal, Renegade, Raider, Falcon, Tjuger, Longeagle V, Noah (ark),
  Small Trader, Large Trader. (Our "Trade Ship"/"Colony Ship" map to their
  Trader / Colonization Ship.)
- Defense: EMP Launcher, Rocket Launchers.
- Buildings: Trading Post, Advanced Chemical Factory; alt names Lutinum
  Refinery / Ship Factory / Water Tanks / Hydrogen Tanks.

### Ready to build now (existing data / mechanics only — no live data)

These need no new subsystems and no external data; the tables/mechanics already
exist:

1. **Highscore / Hall of Fame** — done (Players + Planets). Compute economy
   points from `planet_items` (building levels) + `user_techs` (research
   levels). Economy/research only, no military points.
2. **Search** — find planets by name and players by username (`planets`,
   `users`).
3. **Account settings** — change username / email / password (auth store already
   does hashing + sessions).
4. **Statistics from `reports`** — aggregate the combat JSONB payloads for
   destroyed / plundered. ("Recycled" needs debris first.)
5. **Static Help / Game rules pages + footer links** — pure templates.
6. **Roster additions that work with today's combat model** — new ships
   (Jackal, Renegade, Raider, Falcon, Tjuger, Longeagle V, Noah, Small/Large
   Trader) and defenses (EMP Launcher, Rocket Launchers) are just catalog rows +
   prereqs and immediately build/fight. Prereq-only techs (Ionization,
   Explosive Projectiles, Energy Focusing) can gate them.

**Bigger but still self-contained** (own tables, no external dependency):
Alliances, Trade/market, Vacation mode, Events feed.

**Not standalone** (need a new mechanic first): debris + Recycling Technology,
Advanced Ship Armor hull bonus, extra cargo tech, Space Curvature Drive
movement effect, Premium, Sanctions.
