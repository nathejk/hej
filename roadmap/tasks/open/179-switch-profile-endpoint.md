# 179 — `POST /api/auth/switch` and the profile count on `/api/me`

**Status:** open
**Priority:** high
**Created:** 2026-09-01

## Description

From **PRD 012**. The server half of switching profile, and the fact the client needs to decide
whether to offer it.

**`POST /api/auth/switch`**, behind `requireAuth`. Resolves the caller, reads their own normalized
phone number, and returns `{choice_token, candidates}` in **exactly the shape `/auth/verify`
returns** for a shared number — so the client reuses one code path rather than growing a second.

`/auth/choose` stays **untouched**. It already binds the token to a number and re-checks the chosen
user against that number's current owners, and leaving it alone is what keeps the login and switch
paths from drifting apart.

**`GET /api/me`** gains the number of profiles on the caller's number, so the menu item can be
hidden for the majority who have one. It is a fact about the caller's own number and discloses
nothing new.

Security reasoning is in PRD 012 §8 and must not be re-litigated in the handler comment — but the
handler should say *which* of `/auth/choose`'s three safety properties it preserves and which it
replaces, because that is what a reviewer needs.

## Acceptance Criteria

- [ ] `POST /api/auth/switch` behind `requireAuth`, returning the same body shape as a shared-number
      `/auth/verify`.
- [ ] `409` when the caller's number carries fewer than two profiles, and when the caller has no
      number on file. A single-owner number must not return a list of one.
- [ ] The token is minted for the **caller's own** number, read server-side from the directory —
      never from the request.
- [ ] `401` without a session.
- [ ] `GET /api/me` reports the profile count.
- [ ] Every switch is logged with both the current and the chosen profile id (the second lands in
      `/auth/choose`'s existing path, so check what it already logs before adding).
- [ ] Guardian numbers absent from the candidate payload (`.rules`) — the candidate shape is shared
      with login, so this should already hold; assert it rather than assume.
- [ ] OpenAPI annotations on both endpoints.
- [ ] Tests: happy path; single-owner refusal; no-number refusal; unauthenticated; and that a token
      minted for one number cannot be redeemed for a profile on another.

## Progress Log

- 2026-09-01 — Task created from PRD 012 §10.
