# 324 — Load-test the post-race browse

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §0a.3, §9. **The load peak is the finish line, not the race.** A thousand people on the
same congested network at the same time, each paging through thousands of items, all pulling
media. This is the moment the feature is judged, and **it cannot be fixed on the night** — so it
gets measured before the first event.

Seed a realistic corpus (use `cmd/seed`): a full event's worth of holds, each with a plausible
number of glimt and media items. Then measure:

- `GET /api/glimt/hold/:number` — p50/p95/p99 and error rate under concurrent load
- `GET /api/glimt/feed` paging
- media and thumbnail serving, with and without the `immutable` cache hit

Record the numbers in this task file. PRD 019 §9 names p95 on the hold collection during the
hour after the race as a headline metric, so this task establishes the baseline it is measured
against.

## Acceptance Criteria

- [x] A seed path producing a realistic corpus (documented item counts)
- [x] Measured p50/p95/p99 and error rate for the hold collection, feed and media endpoints
- [x] Numbers recorded in this task's progress log, with the hardware they were taken on
- [x] Any index or query fix the measurements reveal is applied (or filed as a new task)
      — no index fix needed; the measurements revealed a **rate-limit** fault instead, and it is fixed
- [x] Cache-hit vs cache-miss media cost quantified

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.

- 2026-09-17 — Built `go/cmd/glimtload` — `-seed`, `-load` and `-clean` — and ran it. Numbers below.

  **Hardware:** Apple M2, 8 cores, 16 GB; Docker 29.1.2, 8 CPUs to the container; MariaDB 10.8 and
  the API in containers on the same host. **Loopback, so no network is in the path** — see the
  caveats.

  **Corpus:** 2,000 glimt across 120 holds (102 numbered; crew have none), 4,239 media rows, 400
  distinct media objects. Mean 17.1 glimt per hold, long-tailed — the busiest hold has 429, which is
  more than a real patrulje would post and is there deliberately to bound the worst case. Audience
  mix 55/30/15 group/nathejk/public. Media ~278 kB, thumbnails ~12.9 kB.

### Results — 50 concurrent, 50 distinct members, 2,000 requests each

| scenario | p50 | p95 | p99 | max | req/s | errors |
|---|---|---|---|---|---|---|
| `hold/:number` | 11.4 ms | 20.5 ms | 33.1 ms | 51.6 ms | 4,057 | 0% |
| `feed` (paged to offset 900) | 34.3 ms | 54.4 ms | 65.0 ms | 83.8 ms | 1,404 | 0% |
| thumbnail (cold) | 8.7 ms | 23.5 ms | 37.2 ms | 47.3 ms | 4,691 | 0% |
| thumbnail (304, `If-None-Match`) | 7.1 ms | 14.6 ms | 19.4 ms | 30.1 ms | 6,326 | 0% |
| full media | 9.7 ms | 20.9 ms | 27.3 ms | 34.6 ms | 4,453 | 0% |

### Concurrency sweep on the headline metric (`hold/:number`, 4,000 requests)

| concurrency | p50 | p95 | p99 | max | req/s | errors |
|---|---|---|---|---|---|---|
| 100 | 20.8 ms | 39.1 ms | 50.4 ms | 78.4 ms | 4,467 | 0% |
| 200 | 39.7 ms | 79.4 ms | 102.3 ms | 136.7 ms | 4,555 | 0% |
| 400 | 72.4 ms | 157.9 ms | 206.3 ms | 341.3 ms | 4,755 | 0% |

**Throughput plateaus at ~4,500–4,800 req/s and latency then scales linearly with concurrency** —
textbook saturation, with **zero errors at every level**. PRD 019 §9's headline metric (p95 on the
hold collection in the hour after the race) has a baseline of **20 ms at 50 concurrent, 158 ms at
400**.

- 2026-09-17 — **Cache-hit vs cache-miss, quantified.** Thumbnails average **12.9 kB** against
  **278 kB** for full media — a **21.5×** saving, so a thumbnail is 4.6% of a photograph. A grid page
  of ~42 items costs ~540 kB thumbnail-first versus ~11.4 MB if it fetched full media. A revisit
  costs **zero bytes**: the conditional GET returns 304 with no body, and it is also the fastest
  scenario measured (p50 7.1 ms, 6,326 req/s).

  So the two mitigations PRD 019 §8 leans on are real and now have numbers behind them rather than
  reasoning.

- 2026-09-17 — **The measurement found a fault, and it was not an index.** The very first run came
  back **70% 429s**: 600 requests served, 1,400 refused. That was the read limiter from task 311
  doing exactly what it was built to do — and following the arithmetic through, it was set wrong.

  A hold page is 20 glimt averaging 2.1 media, so **~43 requests including the JSON**. At 600 per
  minute that is **fourteen pages a minute, one every 4.3 seconds** — which a member flicking through
  grids at the finish line beats comfortably. The limiter would have fired for precisely the browse
  it exists to protect, and `main.go` already said so in as many words: *"If it ever fires for a real
  member, it is set wrong."*

  **Raised to 3,000/minute** (~70 pages a minute, one every 0.85 s), still some sixty times below
  what a script does. The arithmetic is recorded in `env.go` next to the default, and
  `TestGlimtLimitDefaults` now asserts the limit clears **60 hold pages a minute**, so the next
  person to tighten it has to argue with a number rather than a feeling.

  This is the whole value of the task: nothing about that was visible from reading the code, the
  limit looked generous, and the failure would have arrived as 429s in the one hour the feature is
  judged.

- 2026-09-17 — Two further faults, both in the harness rather than the service, recorded because
  they are the sort of thing that makes a load test lie:

  - **23–31% 404s** on the first multi-user run. My member ids came from `person` without
    `deleted = 0`, which `person.Get` filters on — so tombstoned rows produced sessions whose caller
    does not resolve, and every request took the "caller has no directory record" branch. 404s are
    fast, so they were also flattering the percentiles.
  - **~18% 403s** on the media scenarios. That is `users.MaySeeGlimt` correctly refusing one group's
    photographs to another group's member — but a 403 is decided *before* the blob is opened, so a
    fifth of the sample was a cheap refusal masquerading as a serve. The media scenarios now target
    only `nathejk`/`public` media, which every member can see.

  Both are noted in the tool, because a future run that reports either should look there first.

- 2026-09-17 — **Caveats, so these numbers are not read for more than they are worth:**

  - **No network is in the path.** Loopback, same host. PRD 019 §0a.3's actual constraint is *"the
    worst network of the weekend"*, and this test cannot measure it. What it establishes is that
    **the server is not the bottleneck** — roughly an order of magnitude of headroom against a
    plausible crowd — and that the mitigations for the network (thumbnail-first, `immutable`, 304)
    are worth 21.5× and then everything.
  - **Seeded by SQL, not through the event stream.** The dev fixture publishes events; this does not,
    because the broker is shared with the other Nathejk apps and append-only — two thousand synthetic
    glimt would be irreversible. The corpus is therefore wiped by a projection rebuild, which is
    documented in the tool and is mildly useful.
  - **Media objects are synthetic.** Generated as smooth gradients plus grain to land at a realistic
    ~278 kB. The first attempt used per-pixel noise, which is incompressible and came out at
    **1.25 MB** — six times what the upload path stores. Every media number taken against it would
    have been pessimistic by that factor. Worth knowing if the generator is ever changed.
  - **Dev hardware, not the deployment.** The absolute numbers will differ in production; the shape
    (plateau throughput, linear latency, zero errors) is the transferable part.

  The corpus was removed afterwards with `-clean`, which restored the blob volume to exactly its
  prior 59 objects and 7.6 MB — so the tool round-trips and can be rerun before the next event.

  All Go gates green: `gofmt`, `go vet`, `go tool staticcheck`, full `go test ./...`.
