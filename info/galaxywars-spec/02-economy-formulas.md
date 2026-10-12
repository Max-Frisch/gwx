# GWX ↔ GalaxyWars spec — 02 Economy formulas

All values fitted/observed from the Uni5 capture. Costs are exact per-level (see `01-catalog.md`); production is recorded as the exact per-level table and quoted here at the **x1 base** — the Uni5 capture ran at 2× game speed, which is context only. gwx applies `GAME_SPEED` to production at runtime. There is **no energy/electricity system**. The research bonuses below are the derived closed forms.

## Storage & protected floor (exact)

For every storage building (Iron/Lutinum/Water/Hydrogen Storage):

```
Storage(L)          = 60_000 · L² + 300_000      (exact)
Cannot be plundered = 1_200 · L² + 6_000 = 2% of Storage(L)  (exact)
```
So the **plunder-protected floor is 2%** of a resource's storage capacity (this game's 'concealed area'). Example: L20 → 24.3M storage, 486k protected.

## Extraction production (per hour)

Mines/refinery/tower columns from the technology-tree calculator:
- **Iron Mine → Iron @ mph:** 1: 32, 2: 64, 3: 113, 4: 178, 5: 261, 6: 376, 7: 509, 8: 660, 9: 828, 10: 1.015 …  (≈ quadratic/cubic growth; sample L1–L10)
- **Lutinum Refinery → Lutinum @ mph:** 1: 20, 2: 40, 3: 70, 4: 111, 5: 163, 6: 235, 7: 318, 8: 412, 9: 518, 10: 634 …  (≈ quadratic/cubic growth; sample L1–L10)
- **Drilling Tower → Water @ mph:** 1: 40, 2: 80, 3: 141, 4: 223, 5: 326, 6: 471, 7: 637, 8: 825, 9: 1.036, 10: 1.268 …  (≈ quadratic/cubic growth; sample L1–L10)

## Water → Hydrogen conversion

- **Chemical Factory** converts water into hydrogen at **5:1** (H2 = ⌊water/5⌋); its *water consumption* is exactly **5 × the H₂ produced**. Output stops when water is exhausted.
- **Advanced Chemical Factory** is the same shape at **2:1** (H2 = ⌊water/2⌋) with a much larger water draw. Hydrogen output stops when water is exhausted.

## Research bonuses (exact)

| Research | Effect |
|---|---|
| Drive techs (Combustion/Ion/Space-Curvature/Space-Folding) | `Bonus` = speed increase; `Reduced consumption` = +0.5% per level (B(L) = 0.5·L %). Bonus series: 4.88%, 9.54%, 14.02%, 18.32%, 22.47%, 26.49% … (≈ `1−0.9512^L`, non-linear; drives also gate larger hulls). |
| Ionization / Energy Focusing / Explosive Projectiles / Spy Technology / Advanced Ship Armor / Increased Cargo Capacity | `Bonus = 10·√L %` exactly (10.00, 14.14, 17.32, 20.00, 22.36, 24.49 …). |
| Recycling Technology | `Bonus = 1% · L` (0.01, 0.02, 0.03 …) — debris-recovery percentage. Also unlocks the Recycler. |


## Points

- Points per level are **constant per item** (see catalog). Points are economy-only (buildings + research); there is no military score.

