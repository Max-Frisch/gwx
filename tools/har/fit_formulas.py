#!/usr/bin/env python3
"""Fit GalaxyWars economy formulas from the extracted catalog.json."""
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
CAT = os.path.join(ROOT, "info", "captures", "derived", "catalog.json")

DATA = json.load(open(CAT, encoding="utf-8"))


def num(s):
    s = s.strip().replace("\u00a0", " ").replace("%", "")
    s = s.replace(".", "·").replace(",", ".").replace("·", "")
    return float(s)


def rows(ident):
    for it in DATA:
        if it.get("ident") == ident:
            return it["calc"]["rows"]
    return []


def col(ident, key):
    out = []
    for r in rows(ident):
        v = None
        for k in r:
            if k.replace("\n", " ").split("(")[0].strip() == key:
                v = r[k]
        out.append((int(r["Level"]), num(v) if v not in (None, "") else None))
    return out


def quad(pts):
    (x1, y1), (x2, y2), (x3, y3) = pts
    A = [[x1 * x1, x1, 1], [x2 * x2, x2, 1], [x3 * x3, x3, 1]]
    b = [y1, y2, y3]
    for i in range(3):
        p = max(range(i, 3), key=lambda r: abs(A[r][i]))
        A[i], A[p] = A[p], A[i]
        b[i], b[p] = b[p], b[i]
        for r in range(i + 1, 3):
            f = A[r][i] / A[i][i]
            for c in range(i, 3):
                A[r][c] -= f * A[i][c]
            b[r] -= f * b[i]
    x = [0] * 3
    for i in range(2, -1, -1):
        x[i] = (b[i] - sum(A[i][j] * x[j] for j in range(i + 1, 3))) / A[i][i]
    return x


def report(name, ident, key):
    s = [(L, y) for L, y in col(ident, key) if y is not None]
    c = quad(s[:3])
    err = max(abs(c[0] * L * L + c[1] * L + c[2] - y) for L, y in s)
    print("%-34s first=%s quad=(%.4f,%.4f,%.4f) maxerr=%.1f" %
          (name, [y for _, y in s[:5]], c[0], c[1], c[2], err))


print("== production / consumption ==")
report("ironMine Production", "ironMine", "Production")
report("lutinumRefinery Production", "lutinumRefinery", "Production")
report("drillingTower Production(Water)", "drillingTower", "Production")
report("chemicalFactory Production(H2)", "chemicalFactory", "Production")
report("chemicalFactory Consumption(Water)", "chemicalFactory", "Consumption")
report("extChemicalFactory Production(H2)", "extendedChemicalFactory", "Production")
report("extChemicalFactory Consumption", "extendedChemicalFactory", "Consumption")
print("\n== storage ==")
for i in ("ironStorage", "lutinumStorage", "waterStorage", "hydrogenStorage"):
    report(i + " Storage", i, "Storage")
    report(i + " Plunder-covered", i, "Cannot be plundered")
print("\n== research bonus / reduced consumption ==")
for i in ("combustionDrive", "ionDrive", "spaceCurvatureDrive", "spaceFoldingDrive",
          "ionisation", "energyBundling", "explosiveProjectiles", "spyTechnology",
          "extendedShipArmour", "increasedCargoCapacity", "recyclingTechnology"):
    s = [(L, y) for L, y in col(i, "Bonus") if y is not None]
    print("%-24s Bonus L1..6=%s" % (i, [y for _, y in s[:6]]))
    try:
        rc = [(L, y) for L, y in col(i, "Reduced consumption") if y is not None]
        if rc:
            print("    Reduced consumption L1..3=%s" % [y for _, y in rc[:3]])
    except Exception:
        pass
print("\n== ship build time (per ship-factory level) ==")
for i in ("schakal", "falcon", "longeagleX"):
    s = col(i, "Construction time")
    print("%-12s time L1..6=%s" % (i, [y for _, y in s[:6]]))
