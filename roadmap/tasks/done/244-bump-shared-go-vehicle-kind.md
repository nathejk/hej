# 244 — Bump shared-go in hej and verify GOWORK=off

**Status:** done
**Priority:** medium
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

Task 243 changes shared-go. In dev, `go/go.work` resolves the sibling checkout, so
everything built on top of it compiles locally while `go.mod` still pins an older version —
which means a change that only builds with the workspace active looks finished and fails CI
and production.

The `api` container's dev loop has a `pinned_build` gate (`docker/init/api-dev`) for exactly
this, and task 224's log records it firing during PRD 015. It compiles with `GOWORK=off`,
refuses to start, and prints "push shared-go, then bump".

So: push shared-go, then `go get github.com/nathejk/shared-go@latest` inside the container,
and verify a `GOWORK=off` build and test run.

**Amended 2026-09-14:** the bump for task 234's `Filter.CustodianUserIDs` was done during
task 237 instead of here, because 237 was the first code to use the field and committing it
otherwise would have left `main` building only with the workspace active. `go.mod` is
currently on `v0.0.0-20260914180220-e126b80fd5fa`. What remains for this task is the `kind`
field from task 243.

## Acceptance Criteria

- [x] shared-go's `kind` change (task 243) is committed and pushed
- [x] `go.mod` in `hej` pins a version containing it
- [x] `GOWORK=off go build ./...` and `GOWORK=off go test ./...` pass in the container
- [x] The `api` container's dev loop starts cleanly (its `pinned_build` gate passes)
- [x] The `kind` column exists in the running dev database, verified rather than assumed

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Scope reduced: task 237 already pushed shared-go `e126b80` and bumped `hej`
  to it, since it was the first consumer of `Filter.CustodianUserIDs` and the repo rule
  forbids committing code that builds only with the workspace. Only the `kind` bump is left.
- 2026-09-14 — Bumped to `v0.0.0-20260914184535-5514e4cc51ea` (shared-go `5514e4c`).
  `GOWORK=off` build and full test run pass; `go vet` and `staticcheck` clean.
- 2026-09-14 — Verified the migration against the **running** dev database rather than
  inferring it from the code, which is what task 224's log could not do:
  `SHOW COLUMNS FROM vehicle LIKE 'kind'` reports `varchar(99) NOT NULL DEFAULT 'car'`, and
  `SELECT COUNT(*) FROM vehicle WHERE kind = ''` returns 0 — so `EnsureColumn` ran and no row
  came out of the replay with a blank kind. That second query is the one worth keeping in mind:
  it is the live check for the failure task 243's projector default exists to prevent.
- 2026-09-14 — ✅ All criteria. The api container's dev loop restarts cleanly with
  `projections registered count:3`.
