# 301 — nathejk/table/glimt projection

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `go/nathejk/table/glimt/` with `table.go`, `table.sql`, `consumer.go`, `querier.go`
- [x] `Consumes()` lists all six subjects
- [x] Consumer is idempotent — replaying the same event twice leaves the same row
- [x] Indexes for the feed and the hold collection queries
- [x] Registered in `go/cmd/api/eventing.go` projections
- [x] Tests for the consumer folds (created, deleted, hidden/unhidden, report increments count)
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 11:45 — Picked up. Read `maphandout` (the newest projection, so the most current
  house style) and `person/portrait.go` (the only other event **this** app publishes). Two
  conventions inherited from them: `cqrs.Writer` takes a finished statement, so escaping is the
  consumer's job via `quote()`; and event types are owned by the projection that consumes them,
  because this package may not import `internal/...`.
- 2026-09-17 11:55 — Schema. Three tables rather than one: media get their own because the
  post-race browse reads *thumbnails by the thousand* and never wants the parent row, and
  reports get their own because they are an append-only audit trail while `glimt` is folded.
  Four indexes, one per known read — including `year_team_created` for the hold collection,
  which is the read the whole feature is judged on.
- 2026-09-17 12:10 — `events.go`. Six events rather than one "changed" event with nullable
  fields, for the reason `portrait.go` gives about not encoding a delete as an empty ref: a
  replay must distinguish a deliberate takedown from a malformed message, and the log should
  record *why* a child's photograph disappeared.
- 2026-09-17 12:25 — Decision worth recording: **the create fold does not touch `hiddenAt`,
  `hiddenBy` or `reportCount`.** Those belong to later events. Writing zeros there would be
  harmless on a clean replay (create is applied first) and catastrophic on a re-delivery of an
  old create after a hide — it would silently restore a reported glimt to the public feed. Left
  out of the update clause entirely rather than reasoned about per replay; a test asserts it.
- 2026-09-17 12:30 — Second decision: **`reportCount` is derived, not incremented.**
  `reportCount = reportCount + 1` would climb on every rebuild, and that number is what the
  moderation queue sorts on. Recomputed with a subquery over `glimt_report`, whose primary key
  includes the reporter — so one person tapping twice cannot inflate it either.
- 2026-09-17 12:35 — `handleReported` **hides in the same fold** as it records. Not an
  optimisation: with no approval queue in front of the public scope, a gap between "reported"
  and "hidden" is a gap in which the objected-to thing is still on the open web. `hiddenAt` is
  only set if currently NULL, so a second report does not move the first takedown's timestamp.
- 2026-09-17 12:50 — Querier. `Filter` is duplicated locally from `users.GlimtFeedFilter`
  because this package cannot import `internal/...`; cmd/api converts in one place.
  `Filter.where()` returns **"0" and never "1"** for a denied filter — if a bug ever lets a
  denied read reach SQL, it should return nothing rather than the whole table.
- 2026-09-17 13:00 — Two queries rather than a join for media: a join multiplies each parent by
  its media count, and pages are ten glimt with up to ten items each. One extra round trip beats
  de-duplicating a hundred rows in Go.
- 2026-09-17 13:05 — `PublicFeed` deliberately **takes no caller at all**. The public page will
  share a host with the app, so a logged-in member's browser sends its session cookie to it; a
  read that *could* notice would make the public page silently different for members than for
  parents, and "is this public-safe?" untestable. The signature is the enforcement.
- 2026-09-17 13:10 — `Get` is the one unfiltered read, named plainly so a handler using it
  without a `MaySeeGlimt` check reads as obviously incomplete. It backs DELETE authorization and
  the media handler, both of which need the row in order to decide.
- 2026-09-17 13:20 — ✅ Consumer tests pass, including escaping (captions are the least trusted
  input in the service), path-shaped refs being dropped, and `Subject()` rejecting an id with a
  dot — which would still publish, still match `NATHEJK.>`, and quietly stop matching the
  per-glimt patterns, making that glimt impossible to hide.
- 2026-09-17 13:30 — ✅ Querier tests pass (sqlmock). One caught my own error rather than the
  code's: I expected four bind args where the group clause contributes two, so five. Also
  asserts the deliberate feed-DESC / hold-ASC asymmetry on the SQL, since that is exactly the
  kind of thing a later "tidy-up" would unify.
- 2026-09-17 13:40 — Registered in `main.go` beside the other projections, with the same
  construction condition, and added to the `registerProjections` list.
- 2026-09-17 13:45 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...`
  green. Moving to done.

### Notes for the tasks that build on this

- **`Media.Kind` leniency runs one way only.** An unrecognised kind becomes `image`, so a bad
  value yields a possibly-broken `<img>`. The reverse would hand arbitrary bytes to a media
  element to play. Task 322 should keep that direction.
- Task 304 should build its filter with `users.GlimtFeedFilterFor` → `glimt.Filter` and still
  pass rows through `users.MaySeeGlimt` on the way out. The agreement test in `internal/users`
  is what catches a drift between the SQL and the predicate.
- Task 310 has `Expired()` waiting for it, and it already returns **thumbnail refs as well as
  full ones** — forgetting those would leave recognisable images on disk while technically
  having purged the glimt.
- Task 308 has `Moderation()`, which takes no filter. That means the handler is the only thing
  between it and every group-scoped photo in the event.
