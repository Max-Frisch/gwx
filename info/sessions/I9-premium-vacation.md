# I9 — Premium / vacation / sanctions (optional)

**Kind:** build (optional) · **Depends on:** E6 (spec `11-premium-settings.md`)
**Commit:** `feat: vacation mode + sanction flags`

## Goal

Only do this if we want the account-lifecycle extras. The reference's **Premium /
Galaxium shop** is explicitly **out of scope** for gwx (no monetisation; we are a
hobby retro revival). What is worth having:

- **Vacation mode** — a per-user flag that gates attacks/builds (with the same
  rules the reference describes; capture them in E6). Entering/extending/leaving
  via `/settings/vacation`.
- **Sanctions** — an admin/moderation flag that restricts an account. If gwx has
  no admin surface yet, implement it as a flag + a small `/settings/sanction`
  read-only view and skip the moderation tools.

## Work

- Migration `0005_account_flags.sql`: columns on `users`
  (`vacation_until TIMESTAMPTZ`, `sanctioned BOOLEAN DEFAULT FALSE`,
  `sanction_reason TEXT`).
- Gate the relevant actions in `internal/store` (`Dispatch`, `StartBuilding`,
  `StartResearch`, `StartUnits`) and surface the reason in `internal/web`.
- Web: `GET/POST /settings/vacation`, `GET /settings/sanction`; nav under ACCOUNT.
- Document the exact gate rules in `11-premium-settings.md`.

## Definition of done

- A vacationing/sanctioned account cannot attack or start queues, sees why, and
  can leave vacation; sanctions are visible to the sanctioned user.
- `go build ./... && go vet ./... && gofmt -l internal cmd && go test ./...` clean.

## Notes

- If E6 marks these as "needs mechanic / out of scope", this session can be
  cancelled — record that decision in `info/sessions/LOG.md`.
