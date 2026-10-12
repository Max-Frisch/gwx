# U1 — UI refresh: responsive, fast, GigraWars-grade layout

A build session. Read this file top to bottom, do the work, run the checks, make
**one commit**. Run it in a fresh session with:

> execute `info/sessions/U1-ui-refresh.md`

## Goal

Make gwx look and feel "nice and proper": a polished, **responsive** layout that
stays **fast** (smooth scroll, snappy interactions) and visibly closer to the
GalaxyWars reference — building on the card layout already shipped (see
`info/comparisons/`).

## Decisions (stack — locked for this session)

Fits the repo's stated goal ("stdlib `html/template` + hand-written CSS; no Node,
no build step, no codegen"):

- **Templating: keep `html/template`.** No Templ — codegen isn't worth it and
  conflicts with the no-build-step goal.
- **CSS: keep hand-written `app.css`.** No Tailwind (needs a build step). Formalise
  what Tailwind would give us anyway: a small **design-token layer** (spacing,
  type scale, radii, elevation, motion) as CSS custom properties, then build the
  components on top of those tokens.
- **Interactivity: adopt [htmx](https://htmx.org) — vendored, no build step.**
  Add one minified `htmx.min.js` under `internal/web/static/`, load it from
  `base.html`. It is embedded via the existing `staticFS`, so no toolchain is
  introduced. htmx gives us partial, server-rendered updates (live queue state,
  resource ticker, inline build feedback) without full-page reloads.
- **No SPA framework, no Alpine.** htmx + a few small vanilla helpers is enough.
  Anything genuinely dynamic (canvas star map, combat animation) is a **separate,
  later** concern and gets its own vendored JS — not a reason to change the stack.

## Scope / tasks

- [ ] **Design tokens** — add a `:root` token block to `app.css` (space scale
  `--s1..--s6`, type scale, radii, `--shadow`, `--ease`, durations). Refactor the
  existing rules onto the tokens. No visual overhaul, just consolidation.
- [ ] **Vendor htmx** — `internal/web/static/htmx.min.js` (pinned version, note
  the version + license in a comment or `static/README`). Load with `defer` in
  `base.html`.
- [ ] **htmx partials (progressive enhancement — every page still works without
  JS):**
  - Building / research / ship / defense: POST the build form via `hx-post`, swap
    the affected card or section + the queue panel (`hx-target`/`hx-swap`), keep
    the `POST → redirect` fallback.
  - Resource strip: `hx-get` a tiny partial every N seconds (`hx-trigger="every 5s"`)
    so the topbar ticker updates without reloading.
  - Queue panel: "finishes in …" countdown driven client-side, resynced by htmx.
- [ ] **Responsive layout:**
  - Breakpoints (e.g. `<=720px` and `<=1000px`): sidebar collapses (off-canvas or
    top strip) with the resource strip staying reachable.
  - `.bcard` stacks (image/tile above, body, action full-width) on narrow screens;
    `.bcard-img` scales down.
  - Tables become horizontally scrollable or reflow on narrow screens.
  - Content max-width already exists (`.inner`, 1020px) — verify it breathes.
- [ ] **Visual polish pass:** hover/focus/active states, focus-visible rings,
  section chrome, empty states, consistent badges/tags, `prefers-reduced-motion`
  guard on any transition, tabular numerals for all numbers.
- [ ] **Optional artwork hook:** allow a real `<img>` tile per building (catalog
  `Icon` today). If no asset, keep the SVG placeholder. Do **not** block on art.

## Performance guardrails (the regression this session fixes)

The card layout shipped with a per-card `backdrop-filter: blur(2px)`, which
re-blurs the fixed starfield on **every scroll frame** and lagged badly on the
Construction page. Rules:

- **Never** put `backdrop-filter` (or `filter`) on a repeated list item. Cards use
  an opaque fill; if a frosted look is wanted, apply it once to a container.
- Long lists (buildings, highscore, galaxy): keep the DOM light; consider
  `content-visibility: auto; contain-intrinsic-size: auto <h>;` on repeated rows.
- Keep `background-attachment: fixed` starfield, but don't combine it with
  per-element blur.
- Budget: interactive pages should hit 60fps scroll on a mid-tier laptop; no
  layout thrash in scroll handlers (prefer CSS, avoid JS scroll listeners).
- Confirm with DevTools Performance: record a scroll; look for long "Recalculate
  Style"/"Paint" frames.

## Acceptance criteria

- Construction, Research, Shipyard, Defense and Highscore scroll smoothly (no
  visible jank) at desktop and mobile widths.
- Every action works **without JavaScript** (GET pages, POST forms + redirect).
- With JS on, build/queue/resource updates happen without a full reload.
- Layout is usable at 360px, 768px and 1440px; no horizontal scroll except where
  intentionally allowed (wide tables).
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` pass.
- Template tests still parse/render (`internal/web/*_test.go`).

## Verification

```bash
go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...
make up && make migrate && make run   # app on :8090; click through the pages
```

Manual: at 360/768/1440px, scroll each list page while recording a Performance
profile; build something and confirm the queue updates without a full reload;
disable JS and confirm the same build still works.

## Commit

One commit, e.g.:

```
feat(ui): responsive + fast layout — design tokens, vendored htmx, no per-card blur
```

## Non-goals

- No Tailwind/Templ/Node toolchain.
- No SPA rewrite.
- No new game features; layout/perf/interaction only.
- Real building artwork is welcome but optional (placeholder SVG is fine).
