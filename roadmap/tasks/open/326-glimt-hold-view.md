# 326 — GlimtHoldView: the hold collection grid

**Status:** open
**Priority:** high
**Created:** 2026-09-18
**Picked up by:**
**Started:**
**Completed:**

## Description

Split out of task 319, whose backend half is done. PRD 019 §0a.1, §7.

**This is the primary post-race surface**, not a secondary view. During the race a member opens Glimt
to see what is happening now; afterwards the question is "show me *our* photos, then our friends'
patrulje, then everything", and a chronological feed answers that badly because a hold's twelve photos
are scattered through a thousand.

Build:

- `GlimtHoldView.vue` at `/glimt/hold/:number` — a dense thumbnail grid, oldest-first (the backend
  already returns that order), with the hold attribution as the heading.
- A persistent, one-tap shortcut from the feed to **your own** hold, using `own_number` from
  `GET /api/glimt/hold`. Absent for crew, who have a section rather than a numbered hold — omit the
  shortcut rather than linking to a collection that cannot exist.
- Optionally the hold index itself, so a member can browse other patruljer.

Design for a spejder on a bus with a patchy connection: **thumbnails only** in the grid
(`?variant=thumb`), tap to open the full item in the viewer (task 318), and nothing loaded that is not
on screen. This is the load peak of the whole feature (PRD 019 §0a.3) — a thousand people at the finish
line on the worst network of the weekend.

## Dependencies

- Task 313 (`glimt.store.ts`) — for fetch/cache
- Task 316 (feed view) — shares `GlimtCard`/grid components and is where the shortcut lives
- Task 318 (viewer) — for opening an item

## Acceptance Criteria

- [ ] `GlimtHoldView.vue` at `/glimt/hold/:number`, dense thumbnail grid
- [ ] Hold attribution (number · name · group) as the heading
- [ ] One-tap route from the feed to your own hold, hidden when `own_number` is absent
- [ ] Thumbnails only in the grid; full media fetched on open
- [ ] Lazy/paginated — a cold open does not fetch every item
- [ ] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-18 00:00 — Task created, split out of task 319 (backend endpoints, done). The backend
  serves `GET /api/glimt/hold` (index, with `own_number`) and `GET /api/glimt/hold/:number`
  (collection, oldest first, visibility-filtered).
