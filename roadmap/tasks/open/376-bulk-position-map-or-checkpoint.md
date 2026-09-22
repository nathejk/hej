# 376 — A position for many photographs at once, from the map or from a checkpoint

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

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

## Acceptance Criteria

- [ ] One request sets one coordinate on an arbitrary selection, and one clears it
- [ ] The bounds check is re-run per photograph on every set, and the stored verdict is the new one
- [ ] A curator-placed point outside the race area is stored, shown as rejected, and never plotted —
      tested on the public map read
- [ ] With no positioned checkpoints the verdict is `unknown` and the UI says "kunne ikke vurderes"
- [ ] Clearing publishes `locationcleared`, not `updated` with nulls, and the log shows the intent
- [ ] The checkpoint picker lists only the event's **positioned** checkpoints, year-scoped
- [ ] The map island is reused; no second mapping library is added
- [ ] OpenAPI annotations with `@Failure 401`
