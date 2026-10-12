#!/usr/bin/env python3
"""Generate info/galaxywars-spec/{01..04,06}.md from the derived catalog.

Run after: harscan dump-all + extract_catalog.py + extract_pages.py

Ownership: this script regenerates 01-04 and 06 only. 05 and 07-12 are
hand-authored (see `info/sessions/`) and must not be clobbered by a rerun.
"""
import json
import os
import re

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
DER = os.path.join(ROOT, "info", "captures", "derived")
OUT = os.path.join(ROOT, "info", "galaxywars-spec")
DATA = json.load(open(os.path.join(DER, "catalog.json"), encoding="utf-8"))


def num(s):
    s = s.strip().replace("\u00a0", " ").replace("%", "")
    return float(s.replace(".", "·").replace(",", ".").replace("·", ""))


def de_lines(s):
    return re.sub(r"\s+", " ", (s or "")).strip()


def by_kind(k):
    return [it for it in DATA if it.get("kind") == k]


def calc_col(it, key):
    out = []
    for r in (it.get("calc") or {}).get("rows", []):
        for k in r:
            if k.replace("\n", " ").split("(")[0].strip() == key:
                out.append((int(r["Level"]), r[k]))
    return out


def base_costs(it):
    rows = (it.get("calc") or {}).get("rows", [])
    if not rows:
        return {}
    r = rows[0]
    out = {}
    for k, v in r.items():
        if k in ("Level", "Points", "Construction time") or k.startswith(("Production", "Consumption", "Storage", "Cannot", "Bonus", "Reduced")):
            continue
        try:
            out[k] = num(v)
        except ValueError:
            pass
    return out


def fmt_cost(d):
    order = ["Iron", "Lutinum", "Water", "Hydrogen"]
    parts = []
    for k in order:
        if k in d:
            parts.append("%s %s" % (fnum(d[k]), k))
    return ", ".join(parts) if parts else "—"


def fnum(v):
    if v is None:
        return "—"
    if isinstance(v, float) and v.is_integer():
        return "{:,}".format(int(v)).replace(",", ".")
    return str(v)


def ship_costs(it):
    d = {}
    for k, v in it.get("pageCosts", []):
        try:
            d[k] = num(v)
        except ValueError:
            pass
    return d


def calc_at_level(it, key, level):
    for L, v in calc_col(it, key):
        if L == level:
            return v
    return "—"


def table(headers, rows):
    s = "| " + " | ".join(headers) + " |\n"
    s += "|" + "|".join(["---"] * len(headers)) + "|\n"
    for r in rows:
        s += "| " + " | ".join(str(c) for c in r) + " |\n"
    return s


W = {}

# ---------------------------------------------------------------- 01 catalog
b = by_kind("building")
r = by_kind("research")
sh = by_kind("ship")

lines = []
lines.append("# GWX ↔ GalaxyWars spec — 01 Catalog\n")
lines.append("Derived from the private Uni5 HAR captures "
             "(`info/recordings/Uni5/`). Raw HARs are git-ignored; this spec is "
             "the committed, human-readable distillation.\n")
lines.append("Reference client: `GalaxyWars v3.16.19-beta`, universe `uni5`, "
             "constructor planet `12:147:1` (mid-game account). Endpoint family: "
             "`/app/<coord>/technology-tree/{building,research,ship}/<ident>` plus "
             "Symfony UX LiveComponent `POST /app/_components/<coord>/"
             "Technology{Building,Research,Ship}/calculate`.\n")

lines.append("## Global\n")
lines.append("- **Resources:** Iron, Lutinum, Water, Hydrogen (all four plunderable; "
             "each has its own storage).")
lines.append("- **Cost scaling** (buildings + research): the cost of level `L` is "
             "`base × M(L)` where `M(1)=M(2)=1` and the **same multiplier series** "
             "applies to every item:")
lines.append("  `M = 1, 1, 2, 4, 7, 11.5, 18, 26.5, 37, 51, 67.5, 87, …`")
lines.append("  Fitted (L ≥ 3): `M(L) ≈ (L³ + 15) / 20` — i.e. cost grows ≈ cubically "
             "(~`L³/20`). Coefficients are exact per-level in the tables below.")
lines.append("- **Points:** constant per level, shown on each item (economy points "
             "only; there is no fleet/defense score).")

lines.append("\n## Buildings\n")
rows = []
for it in b:
    bc = base_costs(it)
    reqs = "; ".join(it.get("requirements") or []) or "—"
    rows.append([it["name"], "`%s`" % it["ident"], it.get("category") or "—",
                 fmt_cost(bc), fnum(it.get("pointsPerLevel")), reqs])
lines.append(table(["Name", "ident", "Category", "Base cost (L1)", "Pts/lvl", "Requirements"], rows))

lines.append("\n### Building effects (per level)\n")
for it in b:
    cols = [k for k in (it.get("calc") or {}).get("header", [])
            if k not in ("Level", "Points", "Construction time")
            and not k.startswith(("Iron", "Lutinum", "Water", "Hydrogen"))]
    for col in cols:
        c = re.sub(r"\s+", " ", col).strip()
        series = calc_col(it, c.split("(")[0].strip())
        sample = ", ".join("L%s=%s" % (L, v) for L, v in series[:5])
        lines.append("- **%s** — `%s`: %s" % (it["name"], c, sample))

df = by_kind("defense")
lines.append("\n## Defense\n")
rows = []
for it in df:
    reqs = "; ".join(it.get("requirements") or []) or "—"
    rows.append([it["name"], "`%s`" % it["ident"], fmt_cost(ship_costs(it)), reqs])
lines.append(table(["Name", "ident", "Base cost (per unit)", "Requirements"], rows))

W["01-catalog.md"] = "\n".join(lines) + "\n"

# ------------------------------------------------------------ 02 economy
lines = []
lines.append("# GWX ↔ GalaxyWars spec — 02 Economy formulas\n")
lines.append("All values fitted/observed from the Uni5 capture. Costs are exact "
             "per-level (see `01-catalog.md`); production is recorded as the exact "
             "per-level table and quoted here at the **x1 base** — the Uni5 capture "
             "ran at 2× game speed, which is context only. gwx applies `GAME_SPEED` "
             "to production at runtime. There is **no energy/electricity system**. "
             "The research bonuses below are the derived closed forms.\n")

lines.append("## Storage & protected floor (exact)\n")
lines.append("For every storage building (Iron/Lutinum/Water/Hydrogen Storage):\n")
lines.append("```")
lines.append("Storage(L)          = 60_000 · L² + 300_000      (exact)")
lines.append("Cannot be plundered = 1_200 · L² + 6_000 = 2% of Storage(L)  (exact)")
lines.append("```")
lines.append("So the **plunder-protected floor is 2%** of a resource's storage "
             "capacity (this game's 'concealed area'). Example: L20 → 24.3M "
             "storage, 486k protected.\n")

lines.append("## Extraction production (per hour)\n")
lines.append("Mines/refinery/tower columns from the technology-tree calculator:")
for ident, label in [("ironMine", "Iron Mine → Iron @ mph"),
                     ("lutinumRefinery", "Lutinum Refinery → Lutinum @ mph"),
                     ("drillingTower", "Drilling Tower → Water @ mph")]:
    it = [x for x in b if x["ident"] == ident][0]
    series = calc_col(it, "Production")
    lines.append("- **%s:** %s …  (≈ quadratic/cubic growth; sample L1–L10)" %
                 (label, ", ".join("%s: %s" % (L, v) for L, v in series[:10])))
lines.append("")
lines.append("## Water → Hydrogen conversion\n")
lines.append("- **Chemical Factory** converts water into hydrogen at **5:1** "
             "(H2 = ⌊water/5⌋); its *water consumption* is exactly **5 × the H₂ "
             "produced**. Output stops when water is exhausted.")
lines.append("- **Advanced Chemical Factory** is the same shape at **2:1** "
             "(H2 = ⌊water/2⌋) with a much larger water draw. Hydrogen output stops "
             "when water is exhausted.\n")

lines.append("## Research bonuses (exact)\n")
lines.append(table(
    ["Research", "Effect"],
    [["Drive techs (Combustion/Ion/Space-Curvature/Space-Folding)",
      "`Bonus` = speed increase; `Reduced consumption` = +0.5% per level (B(L) = 0.5·L %). "
      "Bonus series: 4.88%, 9.54%, 14.02%, 18.32%, 22.47%, 26.49% … (≈ `1−0.9512^L`, "
      "non-linear; drives also gate larger hulls)."],
     ["Ionization / Energy Focusing / Explosive Projectiles / Spy Technology / "
      "Advanced Ship Armor / Increased Cargo Capacity",
      "`Bonus = 10·√L %` exactly (10.00, 14.14, 17.32, 20.00, 22.36, 24.49 …)."],
     ["Recycling Technology",
      "`Bonus = 1% · L` (0.01, 0.02, 0.03 …) — debris-recovery percentage. "
      "Also unlocks the Recycler."]]) + "\n")

lines.append("## Points\n")
lines.append("- Points per level are **constant per item** (see catalog). Points are "
             "economy-only (buildings + research); there is no military score.\n")

W["02-economy-formulas.md"] = "\n".join(lines) + "\n"

# ------------------------------------------------------------- 03 combat
lines = []
lines.append("# GWX ↔ GalaxyWars spec — 03 Combat & units\n")
lines.append("Ship stat block is taken from the technology-tree ship pages "
             "(`Base` = hull default, `Current` = with this account's research). "
             "The last stat row is literally labelled **`ckk`** in the client "
             "(unknown meaning; possibly an internal/opaque value) — recorded as-is.\n")

lines.append("## Ships\n")
ship_rows = []
for it in sh:
    st = it.get("stats", {})
    def g(k):
        return fnum(st.get(k, {}).get("base")) if k in st else "—"
    def gc(k):
        return fnum(st.get(k, {}).get("current")) if k in st else "—"
    ship_rows.append([it["name"], "`%s`" % it["ident"], fmt_cost(ship_costs(it)),
                      "%s→%s" % (g("Attack"), gc("Attack")),
                      "%s→%s" % (g("Defense"), gc("Defense")),
                      "%s→%s" % (g("Cargo capacity"), gc("Cargo capacity")),
                      "%s→%s" % (g("Speed"), gc("Speed")),
                      "%s→%s" % (g("Consumption"), gc("Consumption")),
                      "%s→%s" % (g("ckk"), gc("ckk"))])
lines.append(table(["Ship", "ident", "Cost", "Attack", "Defense", "Cargo", "Speed",
                    "Consumption", "ckk"], ship_rows))

lines.append("\n### Requirements\n")
for it in sh:
    reqs = "; ".join(it.get("requirements") or []) or "—"
    lines.append("- **%s** (`%s`): %s" % (it["name"], it["ident"], reqs))

lines.append("\n## Build time\n")
lines.append("The ship calculator's `Level` input is the **Ship Factory level**; it "
             "lists the per-unit build time at each factory level (higher factory → "
             "shorter time). Faster-than-linear: ≈ `K / (1+F)²`.")
for ident in ("schakal", "falcon", "longeagleX"):
    it = [x for x in sh if x["ident"] == ident][0]
    series = calc_col(it, "Construction time")
    lines.append("- **%s:** %s" % (it["name"], ", ".join("F%s=%s" % (L, v) for L, v in series[:11])))

lines.append("\n## Defense\n")
lines.append("Defense units are fixed emplacements: they have **no cargo, speed or "
             "consumption**. Stats come from the technology-tree defense pages "
             "(`Base` = hull default, `Current` = with this account's research). The "
             "calculator's `Level` input is the **Orbital Defense Station** level; the "
             "build time below is at this account's current ODS level (L4, matching "
             "the `/defense` page's `Duration`).\n")
df = by_kind("defense")
def_rows = []
for it in df:
    st = it.get("stats", {})

    def base_of(k):
        return fnum(st.get(k, {}).get("base")) if k in st else "—"

    def cur_of(k):
        return fnum(st.get(k, {}).get("current")) if k in st else "—"

    ods = (it.get("calc") or {}).get("props", {}).get("levelCurrent")
    bt = calc_at_level(it, "Construction time", ods) if ods is not None else "—"
    def_rows.append([it["name"], "`%s`" % it["ident"], fmt_cost(ship_costs(it)),
                     "%s→%s" % (base_of("Attack"), cur_of("Attack")),
                     "%s→%s" % (base_of("Defense"), cur_of("Defense")),
                     "%s→%s" % (base_of("ckk"), cur_of("ckk")),
                     bt,
                     "; ".join(it.get("requirements") or []) or "—"])
lines.append(table(["Unit", "ident", "Cost", "Attack", "Defense", "ckk",
                    "Build time", "Requirements"], def_rows))

lines.append("\n### Client ident → gwx code mapping\n")
lines.append("The client idents differ from gwx's codes; the roster "
             "(`internal/game/catalog.go`) adopts this mapping:\n")
lines.append(table(["GW ident", "Label", "gwx code"],
                   [["lightLaserTower", "Light Laser Turret", "`light_laser` (retune)"],
                    ["laserTower", "Laser Turret", "`heavy_laser` → rename/replace"],
                    ["empLauncher", "EMP Launcher", "`emp_launcher` (new)"],
                    ["plasmaTower", "Plasma Turret", "`plasma_turret` (retune)"],
                    ["rocketDefenseCannons", "Rocket Launchers", "`rocket_launcher` (new)"]]))
lines.append("Planetary Shield + Fusion Reactor are the two defense-relevant "
             "*buildings*: the Fusion Reactor is the Planetary Shield's building "
             "prerequisite (and adds a base attack value), while the shield adds a "
             "base defense value; there is no energy coupling.\n")

lines.append("## Battle simulator\n")
lines.append("`GET/POST /app/<coord>/fleet/battle-simulator` — attacker vs defender "
             "columns for ships + defense, seeded with your own buildings/research "
             "levels (`data-simulator-*` JSON attributes). Confirms the combat model "
             "is an aggregate simulation the server can run; no rapid-fire stats were "
             "exposed in the capture.\n")

W["03-combat.md"] = "\n".join(lines) + "\n"

# ------------------------------------------------------------- 04 fleet
lines = []
lines.append("# GWX ↔ GalaxyWars spec — 04 Fleet & missions\n")
lines.append("Missions are tabs on the fleet screen, each a form posting to "
             "`/app/<coord>/fleet/send` (JSON `{\"message\":\"Fleet launched "
             "successfully\"}` on success):\n")
lines.append(table(["Mission", "Route", "Notes"],
                   [["Attack", "`fleet/attack?coordinate=…`",
                     "ships + target coord + speed + label + **raid order** (draggable "
                     "Iron/Lutinum/Hydrogen/Water) + 'Attacks remaining' bucket (10000)."],
                    ["Espionage", "`fleet/spy?coordinate=…`", "spy probes only."],
                    ["Own", "`fleet/own?coordinate=…`", "move/deploy between own planets; also transport."],
                    ["Expedition", "`fleet/expedition?coordinate=…`",
                     "'There currently seem to be no possible expeditions' (feature exists in GW)."],
                    ["Colonize", "via Colonization Ship", "spend a Colony Ship at an empty slot."]]))
lines.append("")
lines.append("## Fleet form fields\n")
lines.append("- **Ships:** per-ident number input, `All`/`None` shortcuts.")
lines.append("- **Planet:** `toCoordinate[galaxy|system|planet]`.")
lines.append("- **Target list:** select a saved target list (see below).")
lines.append("- **Speed:** 100/90/80/70/60/50/40/30/20/10 %.")
lines.append("- **Label:** fleet label chosen from Settings → Game → fleet labels.")
lines.append("- **Raid order:** drag-order of resources to loot.")
lines.append("- **Start fleet** → `fleet#send`; live **target-info** panel via "
             "`POST /app/<coord>/fleet/target-info` shows owner, one-way duration, "
             "hydrogen consumption, cargo capacity.\n")
lines.append("## Target lists (farm lists)\n")
lines.append("`fleet/target-list` + `overview` / `connections` / `create` / "
             "`edit` / `delete` / `detail/<uuid>`; each list holds NPC/target "
             "coordinates + a comment, and can be attached to an attack.\n")
lines.append("## Travel\n")
lines.append("Distance/fuel/duration are server-computed (oGame-style quadratic "
             "distance). Observed same-coordinate one-way (planet `12:147:1`): Jackal "
             "≈ 11:43, Spy Probe ≈ 00:07; 2 planets over (→`12:147:3`) Spy ≈ 00:01:34. "
             "Speed/consumption are modified by drive research (`Reduced "
             "consumption` = −0.5%/level).\n")
W["04-fleet.md"] = "\n".join(lines) + "\n"

# 05-ui-flows.md is hand-authored (consolidated in E8): gen_spec does not own it.
# 07-12 are hand-authored too. gen_spec owns 01-04 and 06 only.

# ------------------------------------------------------- 06 queue management
lines = []
lines.append("# GWX ↔ GalaxyWars spec — 06 Queue management\n")
lines.append("Universal pattern: an `append` action adds an order and returns "
             "`302` back to the page; a `remove/<id>` action cancels it. Every append "
             "is per-scope (one active order per building/research/unit scope).\n")
lines.append(table(["Scope", "Append", "Remove", "Notes"],
                   [["Building", "`/building/append/<ident>`", "`/building/remove/<id>`",
                     "`id=1634393` observed — orders are addressable by numeric id."],
                    ["Research", "`/research/append/<ident>`", "`/research/remove/<id>`",
                     "`id=88648` observed."],
                    ["Ships/Defense", "`POST /ship` (batch form)", "—",
                     "`add_combat_units_to_queue[combatUnits][<ident>]` per ship; a single "
                     "batch submit enqueues all non-zero quantities."]]))
lines.append("")
lines.append("The shipyard/queue UI exposes **per-item checkboxes** with **Delete "
             "selected** and **Delete all** behind a confirmation popup (batch-delete "
             "semantics; per the capture-runbook note). The removal endpoints above are "
             "the underlying single-order form.\n")
lines.append("## Mode of operation\n")
lines.append("- `POST /app/push-stats` (2-byte JSON) is polled to refresh the header "
             "resource/queue state — the client's lightweight status endpoint.\n")
W["06-queue-management.md"] = "\n".join(lines) + "\n"

os.makedirs(OUT, exist_ok=True)
for name, text in W.items():
    with open(os.path.join(OUT, name), "w", encoding="utf-8") as f:
        f.write(text)
    print("wrote", name, len(text), "bytes")
