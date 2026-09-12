# 215 — Dev-only `GET /api/dev/pin`

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed, delegated sub-agent)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 4. The BFF half of laptop login: return the currently-issued SMS PIN
for a phone number so a developer does not have to read
`docker compose logs api` on every login, switch-profile and shared-number
`/auth/choose` test.

`sms.LogSender` (`go/cmd/api/main.go:356`) already puts the PIN in the log, so
nothing new is disclosed to the operator — this only saves a terminal round-trip.

## Absent, not forbidden

Register the route **only** when the app's env is `development` (`cmd/api/env.go`,
`ENV: development` in `docker-compose.yml`). Not present-and-403: a production
binary must have no such handler at all, so no misconfiguration can reach it.

## Privacy

Return **only** the PIN. The dev database holds real personal data about minors
replayed from the broker, and `.rules` prohibits guardian numbers outright. 404 with
no detail when nothing is outstanding, so a known-but-PIN-less number and an unknown
number are indistinguishable — it must not become a phone-number oracle even in dev.
Log nothing new.

OpenAPI annotations are mandatory (`.rules`), and the description must say the route
does not exist outside `ENV=development`.

## Acceptance Criteria

- [x] `GET /api/dev/pin?phone=` returns `{"pin":"..."}` in development
- [x] The route is not registered outside `development`, with a test proving it
- [x] Phone normalisation is the same call `/auth/request-pin` uses (`phone.Normalize`)
- [x] 404, indistinguishable between "unknown number" and "no PIN outstanding"
      — the two bodies are compared byte-for-byte in a test
- [x] The response body carries nothing but the PIN, asserted by a test
- [x] Reading does not consume the PIN or count an attempt
- [x] OpenAPI annotations present, stating the dev-only nature
- [x] `go vet` and `go test ./...` clean

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 4.
- 2026-09-12 09:05 — Picked up **out of phase order**, deliberately: it is the only slice of
  PRD 014 that writes to `go/` rather than `vue/`, so it was delegated to run in parallel with
  the frontend panel work. Disjoint write scope, no coordination cost.
- 2026-09-12 09:20 — **The PRD's assumption was wrong, and the correction matters.** §8 guessed
  `app.pins` might "just need a read accessor". It does not: PINs are stored **bcrypt-hashed
  only**, so an issued PIN is genuinely unreadable and no getter could expose it. What the
  endpoint needs is plaintext *retention* — a real concession, not a getter.

  Resolved with a separate constructor rather than a flag or a setter:
  `pin.NewDevStoreWithPlaintextRecall()`, selected once in `pinStoreFor(cfg)` at the single call
  site that knows `ENV`. `pin.NewStore()` is unchanged in behaviour, so a production process
  never holds a plaintext PIN in memory at all — which keeps the property that a memory dump or
  a stray log line cannot yield a usable credential. `IssuedPlaintextForDev` reports only the
  code (not attempts, expiry or send time), refuses expired records, and neither consumes the
  PIN nor counts an attempt. Recorded against PRD 014 open question 3.
- 2026-09-12 09:22 — Route registered conditionally in `routes.go`; outside development the path
  falls through to the ordinary `/api/` JSON 404, so there is no handler left to be reached by a
  misconfiguration.
- 2026-09-12 09:24 — The 404 is byte-identical for an unknown number and a known-but-PIN-less
  one, asserted in a test, so the endpoint cannot be turned into a phone-number oracle even in a
  dev environment holding real numbers.
- 2026-09-12 09:25 — The "nothing but the PIN" test reuses `forbiddenKeys` /
  `assertNoForbiddenFields` from `guardiantripwire_test.go`, so the guardian-number prohibition
  is enforced by the same tripwire as everywhere else rather than by a second local list.
- 2026-09-12 09:30 — ✅ Reviewed the diff by hand (the plaintext-retention change is
  security-relevant, so it was not taken on trust). `go vet ./...` clean, `go test ./...` all
  packages ok, 6 `TestDevPin_*` plus `TestPinStoreFor_OnlyDevelopmentCanRecallPlaintext` pass.
- 2026-09-12 09:30 — Note for task 216: the client half is not done, so the PIN is currently
  only reachable by curl. Also noted that the task text said `/api/auth/request`; the real route
  is `/api/auth/request-pin`.
