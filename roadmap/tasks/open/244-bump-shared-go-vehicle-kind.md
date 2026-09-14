# 244 — Bump shared-go in hej and verify GOWORK=off

**Status:** open
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

## Description

Tasks 234 and 243 both change shared-go. In dev, `go/go.work` resolves the sibling
checkout, so everything built on top of them compiles locally while `go.mod` still pins an
older version — which means a change that only builds with the workspace active looks
finished and fails CI and production.

The `api` container's dev loop has a `pinned_build` gate (`docker/init/api-dev`) for exactly
this, and task 224's log records it firing during PRD 015: it compiles with `GOWORK=off`,
refuses to start, and prints "push shared-go, then bump". Expect that state until this task
runs.

So: push shared-go, then `go get github.com/nathejk/shared-go@latest` inside the container,
and verify a `GOWORK=off` build and test run.

## Acceptance Criteria

- [ ] shared-go's vehicle changes (234, 243) are committed and pushed
- [ ] `go.mod` in `hej` pins a version containing them
- [ ] `GOWORK=off go build ./...` and `GOWORK=off go test ./...` pass in the container
- [ ] The `api` container's dev loop starts cleanly (its `pinned_build` gate passes)
- [ ] The `kind` column exists in the running dev database, verified rather than assumed

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
