# 160 — Withdrawn-member handling

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

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

## Status note — server half done, client half blocked

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

What is left is client-side and cannot be built yet:

- the status marking in the list row and on the profile — needs tasks 164 and 167;
- no `tel:` action for a withdrawn member — needs task 164;
- a favourited member who withdraws staying visible with their marking — needs task 166;
- the full sync→withdraw→re-sync assertion against the *client's stored copy* — needs the
  offline store from tasks 161/162.

Deliberately left in `open/` rather than moved to `doing/` and parked, per TASKS.md's
"only one owner at a time" rule: a task sitting in `doing/` with no active owner is worse
than one honestly still open. Pick it up after 166 and 167.

## Acceptance Criteria

- [ ] A withdrawn member's phone number is absent from the manifest.
- [ ] Their name, portrait and status marking are present until end of race.
- [ ] Status marking is visible in the list row and on the profile page.
- [ ] The row offers no `tel:` action for a withdrawn member.
- [ ] **Test: sync → withdraw → re-sync**, asserting the number is gone from the
      client's stored copy, not merely from the server's response.
- [ ] A favourited member who withdraws stays visible with their marking rather than
      vanishing from favourites.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §11.6.
- 2026-08-31 20:55 — Reviewed against what has shipped. The server half was necessarily done
  inside task 154 (the manifest cannot exist without a position on withdrawn members), with
  tests. Recorded above. The remaining criteria are all client-side and blocked on tasks
  164, 166, 167 and the offline store — leaving this open rather than parking it in `doing/`.
