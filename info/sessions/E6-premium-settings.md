# E6 — Premium & Settings spec

**Kind:** extraction · **Source:** `HAR-DTA` · **Depends on:** none
**Commit:** `spec: extract GalaxyWars premium + settings from Uni5 HAR`
**Output:** `info/galaxywars-spec/11-premium-settings.md` (new)

## Goal

Document the account/planet settings surface and the premium shop, plus vacation
mode and sanctions. Most of Settings is directly actionable in gwx; Premium is
mostly out of scope but should be recorded.

## Inputs (`HAR-DTA`, indices stable per file)

### Premium
| idx | URL |
|---|---|
| 1063 | `/premium/packages` |
| 1078 | `/premium/galaxium` |
| 1094 | `/premium/account` |

### Settings
| idx | URL |
|---|---|
| 1109, 1125, 1141 | `/settings/game` |
| 1157 | `/settings/planet` |
| 1173 | `/settings/vacation/` |
| 1188 | `/settings/account` |
| 1204 | `/settings/sanction` |

## Steps

1. Dump:
   ```bash
   HAR="info/recordings/Uni5/defense_trade_alliance_highscore_statistics_premium_settings_search_events_messages.har"
   python tools/har/harscan.py index "$HAR" > info/captures/derived/dta-index.txt
   python tools/har/harscan.py dump-all "$HAR" --mime text/html --pattern '/settings|/premium' \
       --out info/captures/derived/pages/settings-premium
   ```
   Also grab POST bodies for the game/account settings forms (look for POST
   entries near those idx via `index --method POST`).
2. Record:
   - **Settings → Game:** quick-dispatch probe count, max recon probes, raid
     order, fleet labels, tactical retreat, interface options, messages per page.
   - **Settings → Account:** rename / email / password, reset/delete account,
     profile picture, external applications. Note the **password-change**
     confirmation field and any constraints (min length) — gwx already has
     PBKDF2 + sessions.
   - **Settings → Planet:** rename planet, main-planet designation, abandon.
   - **Vacation mode** + **Sanctions**: what they gate, how entered/left.
   - **Premium:** currency (Galaxium), packages, account tier perks; mark as
     *out of scope* for gwx unless a perk maps to an existing mechanic.
3. Write `info/galaxywars-spec/11-premium-settings.md`.
4. Commit only the new spec file.

## Definition of done

- Settings tabs documented field-by-field, with a gwx feasibility tag
  (build now / needs mechanic / out of scope).
- Premium, vacation, sanctions documented (even if out of scope).
- No raw HAR content committed.

## Handoff

Feeds **I1-search-settings** (account settings) and **I9-premium-vacation** (opt).
