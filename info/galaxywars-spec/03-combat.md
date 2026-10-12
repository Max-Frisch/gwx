# GWX ↔ GalaxyWars spec — 03 Combat & units

Ship stat block is taken from the technology-tree ship pages (`Base` = hull default, `Current` = with this account's research). The last stat row is literally labelled **`ckk`** in the client — recorded as-is. Fitting the captured values shows `ckk` is **not an independent stat**: for the combat units it is `(Attack + Defense) / 8000` (e.g. Rocket Launchers `(4500 + 8100) / 8000 = 1.58`; Light Laser Turret `(350 + 150) / 8000 = 0.06`), i.e. a derived combat-strength coefficient.

## Ships

| Ship | ident | Cost | Attack | Defense | Cargo | Speed | Consumption | ckk |
|---|---|---|---|---|---|---|---|---|
| Jackal | `schakal` | 250 Iron, 75 Hydrogen | 50→50 | 100→124 | 250→300 | 800→1.239 | 10→9 | 0.02→0.02 |
| Recycler | `recycler` | 850 Iron, 850 Lutinum, 100 Water | 300→300 | 100→124 | 10.000→12.000 | 1.600→2.479 | 30→28 | 0.05→0.05 |
| Spy Probe | `spySatellite` | 150 Lutinum | 1→1 | 1→1 | 15→18 | 9.000.000→13.942.740 | 1→1 | 0→0 |
| Renegade | `renegade` | 1.000 Iron | 200→200 | 200→248 | 200→240 | 1.300→2.014 | 25→23 | 0.05→0.06 |
| Raider | `raider` | 500 Iron, 500 Lutinum | 150→150 | 300→373 | 900→1.080 | 2.000→2.530 | 40→39 | 0.06→0.06 |
| Falcon | `falcon` | 2.000 Lutinum | 1.300→1.300 | 150→186 | 4.000→4.800 | 1.000→1.265 | 100→97 | 0.12→0.13 |
| Colonization Ship | `colonisationShip` | 10.000 Iron, 15.000 Lutinum, 1.000 Water, 15.000 Hydrogen | 1→1 | 1.000→1.244 | 100.000→120.000 | 15.000→18.974 | 800→776 | 0.01→0.01 |
| Tjuger | `tjuger` | 2.000 Iron, 2.000 Lutinum | 2.000→2.000 | 900→1.120 | 2.000→2.400 | 3.500→3.671 | 60→60 | 0.35→0.39 |
| Cougar | `cougar` | 4.000 Iron, 5.000 Lutinum | 2.250→2.250 | 6.500→8.092 | 24.000→28.800 | 2.100→2.202 | 90→90 | 1→1.12 |
| Longeagle V | `longeagleV` | 50.000 Iron, 5.000 Lutinum | 15.000→15.000 | 35.000→43.573 | 34.000→40.800 | 2.700→2.832 | 200→199 | 5.99→6.69 |
| Small Trader | `smallTrader` | 2.000 Iron, 2.000 Lutinum | 1→1 | 50→62 | 9.000→10.800 | 1.900→2.943 | 10→9 | 0→0 |
| Large Trader | `largeTrader` | 8.000 Iron, 2.000 Lutinum, 5.000 Hydrogen | 15→15 | 12.000→14.939 | 50.000→60.000 | 4.500→4.720 | 30→30 | 0.11→0.12 |
| Noah | `noah` | 50.000 Lutinum, 2.000 Water | 1.000→1.000 | 24.000→29.878 | 250.000→300.000 | 8.000→8.000 | 20→20 | 1.28→1.43 |
| Longeagle X | `longeagleX` | 250.000 Lutinum, 25.000 Water | 92.000→92.000 | 50.000→62.247 | 150.000→180.000 | 6.000→6.000 | 125→125 | 17.73→19.79 |


### Requirements

- **Jackal** (`schakal`): Ship Factory  (1/1); Combustion Drive  (1/1)
- **Recycler** (`recycler`): Ship Factory  (1/1); Combustion Drive  (2/2); Recycling Technology  (1/1)
- **Spy Probe** (`spySatellite`): Ship Factory  (1/1)
- **Renegade** (`renegade`): Ship Factory  (1/1); Combustion Drive  (2/2)
- **Raider** (`raider`): Ship Factory  (1/1); Ion Drive  (1/1)
- **Falcon** (`falcon`): Ship Factory  (1/1); Ion Drive  (2/2); Spy Technology  (5/5)
- **Colonization Ship** (`colonisationShip`): Ship Factory  (1/1); Ion Drive  (4/4)
- **Tjuger** (`tjuger`): Ship Factory  (1/1); Space Curvature Drive  (1/1)
- **Cougar** (`cougar`): Ship Factory  (1/1); Space Curvature Drive  (1/1); Advanced Ship Armor  (1/1)
- **Longeagle V** (`longeagleV`): Ship Factory  (1/1); Space Curvature Drive  (1/4)
- **Small Trader** (`smallTrader`): Ship Factory  (1/1); Combustion Drive  (1/1)
- **Large Trader** (`largeTrader`): Ship Factory  (1/1); Space Curvature Drive  (1/5)
- **Noah** (`noah`): Ship Factory  (1/1); Space Folding Drive  (0/1)
- **Longeagle X** (`longeagleX`): Ship Factory  (1/1); Space Folding Drive  (0/2)

## Build time

The ship calculator's `Level` input is the **Ship Factory level**; it lists the per-unit build time at each factory level (higher factory → shorter time). Level 1 is the captured per-unit duration; every higher factory level divides it by a shared divisor `D(F)` (faster-than-linear):

`D(F) = 1, 2, 3.5, 5.5, 8, 11.5, 15.5, 20, 25, 30.5, 37, 44, 51.5, 59.5, 68, 77.5, 87.5, 98, 109, 120.5, 133, 146, 159.5, 173.5, 188, 203.5, 219.5, 236, 253, …`

The series is identical for **every ship and every defense unit** (confirmed across Jackal, Falcon, Longeagle X and all five defense units). Extrapolated past L29 with `1 + (F²+F−2)/4 + ½·Σ_{k=1}^{F−1} ⌊k/5⌋`, which reproduces all 29 captured values exactly.

- **Jackal:** F1=00:20:00, F2=00:10:00, F3=00:05:42, F4=00:03:38, F5=00:02:30, F6=00:01:44, F7=00:01:17, F8=00:01:00, F9=00:00:48, F10=00:00:39, F11=00:00:32
- **Falcon:** F1=10:00:00, F2=05:00:00, F3=02:51:25, F4=01:49:05, F5=01:15:00, F6=00:52:10, F7=00:38:42, F8=00:30:00, F9=00:24:00, F10=00:19:40, F11=00:16:12
- **Longeagle X:** F1=13 Tage, 21:20:00, F2=6 Tage, 22:40:00, F3=3 Tage, 23:14:17, F4=2 Tage, 12:36:21, F5=1 Tag, 17:40:00, F6=1 Tag, 04:59:07, F7=21:30:19, F8=16:40:00, F9=13:20:00, F10=10:55:44, F11=09:00:32

**gwx:** implemented in `internal/game/game_math.go` (`unitTimeDivisors` / `UnitTimeMultiplier`) as the exact captured divisor series; ship `TimeSecs` are the captured level-1 durations and `def.TimeSecs / D(level)` is the per-unit time.

## Defense

Defense units are fixed emplacements: they have **no cargo, speed or consumption**. Stats come from the technology-tree defense pages (`Base` = hull default, `Current` = with this account's research). The calculator's `Level` input is the **Orbital Defense Station** level; the build time below is at this account's current ODS level (L4, matching the `/defense` page's `Duration`).

| Unit | ident | Cost | Attack | Defense | ckk | Build time | Requirements |
|---|---|---|---|---|---|---|---|
| Laser Turret | `laserTower` | 2.000 Iron, 200 Lutinum | 1.000→1.000 | 700→871 | 0.22→0.24 | 00:15:09 | Orbital Defense Station  (1/1); Combustion Drive  (1/1) |
| EMP Launcher | `empLauncher` | 200 Iron, 5.000 Lutinum | 1.800→1.800 | 1.000→1.244 | 0.35→0.39 | 00:37:52 | Orbital Defense Station  (1/1) |
| Plasma Turret | `plasmaTower` | 2.000 Iron, 2.000 Lutinum, 50 Water | 700→700 | 1.400→1.742 | 0.26→0.29 | 00:01:17 | Orbital Defense Station  (1/1); Ionization  (0/4) |
| Light Laser Turret | `lightLaserTower` | 600 Iron | 350→350 | 150→186 | 0.06→0.07 | 00:01:30 | Orbital Defense Station  (1/1) |
| Rocket Launchers | `rocketDefenseCannons` | 15.000 Iron, 2.000 Lutinum, 2.000 Water | 4.500→4.500 | 8.100→10.084 | 1.58→1.76 | 00:15:09 | Orbital Defense Station  (1/1); Explosive Projectiles  (0/10) |


### Implemented roster (I8)

The roster (`internal/game/catalog.go`) now matches the reference **exactly**; the
client idents map onto gwx codes as follows:

| GW ident | Label | gwx code | Cost | Attack | Defense→`Shield` | `Hull` | Prereqs |
|---|---|---|---|---|---|---|---|
| `lightLaserTower` | Light Laser Turret | `light_laser` | 600 Iron | 350 | 150 | 500 | Orbital Defense Station 1 |
| `laserTower` | Laser Turret | `heavy_laser` | 2.000 Iron, 200 Lutinum | 1.000 | 700 | 1.700 | Orbital Defense Station 1; Combustion Drive 1 |
| `empLauncher` | EMP Launcher | `emp_launcher` | 200 Iron, 5.000 Lutinum | 1.800 | 1.000 | 2.800 | Orbital Defense Station 1 |
| `plasmaTower` | Plasma Turret | `plasma_turret` | 2.000 Iron, 2.000 Lutinum, 50 Water | 700 | 1.400 | 2.100 | Orbital Defense Station 1; Ionization 4 |
| `rocketDefenseCannons` | Rocket Launchers | `rocket_launcher` | 15.000 Iron, 2.000 Lutinum, 2.000 Water | 4.500 | 8.100 | 12.600 | Orbital Defense Station 1; Explosive Projectiles 10 |

Stat-mapping decisions:

- **Attack** → `Attack`; reference **Defense** → `Shield` (it is the stat raised by
  the defensive research and our shields regenerate each round, exactly like the
  reference's defence value).
- The reference exposes **no hull** for emplacements, and `ckk` turns out to be a
  derived coefficient (see the header). We set `Hull = Attack + Defense`
  (equivalently `ckk × 8000`) so durability scales with the captured combat
  weight; `ckk` itself is not stored.
- **Build time.** Defense uses the same captured per-unit divisor curve as ships
  (factory = Orbital Defense Station): `TimeSecs` is the captured level-1 (ODS
  L1) per-unit duration and `TimeSecs / D(ODS level)` gives the build time
  (e.g. Laser Turret ODS L4 → `5000 / 5.5 = 909s`, matching the captured
  `00:15:09`).
- **Dropped placeholders:** the gwx-specific `defense_turret`, `ion_cannon`,
  `gauss_cannon`, `small_shield_dome` and `large_shield_dome` are removed
  (migration `0007`). They were invented stand-ins with no GalaxyWars counterpart.
- **Prereq-only techs.** `plasma_turret` needs **Ionization 4** and
  `rocket_launcher` needs **Explosive Projectiles 10**; neither tech existed in
  gwx, so both are added as research rows (gwx placeholder costs — the capture
  lists them only as prerequisites).

Planetary Shield + Fusion Reactor are the two defense-relevant *buildings*: the Fusion Reactor is the Planetary Shield's building prerequisite (and adds a base attack value), while the shield adds a base defense value; there is no energy coupling.

## Battle simulator

`GET/POST /app/<coord>/fleet/battle-simulator` — attacker vs defender columns for ships + defense, seeded with your own buildings/research levels (`data-simulator-*` JSON attributes). Confirms the combat model is an aggregate simulation the server can run; no rapid-fire stats were exposed in the capture.

