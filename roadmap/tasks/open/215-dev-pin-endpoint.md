# 215 — Dev-only `GET /api/dev/pin`

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `GET /api/dev/pin?phone=` returns `{"pin":"..."}` in development
- [ ] The route is not registered outside `development`, with a test proving it
- [ ] Phone normalisation is the same call `/auth/request-pin` uses
- [ ] 404, indistinguishable between "unknown number" and "no PIN outstanding"
- [ ] The response body carries nothing but the PIN, asserted by a test
- [ ] Reading does not consume the PIN or count an attempt
- [ ] OpenAPI annotations present, stating the dev-only nature
- [ ] `go vet` and `go test ./...` clean

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 4.
