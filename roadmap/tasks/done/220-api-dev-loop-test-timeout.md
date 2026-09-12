# 220 — The api dev loop's test gate times out on a cold cache

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

Found while verifying task 216 against the running stack: **the API was not running**, and
`docker compose logs api` said `==> build failed; waiting for changes`.

It was not a build failure. `docker/init/api-dev` ran its test gate with
`go test -timeout 10s ./...`, and `nathejk.dk/cmd/api` takes **~25s** on an ARM laptop —
a dozen or so tests each issue a login PIN, and `pin.Store.Issue` bcrypts it, which is
expensive by design. So the gate hit Go's test timeout, which panics and dumps every
goroutine; the script saw a non-zero exit and reported a build failure, leaving the API
down with a stack trace where a test failure should have been.

## Why it had not been noticed

With a warm test cache `go test` answers in milliseconds and reports `(cached)`, so the
10s budget was never approached. The first change to invalidate that package's cache —
in this case adding `cmd/api/dev_test.go` for task 215 — exposed it. That makes this a
latent trap for **any** future change to `cmd/api`, not a problem with task 215.

Worth noting the inconsistency it also resolves: `docker/Dockerfile`'s own gate already
allows `-timeout 60s`, so the dev loop was stricter than the image build for no stated
reason.

## Fix

`-timeout 120s` in `docker/init/api-dev`, with a comment recording why the number is
not small. Generous rather than exact: dev machines vary, and the cost of this being too
low is a confusing failure that looks like a broken build, while the cost of it being
high is only how long a genuinely hung test takes to be reported.

## Acceptance Criteria

- [x] The dev loop's test gate no longer times out on a cold cache
- [x] The API starts and answers after a restart
- [x] The reason is recorded in the script, not just in this task
- [x] No test was changed, skipped or made shallower to fit a budget

## Progress Log

- 2026-09-12 09:50 — Found while trying to curl `/api/dev/pin` to verify task 216: the API was
  not listening. Traced from `curl` returning 000, through `docker compose logs api`, to a
  goroutine dump ending `FAIL nathejk.dk/cmd/api 10.020s` — the `.020` is the tell, it is Go's
  timeout rather than an assertion.
- 2026-09-12 09:52 — Confirmed the suite itself is healthy: `go test -count=1 ./cmd/api/` passes
  in 25.2s. So the gate, not the code, was wrong. Deliberately did **not** speed the tests up or
  reduce bcrypt's cost to fit 10s — the expense is the security property.
- 2026-09-12 09:53 — Raised to 120s and documented in the script. ✅
