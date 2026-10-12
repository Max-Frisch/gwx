# I1 — Player/planet search + account settings

**Kind:** build · **Depends on:** E4/E5 (spec `10-messages-events-search.md`, `11-premium-settings.md`)
**Commit:** `feat: player/planet search + account settings`

Two small, self-contained features in one session (both touch the account/nav only).

## A. Search

Add a `/search` page that looks up **planets by name** and **players by username**,
and a `/player/{id}` profile page (public: username, economy points, net worth,
owned planets, alliance tag if any — no fleet info; fog of war stays intact).

- Store: add `SearchPlanets(ctx, universeCode, term)` and reuse
  `ScoreStore`/`PlanetStore` for profile data (`internal/store/planet_store.go`,
  `internal/store/score_store.go`).
- Web: `GET /search` (query param `q`), `GET /player/{id}` in `internal/web/web.go`;
  new templates `search.html`, `player.html`; links from galaxy rows + highscore.
- Reference: `info/galaxywars-spec/10-messages-events-search.md`.

## B. Account settings

Add `/settings` with tabs **Account** (rename, change email, change password) and
**Game** (the subset we support: quick-dispatch probe count, messages per page).
Planet tab (rename planet, set main planet) is optional here.

- Store: extend `internal/store/auth_store.go` with `UpdateUsername`, `UpdateEmail`,
  `UpdatePassword` (reuse PBKDF2 helpers in `internal/auth/password.go`; verify the
  current password before changing). Respect the same validation as registration.
- Web: `GET/POST /settings/account`, `GET/POST /settings/game`; new template
  `settings.html`; nav entry under ACCOUNT in `templates/base.html`.
- Reference: `info/galaxywars-spec/11-premium-settings.md`.

## Definition of done

- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean.
- Manual: register → search a planet/player → open profile → rename user, change
  email, change password, log back in with the new password.
- Add unit tests for the validation paths in `internal/web` (or `internal/auth`).

## Notes

- Keep the existing `requireUser` middleware and `flashRedirect`/`msg` conventions.
- Do not add nodes/JS; server-rendered `html/template` only.
