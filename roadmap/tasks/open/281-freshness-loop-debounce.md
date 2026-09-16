# 281 — Debounce in useFreshnessLoop, with a manual-refresh override

**Status:** open
**Priority:** high
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `FreshnessLoopSpec` gains a debounce setting in seconds, defaulted from config.
- [ ] `FreshnessTarget` gains `now()`; `browserFreshnessTarget` supplies it.
- [ ] A check within the debounce window of the previous one is skipped.
- [ ] `check({ force: true })` (or equivalent) bypasses the debounce but not the
      overlap guard.
- [ ] Zero or less disables the debounce entirely.
- [ ] The mount-time check is never debounced away (nothing precedes it).
- [ ] Tests, with an injected target and clock: repeated foreground is debounced; a
      forced check is not; a forced check during an in-flight check is still skipped;
      the interval tick respects the window.
- [ ] Existing `useContactsFreshness.spec.ts` still passes.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1.
