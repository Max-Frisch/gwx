# E2 — Trade / marketplace spec

**Kind:** extraction · **Source:** `HAR-DTA` · **Depends on:** none
**Commit:** `spec: extract GalaxyWars trade flow from Uni5 HAR`
**Output:** `info/galaxywars-spec/07-trade.md` (new)

## Goal

Document the whole player-trading subsystem so it can be reimplemented: tabs,
offer lifecycle, rates, fees, storage, history, and the POST payload shapes.

## Inputs (`HAR-DTA`, indices stable per file)

| idx | URL | Note |
|---|---|---|
| 270 | `/app/12:147:1/trading` | landing / default tab |
| 285, 302, 367 | `/trading/offer/<uuid>/detail` | offer detail (two different offers) |
| 319, 351, 383, 415, 432 | `/trading/own` | "My offers" |
| 334, 350 | `/trading/create` | create form (GET 334; POST 350) |
| 350 | `POST /trading/create` | **request body** = offer submission |
| 400 | `/trading/own/history` | completed/cancelled offers |
| 431 | `/trading/offer/<uuid>/revoke` | revoke action |
| 447 | `/trading/storage` | trade storage |
| 462 | `/trading/information` | rules/rates/fees |
| 1313 | `/trading/create?target=<uuid>` | create directed at a player |
| 50 | `/static/units/building/tradingPost.jpg` | Trading Post building image |

## Steps

1. Dump the GET pages and the POST bodies:
   ```bash
   HAR="info/recordings/Uni5/defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har"
   python tools/har/harscan.py index "$HAR" > info/captures/derived/dta-index.txt
   python tools/har/harscan.py dump-all "$HAR" --mime text/html --pattern '/trading' \
       --out info/captures/derived/pages/trade
   python tools/har/harscan.py dump "$HAR" --idx 350 --body request --out info/captures/derived/trade-create-post.txt
   python tools/har/harscan.py dump "$HAR" --idx 431 --body request --out info/captures/derived/trade-revoke-post.txt
   ```
2. Also dump the **Trading Post** techtree page from `HAR-CRS`
   (`/technology-tree/building/tradingPost`, idx ~150) for its cost/effect, and
   merge that into `01-catalog.md` if missing.
3. Read the HTML and record:
   - tabs (Marketplace / My offers / Storage / Information), columns shown;
   - offer fields: give/get resource+amount, exchange rate, visibility
     (public/private/target player), duration/expiry, fee;
   - quick-add buttons (25/50/100 % / max), flag/mark, sorting;
   - lifecycle: create → (accept/revoke/expire) → history;
   - storage: capacity, per-resource limits, withdrawals;
   - fees/rates rules from `/trading/information`;
   - the POST field names (`trading_offer[...]` etc.) and any JSON.
4. Write `info/galaxywars-spec/07-trade.md` in the same style as the existing
   spec files (tables + short prose). Cross-link from `05-ui-flows.md` later in E8.
5. Commit only the new spec file.

## Definition of done

- `07-trade.md` covers tabs, offer schema, lifecycle, fees, storage, history,
  and the observed POST fields, with a "differences vs gwx design" note
  (gwx has no Trading Post yet — say what implementing it would need).
- No raw HAR content committed.

## Handoff

Feeds **I7-trade**.
