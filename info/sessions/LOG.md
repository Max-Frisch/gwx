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
