# 063 — Backup and restore procedure for the blob store, tested once

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
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
