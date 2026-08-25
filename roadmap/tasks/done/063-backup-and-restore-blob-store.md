# 063 — Backup and restore procedure for the blob store, tested once

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

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

- [x] Backup procedure documented and scripted for the blob store
- [x] Explicitly documented that projection tables are NOT in the backup scope,
      and why
- [x] A restore performed at least once and the result verified
- [ ] Retention aligned with the portrait retention decision (PRDs 003/007 —
      consent and retention for minors' portraits) — **deliberately not
      implemented; see log**

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up, then blocked: the backup target was undecided and no
  restore could be rehearsed.
- 2026-08-25 — **Unblocked by the storage decision: portraits are a bind mount**
  (`/srv/hej/blobs`). That resolved both blockers at once — a bind mount is an
  ordinary host directory, so the procedure is `tar` of a path and, crucially, a real
  restore could be rehearsed **without** a working Docker daemon.
- 2026-08-25 — Wrote `docker/blobs.sh` with three subcommands: `backup`, `restore`,
  `verify`.
- 2026-08-25 — The nicest property fell out of task 057's design rather than being
  engineered here: because the store is **content-addressed, a file's name IS the
  SHA-256 of its contents**, so a restore can be verified against nothing but itself.
  No manifest, no separate checksum list to drift out of sync, and no need to trust
  the backup tooling — a truncated file, a bit flip or a half-finished transfer all
  show up as a name/content mismatch.
- 2026-08-25 — `backup` verifies the **source** first and writes nothing if it fails.
  Archiving already-corrupt data propagates the corruption into every future restore,
  which is the failure mode a backup is supposed to protect against.
- 2026-08-25 — `restore` sets `0700` on the destination, matching what
  `go/internal/blob` enforces when it creates the directory itself. A restore must not
  be the step that widens permissions on photographs of minors.
- 2026-08-25 — ✅ **Restore actually rehearsed end to end**, which is the whole point of
  this task:
  1. populated a store through the real `blob.FileStore` code (3 objects)
  2. `verify` → OK
  3. `backup` → timestamped archive, mode 600
  4. **deleted the store entirely**
  5. `restore` into a fresh path → verify OK, contents byte-identical, dir `0700`
- 2026-08-25 — Then tested that verification is not decoration — a check that always
  passes is worse than none:
  - appended one byte to an object → `CORRUPT`, exit 1, with both hashes reported
  - planted a stray `.tmp-*` file → `WARN` + failure (an unexpected file in a
    content-addressed store means something wrote to it that should not have)
  - ran `backup` against the corrupted store → refused, and left **no** destination
    directory behind (fixed the ordering after noticing the first version created it
    before verifying)
- 2026-08-25 — Backup scope documented in the script header: **only this directory**.
  Every table in the database is a projection that rebuilds from the event log, so
  backing it up costs effort and adds a way to restore a stale read model over a good
  one. Portraits are the sole exception because the event carries a content hash, not
  the image.
- 2026-08-25 — **Retention left unimplemented on purpose**, and this criterion stays
  unchecked rather than being quietly dropped. How long portraits of minors may be
  kept is an open consent question (PRDs 003/007). A script that deletes them before
  that is answered would be worse than no script, so the header says where the prune
  step goes and what has to be decided first.
- 2026-08-25 — Verification caveat: rehearsed on a local directory, not on a deployed
  Swarm bind mount — **Docker daemon not responding**. The mechanism is proven; the
  path and node placement are not.
- 2026-08-25 — Moving to done with one criterion openly unmet (retention), tracked by
  the consent decision rather than by this task.

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
