# E5 — Messages, Events & Search spec

**Kind:** extraction · **Source:** `HAR-DTA` · **Depends on:** none
**Commit:** `spec: extract GalaxyWars messages/events/search from Uni5 HAR`
**Output:** `info/galaxywars-spec/10-messages-events-search.md` (new)

## Goal

Document (a) the message centre, (b) the events feed, and (c) player/planet
search + the player profile page — the three "information" surfaces gwx lacks.

## Inputs (`HAR-DTA`, indices stable per file)

### Messages (`/message/...`)
| idx | URL |
|---|---|
| 1331 | `/message` (overview / categories) |
| 1332, 1349 | `/message/conversation` |
| 1365 | `/message/conversation/detail/<uuid>` |
| 1380 | `/message/report/attack` |
| 1398 | `/message/report/spy` |
| 1416 | `/message/report/defense` |
| 1434 | `/message/report/archive/` |
| 1449 | `/message/system/own-fleet` |
| 1467 | `/message/system/other-fleet` |
| 1485 | `/message/system/building` |
| 1503 | `/message/system/research` |
| 1522 | `/message/system/system` |

### Events
| idx | URL |
|---|---|
| 478 | `/event` |
| 493 | `/event/<uuid>` |

### Search + profile
| idx | URL |
|---|---|
| 1219 | `/search` |
| 1234, 1249 | `/search?ingame_search[term]=…` |
| 1264, 1297 | `/player/<uuid>` (profile) |
| 1280 | `/galaxy` (for cross-links) |

## Steps

1. Dump:
   ```bash
   HAR="info/recordings/Uni5/defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har"
   python tools/har/harscan.py index "$HAR" > info/captures/derived/dta-index.txt
   python tools/har/harscan.py dump-all "$HAR" --mime text/html --pattern '/message' \
       --out info/captures/derived/pages/messages
   python tools/har/harscan.py dump-all "$HAR" --mime text/html --pattern '/event|/search|/player' \
       --out info/captures/derived/pages/events-search
   ```
2. Record:
   - **Messages:** category tree (conversations, reports by kind, system by
     kind, archive), per-item fields (subject, from, date, read/unread,
     delete/archive actions), the conversation thread view, and pagination.
     Note how reports map onto combat/spy report payloads.
   - **Events:** what appears in the feed (fleet arrivals/returns, builds,
     research, attacks), item fields, per-item detail page, filters/tabs,
     read/delete semantics.
   - **Search:** the search term field name, result row fields (player/planet/
     coords/points/online?), what can be searched, and the profile page layout
     (public stats, planets, alliance, actions like message/trade).
3. Write `info/galaxywars-spec/10-messages-events-search.md`.
4. Commit only the new spec file.

## Definition of done

- All three surfaces documented with fields/actions/pagination.
- A short "gwx subset" proposal for each (what is cheap to implement from
  existing `reports` / `fleets` / `queues` tables).
- No raw HAR content committed.

## Handoff

Feeds **I1-search-settings**, **I2-messages**, **I3-events**.
