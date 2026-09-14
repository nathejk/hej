# 230 — Record the phone numbers check-in already sends us

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

`handleTeamStarted` in `go/nathejk/table/person/consumer.go` already consumes
`messages.NathejkTeamStarted` (subject `NATHEJK:*.patrulje.*.started`), and each
`NathejkTeamStarted_Member` on it carries `Phone` and `PhoneGuardian` for a member who has
started. The handler discards both and writes only `memberStatus=racing`.

Those two fields are the numbers the counter actually established, one member at a time, at
the moment of check-in. Record them.

Why it matters: after the start, the app must show the number **staff hold**, not the
register's. Showing the register's number while the counter is holding a different one is
worse than showing nothing, because the member has no way to know which one anybody will
call. So the settled contact number is served from what arrived on this event.

A useful consequence, worth stating so nobody builds the thing it makes unnecessary: a member
who started **without** verifying in the app also ends up with a recorded contact number,
through the same event. **No backfill job is needed**, and no further prompting — the fast
track is about who can be *skipped* at the counter, not about chasing anybody afterwards.

Beware the third spelling of one number (PRD 015 §11 Q1): the register calls it
`phoneContact`, this event calls it `phoneGuardian`, the projection calls it `phoneParent`.
Document the mapping where the write happens.

Pairs with task 231, which closes the app's edit path at the same signal. Depends on task
222 only insofar as it lands in the same projection.

## Acceptance Criteria

- [x] `handleTeamStarted` writes `Phone` and `PhoneGuardian` for every member in the event,
      alongside the existing `memberStatus=racing`
- [x] After the start, the profile response serves the check-in contact number rather than
      the register's `phoneParent`, and a test covers the case where the two differ
- [x] A member who never verified in the app has a recorded contact number after the start
- [x] An event member with an empty or missing phone field does not overwrite a number we
      already hold with a blank
- [x] The `phoneContact` / `phoneGuardian` / `phoneParent` mapping is documented at the write
      site
- [x] No backfill script was added

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — New columns `startedPhone` / `startedPhoneContact`, rather than writing into
  `phoneParent`: that column is the register's value, and overwriting it would destroy the
  ability to see that the counter recorded something different — which is the most useful thing
  this data can tell an organizer.
- 2026-09-12 — Numbers are normalized on the way in with the projection's own normalizer. Task
  229's rule compares the contact number against the member's own, so an unnormalized
  "20 00 00 01" here would read as a different number and the collision would slip through.
- 2026-09-12 — An omitted number writes nothing at all, asserted by its own test. Writing ""
  would turn silence into "check-in recorded no contact number", and that claim is what the app
  would then show in place of a number we do hold.
- 2026-09-12 — Precedence lives in `Person.ContactNumber()` so it is stated once: check-in's
  value if there is one, otherwise the register's, plus a second return distinguishing "this
  population has none" from "one is expected" — the nil-vs-"" distinction the profile page and
  `confirmationRequired` both read.
- 2026-09-12 — `app.contactNumber` combines the two sources, because the profile is assembled
  from `users.User` (which knows only the register) while check-in's value lives on the person
  row. It degrades to the register's number when the projection cannot answer — not to "none",
  which the client would render as though the member had nothing on file.
- 2026-09-12 — ✅ All criteria. Four projector tests, one HTTP test asserting the check-in number
  wins *and* that the superseded register value does not appear anywhere in the body. The
  four-spelling mapping (`phoneContact` / `phoneGuardian` / `phoneParent` /
  `startedPhoneContact`) is written at the write site and in `table.sql`, which answers PRD 015
  §11's naming question in the one place a reader will be standing when they need it.
- 2026-09-12 — `gofmt -l`, `go test ./...`, `GOWORK=off go build ./...` clean.
