# 094 — BFF: `GET /api/me/profile`

**Status:** done
**Priority:** high
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

The read side of PRD 003's profile page: one request returning the details the
page shows. Behind `requireAuth`, resolving the caller from the session cookie
via `app.models.Users.Get(s.UserID)` — never from a client-supplied id.

Depends on task 093 for the fields.

Shape (snake_case, like the other endpoints):

```json
{ "name": "...", "role": "spejder", "team": "...", "section": "...",
  "address": "...", "postal_code": "...", "city": "...",
  "phone": "...", "phone_parent": null }
```

`phone_parent` is `null` for roles that have no guardian number, and `""` when
one is expected but missing — the client renders those differently, so do not
collapse them.

Repo rule: **OpenAPI annotations are mandatory**. Follow `scans.go`'s handler as
the house style (`app.WriteJSON`, `app.AuthenticationRequiredResponse`, no
hand-rolled `http.Error`).

## Acceptance Criteria

- [x] `GET /api/me/profile` registered in `routes.go` behind `requireAuth`.
- [x] Handler in a new `cmd/api/profile.go`, reading only through `app.models`.
- [x] Full OpenAPI annotation block (`@Summary`, `@Tags`, `@Success`,
      `@Failure 401`, `@Router`).
- [x] 401 without a session; never leaks another user's data.
- [x] Handler test covering: authenticated happy path, 401, nil vs empty
      `phone_parent`.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — `cmd/api/profile.go` added, following `scans.go`'s house style
  (`app.WriteJSON`, `app.AuthenticationRequiredResponse`, no hand-rolled
  `http.Error`). User resolved from the session cookie only; there is no user id
  anywhere in the route, so there is no "forgot to check ownership" bug available
  to write here.
- 2026-08-28 — Decision: an unresolvable session user returns **404**, not an
  empty 200. A member deleted mid-session is a different answer from "we hold
  nothing about you", and a 200 full of empty strings would render as a profile
  page claiming the user has no name. Documented in the handler and annotated as
  `@Failure 404`.
- 2026-08-28 — `phone_parent` kept as `*string` all the way to the JSON, with a
  comment warning against "simplifying" it. Test
  `TestProfile_GuardianPhoneNullForPopulationsWithoutOne` asserts the key is
  *present and null* rather than merely absent, since a missing key and a null one
  are the same in Go's decoder but not in the client's type.
- 2026-08-28 — The empty-string case needed a spejder who shares a phone
  (`mock-sibling-b`), so that test goes through `/auth/choose` — which
  incidentally pins something worth pinning: a chosen account gets **its own**
  profile, not the first candidate's.
- 2026-08-28 — ✅ All criteria met. `gofmt -l`, `go test ./cmd/api`, `go vet ./...`
  and `staticcheck ./...` clean in the `api` container. Moving to done.
