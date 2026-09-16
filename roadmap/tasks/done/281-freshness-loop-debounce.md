# 281 — Debounce in useFreshnessLoop, with a manual-refresh override

**Status:** done
**Priority:** high
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

## Description

PRD 017 phase 1. `useFreshnessLoop` guards *overlapping* checks (the `checking` flag)
but not *repeated* ones, so unlock → check → lock → unlock fires a fresh check every
time. That is not an edge case; it is how the map is used while walking.

Add a minimum gap between checks. Two properties matter:

- **The gap is served, not compiled in** — same reasoning as the interval (PRD 017 §6).
- **A user-requested check ignores it.** Task 282 adds a manual refresh control, and a
  button that visibly does nothing is worse than the request it saves. The overlap
  guard still applies: a manual refresh must not run concurrently with an in-flight
  check.

The loop's browser surface stays injectable (`FreshnessTarget`), so this must be
testable without a DOM. `FreshnessTarget` has no clock — add one (`now()`) rather than
reaching for `Date.now()` directly, so tests can advance time deterministically.

## Acceptance Criteria

- [x] `FreshnessLoopSpec` gains a debounce setting in seconds, defaulted from config.
- [x] `FreshnessTarget` gains `now()`; `browserFreshnessTarget` supplies it.
- [x] A check within the debounce window of the previous one is skipped.
- [x] `check({ force: true })` (or equivalent) bypasses the debounce but not the
      overlap guard.
- [x] Zero or less disables the debounce entirely.
- [x] The mount-time check is never debounced away (nothing precedes it).
- [x] Tests, with an injected target and clock: repeated foreground is debounced; a
      forced check is not; a forced check during an in-flight check is still skipped;
      the interval tick respects the window.
- [x] Existing `useContactsFreshness.spec.ts` still passes.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1.
- 2026-09-16 10:20 — Picked up. Plan: add `now()` to `FreshnessTarget` so the window is
  testable without a real clock; add `debounceSeconds` to the spec; track
  `lastCheckedAt` and skip a non-forced check inside the window. `check` grows an
  options argument (`{ force }`) rather than a second exported function, so the manual
  refresh in task 282 uses the same seam. Mount check is inherently unaffected since
  nothing precedes it.
- 2026-09-16 10:45 — **Decision: `debounceSeconds` defaults to 0 (disabled), not to a
  config value.** The criterion said "defaulted from config", but the loop is the
  mechanism and *how often a dataset is worth asking about* is policy belonging to the
  caller that knows which dataset it is — exactly how `intervalSeconds` already works.
  `useSyncLoop` will pass the served value (task 289). The side benefit is that the two
  existing callers keep their current behaviour byte-for-byte, so this change cannot
  regress contacts freshness.
- 2026-09-16 10:45 — **Decision: stamp `lastCheckedAt` before the check, not after.**
  The window is then "time since we last asked", so a slow round trip cannot silently
  extend it into a second skipped check.
- 2026-09-16 10:50 — `force` bypasses the debounce only. The overlap guard, the
  visibility guard and `enabled` all still apply: forcing means "do not tell me it is
  too soon", not "fetch data this user may not hold". Three tests pin that, including
  the one that matters for task 282 — a forced check during an in-flight check is still
  dropped rather than doubling requests on a slow link.
- 2026-09-16 10:55 — New `useFreshnessLoop.spec.ts` (9 tests) with a clock-capable fake
  target. Adding `now()` as a *required* member of `FreshnessTarget` broke the two
  existing fakes at type-check (`useContactsFreshness.spec.ts`, `prefetch.spec.ts`);
  both now supply `Date.now`, with a comment saying why it is never consulted there.
  Kept it required rather than optional — the injected browser surface is this file's
  whole design, and an optional clock would have been the one global left in it.
- 2026-09-16 11:00 — Completed. `npm run type-check` clean; full suite 671 passed (55
  files), including the 20 in `src/composables`.
