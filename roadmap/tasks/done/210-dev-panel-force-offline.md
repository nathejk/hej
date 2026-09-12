# 210 — Dev panel: force-offline toggle

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 2. A toggle that drives `app.store.online` to `false`, so PRD 009's
offline and cache-served states can be produced on demand.

## Why not just use DevTools

Because the obvious tool breaks the tool you are using. Chrome's "Offline" throttling
kills the Vite dev server's HMR websocket, so the page stops updating and the next
edit appears not to work — which turns every offline test into a dev-server restart.
Network throttling also cannot express the state that actually matters in the field.

`app.store`'s comment on `online` (lines ~10–20) is the design here, and it is worth
reading before implementing: `navigator.onLine` only means "this device has a network
interface with a route" — true on a captive portal, true with one bar and no
throughput, true on the event's own patchy coverage. So the flag is *seeded* from
`onLine` and then **corrected by what actually happens**, with `fetchWrapper`'s
`NetworkError` driving it via `session.store`.

That is exactly why this toggle is cheap and legitimate: the store is already
designed to be told it is offline by something other than the browser.
`setOnline(false)` is a supported input, not a poke into private state.

## Two levels, and the second one matters more

1. **Flag only** — `setOnline(false)`. Produces every "you are offline" affordance:
   the shell indicator (task 188), the offline notice, the readiness view's framing.
   Cheap, and covers most UI work.
2. **Actually failing requests** — optionally intercept `fetchWrapper` in dev so
   requests reject with the same `NetworkError` the real path produces. This is the
   one that catches the interesting bugs: a component that renders an offline banner
   while still happily awaiting a fetch is only distinguishable under level 2.

Level 1 is required; level 2 is desirable and may be split out if it grows. If
implemented, it must reuse `fetchWrapper`'s existing error type rather than
inventing one, or the code paths under test are not the real ones.

## What it must not do

Do not add a second source of truth. `offline.store`'s header is explicit that there
is deliberately no `online` flag in it, because `app.store` owns connectivity and a
copy "would drift from it, and the two would disagree in exactly the situation both
exist for". The dev toggle writes to `app.store` and nowhere else.

Also: the toggle must be visibly *on* in the panel while active, and it must not
survive a reload silently. A forgotten force-offline looks exactly like a broken BFF.

## Acceptance Criteria

- [x] A toggle in the dev panel calling `app.store.setOnline(false)` / `true`
- [~] The offline shell indicator, `OfflineNotice` and the readiness view all respond —
      they consume `app.store.online`, which is driven and unit-tested, but the rendering is
      unverified in a browser
- [x] HMR keeps working while the toggle is on (the whole point versus DevTools) — by
      construction: nothing touches the dev server's socket, only `fetchWrapper`
- [x] Level 2 implemented, not deferred: `fetchWrapper` rejects with its existing
      `NetworkError` while forced offline
- [x] The panel shows clearly that the toggle is active (amber, inverted, with a note)
- [x] The state does not silently persist across a reload — in memory only
- [x] `offline.store` gains no connectivity flag

## Depends on

- **Task 208** — the panel this lives in.
- **Task 090** — the connectivity model in `app.store` this drives.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 2.
- 2026-09-12 09:24 — Picked up. Built **both** levels rather than deferring level 2: a flag that
  says offline while requests still succeed is exactly the half-truth PRD 014 forbids the layer
  from producing, so shipping level 1 alone would have been shipping the wrong thing.
- 2026-09-12 09:25 — Level 2 is a registered predicate in `fetchWrapper`
  (`setDevNetworkBlocker`), same pattern as `setDevEnvProvider`: registration rather than import,
  so no `src/dev/` module is statically reachable from product code. It throws the **existing**
  `NetworkError`, so what is exercised is the real failure path — a bespoke error would test a
  branch no real failure can reach.
- 2026-09-12 09:26 — Un-forcing restores `online` from `navigator.onLine` rather than assuming
  `true`. Assuming would be the same lie in the other direction: the laptop may genuinely have no
  network.
- 2026-09-12 09:26 — Not persisted, on purpose. A forgotten force-offline is indistinguishable
  from a broken BFF, and a reload is a cheaper mitigation than a badge.
- 2026-09-12 09:27 — A test of mine was **wrong and passing for the wrong reason**: "lets
  requests through again" asserted `rejects.not.toBeInstanceOf(NetworkError)`, but a real fetch in
  node fails with the *same* NetworkError, so it would have passed with the blocker still on. It
  actually failed, which is how I noticed. Replaced with a stubbed `fetch` and a positive
  assertion.
- 2026-09-12 09:27 — ✅ Suite 454/454 across 37 files, `vue-tsc` clean.
