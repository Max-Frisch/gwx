#!/usr/bin/env python3
"""Merge category + description + current-level data from the Construction /
Research / Ship pages into catalog.json.

Reads:  info/captures/derived/pages/*.html (dumped by harscan)
Writes: info/captures/derived/catalog.json (augmented)
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
PAGES = os.path.join(DER, "pages")
CAT = os.path.join(DER, "catalog.json")


def de(s):
    return html.unescape(re.sub(r"<[^>]+>", " ", s)).strip()


def parse_index_page(path):
    t = open(path, encoding="utf-8").read()
    i = t.find('<div class="content')
    j = t.find('<div class="content-footer')
    t = t[i:j]
    out = {}
    cat = None
    for part in re.split(r'(full-w-entry-category">[^<]+</div>|<div class="full-w-entry-wrapper">)', t):
        m = re.match(r'full-w-entry-category">([^<]+)<', part)
        if m:
            cat = m.group(1).strip()
            continue
        m = re.search(r'technology-tree/(building|research|ship|defense)/([A-Za-z]+)', part)
        if not m:
            continue
        kind, ident = m.group(1), m.group(2)
        desc = re.search(r'<div class="full-w-entry-description">(.*?)</div>', part, re.S)
        lvl = re.search(r'<span class="addon">\s*(?:Stufe|Level)?\s*([0-9]+)', part)
        costs = re.findall(
            r'<span class="name">([^<]+)</span>\s*<span class="value[^"]*"[^>]*>([^<]+)</span>',
            part)
        out[ident] = {
            "kind": kind,
            "category": cat,
            "descriptionPage": de(desc.group(1)) if desc else None,
            "currentLevel": int(lvl.group(1)) if lvl else None,
            "pageCosts": [(a.strip(), b.strip()) for a, b in costs if a.strip() != "Duration"],
        }
    return out


def main():
    data = json.load(open(CAT, encoding="utf-8"))
    merged = {}
    for p in sorted(glob.glob(os.path.join(PAGES, "*.html"))):
        text = open(p, encoding="utf-8").read()
        if "full-w-entry-wrapper" not in text:
            continue
        idx = parse_index_page(p)
        for k, v in idx.items():
            merged.setdefault(k, v)
    n = 0
    for it in data:
        ident = it.get("ident")
        if ident in merged:
            it.update(merged[ident])
            n += 1
    json.dump(data, open(CAT, "w", encoding="utf-8"), indent=2, ensure_ascii=False)
    sys.stderr.write("augmented %d items with page data\n" % n)
    # show which are missing category
    miss = [it.get("ident") for it in data if "category" not in it]
    if miss:
        sys.stderr.write("no category: %s\n" % miss)


if __name__ == "__main__":
    main()
