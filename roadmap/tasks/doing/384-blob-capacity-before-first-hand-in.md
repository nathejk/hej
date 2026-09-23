# 384 — Confirm blob storage and backup capacity before the first real hand-in

**Status:** doing
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent (code half); **blocked on the maintainer** for the operational half
**Started:** 2026-09-23
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

- [ ] Current blob store size, free disk and growth rate measured and written down — **dev measured, prod
      outstanding**
- [ ] Headroom confirmed for at least one full event's hand-in plus the next two years of monotonic
      growth — **needs the production host**
- [ ] The backup target confirmed to hold the increased scope, and a restore spot-checked — **needs the
      maintainer**
- [x] A disk-space alarm or check exists that fires before a batch can fill the volume
- [x] §11 Q5 answered: whether a storage ceiling applies to admin uploads, and at what number
- [x] Findings recorded in this task file, and the PRD's §11 Q5 updated when resolved
- [ ] Signed off **before** a photographer is invited to upload — **the gate; not mine to close**

## Status: the code half is done, the operational half is not

This task is mostly not a code task, and the PRD says so. What was buildable has been built and verified
live; what remains needs somebody with access to the production host and the backup target. **The task stays
open**, because closing it would assert a sign-off that has not happened — and this is the one task in PRD
022 whose whole point is that the sign-off happens before a photographer is invited to upload.

## §11 Q5, answered: a floor under the volume, not a quota over the feature

The PRD's own recommendation was "a ceiling generous enough never to be hit by a real photograph but present
enough to stop a stuck script". Building it showed the **instrument** was wrong, and the reason is §11 Q2.

`glimtstorage.go`'s byte quota works because retention frees space on a schedule everybody was told about
(PRD 019): a member who hits their allowance has a way through, and the server's total is eventually
reclaimed. §11 Q2 removed exactly that property here — photographs are **never** purged — so the store grows
monotonically and a quota would refuse a *legitimate* hand-in the first year the number was too small. The
refusal would arrive in front of a photographer, mid-batch. That is the failure §8.11 names, produced by the
control meant to prevent it.

A quota is also blind to what actually fills the disk. The volume holds portraits and glimt media too, and in
production it is a bind mount that may share a filesystem with anything: a per-feature quota can sit
comfortably under its limit while `df` reports zero.

So, `ADMIN_DISK_FLOOR_BYTES`:

| Property | Value, and why |
|---|---|
| Default | **2 GiB** — roughly twice one event's hand-in (§6 puts a card at order 1 GB), so the refusal lands with a whole batch's room still on the volume for everything else |
| Measured by | `statfs` on the blob root, via a new optional `blob.FreeSpacer` capability. `Bavail`, not `Bfree`: the difference is blocks only root may use, and this process is not root |
| Checked against | the **result** of the upload, not the current state — otherwise an upload one byte above the floor could write fifty megabytes. Same mistake `checkGlimtStorageCeiling` documents avoiding |
| Refuses with | **507**, and a Danish message that names whose problem it is and says *behold kortet*. Not 429 (nothing the photographer did) and not a retry invitation (the retry fails identically) |
| Warns at | three times the floor, in the log. This is the "check that fires before a batch can fill the volume" criterion — a multiple rather than a second setting, because two numbers needing a sensible relation is two numbers somebody sets inconsistently |
| On failure to measure | **allows the upload.** The asymmetry is the point: wrongly allowing costs disk an operator can see, wrongly refusing costs a hand-in and possibly a wiped card. A floor is a safety margin, not an authorization |
| Defaulted on | unlike the glimt total ceiling, because this needs no adversary — one photographer with a card does it by following instructions |

`FreeSpacer` is a separate interface rather than a widening of `blob.Store`, because "how much room is left"
has no answer for a memory store and a different one for a bucket, and PRD 008 §11 Q4 keeps that choice open.
The interface is declared in the portable file and only its implementation is platform-gated — an interface
that vanished on a platform would take `cmd/api` with it.

## Measurements

**Dev (2026-09-23), which is the only host reachable from here:**

| | |
|---|---|
| Blob store size | 8.1 MB, 67 objects |
| Volume | `/dev/vda1`, 59 GB total, 19 GB available |
| Free bytes as the app sees them | 19,668,021,248 — matching `df`, which is what makes the measurement trustworthy rather than plausible |

At §6's figure of order 1 GB per event, 19 GB is roughly nineteen events' headroom *on this machine*, which
is a development laptop and proves nothing about production. **The number that matters has not been taken.**

## Verified live

Not just in the suite, because a 507 is easy to produce for the wrong reason:

- With `ADMIN_DISK_FLOOR_BYTES=999999999999` (a terabyte, unsatisfiable), an upload answered **507** with the
  Danish refusal, and the log line carried the real numbers:
  `freeBytes=19668021248 incomingBytes=383677 floorBytes=999999999999`.
- With the configured floor, the same file uploaded and stored normally (**200**, `outcome: stored`).

That pair is the evidence: the same request, the same bytes, and only the floor differing.

Broken deliberately and reverted, each failing: the absent-capability path made to fail closed, a failing
`statfs` made to fail closed, and the check made to ignore the incoming size.

`TestTheFileStoreReportsFreeSpace` exists for a specific reason — every other test here runs against a
wrapper, so without it the production store could fail to implement `FreeSpacer` at all, the floor would be
permanently off, and it would be failing open exactly as designed while protecting nothing.

## What remains, and who it needs

These cannot be done from here. They are the reason the task is still open:

1. **Measure the production volume.** `df` on the host holding `${BLOB_DIR}`, plus the current size of that
   directory. The floor makes a full disk loud; it does not make it not happen.
2. **Confirm the trajectory, not just 2026.** §11 Q2 means one event's worth per year, forever. The question
   is whether three years' growth fits, not whether this year's does.
3. **Confirm the backup target holds the larger scope, and restore something.** The blob store is the whole
   backup scope for this service (PRD 008 §8) because everything else replays from the log. A backup nobody
   has restored from is a belief, not a backup — and this data is the one thing here that cannot be rebuilt.
4. **Sign off before inviting a photographer.** The failure this task exists to prevent is a photographer
   uploading a full card onto a disk that fills, with their card already wiped. The 507 turns that into a
   refusal instead of a partial write, which is worth having and is **not** the same as having enough disk.
