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

## Todo (ordered)
- [ ] **Rapid-fire combat** — per-unit rapid-fire table, behind a universe flag.
- [ ] **Player trading** — Trade Ship + resource trade offers between players.
- [ ] **Alliances** — membership, diplomacy (NAP/war), shared board.
- [ ] **Debris + Recycler** — battle debris field at `debris_rate`, recycle mission.
- [ ] **Moon** — optional moon chance on large attacks; jump gate later.
- [ ] **Seasonal lifecycle** — automatic archive/reset at `ENDS_AT`; round seeding.
- [ ] **Hall of Fame / stats** — economy/research only, never fleet composition.
- [ ] **UI reskin** — replace placeholder styling with a proper retro theme.
- [ ] **JSON API** — thin `/api/v1` surface for tooling/bots.
- [ ] **Full-loop integration test** — register → … → abandon.
- [ ] **Anti-farm / noob protection** — optional points-ratio guard.
