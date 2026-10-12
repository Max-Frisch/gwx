# gwx

A deliberately small, self-contained **GalaxyWars-style** browser game — a
personal hobby project reviving the old glory of **GalaxyWars**, not a clone of
any live service.
Four resources, one catalog, no moons, no expeditions, no premium, no academy.

The design goal is **simplicity + fog of war**: there is no military score, so
nobody can tell a 20-ship trader from a hidden monster fleet, and every planet's
storage protects a share of its resources from plunder.

## Stack

- **Go 1.24**, single binary (`cmd/server`).
- **PostgreSQL 16** as the sole source of truth.
- **Standard-library** server-rendered UI (`html/template`) + hand-written CSS.
  No Node, no build step, no templ codegen.
- No Redis, no cron: a durable scheduler polls Postgres.

## Quickstart

```bash
cp .env.example .env          # edit DATABASE_URL if needed
docker compose up -d          # Postgres 16 on :5433
make migrate                  # apply migrations/0001_init.sql
make run                      # http://localhost:8090
```

Register an account in the browser; you spawn on a random homeworld with a
level-1 Command Center.

`make build`, `make test`, `make lint` work as expected. On Windows, run the
underlying `go` commands directly if `make` is unavailable.

## Configuration (per server start)

The universe block is **applied/upserted on every start**, so tuning a round is
just editing `.env` and restarting:

| Variable | Meaning |
|---|---|
| `UNIVERSE_CODE` | stable key, e.g. `universe_gwx_1` |
| `GAME_SPEED` | build + research time **and** resource production multiplier (higher = faster) |
| `FLEET_SPEED` | fleet travel speed multiplier |
| `MAX_GALAXIES`, `MAX_SYSTEMS` | universe bounds |
| `ENDS_AT` | optional RFC3339 seasonal end (empty = long-term universe) |

GalaxyWars has exactly **two** speed modifiers — game speed and fleet speed.
Two flavours of round therefore come from the same binary: a slow long-term
universe, or a fast seasonal one.

## The game

- **Resources:** Iron, Lutinum, Water, Hydrogen. Water is pumped by the Drilling
  Tower; the Chemical Factory (**5:1**) and Advanced Chemical Factory (**2:1**)
  convert it into hydrogen, both drawing from the same water pool — if water
  runs dry, hydrogen stops.
- **Buildings:** Command Center, Research Center, Trading Post, the three
  extraction buildings (Iron Mine, Lutinum Refinery, Drilling Tower), Chemical
  Factory, Advanced Chemical Factory, four storages, Ship Factory, Orbital
  Defense Station, Fusion Reactor and Planetary Shield. Costs, prerequisites and
  the shared cost-curve multiplier are taken verbatim from the Uni5 capture.
- **No energy.** There is no electricity system: mines always produce at 100%.
- **Research (account-wide):** three categories — *Drive* (Combustion, Ion, Space
  Curvature, Space Folding), *Weapons* (Ionization, Energy Focusing, Explosive
  Projectiles) and *Ship* (Spy Technology, Advanced Ship Armor, Increased Cargo
  Capacity, Recycling Technology). Drives gate the bigger hulls.
- **Ships:** Spy Probe, Jackal, Small/Large Trader, Renegade, Raider, Recycler,
  Falcon, Colonization Ship, Tjuger, Cougar, Longeagle V, Noah, Longeagle X.
- **Defense:** Light Laser Turret, Laser Turret, EMP Launcher, Plasma Turret,
  Rocket Launchers.
- **Missions:** Transport, Deploy, Attack, Espionage, Colonize.
- **Account lifecycle:** vacation mode (freezes production, pauses queues, blocks
  attacks both ways; 12h minimum, 21d auto-end) and operator sanctions (flag +
  banner + gated actions).

### Signature mechanics

1. **Fog of war.** No points, no fleet-strength readout anywhere. The galaxy
   view shows owners only; you learn composition by scouting or by getting hit.
2. **Protected storage floor.** `loot = min(cargo capacity, max(0, stored −
   protected))`, where `protected = 2%` of that resource's storage capacity
   (`Storage(L) = 60_000·L² + 300_000` in the capture). A cushion always
   survives a raid.

Combat is a simple aggregate, simultaneous-fire model with **rapid-fire off**
(see the backlog). Shields fully regenerate each round.

## Architecture

```
cmd/server            wiring: HTTP + one durable scheduler
internal/config       env + universe configuration
internal/auth         PBKDF2 passwords, opaque hashed session tokens
internal/game         PURE domain: catalog, economy, tech tree, flight, combat
internal/store        Postgres access (planets, queues, fleets, reports, auth)
internal/engine       durable scheduler: claim due rows, resolve idempotently
internal/web          html/template UI + embedded static assets
migrations            numbered idempotent SQL + update_planet_resources()
```

Key ideas (borrowed from the sibling `neoxnova` project):

- **On-demand accrual.** `update_planet_resources(id)` (PL/pgSQL) row-locks the
  planet and accrues resources from the last calculation. Never cron-updated.
- **Durable scheduler.** Each tick claims due `queues` / `fleets` with
  `FOR UPDATE SKIP LOCKED`, re-checks the condition inside the transaction and
  applies the effect idempotently. Safe to restart; multi-instance safe.
- **One generic queue table** drives construction, research and ship/defense
  production; partial unique indexes enforce one active order per scope.
- **Catalog-keyed store.** `planet_items` holds buildings (as levels) and
  ships/defense (as counts); `user_techs` holds account-wide research.

## License / intent

Hobby project for personal retro play. Names are drawn from the original
GalaxyWars for authenticity and can be renamed; assets are placeholders.
