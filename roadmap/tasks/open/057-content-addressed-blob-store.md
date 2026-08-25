# 057 — Content-addressed blob store for portrait bytes

**Status:** open
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 008 §8. Portrait *metadata* travels on the event stream; the *bytes* do not.
They go to a content-addressed store keyed by a hash carried in the event, which
makes replay idempotent (a rebuild re-projects rows without re-uploading bytes).

This blob store is the **only thing in `hej` not rebuildable from a stream**, so
it is the entire backup scope (task 063) and must stay clear of projection
tables — a replay must never be able to truncate portraits.

**Open decision (PRD 008 §11 Q4):** object store (S3-compatible) vs mounted
volume. Implement behind an interface so the choice is swappable, and default to
a filesystem-backed implementation for dev.

## Acceptance Criteria

- [ ] A `BlobStore` interface: put (returning content hash), get, exists, delete
- [ ] Filesystem implementation, content-addressed (hash-derived path layout)
- [ ] An in-memory implementation for tests
- [ ] Wired into `main.go` behind config (path/bucket from env)
- [ ] Put is idempotent: writing identical bytes twice yields one object
- [ ] Unit tests, including idempotency and a missing-object read
- [ ] Documented that the production choice is open, and where it is decided

## Progress Log

- 2026-08-25 — Task created from PRD 008.
