# 135 — BFF: POST /api/me/profile/confirm

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §8. The endpoint behind the profile-confirmation onboarding step. Body carries
the two digits the user typed and the acknowledgement flag (*"Dette nummer kan kontaktes
i løbet af Nathejk"*). Both are required to advance, so both are required here.

Status codes: `204` on success, `400` on wrong digits, `401` unauthenticated, `409` when
the member has already confirmed.

## The digit check is a sanity check, not a security check

The digits are checked **server-side** so the acknowledgement is recorded against a real
answer rather than against whatever the client felt like claiming. That is the whole
reason the check exists on this side.

It is **not** a confidentiality control, and PRD 005 §11 (2026-08-30) settles this
explicitly: `GET /api/me/profile` legitimately returns `phone_parent` in full to its
owner (PRD 003, shipped), so a determined user can read the two hidden digits straight
out of the network response. **That is accepted.** The purpose of the step is to make the
member *look at the number and recognise it* — a member who cannot complete it has
discovered that the number on file is not one they know. Nobody is being authenticated by
it, and the number is the user's own guardian's, not a secret being kept from them. Do not
document or implement this as if it were an auth factor; an earlier framing that required
the full number never to reach the client has been withdrawn.

**Rate-limit wrong attempts** like the PIN endpoint in `go/cmd/api/auth.go` (per-IP
limiter, `app.RateLimitResponse`). Again not as a secrecy measure — just so the endpoint
cannot be hammered. Reuse the existing limiter pattern rather than inventing a second
shape for the same concern.

## Confirmation state is per-user and server-side

Never `localStorage`. PRD 005 §11 (2026-08-25) is the reason the PRD has BFF scope at
all: per-device state would re-prompt a participant after a reinstall, a new phone or
cleared site data — potentially mid-event, which is precisely when a participant must not
be handed a blocking form. The durable record is the projection from task 134.

The write itself goes through the event publish (task 133). Nothing here touches SQL
directly.

## Acceptance Criteria

- [x] `POST /api/me/profile/confirm` registered in `go/cmd/api/routes.go` behind
      `requireAuth`, handled in `go/cmd/api/profile.go`
- [x] Body carries the two digits and the acknowledgement flag; a request missing the
      acknowledgement is rejected, not silently accepted
- [x] Digits are validated server-side against the member's guardian number
- [x] `204` success / `400` wrong digits / `401` unauthenticated / `409` already confirmed
- [x] The user is resolved from the session, never from a client-supplied id
- [x] Wrong attempts are rate-limited using the same per-IP limiter pattern as the PIN
      endpoint in `auth.go`
- [x] Success publishes the verification event (task 133); no direct SQL write
- [x] Nothing about confirmation state is stored client-side
- [x] OpenAPI annotations present and complete, matching the style in `auth.go` and
      `profile.go`, and stating plainly that the digit check is a recognition check
- [x] Members with no guardian number on file cannot reach a state where this endpoint is
      required of them (spejder-only rule, PRD 005 §6)

## Depends on

- **Task 133** — the publish path.
- **Task 134** — `confirmation_required` / `verified_at`, which is how `409` is decided.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — **Endpoint implemented** in `profile.go`, registered behind `requireAuth`. Landed in
  the same commit as task 136: the two handlers share the file, the limiter and two new error
  helpers, so splitting them would have produced a commit that does not build.

  Decisions:

  - **`409` covers all three "nothing to confirm" reasons** — already verified, already started the
    event, and no guardian number on file (the spejder-only rule). Deliberately one status: the
    client treats 409 as "carry on into the app", and splitting it would tempt a caller into
    inferring which population somebody belongs to from an error code.
  - **Digits are compared as digits, not strings.** The stored number is normalized
    (`+4520000001`) while what the member sees is grouped for reading, and the client is under no
    obligation to send exactly two bare characters. Non-numerics are stripped from both sides, so a
    stray space cannot fail a correct answer — the check exists to catch a member who does not know
    the number, not one who typed it with a space. An empty or too-short number never matches, since
    answering "correct" for a number that is not there would record a verification of nothing.
  - **A missing acknowledgement is a 400**, not an implied yes. The tick is the substance of the
    step; the digits only establish that the member looked. Consent must never be inferred from a
    POST having arrived.
  - **A wrong answer gets a plain-language 400** ("de to cifre passer ikke"), which needed a new
    `BadRequestMessageResponse` — wrapping a Go error string would have read as an accusation, and
    this is most likely not the member's mistake at all. Same reasoning as the existing
    `RateLimitMessageResponse`, whose comment already notes the reader is often twelve and Danish.
  - **No publisher → 503, never 204.** A confirmation the log never saw did not happen; reporting
    success would stop the member being asked *and* leave no organizer able to see it.
  - **Rate limit is per IP, 20/hour**, deliberately generous: a patrol on one hotspot confirming
    during the same briefing must not throttle each other. It is not a secrecy measure, and the
    limiter's comment says so — the digits are not a secret, since `/api/me/profile` returns the
    whole number to its owner by design.
- 2026-08-30 — The OpenAPI description states plainly that this is a recognition check and not an
  authentication factor. That sentence is the point of writing it: the next person to read this
  endpoint will otherwise assume a verified flag means an identity was proven.
- 2026-08-30 — ✅ 12 endpoint tests plus a table test for the digit comparison. `go test ./...` and
  `GOWORK=off go build ./...` clean.
