# 086 — Post-race team track view

**Status:** open
**Priority:** medium
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.1. After the race, a team can see its **own** entire track — the payoff that
makes the recording a feature for participants rather than only for the organizers.

Depends on 081, 083 and 084 (there is nothing to read until tracks are being shipped).

## Read path: from the stream, not a projection

A deliberate departure from "reads come from projections", and worth understanding before
copying it elsewhere.

One team of six at 10 s sampling is about **26,000 points across ~2,160 messages** — cheap
to read back on demand with a subject-filtered consumer. Projecting the alternative means
putting **millions** of points into MariaDB (827 participants × 4,320 points per race) to
serve a view that each team opens roughly once, after the race is over.

The exception is justified by the read being bulk, cold and non-critical — none of which is
true of the member directory or the scan history, which stay on projections.

## Rendering

26,000 raw points is more than a phone wants to draw as a polyline, and more than is
visually meaningful. Simplification (Douglas–Peucker or equivalent) is expected; decide
whether it happens server-side before the response or client-side before rendering, and
record which.

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
