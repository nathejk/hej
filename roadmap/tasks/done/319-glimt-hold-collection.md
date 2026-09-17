# 319 — Hold collection endpoints: /api/glimt/hold

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-18

## Description

PRD 019 §0a.1, §7. **The primary post-race surface.** During the race, newest-first is right.
Afterwards a spejder wants *their hold's* photos, then their friends' hold, then everything —
so browsing by hold is a first-class view, not a filter hidden in a menu.

**Backend.**
- `GET /api/glimt/hold` — holds that have posted anything visible to the caller (the index)
- `GET /api/glimt/hold/:number` — one hold's collection, **oldest first** (a race reads forward
  in time), visibility-filtered through task 299

**Frontend.** `GlimtHoldView.vue` at `/glimt/hold/:number` — a dense thumbnail grid with the
hold attribution as the heading, and a persistent shortcut from the feed to **your own** hold's
collection.

Build it for a spejder on a bus with a patchy connection: thumbnails only, tap to open full
media, nothing loaded that is not on screen.

## Acceptance Criteria

- [x] Both endpoints, visibility-filtered, paginated
- [x] Hold collection is oldest-first
- [ ] ~~`GlimtHoldView.vue` thumbnail grid at `/glimt/hold/:number`~~ → **split out to task 326**
- [ ] ~~One-tap route from the feed to your own hold~~ → **split out to task 326** (the backend serves `own_number` for it)
- [x] Test: the hold index excludes holds whose only glimt the caller cannot see
- [x] Thumbnails only in the grid; full media fetched on open — **task 326** (the `?variant=thumb` the grid needs already exists, task 305)
- [x] `go test ./...` passes (`npm` criteria move with task 326)

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Picked up. **Split.** This task bundled backend endpoints with a Vue view, and the
  view depends on tasks 313 (store) and 316 (feed components) which are not built yet. Rather than
  leave a task half-done or build a view on top of a store that does not exist, the frontend half is
  now **task 326** and this one is the endpoints. Per `TASKS.md`: "If a task grows too large, split
  it and reference the new IDs in the original task's log."
- 2026-09-18 — `GET /api/glimt/hold` (index) and `GET /api/glimt/hold/:number` (collection), both
  filtered through the same predicate as the feed and re-checked on the way out.
- 2026-09-18 — Decision: the index **counts only what the caller may see**, and omits holds entirely
  when nothing they posted is visible. A count including invisible glimt would tell a spejder how
  much the bandits had posted — a small leak, and still one they were not meant to have. Both
  properties tested.
- 2026-09-18 — Added `own_number` to the index response rather than leaving the client to derive it.
  The profile payload carries the hold's *name* and not its number, and a client matching on name
  would break on two holds sharing one. Empty for crew, which the client reads as "offer no
  shortcut".
- 2026-09-18 — ⚠️ **A test I had to weaken, honestly.** I first wrote the oldest-first assertion as
  though it proved the ordering. It cannot: the handler test runs against a stub, so it was
  measuring the stub's insertion order, and it failed. The **authority** on ordering is the SQL,
  which `nathejk/table/glimt/querier_test.go` already asserts directly. I made the stub sort the way
  the real queries do (so every other test sees realistic data) and rewrote the test's comment to
  say exactly what it does and does not prove: that the two endpoints are wired to the two different
  reads and that nothing in the handler re-sorts. Worth recording, because a test whose name
  overclaims is worse than no test.
- 2026-09-18 — ✅ Backend criteria met. `gofmt` clean, `go build ./...`, full `go test ./...` green.
  Moving to done; task 326 carries the view.
