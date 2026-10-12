# E3 — Alliance spec

**Kind:** extraction · **Source:** `HAR-DTA` · **Depends on:** none
**Commit:** `spec: extract GalaxyWars alliance flow from Uni5 HAR`
**Output:** `info/galaxywars-spec/08-alliance.md` (new)

## Goal

Document alliances end-to-end: search/browse, apply, the public profile, and the
whole internal area (members, ranks, rights, applications, board, log, settings),
including the POST payloads.

## Inputs (`HAR-DTA`, indices stable per file)

| idx | URL | Note |
|---|---|---|
| 508, 538, 600, 646 | `/alliance/search` | browse/search alliances |
| 523, 585, 615, 631 | `/alliance/applications` | my sent applications |
| 554 | `/alliance/show/Linux` | public profile |
| 677 | `/alliance/show/wurst` | public profile (tag `wurst`) |
| 569, 584 | `/alliance/apply/Linux` | apply form + POST |
| 630 | `/alliance/application-revoke/<uuid>` | revoke |
| 661, 676 | `/alliance/create` | create form + POST |
| 692, 707, 907 | `/alliance/internal` | internal dashboard |
| 723 | `/alliance/internal/members` | members |
| 738 | `/alliance/internal/board` | shared board |
| 753, 768, 783 | `/alliance/internal/board/thread/create` | new thread |
| 784 | `/alliance/internal/board/thread/detail/<uuid>` | thread |
| 799 | `/alliance/internal/management` | management |
| 814 | `/alliance/internal/management/applications` | review applications |
| 829 | `/alliance/internal/management/log` | audit log |
| 844 | `/alliance/internal/management/ranks` | ranks/rights |
| 859, 875 | `/alliance/internal/management/settings` | settings |
| 891, 906 | `.../settings/change-image` | logo upload |

## Steps

1. Dump pages + POST bodies:
   ```bash
   HAR="info/recordings/Uni5/defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har"
   python tools/har/harscan.py index "$HAR" > info/captures/derived/dta-index.txt
   python tools/har/harscan.py dump-all "$HAR" --mime text/html --pattern '/alliance' \
       --out info/captures/derived/pages/alliance
   for i in 584 676 768 906; do
     python tools/har/harscan.py dump "$HAR" --idx $i --body request \
         --out info/captures/derived/alliance-post-$i.txt
   done
   ```
2. Record:
   - alliance fields: tag, name, description, logo, points, members, founder,
     open/closed, min points to apply;
   - **rank/rights model** (which permissions exist) from `/management/ranks`;
   - application flow (apply/accept/reject/revoke) and member management;
   - board: threads, replies, moderation;
   - audit log entries;
   - POST field names for create/apply/settings/board.
3. Write `info/galaxywars-spec/08-alliance.md` (tables + short prose), and note a
   minimal gwx subset (membership, one board, ranks/rights, applications).
4. Commit only the new spec file.

## Definition of done

- `08-alliance.md` covers public + internal surfaces, the rank/rights model,
  the application lifecycle, board, settings, and observed POST fields.
- No raw HAR content committed.

## Handoff

Feeds **I6-alliances**.
