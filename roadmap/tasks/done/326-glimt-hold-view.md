# 326 — GlimtHoldView: the hold collection grid

**Status:** done
**Priority:** high
**Created:** 2026-09-18
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] `GlimtHoldView.vue` at `/glimt/hold/:number`, dense thumbnail grid
- [x] Hold attribution (number · name · group) as the heading
- [x] One-tap route from the feed to your own hold, hidden when `own_number` is absent
- [x] Thumbnails only in the grid; full media fetched on open
- [x] Lazy/paginated — a cold open does not fetch every item
      (**image bytes** are lazy; the metadata is one request — see the log)
- [x] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-18 00:00 — Task created, split out of task 319 (backend endpoints, done). The backend
  serves `GET /api/glimt/hold` (index, with `own_number`) and `GET /api/glimt/hold/:number`
  (collection, oldest first, visibility-filtered).
- 2026-09-17 — Implemented. `vue/src/views/GlimtHoldView.vue`, route `glimt-hold` in
  `vue/src/router/index.ts`, and `holds` / `ownHoldNumber` / `holdGlimt` / `loadingHold` /
  `fetchHolds()` / `fetchHold()` added to `vue/src/stores/glimt.store.ts`.

  Decisions:

  - **A grid, not cards.** The feed's card carries attribution, an audience chip and a caption
    because each glimt arrives out of context. Here every tile shares the same attribution — it
    is the heading — so that chrome would be repeated twenty times to say nothing. The caption
    moves into the viewer, which is where somebody who has opened a photograph can read it.
  - **Square tiles**, not the card's shared aspect ratio. A grid's job is to be scannable, and a
    uniform block is what makes twenty photographs readable at a glance.
  - **Tiles are per *media item*, not per glimt** (`entries.flatMap`), so a three-photo glimt is
    three tiles. A grid that hid two of them behind a carousel would defeat the point of the
    surface.
  - **The heading comes from the first glimt's frozen attribution**, falling back to the index
    and only then to a bare `Hold {n}`. That ordering is what makes a *direct load* correct — a
    shared link or a reload, where the index has never been fetched. The final fallback stays
    deliberately vague rather than guessing "Patrulje" for what might be a klan.
  - **Not a `destination`.** It is reached by tapping a shortcut, so it takes no bottom-nav slot
    — which matters, because Glimt itself now occupies one (task 320). Roles are *derived from*
    the glimt destination rather than restated, same as `/contacts/:personId`, so a deep link
    cannot become a second, laxer way into the pane. Item visibility is unaffected either way:
    `users.MaySeeGlimt` on the BFF is the only authority.
  - **Empty is a normal answer, not a fault** — it means nothing this hold posted was shared as
    far as the caller. Said plainly rather than left as a blank page.
  - `GlimtView` guards the shortcut with `router.hasRoute('glimt-hold')`, because
    `router.push({ name })` *throws* on an unregistered name.
  - Video tiles get a play glyph with `alt=""`: a glyph reads at 100px where a word does not, and
    twenty "Glimt fra Patrulje 42" in a row is noise to a screen reader, not information.

  **One criterion interpreted rather than met literally.** "Lazy/paginated — a cold open does not
  fetch every item" is satisfied for the thing that costs anything: every tile is
  `loading="lazy"` at `?variant=thumb`, so a cold open transfers only the images on screen. The
  *metadata* is still a single request for the whole collection. That is a deliberate trade — a
  hold has tens of glimt, not thousands, and the JSON is small next to one thumbnail — but it is
  not what the words say, so: if a hold's collection ever grows past a few hundred items,
  paginating `fetchHold` is the change, and the endpoint (task 319) would need a cursor. The load
  peak this task was written against (PRD 019 §0a.3) is image bytes, and those are handled.

  Re-fetched on every visit rather than served from `holdGlimt` when present: this is the surface
  people revisit while photographs are still arriving, and the images are cached regardless
  (task 315), so being current costs a few kilobytes of metadata.

  Vitest (837 passing), `type-check` and `build` all green. Not exercised on a device.
