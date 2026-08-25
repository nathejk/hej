# 052 — Add shared-go, cqrs and stream dependencies

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §8. Add the three external modules the rest of the PRD needs:

- `github.com/nathejk/shared-go` — shared domain types and messages
- `github.com/jrgensen/cqrs` — the `Publisher`/`Writer`/`Reader` seam
- `github.com/jrgensen/stream` — jetstream/xstream/subject

Per `go-bff-layout`: in dev, `go.work` resolves shared-go from the sibling
`../../shared-go` checkout so edits are picked up live; CI/prod build with
`GOWORK=off` against the version pinned in `go.mod`. Both must work — a change
that only builds with the workspace active is a broken build.

`cqrs` and `stream` are NOT in the workspace; they resolve from the proxy at the
pinned version in every environment.

## Acceptance Criteria

- [ ] shared-go, cqrs and stream in `go.mod` at explicit versions
- [ ] `go/go.work` resolving `../../shared-go`, and `go.work.sum` handled
- [ ] `go.work` is gitignored or committed deliberately — decide and document
- [ ] `GOWORK=off go build ./...` green (proves the CI/prod path)
- [ ] `go build ./...` green with the workspace active
- [ ] `go test ./...` and `go vet ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
