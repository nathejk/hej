# PRD 016 — Map handouts, visible checkpoints, and the scan drawer

**Status:** draft
**Author:** agent session (Zed / Claude)
**Created:** 2026-09-15
**Last updated:** 2026-09-15
**Approved:**
**Shipped:**
**Target users:** participant (patrol member), organizer (indirectly — as the source of handout and checkpoint data)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Give a patrol the navigational context it already has on paper, in the app: the
list of **map sheets handed out** to it, the **checkpoints it is allowed to see**
plotted on the event map, **edge arrows** pointing towards the next checkpoints
when they are off-screen, and a **drawer** listing every registration the patrol
has accumulated — with checkpoint scans emphasised and marked as reached on
time or late.

## 2. Problem & Motivation

- **What problem does this solve?**
  - **Handouts are untracked.** Patrols are handed physical map sheets (and
    receive more at posts as the route unfolds). Nobody — not the patrol, not
    the organizers — has a record of which sheets a given patrol holds. When a
    patrol phones in lost, the first question ("which map are you looking at?")
    has no reliable answer, and a patrol that was skipped a sheet only finds out
    when it is already off the edge of what it can see.
  - **The map shows nothing to navigate by.** PRD 002 shipped own position plus
    the patrol's own scan history, and deliberately withholds checkpoint
    positions (`checkpoint.Queries` exposes only the hull, by design). But some
    checkpoints are *not* secret: the checkgroup carries an explicit
    `ShowOnMap` flag on the stream, and those are already printed on the paper
    maps the patrol carries. Withholding them in the app buys no secrecy and
    costs the patrol the one thing the app is better at than paper — knowing
    where it is relative to where it is going.
  - **The scan history is a flat list.** Registrations are shown newest-first
    with no distinction beyond kind, and nothing tells a patrol whether it hit a
    post inside its time window. "Are we behind?" is the question patrols ask
    all night, and the app holds the data to answer it but does not.

- **Why now?** The checkpoint projection, the race area, the offline data layer
  (PRD 009) and the map itself are all in place. `ShowOnMap`,
  `NathejkCheckpointsSorted` and the checkpoint time ranges already exist on the
  stream and are simply not projected yet, so the marginal cost of this feature
  is a wider projection plus frontend work — not new upstream domain events,
  with the one exception of map handouts (§11).

- **Evidence.** PRD 002 §11 recorded "which checkpoints may participants see?"
  as deferred rather than settled. The `ScanList` drawer already exists in
  `vue/src/components/map/ScanList.vue` but is a plain chronological list; the
  emphasis and on-time information this PRD asks for is what makes it worth
  keeping open.

## 3. Goals

- A patrol can see, offline, which map sheets it has been handed and when.
- A patrol can see on the map every checkpoint it is permitted to see, and
  nothing more.
- A patrol on the move can tell which direction and how far the next
  checkpoint(s) are without panning the map.
- A patrol can review its registrations and tell at a glance which were
  checkpoints and whether each was reached inside its time window.
- Checkpoint positions that are *not* flagged visible never leave the BFF.

## 4. Non-Goals

- **Not** exposing hidden checkpoints, in any form — not blurred, not as a
  count, not as a bearing. The hull remains the only aggregate that leaves the
  server for non-visible posts.
- **Not** turn-by-turn routing, route lines between checkpoints, or distance
  along a path. Arrows are straight-line bearing and great-circle distance.
- **Not** an organizer tool for recording handouts. This PRD consumes handout
  data; producing it belongs to the scanner/organizer app (§11).
- **Not** downloadable/printable PDF map sheets in the app. "Map handed out" is
  a record of a physical sheet, not a file.
- **Not** scoring, penalties, or standings. "On time" here is informational for
  the patrol, not an authoritative result.
- **Not** changing the offline tile cache scope (still the race area, PRD 002).

## 5. User Stories & Scenarios

- As a **patrol member**, I want to see which map sheets we were handed, so we
  know whether we are missing one before we need it.
- As a **patrol member**, I want the checkpoints we are allowed to know about
  drawn on the map, so I can relate our position to them.
- As a **patrol member**, I want an arrow at the edge of the screen pointing at
  the next checkpoint with its distance, so I can keep walking in the right
  direction while zoomed in on our own position.
- As a **patrol member**, I want to open the registration drawer and see our
  checkpoint scans stand out with a clear "på tid" / "for sent" marker, so we
  can decide whether to press on or rest.

**Happy path.** A patrol opens Kort. Their position is centred (PRD 002). Two
visible checkpoints are within view as flag markers; a third is off-screen, so a
chevron sits on the screen edge in its direction labelled "3,4 km". They tap the
handle at the bottom: the drawer lists "Post 4 · på tid · lør 02:14", "Bandit
taget · lør 01:02", "Post 3 · 12 min for sent · fre 22:40", and a "Kort udleveret"
section showing sheets 1512 II SV and 1512 II NV. Tapping a row pans the map to
that registration and closes the drawer.

**Edge cases.**

- **No patrol** (personnel roles): no handouts, no arrows, no drawer handle —
  exactly as today, where an empty scan list hides the UI.
- **No visible checkpoints yet** (early in the year, or none flagged): no
  markers, no arrows, drawer still works. Never an error state.
- **Visible checkpoint without a position:** listed nowhere on the map, and it
  produces no arrow. Not an error.
- **No own position** (permission denied, or indoors): markers still draw;
  arrows do not, because a bearing needs an origin. The existing permission card
  is the explanation already on screen.
- **Checkpoint with no time window:** shown without an on-time verdict rather
  than with a guessed one.
- **Relative time windows** (`RelativeTimeDuration`) that depend on when the
  patrol started: if the anchor is unknown to the BFF, no verdict is rendered.
  A wrong "for sent" is worse than a missing one.
- **Offline:** everything in this PRD reads from the cached client copy. Arrows
  keep working, because bearing is computed on device from cached positions plus
  the live GPS fix.
- **Clock skew:** verdicts are computed **server-side**, so a device with a bad
  clock cannot invent lateness.

## 6. Requirements

### Functional

**Map handouts**

- [ ] The BFF exposes the map sheets handed out to the signed-in user's patrol:
      an identifier/name per sheet, and when it was handed out.
- [ ] The frontend lists them; empty is a normal, silent state.
- [ ] Handouts are part of the offline cached payload (PRD 009), so they survive
      loss of signal.

**Visible checkpoints**

- [ ] The BFF exposes only checkpoints whose checkgroup has `ShowOnMap` true,
      that are not deleted, and that have a position.
- [ ] Each carries: id, name, position, sequence/order, and its time window when
      one is known.
- [ ] The frontend plots them on the event map, visually distinct from the
      patrol's own scan markers.
- [ ] A marker tap shows the name and, when known, the time window.
- [ ] Markers respect the current base layer and remain legible on both topo and
      aerial (PRD 002).

**Next-checkpoint arrows**

- [ ] When a visible checkpoint that is *next* for the patrol lies outside the
      viewport, an arrow is drawn at the viewport edge in its direction, with
      distance.
- [ ] "Next" = the earliest not-yet-scanned visible checkpoints in sequence
      order; at most **3** arrows on screen at once, to keep the viewport
      readable.
- [ ] Arrows update as the map is panned/zoomed and as the patrol's position
      changes; an arrow disappears when its checkpoint enters the viewport.
- [ ] Tapping an arrow pans the map to that checkpoint.
- [ ] No own position ⇒ no arrows.

**Scan drawer**

- [ ] The drawer lists **all** registrations, newest first, as today.
- [ ] Checkpoint scans are emphasised relative to other registrations.
- [ ] Each checkpoint scan shows an on-time verdict — on time / late (with how
      late) / early, or nothing when no window is known.
- [ ] The verdict comes from the BFF, not the client.
- [ ] Handed-out map sheets appear in the drawer as their own section.
- [ ] Tapping a positioned row pans the map (existing behaviour, preserved).

### Non-Functional

- **Privacy / secrecy.** The BFF must *project out* non-visible checkpoints
  rather than send-and-hide. Following `checkpoint.Queries`' existing precedent,
  the read interface handed to handlers must not be able to return a hidden
  checkpoint's position at all, so no call site can leak one by accident. A test
  must assert that a checkpoint in a non-`ShowOnMap` checkgroup never appears in
  the response.
- **No guardian data.** Nothing in this feature touches `phoneParent` (repo
  rule); no contact data is added to map or drawer payloads.
- **Offline-first.** All four surfaces read from the cached client store and
  degrade to "no data yet" rather than an error (PRD 009).
- **Performance.** Arrow bearing/distance recomputation runs on map move and
  position update; it must stay cheap enough not to stutter panning on the
  baseline device (iOS Safari 16.4). Expect tens of checkpoints, not thousands.
- **Battery.** No new geolocation subscriptions: arrows consume the existing
  watch, which is already stopped when the page is hidden (PRD 002).
- **Accessibility.** Arrows are decorative graphics *plus* an accessible label
  ("Post 5, 3,4 km mod nordøst"); the drawer remains the non-visual route to the
  same information. Touch targets ≥ 44 px.
- **i18n.** Danish copy throughout; distances metric, `da-DK` formatting.
- **Browser baseline.** iOS/iPadOS Safari 16.4+, Chrome 111+. No polyfills.

## 7. UX / UI Notes

**Map (`vue/src/views/MapsView.vue`)**

- Visible checkpoints render as flag markers, in a distinct colour from the
  patrol's own scan markers. Scanned visible checkpoints read as "done" (muted /
  check) so the map doubles as progress.
- Edge arrows are chevrons pinned to the viewport edge, each with a short
  distance label. They sit in the same `z-10` overlay layer as the existing
  controls and must not collide with the top-right control stack, the top-left
  notices, or the bottom handle — the safe region is the vertical middle band of
  the edges.
- The existing bottom-centre handle stays, relabelled to cover both
  registrations and handouts, and is now shown when the patrol has *either*
  registrations *or* handouts (today: only registrations).

**Drawer (`vue/src/components/map/ScanList.vue`)**

- Keeps the shadcn-vue `Drawer` primitive it already uses. Two sections:
  "Registreringer" and "Kort udleveret".
- Checkpoint rows get the emphasis: stronger icon treatment and a verdict badge
  ("på tid" green, "for sent" amber with the delta). Bandit catches keep their
  current red skull styling.
- Any new heading uses `font-nathejk` only if it is title-level; row text stays
  on the system sans stack.
- Icons from Lucide only (`Flag`, `Skull`, `MapPinOff` already in use; add e.g.
  `Map`, `Check`, `Clock`, `ChevronUp`).

New components expected: `vue/src/components/map/CheckpointMarkers.vue` (or a
layer inside `EventMap.vue`), `vue/src/components/map/EdgeArrows.vue`. No new
routes.

## 8. Technical Considerations

- **Frontend (Vue 3 / TS):**
  - New store `checkpoints.store.ts` (visible checkpoints) and handouts — either
    its own store or an extension of `scans.store.ts`, depending on whether the
    BFF ships one endpoint or several (§11).
  - `EventMap.vue` gains a checkpoints layer and exposes map bounds/centre so
    the arrow overlay can compute what is off-screen.
  - `EdgeArrows.vue` computes bearing + great-circle distance from
    `location.position` to each next checkpoint, projects it onto the viewport
    edge, and re-renders on Leaflet `move`/`zoom` and position change.
  - `ScanList.vue` gains sections, emphasis and verdict badges.
  - Cache new payloads through the existing offline data layer (PRD 009).

- **BFF (Go):**
  - `go/nathejk/table/checkpoint`: project the fields already on the stream but
    currently discarded — name is stored, but `showOnMap` (from
    `NathejkCheckgroupUpdated`, so the projection must now also consume
    checkgroups and the checkpoint→checkgroup link from
    `NathejkCheckpointCreated`), sort order (`NathejkCheckpointsSorted`), and the
    time window (`FixedTimeRange` / `RelativeTimeDuration`) are not.
  - Extend `checkpoint.Queries` with a `VisibleCheckpoints(year)` method whose
    return type contains only publishable checkpoints. Keep `RaceArea` as-is.
    The interface stays the security boundary.
  - `go/internal/scans`: the on-time verdict is computed here (or in a small
    package beside it) by joining a scan to its checkpoint's window. The mock
    source must be able to produce all verdict states for dev/simulation
    (PRD 014).
  - New handout source (`go/internal/handouts`, likely) behind an interface with
    a seeded mock first, mirroring how `internal/scans` was introduced.
  - Wire new projections/sources in `go/cmd/api/app` + `routes.go`, using the
    `…OrNil` adapter pattern (see `raceAreasOrNil`) so "no projection" stays
    checkable.

- **API endpoints** (all behind `requireAuth`, all needing **OpenAPI
  annotations** — repo rule):
  - `GET /api/checkpoints` — visible checkpoints only: id, name, position,
    sequence, optional time window. Empty list (200) when none.
  - `GET /api/patrol/handouts` — map sheets handed out to the caller's patrol.
    Empty list (200) for users without a patrol, matching `/api/patrol/scans`.
  - `GET /api/patrol/scans` — **changed**: each checkpoint scan gains an optional
    verdict (e.g. `on_time`, `delta_seconds`, `checkpoint_id`). Additive and
    backwards compatible; the existing client ignores unknown fields.
  - Consider folding checkpoints + handouts into one map-bootstrap response if
    the offline layer prefers a single cacheable document (§11).

- **Data / storage:**
  - `checkpoint` table gains `checkgroupId`, `showOnMap`, `sortOrder`, and
    window columns; new `checkgroup` table or a denormalised flag folded in at
    consume time. Projections are rebuilt by stream replay on boot, so all
    statements stay idempotent upserts (existing convention).
  - Handouts need a source of truth. If the stream has no handout event yet,
    that is an upstream domain change and the largest unknown in this PRD (§11).

- **Dependencies & risks:**
  - **Secrecy regression is the top risk.** Widening the checkpoint projection
    puts positions of *hidden* posts one struct field away from a response.
    Mitigation: a publishable-only return type, plus an explicit test.
  - `ShowOnMap` semantics are per **checkgroup**, so a mis-set flag reveals a
    whole group at once. Worth a log line reporting how many checkpoints are
    visible at boot, in the spirit of `ReportPositionless`.
  - Relative time windows may not be resolvable without a per-patrol start time
    the BFF does not have; the verdict must be omittable per scan.
  - Leaflet is already lazy-loaded; the arrow overlay must not pull Leaflet into
    the app shell bundle.

## 9. Success Metrics

- A patrol can answer "which map sheet are you on?" from the app — handouts are
  present for ≥ 95 % of patrols during the event (measured as patrols with ≥ 1
  handout record).
- Zero incidents of a non-`ShowOnMap` checkpoint position appearing in any API
  response (asserted by test, verified by a post-event log review).
- Drawer opens per patrol per event ≥ 3 (the drawer is worth reintroducing only
  if it is used).
- Qualitative: fewer "we don't know where we are" calls to the organizers than
  the previous event.

## 10. Rollout / Task Breakdown

Sequenced so the secrecy-sensitive backend work lands and is tested before any
of it is rendered, and so handouts — the piece with an unresolved upstream
dependency — cannot block the map work.

**Phase 1 — projection and contract**
- [ ] Task: project checkgroup `showOnMap` and the checkpoint→checkgroup link
- [ ] Task: project checkpoint sort order and time windows
- [ ] Task: add `VisibleCheckpoints` to `checkpoint.Queries` with a
      publishable-only return type
- [ ] Task: `GET /api/checkpoints` handler + OpenAPI annotations
- [ ] Task: assert hidden checkpoints never leave the BFF (regression test)

**Phase 2 — map rendering**
- [ ] Task: visible-checkpoint markers on `EventMap.vue`
- [ ] Task: `checkpoints.store.ts` with offline caching
- [ ] Task: edge arrows for the next checkpoints (bearing, distance, tap-to-pan)
- [ ] Task: arrow overlay collision rules against existing map controls

**Phase 3 — on-time verdicts and drawer**
- [ ] Task: compute on-time verdict server-side and extend `/api/patrol/scans`
- [ ] Task: emphasise checkpoint scans and render verdict badges in `ScanList.vue`
- [ ] Task: dev-simulation fixtures covering every verdict state (PRD 014)

**Phase 4 — handouts**
- [ ] Task: decide and document the handout source of truth (blocked on §11)
- [ ] Task: `internal/handouts` source + mock
- [ ] Task: `GET /api/patrol/handouts` + OpenAPI annotations
- [ ] Task: "Kort udleveret" section in the drawer, offline-cached

No feature flag is proposed for phases 1–3; phase 4 ships behind "empty list
hides the section", which is its own soft launch.

## 11. Open Questions

1. **Where do map handouts come from?** There is no handout event or field
   anywhere on the stream today (`shared-go/messages` has none). Options: a new
   upstream domain event emitted when a post hands out a sheet; a manual
   organizer entry; or the patrol self-declaring in the PWA. This determines
   whether phase 4 is a projection or a write path, and it is the one thing that
   could make this PRD span two repos.
2. **What identifies a map sheet?** A Kort25 sheet code (e.g. "1512 II SV"), an
   internal etape/leg number, or a free-text name? Affects the payload and
   whether the frontend can render anything meaningful beyond a label.
3. **Is "visible" exactly `ShowOnMap` on the checkgroup?** Or does the product
   want a per-checkpoint override, and/or time-gated visibility (a checkpoint
   becomes visible only once the patrol has reached the previous one)? Time
   gating changes this from a static list into per-patrol filtering.
4. **What is "next" when the route is not linear?** `NathejkCheckpointsSorted`
   gives an order, but do all patrols follow the same sequence? If sequences are
   per-team or per-etape, arrows need a per-patrol route rather than a global
   sort.
5. **Can the BFF resolve relative time windows?** `RelativeTimeDuration` needs a
   per-patrol anchor (start time / previous checkpoint scan). Is that anchor
   available to this service, or must those checkpoints ship without a verdict?
6. **How late is "for sent"?** Is there a grace period (the `Minus`/`Plus`
   fields on control groups suggest one exists upstream), and should the app
   show the delta or only the verdict?
7. **One endpoint or three?** The offline layer may prefer a single cacheable
   map-bootstrap document over `/api/checkpoints` +
   `/api/patrol/handouts` + `/api/patrol/scans`.
8. **Should scanned visible checkpoints stay on the map?** Rendering them as
   "done" makes the map a progress view; hiding them reduces clutter.
9. **Was the drawer ever actually removed?** `ScanList.vue` is still wired into
   `MapsView.vue` on `main`, so "reintroduce" may mean "upgrade the existing
   drawer" (assumed here) rather than restore something deleted. Worth
   confirming so phase 3 is scoped correctly.
