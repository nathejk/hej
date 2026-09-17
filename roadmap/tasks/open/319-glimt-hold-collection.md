# 319 — Hold collection: /api/glimt/hold and GlimtHoldView

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] Both endpoints, visibility-filtered, paginated
- [ ] Hold collection is oldest-first
- [ ] `GlimtHoldView.vue` thumbnail grid at `/glimt/hold/:number`
- [ ] One-tap route from the feed to your own hold
- [ ] Test: the hold index excludes holds whose only glimt the caller cannot see
- [ ] Thumbnails only in the grid; full media fetched on open
- [ ] `go test ./...`, `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
