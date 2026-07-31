# 003 — Mock user directory + phone recognition

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

## Description

Implement phone-number recognition + role lookup in the BFF behind a small
**user-directory interface**, with a **mock** implementation (seeded, in-code
phone → role map) as the only implementation for now. The real Nathejk lookup
replaces it later without touching handlers. See
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`.

Roles to seed: `spejder`, `bandit` (main), `postmandskab`, `guide`, `samarit`.
Phone numbers must be normalized consistently (assume Danish `+45` default —
confirm) before lookup.

Depends on: 002.

## Acceptance Criteria

- [x] A directory interface (`Directory.Lookup(phone) (User{ID, Role}, found)`)
      defined in `internal/users` and consumed via the `data.Models` facade.
- [x] A mock implementation (`users.NewMockDirectory`) seeded with one phone per
      role (spejder, bandit, postmandskab, guide, samarit).
- [x] Phone normalization helper (`internal/phone.Normalize`) with unit tests
      (E.164, bare 8-digit local +45, spaces/dashes/parens, `00` prefix,
      invalid inputs).
- [x] Lookup is used only through the interface; the concrete impl is injected in
      `main.go` (`data.NewModels(users.NewMockDirectory())`) so swapping needs no
      handler changes.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 17:00 — Implemented `internal/users` (`Directory` interface + `Role` consts + `User` + `mockDirectory` seeded per role) and `internal/phone` (`Normalize` + `ErrInvalid`). Wired the directory into `data.Models` (added `Users` field; `NewModels(usersDir)`), injected `users.NewMockDirectory()` in `main.go`, and updated `routes_test.go` to match the new signature.
- 2026-07-30 17:00 — Decision: confirmed Danish `+45` as the default country code for bare 8-digit numbers (per PRD open item; documented in `phone.DefaultCountryCode`). Kept normalization rules minimal + explicit (plus / `00` / bare-8) rather than guessing exotic formats.
- 2026-07-30 17:02 — ✅ Gates green: `go build`, `go vet`, `go test ./...` (users + phone tests pass), `gofmt -l` clean, `go tool staticcheck` clean.
- 2026-07-30 17:02 — Completed. Recognition + normalization ready for the auth request-pin handler (task 004).
