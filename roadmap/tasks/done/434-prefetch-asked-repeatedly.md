# 434 — The prefetch asked for the same photographs repeatedly

**Status:** done
**Priority:** low
**Created:** 2026-09-26
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

Observed in a network panel while stepping through single photographs after task 432 landed: the same
ordinals appear again and again from `prefetchAround` — 0, 1, 2, 1, 2, 0, 3, 2, 1, 3, 4, 3, 4, 2, 5 —
each a fresh 200 of ~900 kB. The session showed 2079 requests and 84 MB transferred.

The overlap itself is **inherent and correct**. PRD 023 §6 budgets two ahead and one back, so walking
0→1→2→3 puts photograph 2 inside the window at every step. Nothing is wrong with the window.

What was wrong is that `prefetchAround` re-issued `new Image()` for URLs it had already asked for, and
leaned entirely on the HTTP cache to make the repeats free. That is *nearly* right — the media routes
answer `immutable` with a year's `max-age`, so a browser normally serves a repeat without touching the
network — and "nearly" was doing a lot of work:

- it is not true with DevTools' *Disable cache* checked, which is the state anybody looking at a
  network panel is in, and is the likeliest explanation for what was actually observed;
- it is not true after a cache eviction, and on a phone stepping through 900 kB photographs an
  eviction is an ordinary event rather than a corner case;
- it means the code's cost depends on a browser setting, which is a poor property for the one thing
  PRD 023 §6 is explicitly a budget about.

So this is mostly removing an assumption rather than fixing a user-visible fault. Recorded as low
priority for that reason.

## What changed

`ui.prefetched` — a set of URLs already handed to a prefetch. `prefetchAround` skips a URL it has
already asked for, and records it **before** issuing the request, so the check short-circuits rather
than merely observing.

Cleared in `open()` alongside `ui.items`, not kept across opens: the admin tool reopens the overlay
against a sheet that may have changed underneath it, and a stale entry would suppress a prefetch for a
photograph that is no longer the one we fetched.

Keyed on the **URL**, which means the variant is part of the key for free — a rotation that moves a
narrow viewport onto the 800 px candidate prefetches that candidate rather than being suppressed by
the display image already being recorded.

Costs one string per photograph, so a 200-photograph album holds 200 short strings.

## Acceptance Criteria

- [x] A photograph inside the window at several steps is prefetched once
- [x] The check short-circuits before the request is issued
- [x] The record is per album, cleared on open
- [x] The variant is part of the key, so a rotation still prefetches what it will display
- [x] Pinned by test, mutation-checked

## Progress Log

- 2026-09-26 — Reported from a network panel after deploying 432: repeated 200s for the same ordinals
  from `prefetchAround`.
- 2026-09-26 — Established that the window's overlap is correct and the repeats are the bug, and that
  the most likely reason they were *visible* is a disabled cache in DevTools. Fixed anyway, because the
  eviction and phone cases are real and because leaning on a browser setting is the wrong shape for a
  budget.
- 2026-09-26 — ✅ `ui.prefetched`, cleared per album. Guard in `viewer_test.go` asserting the check
  exists, that it precedes the request, and that it is reset on open. Mutation-checked by removing the
  dedupe.
- 2026-09-26 — `gofmt`, `GOWORK=off go test -timeout 5m ./...` and `go tool staticcheck ./...` clean.

## Worth knowing when reading the panel again

With the cache enabled, a repeat prefetch was already free, so the visible improvement may be small —
what changed is that it is now free *regardless* of the cache. If a panel still shows repeated
full-size fetches after this, the thing to check is whether the **backfill has been run on that
environment**: without `mediumRef`, the viewer has no 800 px candidate and every prefetch is a ~900 kB
display image rather than a ~105 kB one (task 433).
