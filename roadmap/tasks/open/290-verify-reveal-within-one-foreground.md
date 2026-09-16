# 290 — Verify on device: a reveal appears within one foreground

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 1's acceptance test, and the one that cannot be faked in a unit test:
the happy path from §5, end to end, on a real phone.

**Needs a device and a way to trigger a handout/reveal server-side. Not completable by
an agent session.**

The scenario:

1. A patrol holds Kort 1–2; note what the map draws and what the drawer lists.
2. Lock the phone.
3. Server-side, hand the patrol Kort 3 (and let the reveal rule expose its checkpoints).
4. Unlock the phone and return to the app **without** force-quitting it.
5. The new checkpoints must appear, and the new sheet must be listed, within one
   foreground — no cold start, no pull, no waiting for the 60 s tick.

Then the same for a scan: scan a post, lock, unlock, and the drawer must gain the row.

Why this is worth a task of its own: every part of the mechanism can be green in tests
while this fails, because what makes it work is whether the *resume* fires an event the
loop hears (task 280). This is the difference between the feature working and appearing
to work.

## Acceptance Criteria

- [ ] Reveal: new checkpoints drawn within one foreground, from a locked phone.
- [ ] Handout: new sheet listed within one foreground.
- [ ] Scan: new row in the drawer within one foreground.
- [ ] Same three, returning from the app switcher rather than from lock.
- [ ] Same three, returning after a bfcache navigation (external link and back).
- [ ] Confirmed no duplicate `/api/sync` per resume (debounce working, task 281).
- [ ] Confirmed no payload requests when nothing changed.
- [ ] Network trace or screenshots in the Progress Log.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Blocked on device access and a
  server-side handout trigger.
