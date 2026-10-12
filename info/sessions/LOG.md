# Session log

Append one line per completed session: `date | session | result | commit`.

```
2026-10-12 | planning | split Uni5 HAR work into E1-E8 + I1-I9; recon index committed | (this commit)
2026-10-12 | E1 | defense catalog + combat spec (5 units; 03-combat gap closed) | 074fddc
2026-10-12 | E2 | trade/marketplace spec (07-trade.md) | 717fe63
2026-10-12 | E3 | alliance spec (08-alliance.md; rank rights not exercised in capture) | 747b229
2026-10-12 | E4 | highscore + statistics spec (09-rankings-statistics.md) | 37178ed
2026-10-12 | E5 | messages/events/search spec (10-messages-events-search.md) | c00d3b4
2026-10-12 | E6 | premium + settings spec (11-premium-settings.md) | c31bfa2
2026-10-12 | E7 | login + overview onboarding spec (12-onboarding.md) | 2f744c3
2026-10-12 | E8 | consolidated 01-12 spec; rewrote 05 nav; gen_spec owns 01-04/06; refreshed BACKLOG/HANDOFF | (this commit)
2026-10-12 | I1 | /search (planets+players) + /player/{id} profile + /settings (Account rename/email/password, Game prefs); users.settings JSONB | (this commit)
2026-10-12 | I2 | message centre: /messages inbox (system + report notifications), read/delete/archive, unread badge, report deep-links; messages table + engine emission | (this commit)
2026-10-12 | I3 | /events timeline: in-progress queues (all planets), own + incoming fleets, recent messages; derived, no new table | (this commit)
2026-10-12 | I4 | /statistics (battle balance / farming / resources) from per-unit combat reports; StatsStore + game aggregation | (this commit)
2026-10-12 | I5 | highscore expansion: Alliances placeholder + Planets/Battles tabs, sort/direction/page; Battles headline view from reports (ScoreStore.Battles) | (this commit)
2026-10-12 | I6 | alliances: directory/profile/create/apply/accept, internal members/board/management; fixed MEMBER/OFFICER/LEADER rights in AllianceStore; 0003 migration; highscore Alliances tab + [TAG] in galaxy/highscore/search/player | (this commit)
2026-10-12 | I7 | trade: /trade marketplace (public+private offers, filters), /trade/new (escrowed barter + 2% offer fee), accept/revoke, /trade/history, /trade/storage (credit all, expiry); Trading Post building + 0004 migration; game.Settle fee math | (this commit)
2026-10-12 | economy | drop the energy system; exact per-level production tables (x1 base, Uni5 2× = context); water→H₂ 5:1 (consumption = 5×H₂) in update_planet_resources (0005); exactly two speed modifiers (GAME_SPEED = build+research+production, FLEET_SPEED) | (this commit)
2026-10-12 | economy2 | rip out OGame energy everywhere (drop energy_used/energy_prod, schema + insert); implement Advanced Chemical Factory (water→H₂ 2:1) alongside Chemical Factory (5:1), both drawing one shared water pool that adds up and throttles together at zero (new planets.water_use + 0006 function); docs | (this commit)
2026-10-12 | I8 | defense roster: 5 real GW units retuned (Light/Laser/EMP/Plasma/Rocket), placeholders dropped (0007), Ionization + Explosive Projectiles techs, /defense page split from shipyard, ckk = (Atk+Def)/8000 documented | (this commit)
2026-10-12 | I9 | vacation mode + sanctions: users.vacation_since/until + sanctioned/reason (0008); production freeze in update_planet_resources, scheduler queue-pause + auto-end (FinalizeExpiredVacations), attack/queue/dispatch gates, /settings/vacation + /settings/sanction + banner | (this commit)
2026-10-12 | testing | full-loop integration test: ephemeral Postgres + migrations + real UI/scheduler at x10000/x100000; register→build→research→shipyard→raid→loot→return→colonize (+ transport); skips without a DB | (this commit)
2026-10-12 | UI cards | Construction card layout (grouped: Main/Resource/Military; per-resource cost tint, SVG tiles, Upgrade-to-level buttons) + Resources sub-tab (per-building output incl. −water, totals, storage, plunder-protected); Construction tabs reduced to Buildings+Resources | (this commit)
2026-10-12 | UI perf | fix scroll jank: remove per-card backdrop-filter (each card re-blurred the fixed starfield every frame). Queued U1 (info/sessions/U1-ui-refresh.md) for responsive+fast layout, design tokens, vendored htmx | (this commit)
2026-10-12 | U1 | UI refresh: design-token layer + refactored app.css, vendored htmx 1.9.12 (progressive enhancement), htmx content swaps for build/research/ship/defense + live resource ticker + self-polling queue countdown, off-canvas responsive nav, stacked cards, scrollable tables, focus-visible/reduced-motion polish; no per-card blur | (this commit)
```

## Recon (2026-10-12)

Inventory of the three new HARs in `info/recordings/Uni5/` (see `README.md`):

- `Login_overview.har` — 37 entries. `POST /login` → `GET /app/dashboard` →
  `/app/12:147:1/planet`; assets only otherwise.
- `construction_research_ships_defense_fleets_fleetmissions_galaxy.har` — 2791
  entries. Building (13) + research (11) + ship (14) techtrees with
  LiveComponent `calculate`, `/building` + `/research` + `/ship` index pages,
  fleet attack/spy/own/expedition, target lists, battle-simulator, galaxy.
  **No `/defense`** despite the filename.
- `defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har`
  — 1609 entries. `/defense` + 5 defense techtrees + `TechnologyDefense/calculate`,
  `/trading/*`, `/alliance/*`, `/highscore/{player,alliance,planet,battle-reports}`,
  `/statistic/{battle-balance,farming,resources}`, `/premium/*`, `/settings/*`,
  `/search`, `/event`, `/message/*`.

Conclusion: together the two big HARs cover **every** page the old spec listed as
"not captured". E1–E7 mine them; E8 consolidates.
