# PRD 011 — Post-race experience: diploma, team tracks and scan history

**Status:** draft
**Author:** agent session (Zed)
**Created:** 2026-08-28
**Last updated:** 2026-09-02
**Approved:**
**Shipped:**
**Target users:** participants — spejder and bandit primarily, as members of a team

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 0. Inherited measurements (added 2026-09-02, when PRD 002 closed)

PRD 002 shipped the position track and then measured it on real devices. Whatever this PRD does with a
team's route, it inherits these facts — and the first two constrain what can honestly be drawn:

| | |
|---|---|
| accuracy | **10.5 m median** on an iPhone (GPS) · **35 m** on a Wi-Fi-only iPad |
| coverage | good while the app is foregrounded; **2% of a 22-hour day**, because a web app does not run backgrounded |
| gaps | one per backgrounding, matching it almost to the second |
| iOS kills | 8 of 28 backgroundings, always resumed |

**So a track is a dotted record of where the app was open, not a route.** Drawing it as a continuous line
implies a precision and a continuity it does not have: it would invent path between two points recorded
twenty minutes and half a kilometre apart, and on an iPad it would invent it at 35 m accuracy. The gaps are
information — they say "the phone was in a pocket here" — and smoothing them away is the one presentation
choice this PRD should not make by default.

Task 086 (the original post-race team track) was closed unbuilt in favour of this PRD; its analysis is in
that task file.


## 1. Summary

After the race, the app becomes the place a team relives it: a **diploma**, the **route every
member of the team walked**, and **every scan the team collected** — checkpoints, bandit
catches and whatever else registered them. Three things that only make sense together, because
each is a different answer to the same question: *what did we actually do last night?*

## 2. Problem & Motivation

**What problem does this solve?** The app is currently built entirely for *during* the event.
The moment a patrol crosses the finish line it has nothing left to offer, at exactly the point
where participants are most interested in what just happened and most likely to show it to
someone else. A finished Nathejk is 12 hours of walking through a forest at night that leaves
almost no trace for the people who did it — a paper diploma, if that, and a memory of which
posts they found.

**Why now?** Two things just landed that make it possible, and one that makes it necessary.

- Possible: PRD 002 §11.1's position track now runs end to end (tasks 081–084). Tracks are
  being recorded on phones and published to the `TELEMETRY` stream, per person, retained
  indefinitely. The data exists and currently has **no reader at all**.
- Also possible: real scan events already flow on `NATHEJK.<year>.qr.<n>.scanned`, carrying
  the team, the scanner and a position. 2025's event is fully represented on the stream.
- Necessary: **the consent bargain is currently one-sided.** The app asks a 12-year-old for
  continuous location access, and everything it does with that goes to the organizers. PRD 002
  §11.1 promised the participant something back — "så vi bagefter kan vise jer jeres egen
  rute" — and until that exists, the privacy page is describing a benefit the app does not
  deliver. This PRD is where that promise is kept.

**Evidence.**

- Task 086 (now closed into this PRD) specified the track view and measured its cost.
- The `diplom` repo already generates a per-patrol PDF diploma — background artwork, the
  Nathejk Impact font, the team's number and name, and a finish time derived from the last
  checkpoint scan. It is hardcoded to 2024 and reachable only as its own service. The idea is
  proven; what is missing is a route from a participant's phone to it.
- Task 082's device run showed participants will get a **fragmented** track (iOS does not run
  a backgrounded web app). Anything built here has to be honest about that or it will present
  fiction as fact.

## 3. Goals

- A finishing team can see, in the app, that they finished — something worth showing a parent.
- A member can see the route their team covered, presented honestly, including where it has
  gaps.
- A member can see every registration the team collected during the race, distinguishable by
  what caused it (a post, a bandit, something else).
- The position track becomes a feature *for the participant*, not only for the organizers, so
  the consent asked for in PRD 002 §11.1 is reciprocated.
- Nothing here weakens the during-race app: this is additive and must not slow the map or the
  contact list.

## 4. Non-Goals

- **Live tracking during the race.** Showing a team its own live positions is a different
  feature with real consequences for the race dynamic (a patrol that can watch itself on a map
  navigates differently, and a klan that can watch its patrols is a different game). Out of
  scope, and deliberately not a side effect of building this.
- **Any other team's data.** Not their track, not their scans, not their diploma.
- **Ranking, scoring or comparison between teams.** Nathejk is not scored that way, and
  inventing a leaderboard from scan timestamps would change what the event is. Explicitly out
  unless the maintainer asks for it.
- **Editing or correcting scans or tracks.** The view is read-only. A wrong scan is fixed
  upstream, by the people who own the scanning.
- **Printing, posting or emailing the diploma.** In-app display and whatever the browser's own
  share/save affords. No mail pipeline here.
- **Photos of the team.** The `diplom` service pulls a team photo from `natpas.nathejk.dk`, and
  PRD 007 owns portraits. Neither is in scope here.
- **A post-race view for personnel roles** (postmandskab, guide, samarit, gøgler, crew). They
  have no team and no track of a route they walked as a team. Their experience is a separate
  question.

## 5. User Stories & Scenarios

- As a **spejder**, I want to see a diploma with our patrol's name and finishing time so that
  I have something to show my parents when I get home.
- As a **spejder**, I want to see the route we walked on the map so that I can work out where
  we actually were during the parts of the night I could not place.
- As a **spejder**, I want to see every post we were scanned at and every time a bandit caught
  us, in order, so that we can reconstruct the night and argue about it.
- As a **bandit**, I want to see where my klan operated and which patrols we caught, for the
  same reason.

**Primary happy path**

1. A patrol finishes. The app shows, on its home surface, that the race is over and there is
   something new to look at.
2. They open it and see the diploma first — the emotional payoff, and the thing a parent asks
   about.
3. Below it, the route: the team's members drawn on the familiar topographic map, with gaps
   shown as gaps.
4. Below that, the timeline: every scan in order, with what it was and when.

**Edge cases and error scenarios**

- **A member who never granted location.** They have no track. Their absence from the map must
  not read as "they were not there" — and see §11 Q3, because it also silently reveals who
  declined.
- **A member whose track is tiny** because their phone was in a pocket all night. The common
  case, per task 082.
- **A team that did not finish** (retired, or was driven home). A diploma that says they
  finished would be a lie; a page that says nothing would be worse.
- **A member with no team at all** (personnel). Consistent with `GET /api/patrol/scans`
  today: an empty result, not an error.
- **The event has no scans yet** because it has not happened. The 2026 projection is empty
  right now; the view must be sane rather than broken before the race.
- **A team whose scans have no position.** A post can register a patrol manually, so a scan is
  listable but not always plottable — `internal/scans` already models this with nullable
  coordinates.

## 6. Requirements

### Functional

#### Diploma

- [ ] A finishing team can see a diploma in the app, carrying at minimum the year, the team's
      number and name, and when they finished.
- [ ] The finish time is derived from a defined event, not typed in — the `diplom` service uses
      the team's scan at the **last checkpoint**, and this must use the same definition or state
      why it differs.
- [ ] A team that did not finish sees an honest alternative rather than a diploma claiming
      otherwise.
- [ ] The diploma is reachable from the app without the participant needing a second login.

#### Team tracks

- [ ] A member can see the tracks of **their own team's members** for the event, on the same
      map component the live map uses (PRD 002).
- [ ] Tracks are read from the `TELEMETRY` stream by subject filter, per member of the team —
      `TELEMETRY.<year>.track.<personId>.reported`.
- [ ] Duplicate points are collapsed on `(person, timestamp)`. **This is the contract task 083
      established**: a retry after a timeout can legitimately publish the same point twice, and
      the reader is the only place it can be removed.
- [ ] **Gaps are rendered as gaps.** Joining two points either side of a two-hour hole draws a
      confident straight line through terrain nobody walked. Break the polyline on a time delta
      above a stated multiple of the sampling interval.
- [ ] The view states what the track covers, so a gap is not read as "we stood still here".
- [ ] Each member's track is distinguishable from the others'.
- [ ] Simplification is applied for legibility, and the layer that does it is recorded
      (server-side before the response, or client-side before rendering).
- [ ] Legible on both topographic and aerial backgrounds, and at night on a dimmed screen.

#### Scan history

- [ ] A member can see every registration their team collected, newest-first or in race order,
      with a stated choice.
- [ ] Each entry shows **what kind** of registration it was — a checkpoint, a bandit catch, or
      another kind — and when.
- [ ] Registrations with a position are plottable on the map alongside the tracks; ones without
      are still listed (a manual registration at a post has no coordinates).
- [ ] The mock `internal/scans` source is replaced by a real projection of
      `NATHEJK.*.qr.*.scanned`, behind the existing `scans.Source` interface so handlers do not
      change.

#### Access and gating

- [ ] The team is resolved from the **session**, never from a request parameter. A `teamId` in
      a query string is exactly how one team ends up reading another's night.
- [ ] There is a test proving a member cannot read another team's track or scans by passing an
      id.
- [ ] Availability is gated by a defined rule (see §11 Q1), and the gate is testable.
- [ ] `401` unauthenticated; an empty result rather than an error for a user with no team.
- [ ] All new endpoints carry OpenAPI annotations (repo rule).

### Non-Functional

- **Performance.** One team of six at 30 s sampling is roughly **8,600 points across ~2,160
  stream messages** — cheap to read on demand. The page must not block the map or the app
  shell; it is an extra view, not a dependency.
- **Honesty over polish.** Every requirement above that mentions gaps, non-finishers or missing
  tracks exists because the attractive rendering is the dishonest one. A post-race view that
  overstates what happened is worse than none: participants were there and will notice.
- **Privacy.** This introduces a **new disclosure**: a member's recorded route shown to their
  teammates. PRD 002's consent copy says the route goes to the organizers and says nothing
  about teammates. See §11 Q3 — this must be settled before the tracks view ships, not after.
- **Offline.** Post-race is the one part of the app most likely to be opened somewhere with
  signal (at home, on the bus). It does not need to work offline, but it must fail like the
  rest of the app now does (task 090) rather than render blank.
- **Accessibility.** The scan timeline must be readable as a list, not only as map pins —
  including for a member whose team's scans have no coordinates at all.

## 7. UX / UI Notes

A new route, reachable from the bottom nav or its overflow once the gate opens, and announced
on the map or home surface when it first becomes available — a participant should not have to
be told by a leader that it exists.

Three stacked sections in emotional order, not technical order: **diploma → route → scans**.
The diploma is what a 12-year-old wants and what a parent asks about; the data is what they
argue over afterwards.

- Reuse the existing map component (`vue/src/components/map/EventMap.vue`) and its base layers
  rather than a second map implementation.
- The scan timeline is a list of the same shape as the existing `ScanList`, extended with the
  registration kind.
- Headlines use `font-nathejk`, per `.rules`. The diploma is the one place in the app where
  more visual ceremony than usual is appropriate.
- New view under `vue/src/views/`, registered as a `destination` in
  `vue/src/config/navigation.ts` if it should appear in the nav, or as a plain route if it is
  announced contextually instead. Decide as part of the gating question.

## 8. Technical Considerations

**Frontend (Vue 3 / TS).** One new view; reuse of `EventMap` and the scan list; a store for
the post-race payload. No new map library. The track polyline needs a break-on-gap
representation, which Leaflet supports natively by drawing a multi-segment polyline
(`L.polyline` accepts an array of arrays).

**BFF (Go).** Three pieces, of which only one is routine:

1. **A scan projection** — new, consuming `NATHEJK.*.qr.*.scanned` into a read model, replacing
   the mock behind `scans.Source`. Routine, and follows the `person`/`checkpoint` pattern.
2. **A track reader** — reads the `TELEMETRY` stream by subject filter and does not project.
   This is a **deliberate departure** from "reads come from projections" (PRD 008 §8) and the
   reasoning must travel with it: projecting tracks means putting millions of points into
   MariaDB (827 participants × ~1,440 points per race) to serve a view each team opens roughly
   once, after the event. The exception is justified because the read is **bulk, cold and
   non-critical** — none of which is true of the member directory or the scan history, which
   stay on projections. Do not generalise it.
3. **The diploma** — see the constraint below.

**The diploma and the no-cross-service rule.** The org's architecture forbids a service calling
another service's HTTP API, so `hej`'s BFF **may not** call `diplom`. Three options, and this
needs a decision (§11 Q2):

- **(a) Link the participant's browser to the `diplom` service.** A link is not a
  service-to-service call — the browser fetches it — so this respects the rule. Cheapest by
  far, and reuses artwork and layout that already work. Costs: a second origin (so a second
  authentication story, or a public URL guessable by team number), and `diplom` is hardcoded to
  2024 and would need the 2026 data.
- **(b) Render the diploma in `hej`** from its own projections. One origin, one session, one
  deployment; the participant never leaves the app. Costs: reimplementing PDF/image generation
  (`go-pdf/fpdf` is a small dependency) and re-creating the artwork pipeline.
- **(c) Move diploma generation into `hej` and retire `diplom`.** Cleanest end state, largest
  change, and touches a repo outside this one.

My recommendation is **(b) for the in-app view**, because a post-race page that bounces a
12-year-old to another domain to log in again will simply not be used — but this is the
maintainer's call and (a) is a legitimate way to ship something this year.

**API endpoints (all new, all requiring OpenAPI annotations):**

| endpoint | purpose |
|---|---|
| `GET /api/team/track` | the team's members' tracks, read from `TELEMETRY` |
| `GET /api/team/scans` | the team's registrations, from the new projection |
| `GET /api/team/summary` | diploma facts: team number, name, finish time, finished-or-not |

`GET /api/patrol/scans` already exists for the live map and is patrol-scoped; whether the
post-race scan list reuses it or gets its own endpoint should be decided when the projection
lands, not now.

**Data / storage.** A new scan read model (schema owned by its projection, per PRD 008 §8).
No storage for tracks. Note the scan event's coordinates arrive as **strings**
(`"lat": "55.5942559"`), so parsing is a real step with a real failure mode — a scan with an
unparseable position must remain listable rather than being dropped.

**Dependencies & risks.**

- **Blocked on nothing technically** — tracks are flowing and scan events exist.
- **The scan-kind classification is unresolved** and is the biggest unknown in the whole PRD:
  the event body carries `scannerId` and `scannerPhone` but no kind, so "checkpoint or bandit"
  must be derived from who scanned. A spot check of a 2025 `scannerId` against the person
  projection found no match, so the join is not obvious. See §11 Q4.
- **2026 has no scans yet.** Everything here can be built against 2025 data, which is complete
  on the stream — a genuine advantage, and it should be used rather than waiting for the event.
- **PRD 007 (portraits) and PRD 003 (profile)** are unrelated and must not become
  prerequisites.

## 9. Success Metrics

- A majority of finishing teams open the post-race view at least once within a week of the
  event. This is the whole point; if nobody looks, the feature is decoration.
- The recorded-track coverage reported by the view is consistent with what task 082's status
  page measured on the device — i.e. the view is not quietly hiding gaps.
- Zero cross-team disclosures. Measured by the access-control tests, not by absence of
  complaints.
- The consent claim in PRD 002 §11.1 ("så vi bagefter kan vise jer jeres egen rute") becomes
  true, and the privacy copy can say so without qualification.

## 10. Rollout / Task Breakdown

Sequenced so the two halves that carry risk come first, and so something shippable exists even
if the diploma decision takes time. The scan projection is worth doing first regardless: it
also removes the mock that the *live* map currently depends on.

- [ ] Task: Project team scans from `NATHEJK.*.qr.*.scanned`, replacing the mock `scans.Source`
- [ ] Task: Decide and document how a scan's kind (checkpoint / bandit / other) is determined
- [ ] Task: `GET /api/team/track` — read the team's tracks from `TELEMETRY` by subject filter,
      deduplicated on (person, timestamp)
- [ ] Task: Decide the post-race availability gate (§11 Q1) and implement it
- [ ] Task: Settle the teammate-visibility privacy question (§11 Q3) and align the consent copy
- [ ] Task: Post-race view — route rendering with honest gaps, on the existing map
- [ ] Task: Post-race view — scan timeline, including scans without a position
- [ ] Task: Diploma — implement the chosen option from §11 Q2
- [ ] Task: `GET /api/team/summary` — the diploma facts, including finished-or-not
- [ ] Task: Verify the whole view against **2025** data on a phone viewport

## 11. Open Questions

1. **When does the post-race experience become available?** Carried over from task 086, and
   still the gating decision. A date, an event-status flag, or always-on — where always-on
   means a team can watch its own positions *during* the race, which §4 rules out as a separate
   feature. An event-status flag is the most honest mechanism, but nothing in `hej` models event
   status today.
2. **Who owns the diploma** — link out to the `diplom` service (a), render it in `hej` (b), or
   move generation here (c)? See §8. Also: does the 2026 diploma artwork exist, and who makes
   it?
3. **Should a member see their teammates' individual tracks?** The consent copy says the route
   goes to *the organizers*. Showing it to teammates is a disclosure nobody agreed to, and it
   has two concrete consequences: a member who declined location is visibly absent from the
   map, and a member who left the group is visibly elsewhere. Options: show only a merged team
   route with no attribution; show per-member tracks but only to that member; ask for consent
   at the point of sharing; or accept it and say so in the consent copy up front. **This should
   be settled before the tracks view is built, not retrofitted.**
4. **How is a scan classified?** The event carries `scannerId`/`scannerPhone` but no kind, and a
   2025 `scannerId` did not resolve against the person projection. Is the kind derivable from
   the scanner's role, from a checkpoint/control-group association, or does it need a change
   upstream in the scanning app? Until this is answered, "scans from checkpoint, bandits, etc"
   cannot be labelled correctly — and mislabelling a bandit catch as a checkpoint would be
   worse than showing neither.
5. **Do klans and bandits get the same view?** A bandit's "team" is a klan, and their night is
   catches rather than posts. The same three sections may work with different emphasis, or the
   bandit view may want inverting (who *we* caught, rather than who caught us). Same question
   for gøglere, who staff the night but are not a team.
6. **Does a non-finishing team get anything?** A "you retired" page is honest but bleak, and
   plenty of Nathejk patrols do not finish. What should they see?
7. **How long does the post-race view stay available** — until the next event, or indefinitely?
   Related: the `TELEMETRY` stream's retention is currently indefinite (task 081), so the data
   will outlive any UI decision made here.
