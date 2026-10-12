# E7 — Login / dashboard / overview onboarding spec

**Kind:** extraction · **Source:** `HAR-LOGIN` · **Depends on:** none
**Commit:** `spec: extract GalaxyWars login + overview flow from Uni5 HAR`
**Output:** `info/galaxywars-spec/12-onboarding.md` (new)

## Goal

Capture the entry path and the overview page layout, so gwx's auth + homeworld
flow matches the reference: login POST → dashboard redirect → planet overview.

## Inputs (`HAR-LOGIN`, indices stable per file)

| idx | URL | Note |
|---|---|---|
| 0 | `POST /login` | login body (fields/CSRF) — **302** on success |
| 1 | `GET /app/dashboard` | **302** → overview |
| 2, 19 | `GET /app/12:147:1/planet` | overview page |
| 3–36 | `/build/runtime.*.js`, `544.*`, `105.*` | JS/CSS bundles |

## Steps

1. Dump:
   ```bash
   HAR="info/recordings/Uni5/Login_overview.har"
   python tools/har/harscan.py index "$HAR" > info/captures/derived/login-index.txt
   python tools/har/harscan.py dump "$HAR" --idx 0  --body request  --out info/captures/derived/login-post.txt
   python tools/har/harscan.py dump "$HAR" --idx 2  --out info/captures/derived/pages/overview.html
   python tools/har/harscan.py dump "$HAR" --idx 4  --out info/captures/derived/js/544.js
   ```
2. Record:
   - login form fields (`_username`/`_password`/`_csrf_token`?) and the
     redirect chain `POST /login` → `GET /app/dashboard` → `/{coord}/planet`;
   - overview section-by-section: top resource strip, planet header
     (name/coords/`[M]` tag), building/research/unit queue widgets, incoming
     fleets, fleet slots, storage bar;
   - anything gwx's `overview.html` is missing relative to the reference.
3. Only if it holds config/formulas, grep the JS bundle
   (`python tools/har/harscan.py grep "<name>" "$HAR"`); do **not** dump huge
   bundles unless needed.
4. Write `info/galaxywars-spec/12-onboarding.md` (flow diagram + overview
   checklist vs. gwx).
5. Commit only the new spec file.

## Definition of done

- Login + redirect chain and overview layout documented.
- A concrete checklist of gwx overview/onboarding gaps.
- No raw HAR content committed (`cookies.txt`, POST bodies with passwords).
