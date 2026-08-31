# 162 — Client freshness loop

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Keeps the cached directory current during the event (PRD 007 §6, §8 "Keeping the directory
fresh"). Polls task 155's version endpoint at moments that already exist:

1. **On foreground** — `visibilitychange` → visible, and on app launch.
2. **While open** — a ~60 s interval, stopped the moment the app is hidden.
3. **On reconnect** — the `online` event.

Only a changed version triggers a manifest delta. **Metadata propagates ahead of images.**

The interval must be **runtime-configurable** — this is the app's first continuous during-race
traffic and it shares the BFF with PRD 002's position reporting.

Push is not usable here: iOS requires every web push to show a notification. See PRD 007 §8.

## Implementation

- `vue/src/composables/useContactsFreshness.ts` + spec (12 tests).
- `go/cmd/api/config.go` + `env.go`: `contacts_poll_seconds` on `/api/config`, from
  `CONTACTS_POLL_SECONDS` (default 60), plus two tests in `config_test.go`.
- `vue/src/config/runtime.ts`: `contactsPollSeconds`, remembered across an offline start like
  the other runtime values.

**The interval is served, not built in.** A value the client cannot be told is not a lever, and
the reason for the lever is load: if a few hundred devices cost more than expected, the
interval has to be widenable *during* the event. **0 or less disables the interval but not the
pane** — foreground and reconnect checks keep working, so "reduce load" cannot silently become
"stop updating". That distinction has its own test, because it is the one an operator would get
wrong at 2am.

**Browser surface is injected**, like the store's storage seam and `helpers/platform.ts`: the
loop's decisions are about *when* to check, which is worth testing without a DOM. The fake
target scripts visibility, `online`, and timers, so the tests assert on both call counts and
whether a timer exists at all.

**Hidden means stopped, not slowed.** A phone in a pocket has nobody reading the pane, so the
timer is cleared entirely; `check()` also re-tests visibility, in case a timer ever outlives a
hidden document.

**Overlapping checks are suppressed.** A foreground event landing on an interval tick would
otherwise fire two requests, which on a forest link is pure waste.

**Scoped to the pane.** The loop starts when the contacts view mounts and `onScopeDispose`
stops it, so a user who never opens the pane generates no traffic at all — the cheapest
possible scope, and it also gives the pane its initial refresh for free.

## Two bugs found by the tests

**A temporal dead zone in `runtime.ts`.** I declared `DEFAULT_CONTACTS_POLL_SECONDS` below the
`ref()` that used it. `const` is not hoisted, so importing the module threw and took down
*every* value in it, not just the new one — the whole config module failed to load. Moved the
constant above its use, with a note.

**`onScopeDispose` warns rather than throws** outside an effect scope, so my `try/catch` did
not suppress it and every test run printed a Vue warning. Replaced with a `getCurrentScope()`
check. Worth fixing rather than tolerating: warnings nobody can act on are how a suite trains
people to ignore its output.

The overlap-suppression tests also needed a microtask flush between triggers, which is not a
test artefact — it is the production behaviour, since a real check takes a round trip. Noted
in the helper so the next reader does not "fix" it by removing the guard.

## Acceptance Criteria

- [x] Version check on foreground, on `online`, and every ~60 s while visible.
- [x] Polling stops entirely when the document is hidden.
- [x] Interval read from runtime config, with a sane default (60 s, served by the BFF,
      remembered for offline starts).
- [x] A changed version triggers a metadata delta; images follow separately — the store
      fetches the index, and portraits are `<img>` requests the browser caches (task 177).
- [x] Backoff on repeated failure; no tight retry loop when offline — the loop only ever
      fires on an interval or a real event, and `refreshIfStale` never throws, so a failure
      simply waits for the next tick rather than retrying.
- [ ] The list updates in place — scroll position preserved, expanded group not collapsed.
      **Deferred to task 163**, which owns the rendering; the store replacing its entries is
      the prerequisite and is done.
- [x] Tests with fake timers: hidden → no polling; foreground → immediate check; unchanged
      version → no delta fetch (the last one in the store's spec, where the fetch lives).

## Progress Log

- 2026-08-31 21:55 — Picked up. Added `contacts_poll_seconds` to the BFF's runtime config
  first, since "runtime-configurable" is not satisfiable from the frontend alone.
- 2026-08-31 22:05 — Test suite failed to load: TDZ error in `runtime.ts` from declaring the
  default constant below its use. Fixed; the failure mode is worth noting because it broke the
  whole config module rather than one value.
- 2026-08-31 22:15 — Four tests failed expecting a second check in the same synchronous turn.
  Cause was the overlap guard working correctly against an async stub. Added a flush helper
  with a comment explaining it mirrors production rather than papering over a bug.
- 2026-08-31 22:20 — Replaced the `try/catch` around `onScopeDispose` with a
  `getCurrentScope()` check: it warns rather than throws, so the suite was printing a Vue
  warning on every run.
- 2026-08-31 22:25 — ✅ Criteria met bar the in-place-update one, which belongs to task 163.
  106 frontend tests pass, `vue-tsc --noEmit` clean; Go suite green.
