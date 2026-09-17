# 307 — POST /api/glimt/:id/report — synchronous hide from public

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §0, §6. The public scope publishes immediately with no approval queue, so **reporting
is the safety mechanism** and it must not wait on a human.

Any authenticated member can report any glimt they can see. The report **hides it from the
public scope synchronously**, before anyone reviews it. Hiding is cheap and reversible;
leaving something up is not.

Publishes `NATHEJK.<year>.glimt.<glimtId>.reported`. The projection records the report and
increments `reportCount`; `hiddenAt` is set in the same fold so there is no window where a
reported glimt is still public.

Reports must work **without knowing who posted** — the reporter never sees the author, because
nobody outside moderation does (PRD 019 §6).

## Acceptance Criteria

- [x] `POST /api/glimt/items/:glimtId/report` accepts an optional reason, publishes the event
- [x] The glimt is hidden from the public scope with no human step
- [x] Test: after a report, the public feed query excludes it
- [x] Test: after a report, it is still visible to its author and to moderators
- [x] Reporting the same glimt twice does not double-hide or error
- [x] Rate limited per member so it cannot be used to spam
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 19:45 — Picked up. Note the two middle criteria and the "twice" one are properties of
  the **fold**, not the handler, and task 301 already implemented and tested them:
  `handleReported` records the report and sets `hiddenAt` in the same fold (so there is no window
  where the objected-to thing is still public), `INSERT IGNORE` keyed on the reporter makes a repeat
  report a no-op, and `hiddenAt=IF(hiddenAt IS NULL, ...)` stops a second report moving the first
  takedown's timestamp. Checked those off against the existing tests rather than duplicating them
  at the HTTP layer.
- 2026-09-17 19:55 — An **absent body is accepted**. The common case is a member tapping "Anmeld"
  and confirming, with no reason typed; requiring a JSON body would make the fastest path to the
  safest outcome the one that 400s.
- 2026-09-17 20:00 — Reason capped at 500 characters, and short on purpose: this is a pointer for
  whoever reviews it, not an incident report, and a long field invites writing things about other
  people that then live in a log forever.
- 2026-09-17 20:05 — Decision: **a member may only report what they can see.** Not to protect
  authors — a report on an invisible glimt would be a way to *confirm one exists*, which is exactly
  what the media handler's 403 is careful not to leak (task 305).
- 2026-09-17 20:10 — Decision: **the author may report their own glimt**, which looks pointless and
  is not. It is how a member who realises a picture should not be up gets it off the public feed
  immediately, without waiting for a delete to propagate or hunting for the right control. Refusing
  it would be a small cleverness that removes the fastest path to the safest outcome. Tested, with
  the reasoning in the test.
- 2026-09-17 20:15 — Separate `glimtReportLimiter` at **100/hour**, five times the create limit. The
  asymmetry is the design and there is a test that asserts the *relationship* rather than the
  numbers: a spurious report costs a moderator a glance, a throttled one costs a photograph
  somebody objected to staying up. Reporting must never be the harder action.
- 2026-09-17 20:20 — 503 on a publish failure, with a message the client must show. A silently
  failed report is the worst lie this feature could tell: the member believes they have acted, and
  the photograph stays up.
- 2026-09-17 20:25 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...`
  green. Moving to done.

### Note for task 316 (the feed card)

`Anmeld` must be in the **one-tap overflow on every card**, not behind a long-press, and the failure
case must surface — if this endpoint 503s, the member has to know their report did not land. A silent
catch here would undo the whole mechanism.
