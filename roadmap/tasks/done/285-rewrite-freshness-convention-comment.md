# 285 — Rewrite the freshness convention comment for the multiplexed check

**Status:** done
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

## Description

PRD 017 phase 1 — deliberately *in* phase 1, not cleanup. `useFreshnessLoop.ts` opens
with a numbered convention for "the next dataset that needs to stay current", and three
of its points become actively wrong the moment `useSyncLoop` lands:

- **§1** points at `GET /api/contacts/version` as the reference and describes one
  version endpoint per dataset. The reference is now `GET /api/sync`, and the whole
  point of PRD 017 is that six datasets get *one* request.
- **§3** says "Three trigger points, and no more". There are now four — foreground,
  interval, `online`, and an explicit user request (the manual refresh, task 282) — plus
  whatever task 280's device check adds for iOS resume paths.
- **§4** says to "add another value rather than reusing this one" for a second
  dataset's interval. That advice produced exactly the N-loops shape the PRD replaces.
  The interval is now served in the sync response itself, because the 02:00 load lever
  must take effect on the next check without a config refetch.

A stale convention comment is worse than none: it is the thing the next person copies.
What stays true and must survive the rewrite: opaque per-caller versions from a
projection read, version in the body not the header, no traffic while hidden, zero
disables the interval only, metadata ahead of images, push is not an option for
invalidation, and replace-do-not-merge.

Also document the debounce (task 281) and `unavailable` (task 284) — both are decisions
the next reader will otherwise re-lit­igate.

## Acceptance Criteria

- [x] §1 describes the multiplexed check and points at `/api/sync`.
- [x] §3 lists the real trigger points, including the manual refresh and task 280's
      findings.
- [x] §4 describes the served interval and drops the add-another-value advice.
- [x] Debounce and its manual-refresh exemption documented.
- [x] `unavailable` vs absent documented on the client side.
- [x] The still-true points survive, with their reasoning intact.
- [x] No stale references to `/api/contacts/version` as *the* reference anywhere in
      `vue/src` comments.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Do this after tasks 280–284, so
  the comment describes what shipped rather than what was planned.
- 2026-09-16 16:45 — Rewritten. The new convention leads with the one sentence that would
  have prevented the situation PRD 017 had to fix: **add a key to `/api/sync`, do not add a
  loop.** Everything still true survived with its reasoning — version in the body, opaque
  versions, no traffic while hidden, zero disables the interval only, metadata before images,
  push is not an option, replace-do-not-merge — and four points are new: absence vs
  `unavailable`, the debounce and its manual-refresh exemption, the served values as the
  02:00 lever, and "never two loops".
- 2026-09-16 16:50 — **Kept the wrong advice visible rather than deleting it.** The comment
  now says what the old convention said, that it was right about *what* a version is, and
  what following it a second time actually produced — two loops polling one directory and a
  third dataset fetched once on mount. A convention that only states its current position
  invites someone to rediscover the alternative; this one says it was tried.
- 2026-09-16 16:55 — Also fixed the BFF side, which task 284 had left claiming
  `/api/contacts/version` was "the reference implementation" other datasets "should copy the
  shape" of. That is now the opposite of true and it sits in the file a reader would land in
  first. It points at `sync.go`, says not to copy it, and notes that `contactsVersionFor`
  stays because `/api/sync` composes it. This is what PRD 017 §8 asked for.
- 2026-09-16 17:00 — Completed. Comment-only change. Frontend 708 tests + type-check green;
  all four Go gates green. The remaining `/api/contacts/version` references are the
  deliberately-deprecated `refreshIfStale` path and its tests, which task 292 removes.
