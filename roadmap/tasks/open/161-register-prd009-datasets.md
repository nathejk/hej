# 161 — Register the directory as PRD 009 datasets

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

## Description

The contacts directory is PRD 009's most demanding consumer, not its owner (PRD 007 §8).
Register two datasets:

- **person index** — structured, searchable, high priority, survives image eviction;
- **portrait thumbnails** — cache-first binary, high priority, server-issued expiry,
  purged after the event.

Explicitly **not** registered: the patrol lookup. It is live, `no-store`, and stores
nothing (task 157). A generic "make it available offline" registration applied to it
would silently undo the central privacy property of this feature — see task 170.

Sizing numbers this PRD owes 009's global budget (counts from task 078, thumbnail size
measured in task 104): ~151 banditter ≈ 0.7 MB, ~99 gøglere ≈ 0.4 MB, ~20 crew ≈ 0.1 MB,
so under ~1 MB for the largest role. The budget itself and the priority order against
PRD 002's map tiles are 009's to set.

**Blocked on PRD 009 being approved.** If 009 stalls, the fallback is a private sync
path for this feature — the duplication 009 exists to prevent — so raise it rather than
quietly building one.

## Acceptance Criteria

- [ ] Both datasets registered with 009's engine: cache-first, priorities, TTL, purge.
- [ ] The metadata index is independently usable when images are absent or evicted.
- [ ] The patrol lookup is *not* registered, with a comment saying why.
- [ ] Sizing numbers handed to 009's budget.
- [ ] Bulk image sync respects 009's wifi/pre-race restriction; **metadata deltas are
      exempt** and may run during the race on mobile data (PRD 007 §6).

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8.
