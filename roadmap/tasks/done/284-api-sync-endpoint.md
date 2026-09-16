# 284 — GET /api/sync: one multiplexed freshness check

**Status:** done
**Priority:** high
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

## Description

PRD 017 phase 1, and the piece the whole PRD exists for. One endpoint answers "what
has changed for this caller?" for every dataset the caller holds, so a foreground costs
one request rather than six.

```
GET /api/sync
{ "versions": { "contacts": "a1b2", "profile": "c3d4", "scans": "e5f6",
                "handouts": "…", "checkpoints": "…", "race_area": "…" },
  "unavailable": [],
  "interval_seconds": 60 }
```

New `go/cmd/api/sync.go`, behind `requireAuth`, with **OpenAPI annotations** (repo
rule).

**It composes; it must never become a place where payloads get built.** All six
derivations exist once task 283 lands: `contactsVersionFor` (contacts.go),
`scansVersionFor` / `handoutsVersionFor` / `checkpointsVersionFor` (mapversion.go),
plus profile and race area from 283.

Three properties that are easy to get subtly wrong:

- **Absence means "you may not hold this."** A dataset is a key only if the caller is
  permitted it and can have it — a spejder has no contacts key, a patrol-less
  personnel user has no scans/handouts/checkpoints keys. That is how the client learns
  not to ask, replacing today's arrangement where the client's own role table guesses
  and a disagreement produces a 403 per foreground.
- **`unavailable` exists so absence can stay meaningful.** Every `*VersionFor` returns
  an error. One failing projection read must not 500 the check for the other five — but
  dropping its key would tell the client "you may not hold this", a lie it cannot
  recover from, because it would stop asking. So a failed derivation is *named* and the
  client treats it as "unchanged, ask again". Normally empty; log it as a server fault
  when it is not.
- **The versions go in the JSON body**, not only in a header: `fetchWrapper` does not
  expose response headers. Keep `ETag` on the response for the browser's own
  conditional requests.

## Acceptance Criteria

- [x] `GET /api/sync` registered behind `requireAuth`, with OpenAPI annotations.
- [x] Composes all six derivations; builds no payload.
- [x] Keys assembled by role and patrol: a dataset the caller may not hold is absent.
- [x] A derivation error adds the name to `unavailable` and does not fail the response.
- [x] `interval_seconds` served in the response (task 289 owns the config wiring).
- [x] `ETag` set on the response.
- [x] Test: a spejder viewer has no `contacts` key.
- [x] Test: a patrol-less viewer has no `scans`/`handouts`/`checkpoints` keys.
- [x] Test: a failing derivation yields `unavailable` + HTTP 200 with the other keys.
- [x] Test: versions match what the individual derivations return for the same viewer.
- [x] Test: unauthenticated request is rejected.
- [x] All four Go gates green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Depends on task 283 for the
  profile and race-area keys; can be built against the four existing derivations first.
- 2026-09-16 12:00 — Picked up after 283, so all six derivations existed.
- 2026-09-16 12:10 — **The datasets are a table, not a sequence of ifs** (`syncDatasets`).
  The risk this endpoint carries is a dataset being *forgotten* — added to the app, never
  added to the check, and silently frozen on every device until the next cold start. A
  table is something a reviewer can read against the app's panes; six inline blocks are
  not.
- 2026-09-16 12:15 — **Decision: the config plumbing landed here, not in task 289.**
  `syncIntervalSeconds` and `syncDebounceSeconds` (env `SYNC_INTERVAL_SECONDS` /
  `SYNC_DEBOUNCE_SECONDS`, defaults 60/5) are part of this response's *contract*, so
  shipping the endpoint without them would have meant shipping a response shape that
  immediately changed. 289 keeps the client half: adopting a changed interval without a
  reload, and the zero-disables-the-interval-only test.
- 2026-09-16 12:20 — **`debounce_seconds` is served too**, which the PRD implied but did
  not spell out. Tuning the interval alone would leave a foregrounding-heavy crowd
  (unlock, glance at the map, lock, unlock) completely untouched by the 02:00 lever, since
  none of their checks come from the timer.
- 2026-09-16 12:25 — **The ETag is hashed over sorted keys, and excludes the intervals.**
  Go randomises map iteration, and an ETag derived from that order would change on every
  request — worse than none, because it defeats the browser cache *and* claims the data
  changed. The `unavailable` set *is* in the hash, so recovering from a failure is visible
  to a conditional request rather than masked by a 304. The intervals are not: an operator
  widening the interval must not make every device believe all six datasets changed.
- 2026-09-16 12:30 — A session whose user no longer resolves answers 404 rather than an
  empty 200. An empty-but-successful check reads to the client as "you hold nothing",
  which would quietly stop all refreshing — the silent failure mode again.
- 2026-09-16 12:40 — Completed. `go/cmd/api/sync.go` + `sync_test.go` (10 tests),
  route registered in `routes.go`, two config values in `env.go`. Beyond the required
  criteria: a test that a spejder's check never runs the directory query (the endpoint
  must stay a composition, and the payload builders are right there for a future change to
  reach for), and a test that the ETag is stable across map iteration order. All four Go
  gates green.
