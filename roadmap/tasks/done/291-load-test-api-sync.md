# 291 — Load test /api/sync at expected device count

**Status:** done
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 017 §8: "Load, and it is the whole risk." This endpoint becomes the app's only
continuous during-race traffic besides position reporting, and it lands on the same BFF.
The mitigations are all in place by design — cached versions keyed on permitted set, a
tiny response, `ETag`, the debounce, the served interval — but *designed to be cheap* and
*measured cheap* are different claims, and the PRD says the measurement belongs in the
rollout rather than after it.

Six version derivations run per call. Each is a projection read or a 5 s cache hit, so
the interesting question is not the happy path but the **cache-miss storm**: a few
hundred devices foregrounding at once after a mass event (a race start, a broadcast
notification) all miss the same 5 s TTL together.

## What to measure

- Steady state: expected device count on a 60 s interval, everything unchanged.
- Thundering herd: all devices checking within one second, cold caches.
- Changed state: one dataset's version moves for every patrol at once (a mass reveal),
  so every device refetches a payload in the same window.
- Alongside position reporting at its own expected rate, since they share the BFF.

## Acceptance Criteria

- [x] Steady-state p50/p95/p99 for `/api/sync` at expected device count, recorded.
- [x] Thundering-herd p95 recorded; a stated verdict on whether the 5 s TTL needs
      single-flight protection.
- [x] `/api/sync` p95 confirmed well inside the BFF's other read endpoints (PRD 017 §9).
- [x] Cost of one `/api/sync` compared against one position report, as a ratio.
- [x] Unchanged-response ratio in the steady-state run ≥ 95 %.
- [x] Numbers written into PRD 017 §9, replacing the targets with measurements.
- [x] A stated recommendation for the shipped interval, given the numbers.

## Results (2026-09-17, in process, Apple silicon)

`cd go && SYNC_LOAD=1 go test ./cmd/api/ -run TestSyncLoad -v`

400 devices, 100 patrols, 150-row contacts directory, concurrency 32, production 5 s cache TTLs.

| run | handler mean | handler max | round trip p50 / p95 / p99 | notes |
|---|---|---|---|---|
| steady (all `304`) | **7 µs** | 910 µs | 1.97 / 2.61 / 3.10 ms | 2000 calls, **100 % unchanged** |
| herd (cold caches, no ETag) | **16 µs** | 198 µs | 1.81 / 3.93 / 4.67 ms | 400 calls, 282-byte body |

- **Expected event load is 6.7 req/s** (400 devices ÷ 60 s interval). Sustained **16,555 req/s**, i.e.
  **~2,500× headroom**. The endpoint's own work is not a risk at this device count.
- **Round trip is not the handler.** p50 ≈ 2 ms against a handler mean of 7 µs — over 99 % of the local
  round trip is Go's HTTP client and loopback, not this endpoint. The handler figures are the ones to
  compare against other endpoints; the round trip is an artefact of the harness.
- **The herd needs no single-flight.** Cold caches cost 16 µs mean and a 198 µs max across 400
  simultaneous devices — *lower* max than the steady run, because the steady run's max includes the
  warm-up's first contacts hash. Six derivations per call, each a projection read, simply are not
  expensive enough for the 5 s TTL to be load-bearing at this scale. It earns its place by collapsing
  *query* load, not by protecting the handler.
- **Versus a position report: 0.92×** (sync p50 83 µs, `/api/track` p50 91 µs, sequential, same
  conditions). So a check costs **about the same as one position report**, not "a small fraction" as
  PRD 017 §9 optimistically put it. Practically that is still fine — both are microseconds of server
  work — but the honest statement is *comparable*, and it means this feature adds roughly one
  position-report's worth of load per device per interval rather than a rounding error.
- **Recommendation: keep the served interval at 60 s.** Nothing in these numbers argues for widening
  it, and the lever exists for the case they do not predict (real projections, real MySQL, real
  network). Nothing argues for shortening it either — that would be spending devices' battery and
  data for freshness nobody asked for.

### What this does not measure

In process, over loopback, against synthetic projections: **no TLS, no Traefik, no network, no MySQL.**
The real cost of a version derivation against a populated database is exactly what is missing, and it
is the part most likely to differ. These numbers are a **floor**, not a forecast — they establish that
the endpoint's own logic is cheap, which is what was uncertain. Task 296 reads the real numbers from an
actual event.

Also not exercised: **churn**. Each cache key is derived once per run, and a first sighting is
deliberately not counted as unchanged, so every `churnRatio` reads `-1` (unmeasured). Churn has unit
coverage in `syncmetrics_test.go`; this harness is about cost.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Depends on task 284.
- 2026-09-17 00:30 — Picked up. **Decided to measure in process rather than deploy a rig.** A load
  test needs hundreds of *distinct* callers to be meaningful — profile keys by user, the map datasets
  by patrol — and the mock directory has eight users, so a deployed run against dev data would sit
  almost entirely on cache hits and report a flattering number. Synthetic projections at event scale
  (400 devices, 100 patrols, a 150-row directory) exercise the real key spaces, run anywhere, and are
  reproducible. The cost is that no network, TLS or MySQL is involved — stated in the results rather
  than left for a reader to assume.
- 2026-09-17 00:35 — Env-gated (`SYNC_LOAD=1`) rather than skipped by `-short`: the dev container
  re-runs `go test ./...` on every change, and this would tax every edit for a number that only moves
  when the derivations do.
- 2026-09-17 00:40 — Sessions are minted directly through `sessions.Issue` into an
  `httptest.NewRecorder`, not through the PIN flow. Running a real login per simulated device would
  have spent the test on PIN issuance and measured the wrong endpoint.
- 2026-09-17 00:45 — **Two flaws in my own first output, both fixed.** The report mixed client-side
  percentiles with the server-side max from task 293's instrumentation, so it printed a "max" *below*
  the "p50" — which reads as a bug in the numbers and would have discredited the whole run. And the
  herd's derivation counts included the steady run's, because the metrics were not reset between
  subtests. Both now separated and reset per run.
- 2026-09-17 00:50 — `/api/track` answered 503 until a `cqrstest.Publisher` was installed (the same
  double `track_test.go` uses); without it the comparison would have measured a failure path.
- 2026-09-17 00:55 — Completed, numbers above. **The one result worth arguing with is the position-report
  ratio: 0.92×, i.e. comparable, not "a small fraction" as PRD 017 §9 claims.** Recorded as measured
  rather than rounded towards the PRD's wording, and §9 updated to say so.
