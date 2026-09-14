# 230 — Record the phone numbers check-in already sends us

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `handleTeamStarted` writes `Phone` and `PhoneGuardian` for every member in the event,
      alongside the existing `memberStatus=racing`
- [ ] After the start, the profile response serves the check-in contact number rather than
      the register's `phoneParent`, and a test covers the case where the two differ
- [ ] A member who never verified in the app has a recorded contact number after the start
- [ ] An event member with an empty or missing phone field does not overwrite a number we
      already hold with a blank
- [ ] The `phoneContact` / `phoneGuardian` / `phoneParent` mapping is documented at the write
      site
- [ ] No backfill script was added

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
