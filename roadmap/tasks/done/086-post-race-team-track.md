# 086 — Post-race team track view

> **CLOSED 2026-08-28 WITHOUT BEING IMPLEMENTED — superseded by PRD 011.**
> Filed under `done/` because the board defines only `open | doing | done` and this task is
> off it; nothing here was built. The acceptance criteria below are left **unchecked** on
> purpose, and the analysis they rest on was carried into PRD 011 — see the progress log.

**Status:** done
**Priority:** medium
**Created:** 2026-08-26
**Picked up by:** —
**Started:**
**Completed:** 2026-08-28 (closed, not implemented)

## Description

PRD 002 §11.1. After the race, a team can see its **own** entire track — the payoff that
makes the recording a feature for participants rather than only for the organizers.

Depends on 081, 083 and 084 (there is nothing to read until tracks are being shipped).

## Read path: from the stream, not a projection

A deliberate departure from "reads come from projections", and worth understanding before
copying it elsewhere.

One team of six at 30 s sampling is about **8,600 points across ~2,160 messages** — cheap
to read back on demand with a subject-filtered consumer. Projecting the alternative means
putting **millions** of points into MariaDB (827 participants × 1,440 points per race) to
serve a view that each team opens roughly once, after the race is over.

The exception is justified by the read being bulk, cold and non-critical — none of which is
true of the member directory or the scan history, which stay on projections.

## Rendering

8,600 points per team is drawable as a polyline, though simplification
(Douglas–Peucker or equivalent) is still worth it for legibility and for the case where a
member's track is denser than expected. Decide whether it happens server-side before the
response or client-side before rendering, and record which.

## The track will have gaps, and the UI must not lie about them

A web app cannot record while backgrounded (task 082 — no background geolocation on any
platform, and Screen Wake Lock is released when the document goes inactive). Fragmented
tracks are **accepted** (maintainer, 2026-08-26), so the recorded track covers everywhere the
member was while the app was open — on a night hike with the phone in a pocket, that means
gaps, possibly large ones.

Accepting the gaps does not mean rendering them carelessly, and this view is where it shows.
The obvious rendering is wrong: joining two points either side of a two-hour gap draws a
straight line through terrain the member never walked, and it looks authoritative. Options:
break the polyline where the time delta exceeds some multiple of the sampling interval, render
gaps distinctly, or state the coverage plainly. Whatever is chosen, do not present a gap as if
it were a route.

The PRD's phrase "the team's entire track" cannot be delivered literally; what is deliverable
is honest coverage of when the app was open.

## Access

The team sees its own track and no other team's. Same race-dynamic reasoning that keeps
portraits from crossing populations in PRD 007 — and here it is also simply not the
organizers' data to hand out.

Resolve the team from the **session**, never from a request parameter. A `teamId` in the
query string is exactly how one team ends up reading another's route.

## Acceptance Criteria

- [ ] `GET /api/team/track` behind `requireAuth`, with OpenAPI annotations
- [ ] The team is resolved from the session; there is a test proving a member cannot read
      another team's track by passing an id
- [ ] `200` with the team's points, `401` unauthenticated, `200` with an empty result when
      the user has no team — consistent with `GET /api/patrol/scans`
- [ ] Points are read from the telemetry stream by subject filter, per member of the team
- [ ] Duplicate points (see task 083) do not appear twice in the result
- [ ] The track renders on the existing map, per member, legibly on both topo and aerial
      backgrounds and in the dark
- [ ] Simplification applied, with the layer that does it recorded here
- [ ] Gaps in coverage are rendered as gaps, not interpolated into a straight line through
      terrain nobody walked (see above)
- [ ] The view states what the track covers, so a member does not read a gap as "I stood
      still here"
- [ ] Behaves sanely for a member whose track is empty (never granted permission, or never
      moved), and for a team where only some members have tracks
- [ ] Does not block the map page from loading — this is an extra view, not a dependency of
      the live map

## Open question

**When does this become available?** "After the race" needs a definition the code can act
on: a date, an event-status flag, or simply always-on (in which case a team can watch its
own live positions during the race, which is a different feature with different
implications for the race dynamic). Worth settling with the maintainer before building the
gate.

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
- 2026-08-28 — **Closed at the maintainer's request, superseded by PRD 011 (post-race
  experience).** Not abandoned: the scope grew. A post-race view is to show a **diploma**, the
  tracks of **all team members**, and **all scans the team collected** (checkpoints, bandits and
  whatever else registered them) — three things that only make sense together, which is a PRD's
  job to define rather than a task's.
- 2026-08-28 — Everything of value in this task has been carried into PRD 011 rather than left
  here to rot, specifically:
  * the **read-from-the-stream-not-a-projection** decision and the measurement behind it (one
    team of six is ~8,600 points across ~2,160 messages, against millions of rows if projected),
    plus the warning not to generalise the exception;
  * the requirement that **gaps are rendered as gaps** and never interpolated into a confident
    straight line through terrain nobody walked;
  * resolving the team from the **session, never a parameter**, with a test proving a member
    cannot read another team's data;
  * simplification, legibility on both base layers, and not blocking the live map;
  * the open question of **when** "after the race" begins, which is now PRD 011 §11 Q1.
- 2026-08-28 — Also fixed while here: a paragraph in this file was duplicated verbatim.
- 2026-08-28 — One thing this task did not consider and PRD 011 now raises as a blocking
  question: showing a member's route to their **teammates** is a disclosure PRD 002's consent
  copy does not cover — it promises the route goes to *the organizers*. It also silently reveals
  who declined location and who left the group. PRD 011 §11 Q3.
