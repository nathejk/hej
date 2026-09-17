# 325 — Offline field test: post a glimt with no signal

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §5, §9. The promise is that **the app does not lose a photo it accepted**. That is a
claim about a phone in a field, so it is verified on a phone in a field, not in a unit test.

Follow the existing offline protocol in task 172 where it applies. On a real device, in
airplane mode or genuinely out of coverage:

1. Post a glimt with two photos. Confirm it appears immediately, marked *venter på nettet*.
2. Close the app entirely. Reopen it offline — the pending glimt is still there.
3. Restore the network with the app **foregrounded** — the outbox drains.
4. Repeat, but restore the network with the app **closed**, then reopen: it drains on
   foreground. (It must not claim to have uploaded in the background — iOS does not run a
   backgrounded web app.)
5. Kill the app mid-upload. Reopen: already-uploaded items are not re-uploaded, and the glimt
   completes.

Record what actually happened, including anything that behaved differently from the design.

## Acceptance Criteria

- [ ] All five scenarios run on a real iOS device (16.4+), results logged here
- [ ] Repeated on Android/Chrome
- [ ] Zero lost media across the runs
- [ ] Pending state copy is accurate — never implies background upload
- [ ] Any bug found is filed as a new task and referenced here

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
