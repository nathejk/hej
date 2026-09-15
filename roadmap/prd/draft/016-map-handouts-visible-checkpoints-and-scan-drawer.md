# PRD 016 — Map handouts, visible checkpoints, and the scan drawer

**Status:** draft
**Author:** agent session (Zed / Claude)
**Created:** 2026-09-15
**Last updated:** 2026-09-15
**Approved:**
**Shipped:**
**Target users:** participant (patrol member)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

Give a patrol, in the app, the navigational context it already holds on paper:
the **map sheets it has been handed**, the **checkpoints those sheets have
revealed** plotted on the event map, **edge arrows** pointing towards the next
checkpoints when they are off-screen, and a **drawer** listing every
registration — with checkpoint scans emphasised and marked as reached on time or
late.

The reveal rule is the heart of it: a patrol may see exactly the checkpoints that
are drawn on the sheets it holds, plus the checkgroups it has already reached.
Nothing else.

## 2. Problem & Motivation

- **What problem does this solve?**
  - **The app knows less than the patrol's own pocket.** A patrol is handed
    printed sheets — each carrying a QR code that binds it to the team — and
    those sheets have checkpoints drawn on them. The patrol can see those posts
    on paper but not in the app, which is the one place that also knows where the
    patrol currently *is*. The app is strictly worse than paper at the only
    question that matters at 02:00: which way now, and how far.
  - **Nobody can list a patrol's sheets.** The binding exists (skan records
    QR ↔ team), but the patrol has no view of it. When a patrol phones in lost,
    "which sheet are you holding?" has no reliable answer, and a patrol that was
    skipped a sheet finds out only when it walks off the edge of one.
  - **The scan history is a flat list.** `ScanList.vue` shows registrations
    newest-first with no distinction beyond kind, and nothing says whether a post
    was reached inside its window. "Are we behind?" is the question patrols ask
    all night, and the data to answer it (`openFromUts` / `openUntilUts` /
    `openDuration` on the checkpoint) is already on the stream.

- **Why now?** The upstream work landed. HQ's **PRD 010** shipped `kort` and
  `kortsaet` projections and — crucially — a written contract for *this repo*,
  addressed "**For:** the hej-app team", now copied here as
  `roadmap/api/kort-events.md`. HQ also already keeps a `maphandout` projection
  built from `qr.registered`, which is the shape a handout list needs. The event
  map, offline data layer (PRD 009) and checkpoint projection are all in place
  here. What remains is consuming events that already flow.

- **Evidence.**
  - `roadmap/api/kort-events.md` (in this repo, copied from hq 2026-09-15) — the
    contract, including §1/§1.1, the two reveal rules, which is the policy this
    PRD implements.
  - HQ's `maphandout` schema states "which maps has *this* team ever been given?"
    as a question the model must answer — which is this feature's handout list.
  - PRD 002 §11 deferred "which checkpoints may participants see?" rather than
    settling it. This PRD settles it.

## 3. Goals

- A patrol can see, offline, which map sheets it has been handed and when.
- A patrol sees on the map every checkpoint its sheets and its progress have
  revealed — and no checkpoint that has not been revealed.
- A patrol on the move can tell the direction and distance to its next
  checkpoint(s) without panning the map.
- A patrol can review its registrations and tell at a glance which were
  checkpoint scans and whether each was inside the post's window.
- The reveal rule is enforced in the BFF's read model, not in the client.

## 4. Non-Goals

- **Not** revealing un-revealed checkpoints in any form — not blurred, not a
  count, not a bearing. The race-area hull (PRD 002) remains the only aggregate
  covering posts a patrol has not earned sight of.
- **Not** turn-by-turn routing or route lines. Arrows are straight-line bearing
  and great-circle distance.
- **Not** an organizer tool. Handouts are recorded in skan; sheets are defined in
  HQ. This repo only reads the stream.
- **Not** sharing code with hq. Its projections are prior art to copy, not a
  dependency to import (§8).
- **Not** rendering the sheet's `extents` rectangles on the map (a plausible
  later addition — see §11).
- **Not** showing other teams' handouts, or who currently holds a sheet a patrol
  no longer has. HQ's read model surfaces the successor team for an organizer
  view; **our copy must not select those columns at all** (§6, non-functional).
- **Not** scoring or standings. "On time" is informational for the patrol, not an
  authoritative result.
- **Not** changing the offline tile cache scope (still the race area).

## 5. User Stories & Scenarios

- As a **patrol member**, I want to see which sheets we were handed, so we know
  whether we are missing one before we need it.
- As a **patrol member**, I want the checkpoints from our sheets drawn on the
  map, so I can relate our position to them.
- As a **patrol member**, I want an edge arrow with a distance towards the next
  post, so I can keep walking the right way while zoomed in on ourselves.
- As a **patrol member**, I want the drawer to show our checkpoint scans with a
  clear "på tid" / "for sent" marker, so we can decide to press on or rest.

**Happy path.** A patrol opens Kort. Their position is centred (PRD 002). The two
posts drawn on the sheet they are holding show as flag markers; a third, revealed
when they reached the previous checkgroup, is off-screen, so a chevron sits on the
screen edge in its direction labelled "3,4 km". They tap the bottom handle: the
drawer lists "Post 4 · på tid · lør 02:14", "Bandit taget · lør 01:02", "Post 3 ·
12 min for sent · fre 22:40", and a "Kort udleveret" section with "Kort 2 — Post 2
til Post 5" and "Skitse, Post 5–6". Tapping a row pans the map and closes the
drawer.

**Edge cases** (each one traceable to a documented upstream reality):

- **No patrol** (personnel roles): no handouts, no arrows, no drawer handle —
  as today, where an empty scan list hides the UI.
- **Nothing revealed yet** (before the first handout): no markers, no arrows,
  drawer still opens. Never an error.
- **Sheet with an unknown `mapId`** — a QR registered before its sheet was
  recorded. `maphandout` stores `""`, which means *unknown sheet*, not *no
  sheet*. Listed as a handout without a name; reveals nothing, because there is
  no checkpoint list to reveal.
- **`skitse`** — a hand-drawn slip with **no QR code**, never scanned. It is
  revealed by its `handoutCheckgroupId`, and its `checkpointIds` are its only
  trace in the system. It has no extent. It must appear in the handout list even
  though no QR binding exists for it (§11.1).
- **Revealed checkpoint with no position** — normal for some posts. Listed
  nowhere on the map and produces no arrow. Not an error.
- **Checkpoint with no window** — shown without a verdict rather than with a
  guessed one.
- **Relative window** (`openDuration`) needing a per-patrol anchor — no verdict
  unless the anchor is known. A wrong "for sent" is worse than a missing one.
- **A sheet reassigned to another team** — when a patrol is discontinued its
  scouts and sheets move on. The sheet leaves the patrol's *held* set (so it
  stops revealing), but stays in its *history*, shown as no longer held. The
  successor team's identity is never shown.
- **Deleted checkpoint or checkgroup** — `checkpointIds` must be resolved against
  our own checkpoint projection and unresolvable ids dropped; a deleted handout
  checkgroup reads as `""`, the QR rule. Neither fix travels over the stream
  (kort-events.md §4, §1.1).
- **Offline** — every surface reads the cached client copy. Arrows keep working:
  bearing is computed on device from cached positions plus the live GPS fix.
- **Clock skew** — verdicts are computed **server-side**, so a bad device clock
  cannot invent lateness.

## 6. Requirements

### Functional

**Map handouts**

- [ ] The BFF exposes the map sheets handed out to the signed-in user's patrol,
      newest last (handout order), each with: sheet name, format, when it was
      handed over, and whether the patrol still holds it.
- [ ] Handouts are scoped to sheets in the **patrol map set(s)** — matched on the
      set's `teamType`, **never on its name** (kort-events.md §2). Collect *all*
      matching sets; there is no "the" patrol set.
- [ ] A handout whose sheet is unknown (`mapId == ""`) is still listed.
- [ ] The frontend lists them; empty is a normal, silent state.
- [ ] Handouts are part of the offline cached payload (PRD 009).

**Visible checkpoints**

- [ ] The BFF computes the patrol's revealed checkpoint set as the union of:
      1. the `checkpointIds` of every sheet the patrol **currently holds** whose
         reveal rule is the QR rule (`handoutCheckgroupId == ""`);
      2. the `checkpointIds` of every sheet whose `handoutCheckgroupId` names a
         checkgroup the patrol has **already reached**;
      3. every checkpoint in a checkgroup the patrol has **already scanned**.
      Rules 1–3 do not nest and none can be derived from another
      (kort-events.md §1).
- [ ] Ids are resolved against our own checkpoint projection; unresolvable ids are
      dropped silently.
- [ ] Only positioned, non-deleted checkpoints are returned, each with id, name,
      checkgroup, sort order, and its window when known.
- [ ] The frontend plots them, visually distinct from the patrol's own scan
      markers, and legible on both topo and aerial base layers.
- [ ] A marker tap shows the name and, when known, the window.

**Next-checkpoint arrows**

- [ ] When a revealed checkpoint that is *next* for the patrol lies outside the
      viewport, an arrow is drawn at the viewport edge in its direction, with
      distance.
- [ ] "Next" = the earliest not-yet-scanned revealed checkpoints in sort order;
      at most **3** arrows at once, to keep the viewport readable.
- [ ] Arrows update on pan, zoom and position change; an arrow disappears when
      its checkpoint enters the viewport.
- [ ] Tapping an arrow pans the map to that checkpoint.
- [ ] No own position ⇒ no arrows.

**Scan drawer**

- [ ] The drawer lists **all** registrations, newest first, as today.
- [ ] Checkpoint scans are emphasised relative to other registrations.
- [ ] Each checkpoint scan carries an on-time verdict — on time / late (with how
      late) / early — or nothing when no window is known.
- [ ] The verdict comes from the BFF, not the client.
- [ ] Handed-out sheets appear as their own section in the drawer.
- [ ] Tapping a positioned row pans the map (existing behaviour, preserved).

### Non-Functional

- **The reveal rule is a read-model boundary, not a filter.** Following the
  precedent already set by `checkpoint.Queries` in this repo — which exposes only
  the hull precisely so that no call site *can* leak a position — the query
  handed to handlers must take the patrol id and return only revealed
  checkpoints. A handler must have no way to ask for all of them. A test must
  assert that an un-revealed checkpoint never appears in any response.
- **Other teams never enter the read model.** The handout payload must not carry
  the successor team's id, number or name. Stronger than projecting it out at the
  handler: because the projection is ours (§8), the query does not select those
  columns in the first place, so there is nothing to forget to remove — the same
  discipline as the guardian-phone rule, applied one layer earlier.
- **No guardian data.** Nothing here touches `phoneParent`; no contact data
  enters map or drawer payloads (repo rule).
- **Offline-first.** All surfaces read from the cached client store and degrade
  to "no data yet" rather than an error (PRD 009).
- **Freshness is PRD 017's job, not this one's.** Handouts, revealed checkpoints
  and scans all change during the race, so a copy fetched on mount is wrong within
  minutes. This PRD defines the payloads and requires each to expose a cheap
  version; the foreground sync check that consumes them is **PRD 017**. Neither
  blocks the other — 016 can ship fetching on mount — but shipping 016 without 017
  means a patrol sees a reveal only after a cold start, which is late enough to
  matter.
- **Performance.** Arrow recomputation runs on map move and position update and
  must not stutter panning on the baseline device. Expect tens of checkpoints.
- **Battery.** No new geolocation subscriptions: arrows consume the existing
  watch, already stopped when the page is hidden (PRD 002).
- **Accessibility.** Arrows are decorative graphics plus an accessible label
  ("Post 5, 3,4 km mod nordøst"); the drawer stays the non-visual route to the
  same information. Touch targets ≥ 44 px.
- **i18n.** Danish copy; metric distances, `da-DK` formatting.
- **Browser baseline.** iOS/iPadOS Safari 16.4+, Chrome 111+. No polyfills.

## 7. UX / UI Notes

**Map (`vue/src/views/MapsView.vue`)**

- Revealed checkpoints render as flag markers in a colour distinct from the
  patrol's own scan markers. Already-scanned ones read as "done" (muted /
  check), so the map doubles as a progress view.
- Edge arrows are chevrons pinned to the viewport edge with a short distance
  label, in the same `z-10` overlay layer as the existing controls. They must not
  collide with the top-right control stack, the top-left notices or the bottom
  handle — the safe region is the vertical middle band of each edge.
- The existing bottom-centre handle stays, relabelled to cover registrations and
  handouts, and is shown when the patrol has *either* (today: registrations
  only).

**Drawer (`vue/src/components/map/ScanList.vue`)**

- Keeps the shadcn-vue `Drawer` primitive it already uses. Two sections:
  "Registreringer" and "Kort udleveret".
- Checkpoint rows get the emphasis: stronger icon treatment plus a verdict badge
  ("på tid" green; "for sent" amber with the delta). Bandit catches keep their
  current red skull styling.
- A sheet the patrol no longer holds is shown de-emphasised — "afleveret" —
  without naming who has it.
- Title-level headings use `font-nathejk`; row text stays on the system sans
  stack. Icons from Lucide only (`Flag`, `Skull`, `MapPinOff` already in use;
  add e.g. `Map`, `Check`, `Clock`, `ChevronUp`).

New components expected: a checkpoints layer inside `EventMap.vue` (or
`CheckpointMarkers.vue`), and `EdgeArrows.vue`. No new routes.

## 8. Technical Considerations

### Integration shape

Over **JetStream**, building our own read model — not by calling HQ. This is
stated in the contract for exactly the reason that matters here: a read model we
own keeps answering on a race night while HQ is restarting or unreachable,
whereas polling would make every reveal depend on HQ being up at that moment.

**HQ's projections are copied into this repo, never referenced.** There is no Go
import of hq, no shared module, no build-time path to another checkout, and no
HTTP call to HQ. The `kort`, `kortsaet` and `maphandout` projections are written
here as our own code — informed by HQ's, and initially close to it — and from
that moment they are ours to change. The only cross-repo dependency is the
**event shapes on the stream**, and those are pinned in
`roadmap/api/kort-events.md`, a vendored copy of HQ's contract taken
2026-09-15.

That is a deliberate trade. Sharing a projection would look like reuse and behave
like coupling: HQ's read models answer an organizer's questions, ours answer a
patrol's, and the two diverge immediately — HQ's `maphandout.ByTeam` returns the
successor team that now holds a sheet, which is *precisely* a field we are
forbidden to expose (§4, §6). A copy makes that divergence a normal edit instead
of a negotiation, and it means an HQ refactor cannot break an app running during
an event. The cost — the same bug potentially fixed twice — is small at this size
and is the cheaper of the two.

So:

- The projections are ours. Where we knowingly mirror HQ's reasoning, the comment
  says so and names the upstream file, as prior art rather than as a dependency.
- Where a patrol's needs differ from an organizer's, we simplify rather than
  carry a field we must then project out.
- Copy the *reasoning*, not just the SQL. HQ's schema comments record hazards
  learned the hard way — the `checkpointIds` JSON array, `mapId` never
  overwritten with `""`, a sheet materialising before its set — and a copy that
  drops them re-learns them during an event.

**The `kort` / `kortsaet` message types are not in `shared-go` and are not being
lifted yet** — they have not stabilised enough, and lifting a shape with one
consumer gets it lifted wrong. So this repo decodes the JSON itself against local
mirror types, exactly as the contract anticipates ("another service can consume
these events today by decoding the JSON itself — the shapes are documented for
that purpose"). The vendored copy's own "Prerequisite" note says you cannot
decode until the lift lands; that is out of date, and the copy is what we build
against. Consequences to accept deliberately:

- The mirror types live in `go/nathejk/table/kort/messages.go` here, with a
  comment naming `roadmap/api/kort-events.md` (the copy in *this* repo) as the
  shape's source of truth, and HQ's task 138 as the eventual replacement.
- We are the **second consumer**, which is what tells HQ which parts of the shape
  are real. Anything we find awkward is raised against HQ's PRD 010, the contract
  changes there first, and we then refresh our copy.
- Unknown JSON fields are ignored, not rejected, so an additive upstream field
  cannot break a consumer mid-event.

### BFF (Go)

New projections in `go/nathejk/table/`, written here and owned here:

- **`kort` + `kortsaet`** (new). Subjects: `NATHEJK.*.kort.*.{created,updated,deleted}`,
  `NATHEJK.*.kort.sorted`, `NATHEJK.*.kortsaet.*.{created,updated,deleted}`,
  `NATHEJK.*.kortsaet.sorted`. Three semantics that will be got wrong if not
  written down in the code:
  - a sheet's `updated` is a **patch** — absent means unchanged, and an
    explicitly empty array/string **is** an edit (this repo's `checkpoint`
    consumer already handles exactly this hazard; follow it);
  - a set's `created`/`updated` is a **whole record** — an absent `teamType`
    means *the set has none*, not "unchanged";
  - `…sorted` names only the ids being placed; unnamed ids keep their order, and
    a sheet may precede its set on replay, so an unknown `kortsaetId` is
    tolerated rather than dropped.
- **`maphandout`** (new). Subject `NATHEJK.*.qr.*.registered`, keyed
  `(year, qrId, teamId)` so a re-bind is history rather than an overwrite. The
  sheet id arrives as an **additive `mapId` field that skan adds to the shared
  body**, so it is not on `messages.NathejkQrRegistered` and must be read through
  a struct embedding it. `mapId` is never overwritten with `""` — that means
  *unknown sheet*, not *no sheet*. Ours is **narrower than HQ's on purpose**: we
  need "does this patrol still hold this sheet", never who else holds it, so the
  successor-team derivation collapses to a boolean and the fields we must not
  expose are never selected in the first place.
- **`checkpoint`** (widen). Add `checkgroupId` (from `…checkpoint.*.created`,
  which this repo currently does not consume at all), `sortOrder` (from
  `NathejkCheckpointsSorted`), and the window (`FixedTimeRange` /
  `RelativeTimeDuration`). All already on the stream and currently discarded.
- **`checkgroup`** (new). From `NathejkCheckgroupUpdated` / `…Deleted`: name, and
  the group membership needed for reveal rule 3.
- **`checkpersonnel`** (new) and **`scan`** (new). This is how a QR scan becomes a
  *checkpoint* scan: `qr.scanned` carries the scanner's user id, and
  `NathejkCheckpersonnelAdded` binds a user to a checkpoint for a time range — so
  scan → checkpoint is resolved by scanner id within the shift window. This
  replaces the seeded mock behind `internal/scans`, whose interface was
  introduced for precisely this substitution.

Read API and handlers:

- Extend `checkpoint.Queries` with `RevealedCheckpoints(ctx, year, patrolID)`,
  returning a publishable-only type. `RaceArea` is untouched — the hull stays the
  answer for everyone else.
- New `maphandout.Queries.ByPatrol`, returning a type with no successor-team
  fields.
- The on-time verdict is computed in the BFF, joining a scan to its checkpoint's
  window; `internal/scans`' mock must be able to produce every verdict state for
  dev simulation (PRD 014).
- Wire everything in `go/cmd/api/app` + `routes.go` using the `…OrNil` adapter
  pattern (see `raceAreasOrNil`) so "no projection" stays checkable and a nil
  interface cannot be non-nil.
- Log an aggregate at boot — how many sheets, sets and revealed checkpoints the
  year has — in the spirit of `ReportPositionless`. A reveal rule that silently
  resolves nothing is the failure mode most likely to reach an event.

### Frontend (Vue 3 / TS)

- New `checkpoints.store.ts` and `handouts.store.ts` (or one map-bootstrap store
  — see §11.7), cached through the offline data layer (PRD 009).
- `EventMap.vue` gains a checkpoints layer and exposes bounds/centre so the arrow
  overlay knows what is off-screen.
- `EdgeArrows.vue` computes bearing + great-circle distance from
  `location.position`, projects onto the viewport edge, re-renders on Leaflet
  `move`/`zoom` and position change. Must not pull Leaflet into the app-shell
  bundle — the map is lazy-loaded and stays so.
- `ScanList.vue` gains sections, emphasis and verdict badges.

### API endpoints

All behind `requireAuth`, all needing **OpenAPI annotations** (repo rule):

- `GET /api/checkpoints` — the caller's patrol's revealed checkpoints. Empty list
  (200) when none, including for users without a patrol.
- `GET /api/patrol/handouts` — the patrol's map sheets. Empty list (200) for
  users without a patrol, matching `/api/patrol/scans`.
- `GET /api/patrol/scans` — **changed, additively**: each checkpoint scan gains
  `checkpoint_id` and an optional verdict (`on_time`, `delta_seconds`).
  Backwards compatible; the shipped client ignores unknown fields.

Each of the three must also expose a cheap version derivation for PRD 017's sync
check — a projection read or a cached hash, never a built-and-hashed payload.

### Dependencies & risks

- **Reveal-rule regression is the top risk.** Widening the checkpoint projection
  puts un-revealed positions one struct field from a response. Mitigated by a
  patrol-scoped query returning a publishable-only type, plus an explicit test.
- **Contract drift.** The `kort` shapes are unstable by their owner's own
  assessment. Mirror types, tolerant decoding, and the vendored
  `roadmap/api/kort-events.md` are the mitigation; a shape change is an HQ
  PRD 010 change first, then a refresh of our copy. The copy carries the date it
  was taken so drift is at least visible.
- **Copy divergence.** Copying rather than sharing means an upstream bug fix does
  not reach us. Accepted (see §8, integration shape); the mitigation is that our
  projections are small, tested here, and rebuilt by replay, so a fix is an edit
  and a restart rather than a migration.
- **Reveal correctness depends on two things HQ cannot fix for us**: resolving
  `checkpointIds` against our own checkpoints, and treating a dangling
  `handoutCheckgroupId` as the QR rule. Both are fixes that do not travel over
  the stream.
- **Scan → checkpoint resolution is inferential** (scanner id + shift window). A
  scanner covering two posts, or a shift recorded loosely, produces a wrong or
  missing attribution — which affects the verdict badge and reveal rule 3.
- **Projection volume.** Six new/widened projections replaying from sequence zero
  on every boot. All statements stay idempotent upserts (existing convention).

## 9. Success Metrics

- ≥ 95 % of patrols have at least one handout listed in the app during the event.
- Zero un-revealed checkpoint positions in any API response (asserted by test,
  confirmed by a post-event review of the reveal log line).
- ≥ 3 drawer opens per patrol per event — the drawer is worth the work only if
  it is used.
- Qualitative: fewer "we don't know where we are" calls to the organizers than
  the previous event.

## 10. Rollout / Task Breakdown

Sequenced so the reveal rule is built and tested before anything renders it, and
so the projections land in dependency order.

**Phase 1 — consume the upstream facts**
- [ ] Task: vendor `roadmap/api/kort-events.md` from hq and mirror the
      `kort`/`kortsaet` message shapes locally (no import of hq)
- [ ] Task: `kort` + `kortsaet` projection (patch vs whole-record semantics,
      sorted events, sheet-before-set tolerance)
- [ ] Task: `maphandout` projection from `qr.registered`, incl. the additive
      `mapId` field, narrowed to "does this patrol still hold it"
- [ ] Task: widen `checkpoint` with `checkgroupId`, sort order and window
- [ ] Task: `checkgroup` projection
- [ ] Task: `checkpersonnel` + `scan` projections; retire the `internal/scans`
      mock as the production source

**Phase 2 — the reveal rule**
- [ ] Task: `RevealedCheckpoints(year, patrolID)` implementing rules 1–3, with a
      publishable-only return type
- [ ] Task: resolve `checkpointIds` and dangling `handoutCheckgroupId` on read
- [ ] Task: regression test — an un-revealed checkpoint never leaves the BFF
- [ ] Task: `GET /api/checkpoints` + OpenAPI annotations
- [ ] Task: boot-time aggregate log of sheets/sets/revealed counts

**Phase 3 — map rendering**
- [ ] Task: `checkpoints.store.ts` with offline caching
- [ ] Task: checkpoint markers on `EventMap.vue`, incl. scanned/"done" state
- [ ] Task: edge arrows (bearing, distance, tap-to-pan, ≤ 3)
- [ ] Task: arrow overlay collision rules against existing map controls

**Phase 4 — verdicts and drawer**
- [ ] Task: server-side on-time verdict; extend `/api/patrol/scans`
- [ ] Task: emphasise checkpoint scans and render verdict badges in `ScanList.vue`
- [ ] Task: `GET /api/patrol/handouts` (successor-team fields projected out) +
      OpenAPI annotations
- [ ] Task: "Kort udleveret" drawer section, offline-cached
- [ ] Task: cheap version derivations for handouts, checkpoints and scans, for
      PRD 017's sync check
- [ ] Task: dev-simulation fixtures covering every verdict and reveal state
      (PRD 014)

No feature flag: each surface ships behind "empty hides the section", which is
its own soft launch. Phases 1–2 are invisible to users by construction.

## 11. Open Questions

1. **`teamType` for the patrol set is `patrulje`, not `spejder`.** The set we
   want is the one an operator calls the scout/patrol set, but HQ **rejects**
   `"spejder"` as a team type on write (`ErrInvalidTeamType`, with the reasoning
   that spejder is the domain's word for a *person*, not a team type), and
   `shared-go` has no such value. So the filter is `teamType == "patrulje"` —
   collecting *all* sets that carry it, per kort-events.md §3. **Confirm** this
   is what was meant, because a filter on a value that can never be stored would
   match nothing and silently reveal nothing to anybody.
2. **How does a `skitse` become a handout?** It has no QR code, so no
   `qr.registered` event ever names it, so it cannot appear in `maphandout` — yet
   it is physically handed over and does reveal checkpoints via
   `handoutCheckgroupId`. Options: synthesise a handout when the patrol reaches
   the handout checkgroup, or list only QR-bound sheets and accept that skitser
   reveal checkpoints without appearing in the list. This is the one modelling
   gap in the handout list.
3. **Is `showOnMap` (on the checkgroup) relevant here at all?** It exists on
   `NathejkCheckgroupUpdated` and sounds like it answers "may participants see
   this", but it plausibly means "show on HQ's own planning map". If it is a
   participant-facing flag it becomes a fourth reveal input — or a gate over all
   three. Needs an answer from HQ before phase 2.
4. **Should a sheet the patrol no longer holds keep its checkpoints revealed?**
   This PRD says no (reveal follows current holdings) but keeps the sheet in the
   history. The opposite — once seen, always visible — is arguably kinder and is
   certainly simpler. Un-revealing something a patrol has already seen on paper
   achieves no secrecy.
5. **Can the BFF resolve relative windows?** `openDuration` needs a per-patrol
   anchor (start time, or the previous checkpoint scan). If unavailable, those
   checkpoints ship without a verdict.
6. **How late is "for sent", and do we show the delta?** The control-group
   `Minus`/`Plus` fields suggest a grace period exists upstream. Worth confirming
   before rendering an amber badge at one second past.
7. **One endpoint or three?** The offline layer may prefer a single cacheable
   map-bootstrap document over `/api/checkpoints` + `/api/patrol/handouts` +
   `/api/patrol/scans`.
8. **Draw the sheet `extents` on the map?** Zero, one or two normalised
   north-west/south-east rectangles per sheet are available, and "here is what
   your paper covers" is a compelling overlay. Deliberately out of scope for now;
   easy to add once handouts exist.
9. **Is "next" the same sequence for every patrol?** `NathejkCheckpointsSorted`
   gives a global order. If routes are per-team or per-etape, arrows need a
   per-patrol sequence rather than a global sort.
10. **Was the drawer ever removed?** `ScanList.vue` is still wired into
    `MapsView.vue` on `main`, so "reintroduce" is read here as "upgrade the
    existing drawer". Confirm, so phase 4 is scoped correctly.
11. **Who refreshes the vendored contract, and when?** A copy with a date on it
    is honest but passive. Options: refresh at the start of each season, or ask hq
    to note in its own file which repos hold copies. Worth deciding once rather
    than discovering a stale copy mid-event.
