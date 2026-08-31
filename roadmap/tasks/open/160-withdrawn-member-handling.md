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
