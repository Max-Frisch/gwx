#!/usr/bin/env python3
"""Extract GalaxyWars catalog + level curves from the captured technology-tree
pages and Symfony UX LiveComponent `calculate` responses.

Inputs (produced by harscan dump-all):
  info/captures/derived/techtree/*.html   one page per item
  info/captures/derived/livecomp/*        calculate responses (decoded HTML)

Output:
  info/captures/derived/catalog.json      structured catalog
  info/captures/derived/catalog.tsv       flat level curve rows
"""
import glob
import html
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
DER = os.path.join(ROOT, "info", "captures", "derived")
TT = os.path.join(DER, "techtree")
LC = os.path.join(DER, "livecomp")


def de(s):
    return html.unescape(re.sub(r"<[^>]+>", " ", s)).strip()


def num(s):
    s = s.strip().replace("\u00a0", " ")
    if not s or s in {"-", "—"}:
        return None
    neg = s.startswith("-")
    s = s.lstrip("+-")
    m = re.match(r"^([\d.,]+)", s)
    if not m:
        return None
    raw = m.group(1)
    if "," in raw and "." in raw:
        raw = raw.replace(".", "").replace(",", ".")
    elif "," in raw:
        raw = raw.replace(",", ".")
    else:
        raw = raw.replace(".", "")
    try:
        v = float(raw)
    except ValueError:
        return None
    return -v if neg else v


def dur(s):
    s = s.strip()
    if not s:
        return None
    days = 0
    m = re.search(r"(\d+)\s*Tag", s)
    if m:
        days = int(m.group(1))
        s = s[m.end():]
    parts = re.findall(r"\d+", s)
    parts = [int(x) for x in parts]
    while len(parts) < 3:
        parts.append(0)
    h, mi, sec = parts[-3], parts[-2], parts[-1]
    return days * 86400 + h * 3600 + mi * 60 + sec


def parse_meta(path):
    t = open(path, encoding="utf-8").read()
    meta = {}
    m = re.search(r'<th colspan="2" class="text-center">(.*?)</th>', t, re.S)
    meta["name"] = de(m.group(1)) if m else None
    # description: first <td colspan="2"> after the image row
    descs = re.findall(r'<td colspan="2">(.*?)</td>', t, re.S)
    meta["description"] = de(descs[0]) if descs else None
    # requirements block
    m = re.search(r"<td>Requirements</td>\s*<td>(.*?)</td>", t, re.S)
    reqs = []
    if m:
        for line in m.group(1).split("<br/>"):
            s = de(line)
            if s and "No requirements" not in s:
                reqs.append(s)
    meta["requirements"] = reqs
    # points per level (buildings)
    m = re.search(r"<td>Points per level</td>\s*<td>(.*?)</td>", t, re.S)
    meta["pointsPerLevel"] = num(de(m.group(1))) if m else None
    # base/current stat table (ships/defense)
    stats = {}
    for row in re.findall(r"<tr>(.*?)</tr>", t, re.S):
        cells = [de(c) for c in re.findall(r"<t[hd][^>]*>(.*?)</t[hd]>", row, re.S)]
        if len(cells) == 3 and cells[0] not in ("", "Base"):
            stats[cells[0]] = {"base": num(cells[1]), "current": num(cells[2])}
    meta["stats"] = stats
    # special features
    m = re.search(r"<td>Special features</td>\s*<td>(.*?)</td>", t, re.S)
    meta["special"] = de(m.group(1)) if m else None
    # live component props (ident)
    m = re.search(r'data-live-name-value="([^"]+)"[^>]*data-live-props-value="([^"]+)"', t, re.S)
    if m:
        meta["component"] = m.group(1)
        props = json.loads(html.unescape(m.group(2)))
        meta["props"] = props
        for k in ("buildingIdent", "researchIdent", "shipIdent", "defenseIdent"):
            if k in props:
                meta["ident"] = props[k]
                meta["kind"] = k.replace("Ident", "")
    return meta


def parse_calc(path):
    t = open(path, encoding="utf-8").read()
    m = re.search(r'data-live-name-value="([^"]+)"[^>]*data-live-props-value="([^"]+)"', t, re.S)
    if not m:
        return None
    comp = m.group(1)
    props = json.loads(html.unescape(m.group(2)))
    rows = []
    header = None
    for row in re.findall(r"<tr[^>]*>(.*?)</tr>", t, re.S):
        cells = [de(c) for c in re.findall(r"<t[hd][^>]*>(.*?)</t[hd]>", row, re.S)]
        if not cells:
            continue
        if header is None:
            header = cells
            continue
        if len(cells) != len(header):
            continue
        rows.append(dict(zip(header, cells)))
    return {"component": comp, "props": props, "header": header, "rows": rows}


def main():
    items = []
    for path in sorted(glob.glob(os.path.join(TT, "*.html"))):
        base = os.path.basename(path)
        if base.endswith("-building.html"):
            continue
        meta = parse_meta(path)
        meta["source"] = base
        items.append(meta)

    # attach calculate data by ident
    calcs = {}
    for path in sorted(glob.glob(os.path.join(LC, "*"))):
        c = parse_calc(path)
        if not c:
            continue
        props = c["props"]
        ident = (props.get("buildingIdent") or props.get("researchIdent")
                 or props.get("shipIdent") or props.get("defenseIdent"))
        if not ident:
            continue
        c["source"] = os.path.basename(path)
        prev = calcs.get(ident)
        # keep the widest level range
        if prev is None or len(c["rows"]) > len(prev["rows"]):
            calcs[ident] = c

    for it in items:
        ident = it.get("ident")
        if ident and ident in calcs:
            it["calc"] = calcs[ident]

    with open(os.path.join(DER, "catalog.json"), "w", encoding="utf-8") as f:
        json.dump(items, f, indent=2, ensure_ascii=False)

    # flat tsv of level curves
    with open(os.path.join(DER, "catalog.tsv"), "w", encoding="utf-8") as f:
        f.write("ident\tkind\tcomponent\t" + "col\n")
        for it in items:
            c = it.get("calc")
            if not c:
                continue
            for r in c["rows"]:
                f.write("\t".join([str(it.get("ident")), str(it.get("kind")),
                                   c["component"], it.get("name", "")] +
                                  ["%s=%s" % (k, v) for k, v in r.items()]) + "\n")
    sys.stderr.write("extracted %d items, %d with calc data\n" %
                     (len(items), sum(1 for i in items if "calc" in i)))


if __name__ == "__main__":
    main()
