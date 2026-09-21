# 341 — Patruljens egen side

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §6, §7. The page itself, at `/offentligt/patrulje/{number}`, server-rendered and complete without
JavaScript. The map is a separate task (342) and a progressive enhancement on top of this.

Layout, from the maintainer's brief:

```
┌──────────────────────────────────────────────┬──────────┐
│ Patrulje 42 · Ørnene                         │ ┌──────┐ │
│ 1. Søllerød Gruppe · Det Danske Spejderkorps │ │diplom│ │
│ mindst ~24 km                                │ └──────┘ │
├──────────────────────────────────────────────┴──────────┤
│   map of the race area (task 342, enhancement)          │
├─────────────────────────────────────────────────────────┤
│   the scans, in order, as a list                        │
└─────────────────────────────────────────────────────────┘
```

Patrol name in `font-nathejk` and large; gruppe and korps beneath it, smaller — a patrol is known by its
name, and the korps is context. The distance is the one number on the page, so it gets room. The diploma
thumbnail is the visual anchor and the thing a parent clicks.

**The scan list is not the map's fallback, it is a requirement.** Every registration the patrol collected,
in race order, with its kind and time — **including the ones with no position**, which are listable but not
plottable (a post can register a patrol manually). A member whose patrol's scans have no coordinates at all
must still get a usable page.

**The page must state what the track covers** — that it records where a phone had the app open, not where
the patrol walked — in one sentence a parent will read. Without it, a gap reads as "they stood still here".
This sentence is the honesty requirement of the whole feature; do not shorten it into meaninglessness.

**The not-yet page** (gate closed, task 330) is a **first-class screen, not an error path**: it is what
every link to a patrol page shows for most of the year. It says what the page will contain and when it
appears, carries **no patrol data at all**, and is **indistinguishable from the answer for a patrol number
that does not exist** — otherwise the URL space becomes a live finish-order feed.

Depends on 330 (gate), 338 (read model), 339 (distance). The diploma thumbnail's source is task 345; until
then leave the slot structurally present but empty rather than faking it.

## Acceptance Criteria

- [x] `GET /offentligt/patrulje/{number}` renders the header (name, gruppe, korps), the distance, the
      diploma slot and the ordered scan list. OpenAPI annotated.
- [x] Complete and useful with **JavaScript disabled**; the map's absence degrades to nothing worse than a
      missing section.
- [x] Scans listed in race order with kind and time, including un-positioned ones.
- [x] The track-coverage sentence is present and readable by a parent.
- [x] Headline uses `font-nathejk`; no font family set anywhere per-component (`.rules`).
      *— via the shared layout's `h1`, as task 332 established; the font is unreachable outside the Vue
      bundle so it falls back to the same Impact stack `--font-nathejk` does.*
- [x] `korps` rendered as its label; `andet`/empty omitted (task 338).
- [x] Not-yet page implemented, carrying no patrol data, identical to the unknown-patrol answer.
- [x] A patrol with no track, or no positioned scans, still reads as a page about a patrol that was there.
- [x] Unauthenticated, ignores the session cookie, `noindex, nofollow`.
- [x] Renders on the oldest device available and with CSS disabled. *— constraints held in the source; the
      device run is task 348.*
- [x] **Added:** the finish time, moved here from task 338.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §7 / §10 (Phase 2). Depends on 330, 338, 339.
- 2026-09-19 — Picked up. **The gate is finally consulted here.** Task 332 registered this route answering
  the not-yet page for everybody and deliberately did *not* call the gate — a call that looked like a check
  and then rendered the same page regardless would have been worse than no call. The gate and the open
  branch arrive together, with the tests that prove the closed path stays closed.
- 2026-09-19 — **Changed `publicgate.Gate.For` to return a `Verdict` carrying the finish time**, rather than
  having the page re-derive it. Finding it is the *same work* as opening the gate — both are "the patrol's
  scan at the last checkgroup" — so returning it costs nothing, and PRD 011 §0b.3 is explicit that the gate,
  the page and the diploma must share one definition of "finished". Now there is only one place that looks.
- 2026-09-19 — Two properties of the finish time worth recording:
  * `FinishedAt` is nil for a **backstop-opened** page, and `Open()` does not imply `Finished()`. Those are
    different facts and the diploma follows the second, not the first.
  * The **earliest** matching scan is taken, not the latest. A patrol re-scanned at the finish, or scanned
    at two posts in the same group, finished when it first arrived — taking the later time would quietly
    add the minutes it stood there to its night.
  * An **override does not invent or discard** a finish time: it is about visibility, not about whether the
    patrol reached the line. Asserted in both directions.
- 2026-09-19 — The diploma slot is **absent, not empty**, for a patrol with no finish. An empty frame where
  a diploma should be is a page pointing at what is missing (task 346). Asserted, along with the stronger
  requirement that nothing on such a page says the patrol gave up — `udgået`, `opgav`, `gennemførte ikke`
  are all checked for.
- 2026-09-19 — Registrations are listed in **race order, oldest first** — the opposite of the app's list,
  deliberately and stated because §6 asks for the choice to be explicit: during the race a participant wants
  the last thing that happened, but a page read afterwards is a story, and a story is read forwards.
- 2026-09-19 — Scans are read **once** for both the list and the distance. Two reads would be two chances
  for them to disagree, and a page showing nine posts with a distance built from eight is a page nobody can
  explain.
- 2026-09-19 — The gate is asked about the **team id**, never the number a visitor typed. Asserted with a
  recording fake, because this is the kind of thing that works either way until a number collides.
- 2026-09-19 — **A known weakness, written into the code rather than left to be discovered.**
  `trackPointsForDistance` returns nil, so the track raises no leg and the distance is the pure scan-leg
  floor. The cause: `patroltrack.Point` carries only a latitude and a longitude — correctly, since the map
  draws a line — but `distance.Compute` matches track points into legs **by time**. The fix needs a
  timestamped shape, which task 342's map endpoint will want anyway. So the figure is a floor by *omission*
  as well as by design, and the function says so instead of silently returning nil.
- 2026-09-19 — `FinishedLabel` is precomputed in Go rather than formatted in the template, because
  `html/template` cannot hand a `*time.Time` to a `time.Time` formatter — and the alternative (a formatter
  taking `any`) moves nil-handling into a template function, which is the one place a nil surfaces as a
  half-rendered page rather than an error.
- 2026-09-19 — ✅ **A test of mine was wrong about the clock, in an instructive way.** I expected the finish
  time to read "19. september"; it renders "20. september kl. 07:00", because the fixture starts at 21:00
  and finishes ten hours later. That is exactly what a night race does. Fixed the expectation and pinned it,
  with a comment — so a timezone or date-rollover bug shows up in a test rather than on a patrol's page.
- 2026-09-19 — Verified against real 2026 data in the dev stack. `/offentligt/patrulje/1` renders
  **"Patrulje 1 · Skjoldungerne 2"** with *"Skjoldungerne, Kongslejre division · Det Danske Spejderkorps"* —
  the korps as a label, from the slug. The page is **backstop-opened** (the last checkpoint closed on
  2026-09-20), so it correctly shows no diploma slot and no finish time.
- 2026-09-19 — ✅ **And the page showed no distance, which turned out to be task 339's filter working.** The
  dev scans jump 0.1° of latitude — about 11 km — in **two seconds**, because `cmd/simscan` generates them
  in a burst. Implied speeds of thousands of km/h, so every leg is excluded as a vehicle and there is no
  figure. That is the right outcome and worth naming: the filter means nonsense input produces **no number**
  rather than an absurd one, and a page that says nothing is far better than a page telling a patrol it
  walked 35 km while standing still.
- 2026-09-19 — `gofmt`, `go vet ./...` and `go test ./...` clean. Moving to done.
