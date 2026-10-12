# GWX ↔ GalaxyWars spec — 04 Fleet & missions

Missions are tabs on the fleet screen, each a form posting to `/app/<coord>/fleet/send` (JSON `{"message":"Fleet launched successfully"}` on success):

| Mission | Route | Notes |
|---|---|---|
| Attack | `fleet/attack?coordinate=…` | ships + target coord + speed + label + **raid order** (draggable Iron/Lutinum/Hydrogen/Water) + 'Attacks remaining' bucket (10000). |
| Espionage | `fleet/spy?coordinate=…` | spy probes only. |
| Own | `fleet/own?coordinate=…` | move/deploy between own planets; also transport. |
| Expedition | `fleet/expedition?coordinate=…` | 'There currently seem to be no possible expeditions' (feature exists in GW). |
| Colonize | via Colonization Ship | spend a Colony Ship at an empty slot. |


## Fleet form fields

- **Ships:** per-ident number input, `All`/`None` shortcuts.
- **Planet:** `toCoordinate[galaxy|system|planet]`.
- **Target list:** select a saved target list (see below).
- **Speed:** 100/90/80/70/60/50/40/30/20/10 %.
- **Label:** fleet label chosen from Settings → Game → fleet labels.
- **Raid order:** drag-order of resources to loot.
- **Start fleet** → `fleet#send`; live **target-info** panel via `POST /app/<coord>/fleet/target-info` shows owner, one-way duration, hydrogen consumption, cargo capacity.

## Target lists (farm lists)

`fleet/target-list` + `overview` / `connections` / `create` / `edit` / `delete` / `detail/<uuid>`; each list holds NPC/target coordinates + a comment, and can be attached to an attack.

## Travel

Distance/fuel/duration are server-computed (oGame-style quadratic distance). Observed same-coordinate one-way (planet `12:147:1`): Jackal ≈ 11:43, Spy Probe ≈ 00:07; 2 planets over (→`12:147:3`) Spy ≈ 00:01:34. Speed/consumption are modified by drive research (`Reduced consumption` = −0.5%/level).

