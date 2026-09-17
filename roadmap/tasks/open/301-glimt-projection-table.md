# 301 — nathejk/table/glimt projection

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §8. A projection package in the house shape: `table.go` with an embedded `table.sql`
applied via `CREATE TABLE IF NOT EXISTS` plus `cqrs.EnsureColumn` / `cqrs.EnsureIndex` for
additive drift, `consumer.go` folding events, `querier.go` for reads.

Three tables:

- `glimt` — `glimtId`, `year` (PK), `authorPersonId`, `authorGroup`, `teamNumber`,
  `teamName`, `audience`, `caption`, `createdAt`, `mediaCount`, `hiddenAt`, `hiddenBy`,
  `reportCount`, `deleted`
- `glimt_media` — `glimtId`, `ordinal`, `blobRef`, `thumbRef`, `kind`, `width`, `height`,
  `durationMs`
- `glimt_report` — `glimtId`, `reporterPersonId`, `reason`, `createdAt`

`teamNumber` / `teamName` are the **hold's** number and name (PRD 019 §0b) — the names mirror
`person` deliberately. Index on `(year, teamNumber)` for the hold collection query and on
`(year, audience, createdAt)` for the feed.

Subjects consumed: `NATHEJK.*.glimt.*.created` / `.deleted` / `.reported` / `.hidden` /
`.unhidden` / `.purged`.

Projections are rebuilt by replaying the stream on boot, so the consumer must be idempotent.

## Acceptance Criteria

- [ ] `go/nathejk/table/glimt/` with `table.go`, `table.sql`, `consumer.go`, `querier.go`
- [ ] `Consumes()` lists all six subjects
- [ ] Consumer is idempotent — replaying the same event twice leaves the same row
- [ ] Indexes for the feed and the hold collection queries
- [ ] Registered in `go/cmd/api/eventing.go` projections
- [ ] Tests for the consumer folds (created, deleted, hidden/unhidden, report increments count)
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
