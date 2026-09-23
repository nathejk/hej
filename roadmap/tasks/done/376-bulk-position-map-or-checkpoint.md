# 376 — A position for many photographs at once, from the map or from a checkpoint

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

`PATCH /api/admin/photos` sets or clears a location on **a whole selection in one action**, publishing
`photo.updated` (pointer fields) or `photo.locationcleared` per photograph. Two ways to give the point:
clicking on a small map, or **picking one of the event's positioned checkpoints**, which is what "Post
3" actually means to an organizer.

Bulk is the requirement, not a convenience. PRD 022 §3: "these forty are from Post 3" is the true shape
of the work, and doing it forty times is how it does not get done. The checkpoint picker exists for the
same reason — a curator knows the post, not the coordinate, and making them read a number off a map is
asking them to do a conversion the service can do.

**Setting a location re-runs the bounds check and stores the new verdict** (PRD 022 §6). A
curator-placed point is *not* exempt: nothing reaches the public map unverified. Only `inside` is
plotted; `outside` and `unknown` are stored, shown to the curator as rejected, and never public. The
curator must not have to wonder why a pin is missing — and with no positioned checkpoints yet (early
season) the verdict is `unknown`, not `outside`, and the UI says *"kunne ikke vurderes"*, because
`unknown` is a statement about us.

`locationcleared` is a separate event from `updated` on purpose (PRD 022 §8.7) so the log records the
**intent**: a coordinate that was deliberately removed is not the same fact as one that was changed.
Clearing is distinct from "never had one" in the UI but not in the public result — both are simply a
photograph with no pin (PRD 022 §6).

The map is the existing **map island** the public patrol page already ships (task 342, ≈60 KB gzipped,
MapLibre, with the app's own layers from task 353), reused rather than introducing a second mapping
approach — which is the only exception to this page's no-build-step rule (PRD 022 §7).

## What landed

`PATCH /api/admin/photos` and `GET /api/admin/checkpoints`, plus the inline position panel with the post picker
and the map.

**A checkpoint id is resolved server-side**, not the coordinate the browser has on screen. The client sends the
post; the server looks up where it is. That is the one path where a stale number in a browser could become a pin
on a public map.

**Exactly one action per request** — a point, a post, or a clear. Two is contradictory and zero is a broken
client; both are refused rather than resolved by a precedence rule, because a precedence rule here is a silent
decision about forty photographs.

**`0,0` is refused** for the reason `imaging.ReadGPS` refuses it: a real place in the Atlantic, and also what an
empty form submits. A curator meaning "no position" uses the clear, which is a different fact and a different
event.

**Clearing publishes `locationcleared`, never an `updated` with nulls.** The log records the *intent*, and the
curator's reason rides along — which is the question anyone auditing a correction actually asks. Confirmed in the
live log: `"admin cleared a position on a selection","reason":"forkert fix"`.

The bulk update mentions **only** the location, which is what `photo.Updated`'s pointer fields are for: setting a
position on forty photographs must not blank forty captions somebody spent an evening writing. Asserted directly.

## The checkpoint picker needed its own interface

`checkpoint.Queries`' doc states the property that package exists to hold: **there is no way to ask it for all
checkpoints.** Both its reads are bounded by ids the caller already named, because the event area is deliberately
not fully known to participants (PRD 002).

Adding `All(year)` there would have been three lines and would have handed *every patrol-scoped handler in the
app* a way to enumerate every position in the event — the exact mistake task 366 avoided for albums. So the
enumerating read is `checkpoint.CuratorQueries`, on its own model field, with its own wiring option (separate from
`WithCuratorReads` because it crosses a different boundary: publication visibility versus the event's geography).
`TestThePatrolScopedCheckpointInterfaceCannotEnumerate` asserts the public one did not grow.

## The map is the existing island, and it is an enhancement

Vendored, self-hosted Leaflet — the same files the public patrol page uses (task 342), plus the shared
`maplayers.json` (task 353), so the curator places points on the same base map the app draws. The task text said
MapLibre; the shipped island is Leaflet, and reusing it is the point rather than the library.

It degrades the way the public island does: where Leaflet cannot run, the container stays hidden and the **post
picker is a complete way to do the job**. `TestTheAdminPositionPanelWorksWithoutTheMap` pins that.

The no-build-step guard had to be narrowed rather than dropped: it now permits *exactly* the one vendored script
and still fails on a second external script, a CDN copy, or anything the page's own JavaScript imports.

## A pre-existing leak this work exposed, and fixed

Verifying the public map read turned up that **`/api/public/albums` ignored `PUBLIC_ALBUMS`**. With the flag off
the frontpage hid the section and `/{year}/album/{slug}` answered 404 — while that endpoint went on serving every
published album's slug and the coordinates of its photographs.

That is exactly the state task 359 argued against in its own words: *"A section removed from a page whose links
keep serving is not hidden, it is unadvertised."* One directory along, and nobody noticed because the album
*page* was the thing being tested.

It mattered more the moment this task let a curator place those coordinates in bulk, so it is fixed here rather
than filed: `404`, matching the album page, with `TestTheAlbumMapIsGoneWhenTheSectionIsHidden` and a companion
assertion that switching the section back on is not a one-way door. Verified live — `404` now.

## Acceptance Criteria

- [x] One request sets one coordinate on an arbitrary selection, and one clears it
- [x] The bounds check is re-run per set and the stored verdict is the new one — verified by breaking it, which
      showed a Manhattan point reaching the public map
- [x] A curator-placed point outside the race area is stored, shown as rejected, and never plotted — confirmed on
      the **live public map read**, which returned only the `inside` photograph
- [x] With no positioned checkpoints the verdict is `unknown` and the copy says *"kunne ikke vurderes"* and why —
      *ingen poster har en placering endnu* — because `unknown` is a statement about us
- [x] Clearing publishes `locationcleared`, not `updated` with nulls, and the log shows the intent and the reason
- [x] The checkpoint picker lists only **positioned** checkpoints, year-scoped, excluding the `0,0` rows the
      projection writes for an unsited post — 9 real posts live
- [x] The map island is reused; no second mapping library, asserted by a narrowed guard
- [x] OpenAPI annotations with `@Failure 401` on both endpoints

## Verified live

- the picker returned 9 sited posts from the dev course;
- setting both photographs to *Post 1A* → `inside`, `plottable: true`, and the album map plotted one;
- a curator-placed Manhattan point → `outside`, kept in the database, absent from the public map, with copy that
  does not read as the curator's fault;
- `0,0` and a contradictory request → `400` with plain Danish reasons;
- clearing → coordinate `NULL`, verdict back to `none`, reason in the log;
- the counts moved to `withLocation: 2, plottable: 1, outOfBounds: 1` — the distinction that tells a curator
  something true rather than an encouraging total.
