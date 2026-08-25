# 057 — Content-addressed blob store for portrait bytes

**Status:** done
**Priority:** medium
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

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

- [x] A `BlobStore` interface: put (returning content hash), get, exists, delete
- [x] Filesystem implementation, content-addressed (hash-derived path layout)
- [x] An in-memory implementation for tests
- [x] Wired into `main.go` behind config (path/bucket from env)
- [x] Put is idempotent: writing identical bytes twice yields one object
- [x] Unit tests, including idempotency and a missing-object read
- [x] Documented that the production choice is open, and where it is decided

## Progress Log

- 2026-08-25 — Task created from PRD 008.
- 2026-08-25 — Picked up. New package `internal/blob`: `Store` interface, `FileStore`,
  `MemoryStore`, `BLOB_PATH` config, wired in `main`.
- 2026-08-25 — Made `Ref` a **distinct type**, not a string. A Ref travels through
  event bodies and database rows, and being able to confuse it with a filename, a
  user id or a URL path is exactly how a content-addressed store stops being one.
- 2026-08-25 — Security decision: `Ref.Valid()` rejects anything that is not 64 hex
  characters, and `FileStore` checks it **before touching the filesystem**. A Ref
  arrives from an event body or a URL, so it is untrusted input and `"../../etc"` is a
  Ref-shaped string. Rejecting rather than sanitising, because every Ref should have
  come from `ComputeRef` and anything else is a bug or an attack — both deserve to
  fail loudly. Covered by a test.
- 2026-08-25 — Writes go to a temp file in the destination directory and are then
  renamed. Rename within a directory is atomic, so a reader can never see a
  partially written object — which for a content-addressed store is worse than a
  missing one, because the ref would name bytes that do not hash to it.
- 2026-08-25 — Permissions are `0700`/`0600`, not the usual `0755`/`0644`. These are
  photographs of identifiable minors; a world-readable directory on a shared volume
  is the kind of default nobody notices until it matters. Asserted in a test so it
  cannot regress silently.
- 2026-08-25 — Objects fan out on the first two hex characters (256 buckets). Not
  premature: a flat directory of a few thousand portraits is awkward to list and slow
  to look up on some filesystems.
- 2026-08-25 — `MemoryStore.Put` copies the input. A caller may reuse its buffer, and
  a content-addressed store whose contents mutate out from under their own hash is
  not one.
- 2026-08-25 — Dev compose: `BLOB_PATH=/blobs` backed by a **named volume**, not a
  bind mount — this is the one store that cannot be rebuilt from the event log, so it
  should not be wiped casually along with a working tree.
- 2026-08-25 — An unset or unusable `BLOB_PATH` falls back to memory and **says so at
  warn level**. Nothing writes blobs yet (PRD 003 starts that), so refusing to boot
  would trade a working API for a feature that does not exist — but an in-memory blob
  store is a real limitation, not a convenience, so it must not look configured.
- 2026-08-25 — Production choice (object store vs mounted volume) left open and
  pointed at PRD 008 §11 Q4, which now owns it. The interface is narrow precisely so
  that stays a wiring change.
- 2026-08-25 — ✅ All criteria complete. 8 tests, table-driven across **both**
  implementations so the contract is verified once: round-trip, idempotency,
  ErrNotFound, idempotent delete, traversal rejection, permissions, no leaked temp
  files, and concurrent identical puts. Race detector clean.
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so the `/blobs`
  volume mount was validated by parsing the compose file, not by running it.
- 2026-08-25 — Moving to done.

### Verified against real infrastructure (2026-08-25, later)

- 2026-08-25 — Docker became available, so the caveat above was closed — and it found
  a **real bug**. `/blobs` in the running container was mode **0755, root-owned**, not
  the 0700 this task claimed to enforce.
- 2026-08-25 — Cause: `os.MkdirAll(root, 0o700)` only applies its mode when it
  *creates* the directory. A Docker named volume or bind mount pre-creates the mount
  point, so `MkdirAll` was a no-op and left the runtime's default 0755. Every portrait
  would have been readable by any process or user able to reach the volume, despite
  the correct 0600 on each file — and the unit test passed throughout, because it
  always created the directory itself.
- 2026-08-25 — Fixed by *enforcing* the mode with an explicit `os.Chmod` after
  `MkdirAll`, and by **failing** rather than warning if it cannot be applied: if the
  directory cannot be made private, the right outcome is not to store minors'
  photographs in it. `main` treats that as "blob store unavailable" and falls back to
  memory, which loses persistence but exposes nothing.
- 2026-08-25 — Added `TestFileStoreTightensAnExistingWorldReadableRoot`, which
  pre-creates the root at 0755 and asserts the premise before checking the fix — so it
  cannot pass for the wrong reason. This is the case a volume mount actually produces
  and the one the original test missed.
- 2026-08-25 — Re-verified in the container: `stat` now reports `700 root /blobs`.
- 2026-08-25 — Worth stating plainly: a unit test that constructs its own fixture
  cannot see this class of bug. It needed the real deployment shape.
