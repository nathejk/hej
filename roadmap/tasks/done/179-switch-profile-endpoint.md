# 179 — `POST /api/auth/switch` and the profile count on `/api/me`

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

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

## Implementation

`go/cmd/api/auth.go` (`switchProfileHandler`, `candidatesFor`, `identityResponse.ProfileCount`),
route in `routes.go`, tests in `switchprofile_test.go`.

**`/auth/choose` untouched**, as intended. The switch mints a token and hands off; ownership
re-checking and session issuing stay in the one place that already does them.

**Extracted `candidatesFor`** rather than building the payload twice. The `candidate` type's comment
explains at length why the disclosure is as thin as it is (first name plus affiliation, no surname or
role) — a second construction site is exactly how that stops being true, so login and switching now
share one.

**`ProfileCount` is `omitempty` and left at zero when unknown.** "One profile" and "we could not
look" must not be the same answer: zero means the client hides the switcher, which is the safe
direction, since offering the control is what would mislead.

**No signed-out gap.** `/auth/choose` issues a session cookie that *replaces* the previous one, so a
switch is one request rather than a sign-out followed by a sign-in. Asserted.

**A test helper the shared-number case forced.** The existing `authedCookies` assumes a single-owner
number — for a shared one `/auth/verify` answers with candidates instead of a session. Since every
test here needs a shared number by definition, `authedCookiesChoosing` completes the choice first.
Worth noting because reaching for `authedCookies` would have failed confusingly.

## Acceptance Criteria

- [x] `POST /api/auth/switch` behind `requireAuth`, returning the same body shape as a shared-number
      `/auth/verify`.
- [x] `409` when the caller's number carries fewer than two profiles, and when the caller has no
      number on file. A single-owner number must not return a list of one.
- [x] The token is minted for the **caller's own** number, read server-side from the directory —
      never from the request.
- [x] `401` without a session.
- [x] `GET /api/me` reports the profile count.
- [x] Every switch is logged with both the current and the chosen profile id — this handler logs the
      origin and the profile count; `/auth/choose` already logs the outcome, so the pair is covered
      without duplicating it.
- [x] Guardian numbers absent from the candidate payload — asserted against a fixture that has one.
- [x] OpenAPI annotations on both endpoints.
- [x] Tests: happy path; single-owner refusal; unauthenticated; completion through `/auth/choose`;
      and that a token minted for one number cannot be redeemed for a profile on another.

## Progress Log

- 2026-09-01 — Task created from PRD 012 §10.
- 2026-09-01 15:10 — Implemented. Found the candidate payload was built inline in the login handler
  and extracted it, so the switch cannot drift on what it discloses.
- 2026-09-01 15:20 — Tests needed their own sign-in helper: `authedCookies` cannot log in on a shared
  number, which is the only kind of number this feature applies to.
- 2026-09-01 15:25 — ✅ All criteria met. Verified with the pinned resolution too (`GOWORK=off`
  build/vet/staticcheck/test), which is the gate that caught me out last time.
