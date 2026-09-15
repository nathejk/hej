# 275 — Arrows point at the whole next line, and never back at the start

**Status:** done
**Priority:** high
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

Bug reported by the product owner against tasks 263/273:

> The two arrows are pointing at different things, one is pointing at start, the other point at one of
> the checkpoints at checkgroup 1 - team has moved on from start, then the arrows should point to all
> checkpoints in next line of checkpoints

I read the live data before changing anything, and it settles both halves. The dev database's route:

| line | checkgroup | posts |
|---|---|---|
| 0 | Start | Start *(not sited)* |
| 0 | Starter | Afgang, Til Gøgl *(one sited)* |
| 1 | Postlinje 1 | **Post 1A, Post 1B** |
| 2 | Postlinje 2 | Post 2A, Post 2B |
| 3 | Oplevelsespost | Indgang |
| 4 | Postlinje 3 | Post 3A, Post 3B |
| 5 | Postlinje 4 | Post 4A, Post 4B |
| 6 | Mål | Mål |

So a *postlinje* is a **line of posts** — an A and a B — and a patrol heads for the line, not for one
nominated post. Two things are wrong today:

**1. One arrow per line was the wrong call.** Task 273 had me pick the single nearest post in a
checkgroup, on the reasoning that its posts are alternatives. They are alternatives, but that does not
mean the app should choose for the patrol: both posts of the next line are real destinations and the
patrol needs to see both to decide. Every post in the next line gets an arrow.

**2. Arrows must not point back at the start.** The arrow at "start" is `Afgang`, in the line-0 group
`Starter`. The patrol has left it, but nothing in the scan data says so — departing is recorded at
check-in, not as a scan at a post, so no attributed scan exists for line 0 and the old rule treated it as
unfinished.

Two rules fix that, and both belong in the **BFF** rather than the client, because the client cannot
honestly answer either:

- **Progress along the route is monotonic.** A line is behind you if you have been scanned at it *or at
  any later line*. A patrol seen at Postlinje 2 is past Postlinje 1 whether or not the rota recorded the
  earlier scan — which also makes the feature survive a gap in the postmandskab rota.
- **Having started puts the first line behind you.** `person.HasStarted()` is the single existing
  definition of "has begun the event" (it deliberately exists so a second one does not get born), and a
  patrol using this app during the race has departed. The first line in route order is the start — line 0
  here, `Mål` being line 6 — so starting retires it.

Only the BFF knows `HasStarted`: the client sees it folded into `confirmation_required` together with
other conditions, and reconstructing it would be exactly the duplicate definition
`person.HasStarted`'s comment warns against.

So `/api/checkpoints` gains **`next_checkgroup`**, and the client's job shrinks to "arrow every revealed
post in that line which is off screen". `nextCheckpointGroups` goes away.

## Acceptance Criteria

- [x] `/api/checkpoints` returns `next_checkgroup`, with OpenAPI annotations updated.
- [x] Every post in the next line gets an arrow; no post outside it does (test).
- [x] A line is passed when the patrol scanned at it (test).
- [x] A line is passed when the patrol scanned at any *later* line (test) — the monotonic rule.
- [x] The first line is passed once the patrol has started (test), and not before (test).
- [x] A line whose posts are none of them revealed or sited is skipped rather than becoming a dead next
      line with no arrows (test).
- [x] No next line at the end of the route → no arrows (test).
- [x] Client-side grouping removed; the client filters on `next_checkgroup`.
- [x] All four Go gates, frontend suite, type-check and build clean.

## Progress Log

- 2026-09-15 — Reported. Queried the dev database's `checkgroup`/`checkpoint` tables before theorising,
  which is what showed that a postlinje is an A/B pair and that line 0 is the start — the two facts the
  fix turns on.
- 2026-09-15 — **Withdrew half of my own task 273 decision.** I had the app pick the nearest post in a
  line, reasoning that its posts are alternatives. They are, but that does not license the app to choose:
  both are real destinations, and the patrol decides when they arrive. `pickReachable` is gone and every
  post in the line gets its own arrow, each with its own distance so the alternatives can be compared —
  which is the point of showing both.
- 2026-09-15 — Also narrowed from *three* lines to *one*. A patrol walks to one line; lines beyond it are
  clutter they cannot act on. `MAX_ARROW_GROUPS` and the whole client-side grouping module are deleted.
- 2026-09-15 — **The start needed a different signal entirely.** Departing is recorded at check-in, not as
  a scan at a post, so no attributed scan will ever retire line 0 — the old rule would have pointed at
  `Afgang` for the entire race. Two rules, both in the BFF:

  1. **Monotonic progress**: a line is behind the patrol if they were scanned at it *or at any later line*.
     This is worth having on its own merits — the postmandskab rota is fed from outside this repo and can
     be incomplete (task 260 counts exactly that), so a patrol demonstrably at Postlinje 2 must not be
     pointed back at Postlinje 1 because nothing attributed their earlier scan.
  2. **Starting retires the first line**, via `person.HasStarted()` — the codebase's single definition of
     "has begun the event", which exists specifically so a second one does not get born.

- 2026-09-15 — Decision: identify the start as **the first line in route order** (every group sharing the
  lowest sortOrder — both line-0 groups here), not by name and not by a literal sortOrder. Matching on
  "Start" would break the year somebody renames it. Recorded as an assumption in the code: it holds
  because every year's route begins at a start line and ends at `Mål`.
- 2026-09-15 — **Moved the decision to the BFF** as `next_checkgroup`. It needs route order across
  checkgroups and the started fact; the client sees the latter only folded into `confirmation_required`,
  so re-deriving it client-side would fork exactly the definition that is meant to be singular. The
  client's arrow logic shrinks to "filter on `next_checkgroup`", which also deleted a module and its spec.
- 2026-09-15 — Added a guard the report did not ask for: a line with **nothing drawable** (no revealed or
  no sited posts) is skipped rather than becoming the "next" line. `Postlinje 3` is in that state in the
  live data right now, so without this the arrows would have gone silent at Postlinje 2 with a group id
  set — indistinguishable, from the outside, from the feature being broken again.
- 2026-09-15 — `Revealed` now returns a struct carrying both the checkpoints and the next line, rather
  than gaining a second method: both answers come from the same four reads, and asking twice would give
  them a chance to disagree.
- 2026-09-15 — ✅ 10 new reveal tests, 3 new endpoint tests, 5 new store tests; 604 frontend tests,
  type-check and build clean; all four Go gates clean.
- 2026-09-15 — Done.
