# 004 — Auth endpoint: `POST /api/auth/request-pin`

**Status:** done
**Priority:** high
**Created:** 2026-07-30
**Picked up by:** agent (opus-4.8)
**Started:** 2026-07-30
**Completed:** 2026-07-30

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

- [x] `POST /api/auth/request-pin` accepts `{ phone }` and always returns the
      same success-shaped response regardless of recognition.
- [x] On recognition: 6-digit PIN generated, stored **hashed** (bcrypt) with a
      10-min TTL + attempt counter keyed by phone, and sent via `internal/sms`.
- [x] Resend enforces a 60s cooldown and invalidates prior PINs.
- [x] Per-IP rate limiting in place (`internal/ratelimit`; 5/min in prod, tuned
      in tests). *(Per-phone throttling is covered by the resend cooldown.)*
- [x] Endpoint has OpenAPI (swaggo) annotations.
- [x] `go test`, `go vet`, `staticcheck`, `gofmt` pass.

## Progress Log

- 2026-07-30 13:12 — Task created.
- 2026-07-30 17:30 — Implemented. New packages: `internal/sms` (`Sender` interface + dev `LogSender` — no real provider assumed yet), `internal/pin` (`Store`: bcrypt-hashed 6-digit PINs, 10-min TTL, 5-attempt lockout, 60s resend cooldown; crypto/rand `Generate`), `internal/ratelimit` (per-key fixed-window `Limiter`). Added `cmd/api/auth.go` (`requestPinHandler`, anti-enumeration message, swag annotations), `cmd/api/helpers.go` (`clientIP` with X-Forwarded-For), and `RateLimitResponse` (429) to `cmd/api/app`. Wired `pins`/`sms`/`requestPinLimiter` onto the application struct + `main.go` + `routes.go` + test helper.
- 2026-07-30 17:30 — Decision: added `golang.org/x/crypto/bcrypt` (direct dep) to hash PINs — slows brute force over the small 6-digit space. Decision: recognized-but-cooldown and unrecognized both return the identical 200 body; only a genuinely malformed phone returns 400 (reveals nothing about who is known). SMS is the dev `LogSender` (logs the PIN) until a real provider is configured.
- 2026-07-30 17:31 — ✅ Gates green (build/vet/test/gofmt/staticcheck). New tests: pin store (issue/verify/mismatch/expiry/lockout/cooldown, clock-injected), ratelimit (limit/window/keys), and request-pin handler (recognized sends PIN, unrecognized is indistinguishable with no SMS, invalid → 400, rate-limited → 429) via a recording sender.
- 2026-07-30 17:31 — Completed. Ready for verify (task 005), which will consume `pins.Verify` + create a session.
