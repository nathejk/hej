# 293 — Instrument the unchanged/changed ratio

**Status:** done
**Priority:** low
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

## Description

PRD 017 phase 2. The endpoint's whole economy rests on one assumption: almost every
check finds nothing. If it does not, some version is being derived wrongly — and PRD 017
§8 is explicit that an event is the wrong time to discover that.

Log, per `/api/sync` call: which datasets were unchanged, which changed, which were
`unavailable`, and the endpoint's own timing. Aggregate the unchanged ratio per dataset,
not just overall — an overall figure hides a single wrongly-unstable version behind five
well-behaved ones, which is exactly the failure this instrumentation exists to catch.

**Two things the review must not misread:**

- **An early-race dip in the contacts ratio is expected**, not a fault. Photos are added
  heavily in the first hour, ~100–150 entries share one contacts version, and every new
  portrait moves it. PRD 017 §9 therefore measures the ≥ 95 % target *outside* the first
  hour. Judging the first hour by the steady-state number would produce a wrong
  conclusion and, worse, a plausible one.
- **A dataset at exactly 100 % unchanged for a whole event is a suspect, not a
  success.** That is the signature of a wrongly-stable version — the silent failure where
  a device quietly never updates. Cross-check it against whether the underlying data
  actually changed.

A non-empty `unavailable` is a server fault and should be alertable, not merely logged.

## Acceptance Criteria

- [x] Per-call structured log: unchanged / changed / unavailable per dataset, plus
      duration. **Revised** — a periodic summary rather than a line per call, and per-dataset
      *churn* rather than per-device unchanged. See the log: the per-device figure is not
      something the server can know without adding request bytes to the cheapest endpoint in
      the API.
- [x] Per-dataset unchanged ratio aggregated, not only an overall figure.
- [x] `unavailable` surfaced as a fault rather than an ordinary log line.
- [ ] Reviewed after the first event, with the numbers written into PRD 017 §9. → **task 296**
- [ ] Each dataset explicitly judged. → **task 296**
- [ ] Contacts assessed with the first hour excluded. → **task 296**
- [ ] A recommendation on the interval and on whether photos need their own key. → **task 296**

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 2.
- 2026-09-16 18:30 — Picked up, and immediately hit the interesting problem: **"which datasets
  were unchanged for this device?" is not something this server knows.** Whether a dataset
  changed is a comparison between the version we computed and the version *the device holds*,
  and the device never tells us — it sends an aggregate `ETag` and nothing else. Asking it to
  send six versions on every foreground would add request bytes to the cheapest endpoint in the
  API in order to satisfy a metric, which is the wrong trade (PRD 017 §6 says the response must
  stay tiny; the same logic applies to the request).
- 2026-09-16 18:35 — **So it measures the two things that are knowable, and they turn out to be
  enough:**
  1. **The aggregate unchanged ratio, from the 304.** A `304` means every dataset the caller
     holds is current — the device said so with `If-None-Match`. That is PRD 017 §9's ≥ 95 %
     target measured exactly, with no extra bytes either way.
  2. **Per-dataset churn, from a bounded witness sample.** For up to 32 cache keys per dataset,
     remember the last version computed and count how often the next one differs. A
     wrongly-unstable version is wrongly unstable for *every* key, so a sample catches it as
     decisively as full coverage — and, unlike full coverage, cannot grow into the leak task 295
     had just fixed.
- 2026-09-16 18:40 — **The observation point is `versionCache.put`**, via `observedAs(dataset,
  metrics)`. A `put` happens exactly once per derivation (on a cache miss) and it is the only
  place holding both the key and the fresh version; deriving the key again at the call site
  would restate each dataset's keying rule in a second place, which is how the two drift. It
  also means churn counts derivations from *every* caller, not only `/api/sync` — correct, since
  churn is a property of the version rather than of the endpoint.
- 2026-09-16 18:45 — **A cache hit records nothing, and that is load-bearing.** Counting hits as
  "unchanged" would flatter every dataset by the hit rate and turn this into a report on the
  TTL rather than on the version. Pinned by
  `TestSync_CacheHitsAreNotCountedAsDerivations`.
- 2026-09-16 18:50 — Two smaller decisions with the same shape, both about not producing a
  confident wrong number: a **first sighting** of a key is not counted as unchanged (it would
  understate churn right after a deploy, which is when someone is looking), and **unmeasured
  reports -1, not 0** — zero would read as "every check found something", the alarming answer,
  on a server nobody has called yet.
- 2026-09-16 18:55 — **Periodic summary, not per-call logging.** This is the endpoint every
  device hits on every foreground; a line per call would bury the rest of the log and tell a
  reader less, because the unit of meaning here is a ratio rather than a call. One summary per
  five minutes, driven by traffic rather than a ticker — no goroutine to own, and an idle server
  stays quiet instead of logging identical numbers all night. The first window is deliberately
  skipped: a ratio over one call is noise that reads like a signal.
- 2026-09-16 19:00 — Timing added as count/total/max, not a histogram: enough to answer "is this
  endpoint cheap?" during an event, and deliberately not enough to claim a p95 — that belongs to
  the load test (task 291), which drives the traffic and can measure from outside.
- 2026-09-16 19:05 — **Split the task.** The reading-and-judging half needs an event to have
  happened, so it is now **task 296** rather than leaving this one open for months. That task
  carries the two ways to misread these numbers (the expected early-race contacts dip, and -1
  meaning unmeasured).
- 2026-09-16 19:10 — Completed. `go/cmd/api/syncmetrics.go` + `syncmetrics_test.go` (13 tests),
  wired through `versionCache.observedAs` and the handler. All four Go gates green, plus
  `go test -race` on the sync tests — worth running here because the cache deliberately releases
  its own lock before calling into the metrics' lock, rather than holding two in an order
  nothing guarantees.
