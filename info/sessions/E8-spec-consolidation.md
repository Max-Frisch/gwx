# E8 — Spec consolidation & regeneration

**Kind:** extraction · **Source:** `HAR-CRS` + `HAR-DTA` + `HAR-LOGIN` · **Depends on:** E1–E7
**Commit:** `docs: consolidate GalaxyWars spec (full Uni5 walk) + refresh backlog`

## Goal

Tie E1–E7 together: make `gen_spec.py` emit a consistent, complete spec set,
refresh the navigation map, and fold the real values into `BACKLOG.md` /
`HANDOFF.md`. Run this only after E1–E7 have committed.

## Steps

1. Re-derive the machine catalog from both big HARs into `info/captures/derived/`
   (dumps already exist from E1/E2/E7, but regenerate to be safe):
   ```bash
   python tools/har/extract_catalog.py
   python tools/har/extract_pages.py
   python tools/har/fit_formulas.py      # sanity-check the fitted series
   python tools/har/gen_spec.py
   ```
2. Make `gen_spec.py` emit the whole set (`01`–`12`) — it should own all files it
   can derive and **not** clobber the hand-written sections added in E2–E7.
   Safest: have `gen_spec.py` write derived sections into `01`–`04` and `06`, and
   leave `07`–`12` (hand-authored) untouched.
3. Rewrite `05-ui-flows.md`: the nav map must now show every captured route
   (no "(not captured)"), grouped PLANET / UNIVERSE / COMMUNITY / ACCOUNT, with
   the endpoints from E2–E6 and cross-links to `07`–`12`.
4. Update `BACKLOG.md`:
   - move "Follow-up HAR walk" from Todo to Done;
   - replace the "Not captured at all" / "Defense page gap" notes with the real
     figures (defense costs/stats, trade/alliance/statistics/highscore fields);
   - refresh the **Ready to build now** list ordering.
5. Update `HANDOFF.md`: mark the Uni5 pipeline complete, point to
   `info/sessions/README.md` for the implementation queue, and note the new spec
   files.
6. Commit the changed spec files + `BACKLOG.md` + `HANDOFF.md`.

## Definition of done

- `info/galaxywars-spec/` contains `01`–`12` with no "not captured" gaps for the
  pages present in the HARs.
- `python tools/har/gen_spec.py` is idempotent (running twice yields no diff).
- `BACKLOG.md` / `HANDOFF.md` reference the session playbook.
- No raw HAR content committed.
