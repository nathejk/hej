# PRD 016 — Map handouts, visible checkpoints, and the scan drawer

**Status:** doing
**Author:** agent session (Zed / Claude)
**Created:** 2026-09-15
**Last updated:** 2026-09-15 (approved; §6/§8 amended in implementation — see §11.12)
**Approved:** 2026-09-15
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
  trace in the system. It has no extent. Because no QR binding exists for it, it
  cannot come from the handout projection at all — it is **synthesised** into the
  handout list when the patrol reaches its handout checkgroup (§11.2).
- **Revealed checkpoint with no position** — normal for some posts. Listed
  nowhere on the map and produces no arrow. Not an error.
- **Checkpoint with no window** (`scheme: none`) — shown without a verdict rather
  than with a guessed one.
- **Relative window** (`scheme: relative`) — resolved from the patrol's own scan
  at the checkgroup named by `relativeCheckgroupId`, plus `openDuration` minutes
  (§11.5). Until that anchoring scan exists there is no window, so no verdict.
- **A sheet reassigned to another team** — when a patrol is discontinued its
  scouts and sheets move on. The sheet leaves the patrol's *held* set and is shown
  as no longer held, but **its checkpoints stay revealed** (§11.4): un-revealing
  something the patrol has already studied on paper achieves no secrecy. The
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
      in handout order, each with: the **QR sticker number**, the sheet name, its
      format, when it was handed over, and whether the patrol still holds it.
- [ ] Handouts are scoped to sheets in the **patrol map set(s)** — matched on the
      set's `teamType` being `patrulje` (**not** `spejder`, which HQ refuses to
      store; §11.1), **never on the set's name** (kort-events.md §2). Collect *all*
      matching sets; there is no "the" patrol set.
- [ ] A `skitse` — no QR code, so never in a QR binding — is listed as a handout
      when the patrol reaches the checkgroup named by its `handoutCheckgroupId`
      (§11.2).
- [ ] A handout whose sheet is unknown (`mapId == ""`) is still listed, labelled
      **"Ukendt kort"** — the same wording HQ's patrol page uses, because a patrol
      and an organizer discussing the same sheet over the phone should be reading
      the same words.
- [ ] The frontend lists them; empty is a normal, silent state ("Ingen kort
      udleveret").
- [ ] Handouts are part of the offline cached payload (PRD 009).

**Visible checkpoints**

- [ ] The BFF computes the patrol's revealed checkpoint set as the union of:
      1. the `checkpointIds` of every sheet **ever handed to the patrol** whose
         reveal rule is the QR rule (`handoutCheckgroupId == ""`);
      2. the `checkpointIds` of every sheet whose `handoutCheckgroupId` names a
         checkgroup the patrol has **already reached**;
      3. every checkpoint in a checkgroup the patrol has **already scanned**.
      Rules 1–3 do not nest and none can be derived from another
      (kort-events.md §1). Revealing is **monotonic**: nothing already revealed is
      ever withdrawn, including when a sheet changes hands (§11.4).
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
- [ ] "Next" is a **line** — a checkgroup — not a post, and not a set of lines (§11.12,
      §11.13). The BFF names it as `next_checkgroup`, because deciding it needs route order
      across checkgroups *and* whether the patrol has started, neither of which the client
      can honestly hold.
- [ ] **Every revealed post in that line gets an arrow.** A postlinje holds several posts —
      an A and a B — and the patrol chooses which to walk to when they arrive; the app must
      not nominate one for them.
- [ ] A line is behind the patrol once they have been scanned at it **or at any later
      line**, so an unattributed scan cannot point them backwards.
- [ ] Having **started** retires the first line: departing is recorded at check-in, not as
      a scan at a post, so nothing else ever will.
- [ ] Arrows update on pan, zoom and position change; an arrow disappears when
      its checkpoint enters the viewport.
- [ ] Tapping an arrow pans the map to that checkpoint.
- [ ] No own position ⇒ no arrows.

**Scan drawer**

- [ ] The drawer lists **all** registrations, newest first, as today.
- [ ] Checkpoint scans are emphasised relative to other registrations.
- [ ] Each checkpoint scan carries an on-time verdict — inside the window, or
      late/early with the delta — or nothing when the checkpoint has no window.
      The window is the window: the current model carries **no grace minutes**
      (§11.6).
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
  label, in the same `z-10` overlay layer as the existing controls. They **hug the
  edge** and slide *along* it to clear the controls that occupy corners — the
  top-right stack and the bottom-centre handle — rather than being confined to a
  middle band (that was task 264's first attempt, and §11.14 records why it was
  wrong). The top-left notices get no reserved space, since they are conditional
  and reserving for them floats the whole left of the top edge.
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
  without naming who has it. HQ's organizer view says "Flyttet til …" and names
  the successor; ours must not, and the difference is the whole reason our
  projection is a copy rather than a shared one (§8).
- The QR sticker number is shown alongside the sheet name. It is printed on the
  physical sheet, so it is the one identifier a patrol can read aloud down a phone
  when nothing else matches — which is the original problem this feature exists to
  solve.
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
  `(year, qrId, teamId)` so a re-bind is history rather than an overwrite.

  **`registered` is the handout; `scanned` is a post visit.** The two `qr`
  subjects look interchangeable and are not: `qr.registered` binds a printed code
  (and its sheet) to a team — that *is* the handover — while `qr.scanned` is a
  scan of the team's code at a post. This projection consumes the former; the
  `scan` projection below consumes the latter. Getting them the wrong way round
  would produce a handout list that grows at every checkpoint.

  The sheet id arrives as an **additive `mapId` field that skan adds to the shared
  body**, so it is not on `messages.NathejkQrRegistered` and must be read through
  a struct embedding it. `mapId` is never overwritten with `""` — that means
  *unknown sheet*, not *no sheet*. Times are **unix seconds** (`firstUts` /
  `lastUts`), not milliseconds; HQ's own view scales them on render and so must
  we.

  Ours is **narrower than HQ's on purpose**: we need "does this patrol still hold
  this sheet", never who else holds it, so the successor-team derivation collapses
  to a boolean and the fields we must not expose are never selected in the first
  place.

  **Prior art:** `PatruljeView.vue` in hq renders exactly this list for an
  organizer — QR, sheet name (or "Ukendt kort"), handed-out time, and a status of
  "Hos patruljen" / "Flyttet til …". We copy all of it except the last, where the
  successor team's name is precisely what a participant may not see.
- **`checkpoint`** (widen). Add `checkgroupId` (from `…checkpoint.*.created`,
  which this repo currently does not consume at all), `sortOrder` (from
  `NathejkCheckpointsSorted`), and the window — `FixedTimeRange` as
  from/until instants, `RelativeTimeDuration` as minutes. All already on the
  stream and currently discarded.
- **`checkgroup`** (new). From `NathejkCheckgroupUpdated` / `…Deleted`: name,
  `sortOrder`, `scheme` (`fixed` | `relative` | `none`) and
  `relativeCheckgroupId` — the last two being what makes a relative window
  resolvable (§11.5) — plus the group membership needed for reveal rule 3.
  `showOnMap` is deliberately **not** consumed (§11.3).
- **`checkpersonnel`** (new) and **`scan`** (new). This is how a QR scan becomes a
  *checkpoint* scan, and HQ's live implementation is the reference —
  `scansByCheckgroup` in `cmd/api/checkgroupteams.go`:

  ```sql
  FROM scan s
  JOIN checkpersonnel cpn ON s.scannerId = cpn.userId
                         AND s.uts >= cpn.startUts AND s.uts <= cpn.endUts
  JOIN checkpoint    cpt ON cpn.checkpointId = cpt.id
  ```

  A scan carries no checkpoint; the only link is who scanned it and when, so a
  scan counts for a post if the scanner was on a registered shift there at that
  moment. **The postmandskab rota is therefore load-bearing**: with no shifts
  recorded, no scan can be attributed, no checkpoint scan appears, and reveal
  rule 3 never fires. HQ's code says so in as many words, and it is the failure
  mode most likely to reach an event.

  This replaces the seeded mock behind `internal/scans`, whose interface was
  introduced for precisely this substitution.

Read API and handlers:

- **The reveal rule lives in `go/internal/reveal`, not on `checkpoint.Queries`.**
  Amended during implementation (task 255). This PRD originally sketched it as
  `checkpoint.Queries.RevealedCheckpoints`, which turned out to be the wrong home for a
  constraint the sketch did not weigh: the `nathejk/table/*` packages are bound for
  shared-go and must not import one another, while the rule needs four of them at once
  (sheets, handouts, scans, checkpoints). Whichever projection hosted it would have to read
  another's tables.

  The security property is preserved, one layer out. `checkpoint.Queries` gained two reads
  that are **bounded by what the caller already names** — `ByIDs(year, ids)` and
  `ByCheckgroups(year, groups)` — so there is still no way to ask the projection for *all*
  checkpoints, and a handler cannot leak a position it had no grounds to know about.
  `internal/reveal` decides which ids a patrol has earned; the projection refuses every
  broader question. `RaceArea` is untouched — the hull remains the answer for everyone else.
- New `maphandout.Queries.ByPatrol`, returning a type with no successor-team
  fields.
- The on-time verdict is computed in the BFF, joining a scan to its checkpoint's
  window. Three schemes, three behaviours: `fixed` compares against
  `openFrom`/`openUntil` directly; `relative` anchors on the patrol's own scan at
  `relativeCheckgroupId` plus `openDuration` minutes; `none` yields no verdict.
  The comparison is inclusive and has **no grace margin** — the `minusMinutes` /
  `plusMinutes` fields that appear in HQ's older `controlpoint` queries belong to
  a superseded model and have no equivalent in the current `checkpoint` table
  (§11.6). `internal/scans`' mock must be able to produce every verdict state for
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
- **Scan → checkpoint resolution is inferential** (scanner id + shift window), and
  its accuracy is the rota's accuracy, not ours. A scanner covering two posts, or
  a shift recorded loosely, produces a wrong or missing attribution — which
  affects the verdict badge *and* reveal rule 3. Worth an explicit metric: how
  many of a patrol's scans failed to attribute.
- **Projection volume.** Six new/widened projections replaying from sequence zero
  on every boot. All statements stay idempotent upserts (existing convention).

## 9. Success Metrics

- ≥ 95 % of patrols have at least one handout listed in the app during the event.
- Zero un-revealed checkpoint positions in any API response (asserted by test,
  confirmed by a post-event review of the reveal log line).
- ≤ 5 % of a patrol's scans fail to attribute to a checkpoint — the rota-shaped
  failure mode, which is invisible without a number on it.
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
- **`maphandout`** projection from `qr.registered`, incl. the additive
      `mapId` field, keyed `(year, qrId, teamId)` as history
- [ ] Task: widen `checkpoint` with `checkgroupId`, sort order and window
- [ ] Task: `checkgroup` projection incl. `scheme`, `relativeCheckgroupId` and
      `sortOrder` (not `showOnMap`)
- [ ] Task: `checkpersonnel` + `scan` projections; retire the `internal/scans`
      mock as the production source

**Phase 2 — the reveal rule**
- [ ] Task: the reveal rule in `internal/reveal` implementing rules 1–3, over
      bounded projection reads
- [ ] Task: resolve `checkpointIds` and dangling `handoutCheckgroupId` on read
- [ ] Task: synthesise `skitse` handouts from `handoutCheckgroupId` reach
- [ ] Task: regression test — an un-revealed checkpoint never leaves the BFF
- [ ] Task: `GET /api/checkpoints` + OpenAPI annotations
- [ ] Task: boot-time aggregate log of sheets/sets/revealed counts, plus an
      unattributed-scan count (the rota failure mode)

**Phase 3 — map rendering**
- [ ] Task: `checkpoints.store.ts` with offline caching
- [ ] Task: checkpoint markers on `EventMap.vue`, incl. scanned/"done" state
- [ ] Task: edge arrows (bearing, distance, tap-to-pan, ≤ 3)
- [ ] Task: arrow overlay collision rules against existing map controls

**Phase 4 — verdicts and drawer**
- [ ] Task: server-side on-time verdict for all three schemes (`fixed`,
      `relative`, `none`); extend `/api/patrol/scans`
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

**All settled 2026-09-15.** Kept rather than deleted, because most were answered
by reading the upstream code and the answer is worth more than the decision — the
next person to wonder should not have to go and look again.

1. **The patrol set is `teamType: "patrulje"`, never `"spejder"`.** *Settled by
   code, confirmed by the product owner 2026-09-15.* HQ **refuses** `"spejder"` on write (`ErrInvalidTeamType`), with the
   reasoning that spejder is the domain's word for a *person*, not a team type,
   and `shared-go` has no such value — the valid set is `patrulje`, `klan`,
   `crew`, `gøgler`. "The spejder set" is what the set is *called* in
   conversation; `patrulje` is what it *is*. We match on `patrulje`, collect **all**
   sets carrying it (kort-events.md §3 — it is a filter, not a key), and never
   match on the set's name. A filter on `"spejder"` would match nothing and
   silently reveal nothing to anybody, so this is worth a test that fails loudly
   if the year has no matching set.

2. **A `skitse` is synthesised into the handout list when its handout checkgroup
   is reached.** *Decided, confirmed by the product owner 2026-09-15.* It has no QR code, so no `qr.registered` event can
   ever name it and it cannot come from the handout projection — but it *is*
   physically handed over, and it does reveal checkpoints through
   `handoutCheckgroupId`. Listing it only when the patrol reaches that checkgroup
   makes the handout list say the same thing the reveal rule already acts on,
   which is better than a list that silently omits a sheet the patrol is holding.
   The synthesised entry has no QR id and no "still held" state — there is no
   binding to lose.

3. **`showOnMap` is not consumed.** *Settled by code.* It exists on the
   checkgroup, is written by HQ's `PostlinjeModal` toggle, and — as far as the
   code goes — is read by **nothing**: no map, no export, no participant surface.
   Its intent is therefore unverified, and both ways of guessing are bad: treating
   it as a gate would hide everything if organizers never set it, and treating it
   as permission would leak if it means "show on the planning map".

   The deeper reason to ignore it is that our rule does not need it. Reveal is
   grounded in **physical possession**: a checkpoint drawn on the paper sheet in
   the patrol's hand, or on a post they have already stood at, cannot be a secret
   from that patrol. A rule anchored in what someone already holds cannot
   over-reveal, whatever a flag says. If organizers later confirm `showOnMap` is
   participant-facing it becomes an *additional* filter, never a replacement.

4. **Revealing is monotonic — once revealed, always revealed.** *Decided,
   reversing the first draft.* When a patrol is discontinued its scouts and sheets
   are reassigned, so a sheet legitimately changes hands. The draft had reveal
   follow *current* holdings, which meant checkpoints could vanish from the map of
   a patrol that had already studied them on paper. That achieves no secrecy — the
   knowledge left with the scout, not the sheet — and it makes the map lie about
   the ground the patrol has already walked. So the handout list tracks possession
   ("afleveret"), and the revealed set only ever grows. It is also the simpler
   thing to implement and to reason about mid-event.

5. **Relative windows are resolvable.** *Settled by code.* The checkgroup carries
   `scheme` — `fixed` | `relative` | `none` — and `relativeCheckgroupId`. So
   `relative` is not an unknown anchor: it is *this patrol's own scan* at the named
   checkgroup, plus `openDuration` minutes, and we hold that scan. `fixed` uses
   `openFrom`/`openUntil` directly; `none` yields no verdict. Only the case where
   the anchoring scan does not exist yet yields no verdict — correctly, since the
   window has not started.

6. **There is no grace period. The window is the window.** *Settled by code.* The
   `minusMinutes` / `plusMinutes` grace appears only in HQ's `controlpoint`
   queries, which belong to the superseded control-group model (the one live copy
   is commented out). The current model's live path — `scansByCheckgroup` in
   `cmd/api/checkgroupteams.go` — is a plain inclusive
   `s.uts BETWEEN cpt.openFromUts AND cpt.openUntilUts`, and the `checkpoint`
   table has no grace columns to read. We match the live rule exactly, so the app
   and the organizers' own screens cannot disagree about who was on time. We show
   **the delta** ("12 min for sent"), because a number is information a patrol can
   act on where a bare verdict is just a judgement.

7. **Three endpoints, not one bootstrap document.** *Decided.* They have
   genuinely different change rates — scans change on every post, handouts a
   handful of times a night, revealed checkpoints only when one of those two does
   — and PRD 017's sync check gives each its own version key, so a new scan must
   not re-download the checkpoint list. One document would couple all three to the
   fastest-changing one.

8. **Sheet `extents` are not drawn.** *Confirmed out of scope.* Zero, one or two
   normalised north-west/south-east rectangles are available per sheet, and "here
   is what your paper covers" is a compelling overlay — but it is additive, it
   needs its own design work on a screen that is already busy with markers and
   arrows, and nothing else waits on it. Revisit once handouts exist.

9. **One route order for every patrol:** `(checkgroup.sortOrder,
   checkpoint.sortOrder)`. *Settled by code.* Both orders are collection-level
   — `NathejkCheckgroupsSorted` and `NathejkCheckpointsSorted` name ids with no
   team in sight — and nothing in the checkpoint, checkgroup or kort model carries
   a per-team sequence. The route is the postlinje, and it is the same line for
   everyone. Arrows therefore point at the earliest unscanned *revealed*
   checkpoints in that order, which also means a patrol never gets an arrow
   towards something it has not been shown.

10. **The drawer is upgraded, not restored.** *Settled by code.* `ScanList.vue`
    is still wired into `MapsView.vue` on `main` and already uses the shadcn-vue
    `Drawer` primitive. "Reintroduce" means giving it the emphasis, verdicts and
    handout section it lacks — phase 4 is edits to a live component, not a
    resurrection.

11. **The vendored contract is refreshed at the start of each season, and on any
    decode failure.** *Decided.* A dated copy is honest but passive, so it gets
    one scheduled moment (season start, alongside the year rollover the maps
    already require) and one event-driven one: a decode that finds an unexpected
    shape is a signal the copy is stale, and the mirror types log rather than
    silently ignore a body they cannot make sense of. We do not ask hq to track
    who holds copies — that makes our dependency their bookkeeping.

    **It is already stale, and usefully so.** The copy's "What is *not* published
    yet" section says "which QR code sits on which sheet" is not built and that
    these events therefore give only the *candidate* sheets for a team type. That
    is no longer true: skan now publishes `mapId` on `qr.registered`, and HQ's
    `maphandout` projection and patrol page are built on it. Had we designed
    against the prose alone we would have concluded a handout list was impossible
    and shipped a worse feature. The lesson is recorded here rather than fixed in
    the copy: **the code is the contract; the document is a guide to it.** Verify
    against the projections before concluding something cannot be done.

12. **Arrows are per checkgroup, not per checkpoint.** *Corrected by the product owner
    2026-09-15, during phase 3 (task 273).* The first implementation skipped a checkpoint the
    patrol had scanned and pointed at the next unvisited *post*. Wrong unit.

    A checkgroup is one **leg** of the route and its posts are **alternatives** — hq's own model
    records "a started team's standing at one checkgroup" (on time / late / missing) per *group*,
    and reveal rule 3 only makes sense on the same reading: scanning any post reveals the whole
    group because the group is the thing you either reached or did not.

    So a patrol scanned at Post 4A has finished that leg, and an arrow to Post 4B — the
    alternative post beside it — sends them somewhere they have no reason to go *and* spends one
    of three arrow slots that should be showing the legs ahead. Two consequences:

    - a leg with any scanned post produces no arrow;
    - a leg produces exactly **one** arrow, pointing at its **nearest** post, because the patrol
      needs only one of them and the useful answer is the one they can walk to.

    **Markers are unaffected**: they still show every revealed post, ticked where scanned. The map
    should show the ground truth; only the arrows are an instruction, and only an instruction has
    to be about the leg.

13. **The arrows point at the whole next line, and the start is never next.** *Corrected by the
    product owner 2026-09-15 (task 275), superseding half of §11.12.*

    Reading the live data settled both halves. The route is: line 0 `Start` / `Starter`
    (`Afgang`, `Til Gøgl`), then `Postlinje 1`–`4` each holding an **A and a B** post, then
    `Mål`.

    - **§11.12 was half wrong.** A line's posts *are* alternatives, but it does not follow that
      the app should choose between them. Both are real destinations and the patrol picks one
      when they arrive, so **every** revealed post in the next line gets an arrow. The
      "nearest post per line" rule is withdrawn.
    - **Only the next line is arrowed**, not the next three. A patrol is walking to one line;
      lines beyond it are clutter they cannot act on.
    - **Departing the start is not a scan.** It is recorded at check-in, so no attributed scan
      will ever retire line 0 and the arrows pointed back at `Afgang` all night. Two rules fix
      it: progress is **monotonic** (a line is behind you if you were scanned at it *or at any
      later line*, which also survives a gap in the postmandskab rota), and **having started
      retires the first line** — `person.HasStarted()` being the codebase's single definition of
      "has begun the event".
    - **The decision moved to the BFF**, surfaced as `next_checkgroup`. It needs route order
      across checkgroups and the started fact; the client sees the latter only folded into
      `confirmation_required`, so re-deriving it would fork a definition that exists precisely
      to be singular.

    A line whose posts are none of them revealed or sited is skipped rather than becoming a
    dead "next" with no arrows — `Postlinje 3` is in exactly that state in the live data, its
    posts having no positions yet.

14. **Edge arrows hug the edge and slide past the controls; they are not confined to a band.**
    *Corrected by the product owner 2026-09-15 (task 276), superseding the placement rule task
    264 shipped.* To keep arrows off the floating controls, task 264 reserved a full-width band
    — 112 px off the top, 96 off the bottom — and clamped every arrow into the strip between. But
    the controls sit in **corners** (layer/locate stack top-right, registrations handle
    bottom-centre), so an arrow leaving the top edge on the left, where nothing floats, hung 112
    px in mid-air. The keep-out is now corner **rectangles** an arrow slides *along its edge* to
    clear, and every other part of every edge is free. The top-left notices get no reserved
    space — they are conditional, and reserving for them is what floated the left of the top
    edge.

15. **The arrow's rotation points at the checkpoint on screen, and re-aims on every map move.**
    *Corrected by the product owner 2026-09-15 (task 278), reversing part of the task-274
    decision.* Task 274 kept the chevron's rotation as the geographic bearing from the patrol — a
    ground fact, deliberately viewport-independent — on a "walk this way" reading. But an edge
    arrow is an off-screen indicator, and its job is to point at where the post *is*; a rotation
    that does not track the map is the bug. Rotation is now the screen-space direction from the
    arrow to the checkpoint's projected position, recomputed on every pan and zoom (and after a
    slide). The **distance** stays patrol-to-post — a ground fact that a pan does not change — and
    on the north-up map the screen bearing doubles as the compass word, so chevron and label agree.

### Consequences worth carrying forward

- **The postmandskab rota is load-bearing for this feature.** Scan → checkpoint
  attribution runs through `checkpersonnel` shifts, so an unrecorded shift means
  no checkpoint scan, no verdict, and no reveal by rule 3. It is upstream of us
  and invisible from here, which is why §9 now carries an unattributed-scan
  metric.
- **`showOnMap` remains an open question for *HQ*, not for us.** Worth asking
  what it is for; nothing here waits on the answer.

