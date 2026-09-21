# 347 — Rate limits and cache headers on the public routes

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

PRD 011 §6, §8. Every new public route gets a by-IP read limit and a deliberate cache header.

**Why this is its own task and not an afterthought:** every other route in this repo is behind a login, which
is an extremely effective rate limiter. These are not. And the load profile is genuinely new — this page is
shared in family WhatsApp groups the morning after the event, so **a hundred simultaneous visitors is the
normal case**, not the stress case.

Follow `publicGlimtReadLimiter` in `go/cmd/api/glimtpublic.go`, and carry across the reasoning already
recorded there:

- **A separate limiter from the member-keyed ones.** An IP is a much worse key — a school or a workplace
  behind one NAT shares a single budget — so the ceiling has to be correspondingly generous, and mixing the
  two drags the member-keyed limit down with it.
- **Cache-Control: public, max-age=60** on the HTML, the same short window the glimt page chose: shareable,
  but a takedown lands quickly. That window is also what task 335 depends on for "promptly", so do not
  lengthen it without reading that task.

**The expensive route is the patrol map endpoint** (task 342), because it reads a merged track. Task 340
already requires that result to be cached or materialised per patrol — verify here that the public endpoint
actually reads the cache and cannot be made to walk the stream by a stranger with a loop.

## Acceptance Criteria

- [x] Every new public route (`/offentligt*`, `/api/public/*`) is by-IP rate limited, using a limiter separate
      from the member-keyed ones.
- [x] Limits are generous enough for a NAT'd school; the chosen ceilings and their reasoning recorded here.
- [x] `Cache-Control` set deliberately on every public response, `max-age=60` on HTML unless a route argues
      otherwise in a comment.
- [x] The patrol map endpoint demonstrably reads the cached/materialised track and cannot be driven to walk
      `TELEMETRY` per request. **It could, until this task — see the log.**
- [x] Media responses cache longer than HTML — content-addressed bytes do not change.
- [x] A rate-limited response returns a Danish message a visitor can understand, matching the existing
      `RateLimitMessageResponse` usage rather than a bare 429.
- [x] Load-checked against the morning-after scenario: a burst on one patrol page and on the frontpage.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (throughout).
- 2026-09-21 — **Done, and the load check found a real hole rather than confirming the design.**

  ### The hole: a cache is not a single-flight

  The patrol map endpoint reads the merged track through a per-patrol cache with an hour's TTL (task 340), and
  that looked like enough. It is not, on an **unauthenticated** route with this load profile: every visitor in
  the first second misses a cold cache, so every one of them walks `TELEMETRY` and merges it. Nothing bounded
  how many — only the scheduler.

  Measured, not reasoned: a burst test of 400 concurrent requests on one patrol produced **two** merges under
  `-race`, where the slower scheduling widens the window. Two is not a problem; the absence of a bound is, and
  a stranger with a concurrent loop is exactly who exploits it.

  So `patrolTrackReader` now **claims** a miss: the first caller merges and the rest wait for its answer. One
  merge per patrol per TTL, however many people ask. Confirmed by disabling the claim and watching the new
  test fail, which is the only way to know a concurrency test tests anything.

  Two details that would be easy to get wrong and are asserted:
  - **A failed merge is shared but not cached.** Caching it would turn one database blip into a minute of
    blank maps; swallowing it for the waiters would hand them an empty track, which the page renders as "this
    patrol recorded nothing" — a lie that looks like data.
  - The result is stored **before** the waiters are released, or a waiter could wake up, miss the cache, and
    start the merge again.

  ### Media gets its own budget

  Everything public shared one IP-keyed ceiling. That is wrong by construction: an album page is 1 HTML
  request and up to **60** thumbnails, and the HTML does database reads while a thumbnail is a blob read with
  an ETag and a year-long cache. One budget means the cheap and numerous starve the expensive and few — a
  visitor scrolling two albums could spend the allowance their next page load needs, and the failure would
  look like the site being broken rather than like a limit being hit.

  So `publicMediaReadLimiter` (`PUBLIC_MEDIA_READS_PER_MINUTE`, default **12,000/min per IP**) now serves the
  album and public-glimt media routes, while pages and JSON keep the existing 3,000/min. The arithmetic behind
  12,000, which is the worst *honest* case rather than an attack:

  | scenario | requests/min from one IP |
  |---|---|
  | one album page | ≈ 61 |
  | a family, 4 devices, 5 albums | ≈ 1,220 |
  | 30 devices behind one school NAT | ≈ 9,150 |

  A scripted scrape runs at thousands per *second*, so the ceiling separates the two cases cleanly. The
  asymmetry is deliberate: a throttled thumbnail costs a family a broken-looking page the morning after, and
  an unthrottled one costs us bandwidth.

  ### Cache headers

  The HTML already carried `public, max-age=60`. The two JSON endpoints carried **nothing** — now
  `publicJSONCacheControl`, the same 60 seconds, because the responses are identical for every caller (so a
  shared cache absorbs the burst) and because a takedown must land promptly (tasks 335, 343). Not longer, even
  though a finished patrol's route never changes again: a cached copy is the one thing a takedown cannot reach.

  Media stays at a year, `immutable` — content-addressed bytes mean a changed photograph is a changed URL, so
  there is nothing stale to serve. A test pins the *asymmetry* rather than the two values, since that is the
  property worth keeping.

  **Failures are deliberately not cached**, and that is asserted too: a 503 pinned for a minute is a blip that
  every shared cache repeats back at you.

  ### Also fixed: my own test doubles were lying

  The burst test's counters (`trackPeople.calls`, `trackPoints.calls`, `patrolStore.asked`) were unsynchronised,
  so `-race` flagged them — and, worse, "exactly one read" cannot be checked with a count that loses
  increments. Mutex-guarded now, with accessors. The first version of the burst test passed for the wrong
  reason; the race detector is what turned it into a measurement.

  `gofmt`, `go vet`, `go test ./...` clean; `-race` clean over `cmd/api`'s concurrency tests and all of
  `internal/...`. No dev-stack run: Docker is down on this machine, and every claim here is asserted in a
  test rather than observed by hand.
