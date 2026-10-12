# GalaxyWars capture runbook — Universe6 (launch Oct 17)

Purpose: capture the **level-0** state and the first days of progression of the
fresh Uni6 as `.har` files, so we can extract the catalog, formulas, combat
model and UI flows into `info/galaxywars-spec/`.

Raw captures are **private** and git-ignored (`info/captures/`, `*.har`). HARs
contain session cookies and request bodies — never commit or share them.

---

## TL;DR checklist

- [ ] DevTools → Network: enable **Preserve log** + **Disable cache**.
- [ ] Register on Uni6 the moment it opens. Do **not** build anything yet.
- [ ] Clear the Network log (removes the login/registration request).
- [ ] **HAR A** — untouched level-0 overview. Export.
- [ ] **HAR B** — walk *every* page + open every info tooltip. Export.
- [ ] Days 1–3: one HAR per milestone (see Phase 2). Keep the milestone log.
- [ ] Drop HARs in `info/captures/` with the naming scheme below.

---

## Setup (do once)

1. Open the game in Chrome/Edge and press `F12` → **Network** tab.
2. Enable:
   - **Preserve log** (keeps history across navigations),
   - **Disable cache** (forces JS/CSS to re-download so they land in the HAR).
3. Leave the request filter on **All** (not just Fetch/XHR) so documents and
   bundles are captured.
4. First hard reload with `Ctrl+Shift+R` so the JS bundles are fetched fresh
   with the cache disabled.

## Privacy note

The login and registration `POST` bodies contain your password. Easiest fix:
finish logging in **before** you start capturing, then click the Network tab's
**Clear** (🚫) button. Session cookies will still be in the HAR — keep the file
private. Use a throwaway password if you prefer.

---

## Naming scheme

Save into `info/captures/`:

```
uni6-20261017-HHMM-level0.har      # HAR A
uni6-20261017-HHMM-walk.har        # HAR B
uni6-20261018-HHMM-milestone-mine.har
uni6-20261019-HHMM-milestone-research.har
...
```

Exporting: right-click any request → **Save all as HAR with content**
(or the download ⤓ icon in the Network toolbar). Verify "with content" so
response bodies are included.

---

## Phase 1 — launch day (level 0)

**HAR A — untouched state.** Right after registering and landing on the first
overview, before building/researching/colonizing anything. This anchors:
initial resource amounts, spawn/homeworld defaults, starting fields/diameter.

**HAR B — full walk.** With **Preserve log** on, visit every page and open every
item's info/detail (costs and effects often load lazily):

- PLANET: Overview, Construction, Research, Ships/Defense, Trade, Technology
- UNIVERSE: Fleets, Galaxy, Events
- COMMUNITY: Messages, Search, Alliance, Highscore
- ACCOUNT: Statistics, Premium, Settings (each tab), Logout
- Footer: Help, Game rules, Support, Status, Legal notice

For each buildable/researchable/unit item, click its **info/"i"** control so the
description + effect text is requested. Open the Trade tabs
(Marketplace / My offers / Storage / Information) and Highscore tabs
(Players / Alliances / Planets / Battles) even if empty.

---

## Phase 2 — days 1–3 (formula data points)

Build in a controlled order and export a HAR **after each milestone**, so we get
multiple (level → cost/time/production) points:

1. First Iron Mine / Lutinum Mine / Drilling Tower.
2. First storage + first Chemical Factory.
3. First research (Energy, then a drive).
4. First ship(s) in the shipyard.
5. Whatever early raids/transports are feasible.

Keep a running milestone log (template below) in `info/captures/` (ignored).
Final numbers get folded into the committed spec.

### Milestone log template

```text
date/time | item (code/name) | level | cost shown | build time shown | production/effect shown
----------|------------------|-------|------------|-------------------|-------------------------
```

---

## What we are hunting for (so you know what matters)

- Full **catalog**: names, costs, cost factors, build times, prerequisites,
  combat stats, cargo/speed/fuel, research effects (incl. Recycling Tech's two
  jobs, debris %).
- **Economy formulas**: mine/refinery production, hydrogen conversion (5:1 and
  2:1), storage caps, protected floor, points (we use 1 pt / 1000 resources).
  (No energy — GalaxyWars has none.)
- **Combat**: attack/shield/hull, per-unit **rapid-fire** table, debris rate,
  loot, shield regen.
- **Fleet**: speed/distance/fuel formulas and mission behaviour.
- **UI flows**: Trade/marketplace, Settings, Statistics, Highscore tabs,
  Messages, Alliance, Galaxy actions.
- The **JS bundles** (captured by Disable cache) usually hold the whole config
  and formula names — often the single best source.

---

## After capture

Tell me when the first HAR is in `info/captures/`. I will then build
`tools/har/harscan.py` against its real structure, index the endpoints, extract
the JSON catalog/config, mine the JS bundles, and turn it into
`info/galaxywars-spec/` (catalog, economy, combat, fleet, UI flows) plus concrete
`BACKLOG.md` values.
