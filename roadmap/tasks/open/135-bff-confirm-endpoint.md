# 135 — BFF: POST /api/me/profile/confirm

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `POST /api/me/profile/confirm` registered in `go/cmd/api/routes.go` behind
      `requireAuth`, handled in `go/cmd/api/profile.go`
- [ ] Body carries the two digits and the acknowledgement flag; a request missing the
      acknowledgement is rejected, not silently accepted
- [ ] Digits are validated server-side against the member's guardian number
- [ ] `204` success / `400` wrong digits / `401` unauthenticated / `409` already confirmed
- [ ] The user is resolved from the session, never from a client-supplied id
- [ ] Wrong attempts are rate-limited using the same per-IP limiter pattern as the PIN
      endpoint in `auth.go`
- [ ] Success publishes the verification event (task 133); no direct SQL write
- [ ] Nothing about confirmation state is stored client-side
- [ ] OpenAPI annotations present and complete, matching the style in `auth.go` and
      `profile.go`, and stating plainly that the digit check is a recognition check
- [ ] Members with no guardian number on file cannot reach a state where this endpoint is
      required of them (spejder-only rule, PRD 005 §6)

## Depends on

- **Task 133** — the publish path.
- **Task 134** — `confirmation_required` / `verified_at`, which is how `409` is decided.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
