# 094 — BFF: `GET /api/me/profile`

**Status:** open
**Priority:** high
**Created:** 2026-08-28

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

- [ ] `GET /api/me/profile` registered in `routes.go` behind `requireAuth`.
- [ ] Handler in a new `cmd/api/profile.go`, reading only through `app.models`.
- [ ] Full OpenAPI annotation block (`@Summary`, `@Tags`, `@Success`,
      `@Failure 401`, `@Router`).
- [ ] 401 without a session; never leaks another user's data.
- [ ] Handler test covering: authenticated happy path, 401, nil vs empty
      `phone_parent`.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
