# 063 — Backup and restore procedure for the blob store, tested once

**Status:** doing
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:**

## Description

PRD 008 §6/§8. Under the no-direct-writes rule, everything in SQL is a projection
and rebuilds from a stream. The **blob store is the exception** — portrait bytes
cannot be reconstructed from events — so it is the entire backup scope.

Projections deliberately need no backup. The schema should make that distinction
obvious, so nobody backs up 40 GB of rebuildable projections and misses the one
directory that matters.

A backup that has never been restored is a hope, not a backup: this task includes
one actual restore.

## Acceptance Criteria

- [ ] Backup procedure documented and scripted for the blob store
- [ ] Explicitly documented that projection tables are NOT in the backup scope,
      and why
- [ ] A restore performed at least once and the result verified
- [ ] Retention aligned with the portrait retention decision (PRDs 003/007 —
      consent and retention for minors' portraits)

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. **Blocked**, on two counts.
- 2026-08-25 — First: the backup *target* is not decided. The blob store landed in
  task 057 behind an interface with a filesystem implementation, but whether
  production uses a mounted volume or object storage is open (PRD 008 §11 Q4), and the
  backup procedure is completely different in each case — `tar` of a volume versus
  bucket replication/versioning. Writing one now would likely be throwaway.
- 2026-08-25 — Second, and firmer: this task's whole point is the criterion "a restore
  performed at least once and the result verified". The **Docker daemon on this
  machine is not responding**, so no volume can be created, populated, backed up or
  restored. A backup script that has never been run is precisely the thing this task
  exists to prevent — marking it done on an untested script would be worse than
  leaving it open, because it would look finished.
- 2026-08-25 — What *is* settled and recorded, so the eventual script is short: the
  backup scope is **only** the blob store. Every SQL table in this service is a
  projection and rebuilds from the event log, so backing them up is wasted effort that
  also risks restoring a stale read model over a good one. Task 057 keeps portrait
  bytes clear of projection tables specifically so this distinction stays true, and
  content addressing makes the restore idempotent (re-putting identical bytes is a
  no-op).
- 2026-08-25 — Note the dependency the criteria already flag: retention must line up
  with the portrait retention decision in PRDs 003/007, which is **blocking on the
  consent question** for photographs of minors. There is no point scripting a
  retention window before anyone has said how long portraits may be kept.
- 2026-08-25 — **Needs from the maintainer:** PRD 008 §11 Q4 (volume or object
  storage), the portrait retention period, and a working Docker daemon (or a
  environment where a restore can actually be rehearsed).
