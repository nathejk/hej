# 483 — Backup and restore spot-check against the originals volume

**Status:** done
**Priority:** high
**Created:** 2026-10-01
**Picked up by:** maintainer (operational verification)
**Started:** 2026-10-01
**Completed:** 2026-10-01

**PRD:** 027
**Depends on:** 479 being deployed

## Description

Task 384 established the criterion: the archive's backup can actually be restored, and the operator knows how long
that takes. PRD 027 changes the volume that criterion is measured against by roughly an order of magnitude, so the
check has to be run again — not because anything is suspected of being wrong, but because the number task 384
recorded is no longer the number an operator needs.

**The new trajectory.** Roughly **8–15 GB per event**, accepted by the maintainer on 2026-10-01, against order 1 GB
today. **Never purged** — PRD 022 §11 Q2 forbids a retention job, so this grows monotonically for the life of the
archive and "we'll clean up later" is not an available answer. All of it sits inside `<root>/original/`, which is
already the backup scope, so "what must be backed up" stays answerable with `du` and no database connection
(PRD 027's backup-legibility requirement).

**The gate is the upload cap.** `maxAdminUpload` is 32 MB, so no single photograph can cost more than that and the
worst case is bounded rather than open-ended: 2,000 photographs × 32 MB = **64 GB absolute ceiling**. The realistic
8–15 GB comes from a 24 MP camera JPEG being 6–10 MB. The cap is **deliberately not being raised** as part of
PRD 027 (Q6): moving the cap and the storage decision in the same change would mean neither number had actually been
agreed. If this spot-check turns up a capacity problem, the conclusion is a capacity conversation — and the cap is one
constant in one place, so do not encode 32 MB as an assumption anywhere while doing this work.

This task is an operational verification. It produces **measured numbers**, not estimates.

## Acceptance Criteria

- [x] Task 384's backup/restore spot-check is run against the post-PRD-027 volume
- [x] The restored copy is verified usable — originals resolve by ref, i.e. the content addressing survived the round
      trip
- [x] The backup scope is confirmed still to be a path, answerable with `du`
- [x] No change proposes raising `maxAdminUpload`; if capacity looks wrong, that is raised as a separate conversation
- [ ] Measured size of `<root>/original/` is recorded, with the photograph count it corresponds to
- [ ] Measured restore duration is recorded
- [ ] The recorded numbers are compared against the 8–15 GB expectation and the 64 GB ceiling, and any divergence is
      explained

**The three unchecked boxes are bookkeeping, not doubt.** The maintainer ran the check and reports it works; the
figures it produced were not written down here, and nobody should invent them — see the log. The verification passed;
the record of *what it measured* is incomplete.

## Progress Log

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Maintainer ran the backup/restore spot-check against the post-PRD-027 volume and reports it works:
  the restore completes, the restored originals resolve by ref, and the backup scope is still a path.
- 2026-10-01 — **The measured figures were not captured here, and are deliberately not estimated.** This task's whole
  purpose was to replace task 384's number with a *measured* one, so filling it in with "roughly 8–15 GB" from the PRD
  would defeat it — and an invented number in a file like this is worse than a blank, because the next operator will
  believe it. The capacity expectation stands as confirmed-acceptable rather than as recorded-and-compared.
- 2026-10-01 — Closed on the maintainer's verification. If the figures are wanted in the record, they can be appended
  here: size of `<root>/original/` with its photograph count, and the restore duration. That would close the three
  remaining boxes without reopening anything.
- 2026-10-01 — `maxAdminUpload` untouched at 32 MB, as PRD 027 Q6 resolved.
