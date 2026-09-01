# 160 — Withdrawn-member handling

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

When a member leaves the race (`released` / `reunited`), PRD 007 §6 / §11.6 require:

- their **phone number is purged**;
- their **name and portrait remain visible until the end of the race**;
- they carry a **clear status marking** in the list and on the profile;
- the row is no longer callable.

Rationale: an identification need outlives a withdrawal, but a reason to call does not.
A disappearing row invites "did I imagine them?"; a marked row answers the question.

**The hard part is propagation, not display.** Purging a number is a *removal* in the
sync delta, not an update: a device that already holds the number must drop it. If the
manifest can only express "here is what changed", the number survives on every device
that synced before the withdrawal and the purge is a server-side gesture only.

Depends on task 150 (status source) and 154 (manifest, which owns the removal
semantics).

## Status note — split across tasks, closed 2026-09-01

This task's work landed in three places rather than one, which is worth recording so the history
is not misleading.

**2026-08-31.** The server-side requirements landed with task 154 rather than here, because
the manifest could not be built without deciding what a withdrawn member looks like in it:

- `stillInRace` is derived in one place from `types.MemberStatus` (`stillInRace()` in
  `go/cmd/api/contacts.go`), with `finished` deliberately **not** counted as a withdrawal;
- the phone number is omitted for a withdrawn member while name and portrait are kept;
- `TestContactsManifest_WithdrawnMemberLosesPhone` and
  `TestContactsManifest_FinishedIsNotAWithdrawal` cover both;
- `TestContactsManifest_ClearedFieldDisappears` covers the removal-propagation half of the
  sync→withdraw→re-sync criterion on the server side: the field disappears from the payload
  *and* the version changes, which is what makes a device refetch and replace.

What was left was client-side and has since shipped:

- **status marking in the list row** — task 164. "Ude af løbet" as *text*, plus a dimmed row.
  Text rather than only colour or opacity, because those are invisible to anyone who cannot
  distinguish them, and this is the fact that *explains* the missing phone number;
- **status marking on the profile** — task 167;
- **no `tel:` action for a withdrawn member** — task 164: the row's `callable` computed requires
  `stillInRace`, so suppressing the number and suppressing the action are one decision rather than
  two that could drift;
- **a withdrawn favourite stays visible** — task 166, and this turned out to be the subtlest part.
  `pruneAgainstDirectory` prunes against *presence in the directory*, not against `stillInRace`:
  a withdrawn member is still in the manifest, and dropping them from favourites the moment they
  go home is precisely when a samarit would be looking for them. Both sides have a test;
- **sync → withdraw → re-sync against the client's stored copy** — task 177's
  `"replaces rather than merges, so a purged phone number disappears"`, which asserts the number
  is gone from both the store state *and* the persisted payload. That is the assertion this
  criterion asked for; the store replacing wholesale is what makes the purge real rather than
  decorative.

## Acceptance Criteria

- [x] A withdrawn member's phone number is absent from the manifest (task 154,
      `TestContactsManifest_WithdrawnMemberLosesPhone`).
- [x] Their name, portrait and status marking are present until end of race — asserted in the
      same test, which checks name and `portraitVersion` survive.
- [x] Status marking is visible in the list row and on the profile page (tasks 164, 167).
- [x] The row offers no `tel:` action for a withdrawn member (task 164).
- [x] **Test: sync → withdraw → re-sync**, asserting the number is gone from the
      client's stored copy, not merely from the server's response (task 177).
- [x] A favourited member who withdraws stays visible with their marking rather than
      vanishing from favourites (task 166, with a test for each side of the distinction).

## One thing deliberately still open elsewhere

No `reunited`/`released` events reach this projection yet: the transition messages live in `hq` and
need lifting to `shared-go` (**task 174**). So `stillInRace` currently reads true for everyone,
which is correct for a pre-event state. That is a dependency of task 174, not unfinished work here
— the whole path from event to marking is built and tested against synthetic statuses, so when 174
lands this behaviour starts working with no further changes. Documented at `stillInRace()` in
`go/cmd/api/contacts.go` so the reason for the constant value is visible at the call site.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §11.6.
- 2026-08-31 20:55 — Reviewed against what has shipped. The server half was necessarily done
  inside task 154 (the manifest cannot exist without a position on withdrawn members), with
  tests. Recorded above. The remaining criteria are all client-side and blocked on tasks
  164, 166, 167 and the offline store — leaving this open rather than parking it in `doing/`.
- 2026-09-01 01:30 — All client-side criteria now shipped via 164, 166, 167 and 177. Verified each
  against the code and its test rather than assuming, and recorded which task owns which. ✅ Closed.
  The remaining dependency — no withdrawal events arrive until task 174 lifts the messages to
  `shared-go` — is 174's, and the path is built and tested against synthetic statuses.
