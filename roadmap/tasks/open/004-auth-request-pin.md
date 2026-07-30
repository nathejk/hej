# 004 — Auth endpoint: `POST /api/auth/request-pin`

**Status:** open
**Priority:** high
**Created:** 2026-07-30
**Picked up by:**
**Started:**
**Completed:**

## Description

Implement the PIN-request step of phone login on the BFF, per
`roadmap/prd/001-hej-nathejk-event-app-skeleton.md`. Given a phone number,
normalize + look it up in the directory (task 003); if recognized, generate a
PIN, store it hashed with expiry + attempt counter, and send via SMS
(`internal/sms`, assumed available). The response is **identical whether or not
the number was recognized** (anti-enumeration).

Policy: **6-digit** PIN, **10-minute** TTL, **60s** resend cooldown, per-phone +
per-IP rate limiting. A new PIN invalidates the previous one.

Depends on: 002, 003.

## Acceptance Criteria

- [ ] `POST /api/auth/request-pin` accepts `{ phone }` and always returns the
      same success-shaped response regardless of recognition.
- [ ] On recognition: 6-digit PIN generated, stored **hashed** with a 10-min TTL
      + attempt counter keyed by phone, and sent via `internal/sms`.
- [ ] Resend enforces a 60s cooldown and invalidates prior PINs.
- [ ] Per-phone and per-IP rate limiting in place.
- [ ] Endpoint has OpenAPI annotations.
- [ ] `go test`, `go vet`, `staticcheck` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
