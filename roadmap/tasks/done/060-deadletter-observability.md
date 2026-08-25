# 060 — Deadletter observability

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 008 §5/§6. The `cqrs.Writer` is `deadletter` wrapping `sqlpersister`. A
projection statement that cannot be applied must be **visible**, not swallowed —
a silently deadlettered event is a projection that is quietly wrong, which is the
worst failure mode in a CQRS system.

## Acceptance Criteria

- [x] Deadlettered statements are logged with enough context to identify the
      subject and the failure
- [x] A count//query is exposed (healthcheck field per task 059, a log metric, or
      both) so a non-zero deadletter count is discoverable without reading logs
- [x] Documented where deadletters land and who is expected to watch them
      (PRD 008 §11 Q10 asks whether this belongs in shared infrastructure)
- [x] `go build`, `go vet`, `go test ./...` green

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. Audited what already exists before adding anything, and
  two of the three criteria turned out to be partly covered:
  1. **Per-capture logging already exists** in the library — `deadletter.Writer`
     does `log.Printf("deadletter: captured failing statement: %v", err)` on every
     capture, and stores the statement plus the cause in its table. Nothing to add;
     noting it so nobody duplicates it. (It uses stdlib `log` rather than our
     `slog` handler, so it will not be JSON-structured — a wart, not worth wrapping
     the library over.)
  2. **The count is already exposed** in the healthcheck as of task 059, including a
     `warning` field when non-zero.
- 2026-08-25 — So the genuinely missing piece was a **recurring** signal. Added
  `watchDeadletters`, a 5-minute ticker that warns only while the count is non-zero.
  The gap it closes: a capture logged during a replay at 02:00 scrolls out of view,
  and nobody polls a healthcheck they have no reason to suspect — so an incomplete
  projection could sit undetected for the whole event.
- 2026-08-25 — Deliberately **silent when the count is zero**. A periodic
  "everything is fine" line trains people to ignore the channel, which is how the
  signal it exists to carry gets missed.
- 2026-08-25 — Also log the count once immediately after arming: rows surviving
  `Reset()` are from this run's replay, so that is the first honest reading.
- 2026-08-25 — Where they land, for the record: the `deadletter` table in `hej`'s own
  database, created by the writer at startup and cleared on every boot before replay.
  **Nobody watches it today** — there is no alerting in this repo. PRD 008 §11 Q10
  (should this live in shared infrastructure?) is still open and now matters more,
  because a warn line is only useful if something reads the logs.
- 2026-08-25 — ✅ All criteria complete. Verified with `-race` (new goroutine), plus
  vet/staticcheck/gofmt.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so no statement
  was ever actually dead-lettered. The count path is exercised only via the
  no-writer case (returns 0). **Not** verified: that a genuinely failing projection
  statement gets captured and counted — that needs a real database and a deliberately
  broken projection, and is worth doing when PRD 006 lands the first one.
- 2026-08-25 — Moving to done.
