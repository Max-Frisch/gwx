# GWX ↔ GalaxyWars spec — 01 Catalog

Derived from the private Uni5 HAR captures (`info/recordings/Uni5/`). Raw HARs are git-ignored; this spec is the committed, human-readable distillation.

Reference client: `GalaxyWars v3.16.19-beta`, universe `uni5`, constructor planet `12:147:1` (mid-game account). Endpoint family: `/app/<coord>/technology-tree/{building,research,ship}/<ident>` plus Symfony UX LiveComponent `POST /app/_components/<coord>/Technology{Building,Research,Ship}/calculate`.

## Global

- **Resources:** Iron, Lutinum, Water, Hydrogen (all four plunderable; each has its own storage).
- **Cost scaling** (buildings + research): the cost of level `L` is `base × M(L)` where `M(1)=M(2)=1` and the **same multiplier series** applies to every item:
  `M = 1, 1, 2, 4, 7, 11.5, 18, 26.5, 37, 51, 67.5, 87, …`
  Fitted (L ≥ 3): `M(L) ≈ (L³ + 15) / 20` — i.e. cost grows ≈ cubically (~`L³/20`). Coefficients are exact per-level in the tables below.
- **Points:** constant per level, shown on each item (economy points only; there is no fleet/defense score).

## Buildings

| Name | ident | Category | Base cost (L1) | Pts/lvl | Requirements |
|---|---|---|---|---|---|
| Command Center | `commandCenter` | Main buildings | 500 Iron, 200 Lutinum | 1.75 | — |
| Research Center | `researchCenter` | Main buildings | 300 Iron, 100 Lutinum | 1 | — |
| Trading Post | `tradingPost` | Main buildings | 2.000 Iron, 2.000 Lutinum | 10 | Ship Factory  (1/1) |
| Iron Mine | `ironMine` | Resource buildings | 250 Iron, 20 Lutinum | 0.68 | — |
| Lutinum Refinery | `lutinumRefinery` | Resource buildings | 130 Iron, 20 Lutinum | 0.38 | — |
| Drilling Tower | `drillingTower` | Resource buildings | 100 Iron, 20 Lutinum | 0.3 | — |
| Chemical Factory | `chemicalFactory` | Resource buildings | 350 Iron, 20 Lutinum | 0.93 | Drilling Tower  (1/1) |
| Advanced Chemical Factory | `extendedChemicalFactory` | Resource buildings | 20.000 Iron, 15.000 Lutinum, 1.000 Hydrogen | 90 | Drilling Tower  (10/10); Command Center  (6/6) |
| Iron Storage | `ironStorage` | Resource buildings | 2.000 Iron, 2.000 Lutinum | 10 | Iron Mine  (5/5) |
| Lutinum Storage | `lutinumStorage` | Resource buildings | 2.000 Iron, 2.000 Lutinum | 10 | Lutinum Refinery  (5/5) |
| Water Tanks | `waterStorage` | Resource buildings | 2.000 Iron | 5 | Drilling Tower  (5/5) |
| Hydrogen Tanks | `hydrogenStorage` | Resource buildings | 2.000 Iron, 2.000 Lutinum | 10 | Chemical Factory  (5/5) |
| Ship Factory | `shipFactory` | Attack / Defense | 2.000 Iron, 4.000 Lutinum, 20 Hydrogen | 15.05 | — |
| Orbital Defense Station | `orbitalDefenceStation` | Attack / Defense | 400 Iron, 400 Lutinum, 20 Hydrogen | 2.05 | — |
| Planetary Shield | `planetaryShield` | Attack / Defense | 10.000 Iron, 2.000 Water, 10.000 Hydrogen | 55 | Ship Factory  (1/1); Orbital Defense Station  (1/1); Fusion Reactor  (1/1) |
| Fusion Reactor | `fusionReactor` | Attack / Defense | 10.000 Iron, 2.000 Water, 10.000 Hydrogen | 55 | Chemical Factory  (5/5) |


### Building effects (per level)

- **Iron Mine** — `Production (Iron)`: L1=32, L2=64, L3=113, L4=178, L5=261
- **Lutinum Refinery** — `Production (Lutinum)`: L1=20, L2=40, L3=70, L4=111, L5=163
- **Drilling Tower** — `Production (Water)`: L1=40, L2=80, L3=141, L4=223, L5=326
- **Chemical Factory** — `Production (Hydrogen)`: L1=8, L2=16, L3=28, L4=44, L5=65
- **Chemical Factory** — `Consumption (Water)`: L1=40, L2=80, L3=141, L4=223, L5=326
- **Advanced Chemical Factory** — `Production (Hydrogen)`: L1=100, L2=201, L3=354, L4=558, L5=816
- **Advanced Chemical Factory** — `Consumption (Water)`: L1=200, L2=403, L3=708, L4=1.117, L5=1.632
- **Iron Storage** — `Storage`: L1=360.000, L2=540.000, L3=840.000, L4=1.260.000, L5=1.800.000
- **Iron Storage** — `Cannot be plundered`: L1=7.200, L2=10.800, L3=16.800, L4=25.200, L5=36.000
- **Lutinum Storage** — `Storage`: L1=360.000, L2=540.000, L3=840.000, L4=1.260.000, L5=1.800.000
- **Lutinum Storage** — `Cannot be plundered`: L1=7.200, L2=10.800, L3=16.800, L4=25.200, L5=36.000
- **Water Tanks** — `Storage`: L1=360.000, L2=540.000, L3=840.000, L4=1.260.000, L5=1.800.000
- **Water Tanks** — `Cannot be plundered`: L1=7.200, L2=10.800, L3=16.800, L4=25.200, L5=36.000
- **Hydrogen Tanks** — `Storage`: L1=360.000, L2=540.000, L3=840.000, L4=1.260.000, L5=1.800.000
- **Hydrogen Tanks** — `Cannot be plundered`: L1=7.200, L2=10.800, L3=16.800, L4=25.200, L5=36.000
