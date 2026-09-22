# 384 — Confirm blob storage and backup capacity before the first real hand-in

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

Not a code task. PRD 022 §8.11 calls this **the largest operational risk in the PRD** and says plainly
that it is not a code risk: it is somebody confirming there is disk and backup capacity before three
hundred photographs arrive.

The numbers from PRD 022 §6 Non-Functional: a card's worth of photographs at roughly 3 MB of stored
rendition each is **order 1 GB per event**. The blob store is the only non-rebuildable data in this
service and is therefore already the whole backup scope (PRD 008 §8) — every projection replays from the
log, the blobs do not. So this materially increases what the backup must hold, and it increases it
monotonically: PRD 022 §11 Q2 resolved that photographs are **never purged**. Unlike glimt, which is
participant-shared material under a retention promise (PRD 019), nobody was told these would disappear —
they are our photographers' work and the event's raw record. There is no retention setting and no purge
job in this PRD, by decision.

So the growth is one event's worth per year, forever, and the thing to confirm is not just "is there room
for 2026" but "is the trajectory funded". The failure mode this task prevents is the ugly one: a
photographer invited to upload a full card, the disk filling mid-batch, and half a hand-in in an
inconsistent state with the photographer's card already wiped.

Worth measuring at the same time (PRD 022 §8.9): an organizer with a stuck upload script can fill a disk
just as effectively as a participant, and the per-user glimt rate limiters do not apply here because
there is no user. Whether a **storage ceiling** applies to an organizer upload at all is PRD 022 §11 Q5,
still open, and this task is the right place to answer it with a real number.

## Acceptance Criteria

- [ ] Current blob store size, free disk and growth rate measured and written down
- [ ] Headroom confirmed for at least one full event's hand-in plus the next two years of monotonic
      growth
- [ ] The backup target confirmed to hold the increased scope, and a restore spot-checked
- [ ] A disk-space alarm or check exists that fires before a batch can fill the volume
- [ ] §11 Q5 answered: whether a storage ceiling applies to admin uploads, and at what number
- [ ] Findings recorded in this task file, and the PRD's §11 Q5 updated when resolved
- [ ] Signed off **before** a photographer is invited to upload
