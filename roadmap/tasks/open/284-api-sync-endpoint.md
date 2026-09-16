# 284 — GET /api/sync: one multiplexed freshness check

**Status:** open
**Priority:** high
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `GET /api/sync` registered behind `requireAuth`, with OpenAPI annotations.
- [ ] Composes all six derivations; builds no payload.
- [ ] Keys assembled by role and patrol: a dataset the caller may not hold is absent.
- [ ] A derivation error adds the name to `unavailable` and does not fail the response.
- [ ] `interval_seconds` served in the response (task 289 owns the config wiring).
- [ ] `ETag` set on the response.
- [ ] Test: a spejder viewer has no `contacts` key.
- [ ] Test: a patrol-less viewer has no `scans`/`handouts`/`checkpoints` keys.
- [ ] Test: a failing derivation yields `unavailable` + HTTP 200 with the other keys.
- [ ] Test: versions match what the individual derivations return for the same viewer.
- [ ] Test: unauthenticated request is rejected.
- [ ] All four Go gates green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Depends on task 283 for the
  profile and race-area keys; can be built against the four existing derivations first.
