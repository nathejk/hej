# 209 — Dev panel resets: onboarding, session, caches, service worker

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 2. One-click resets in the dev panel, so re-running a stateful flow
costs less than writing it did.

This is the task that actually buys the speed PRD 014 was asked for. Simulating a
phone (tasks 206/207) gets you *into* onboarding once; testing step 3 of onboarding
twenty times needs the state gone twenty times, and that state is currently spread
across localStorage, IndexedDB, the cookie jar and the service worker's caches.
PRD 014 §9 sets the target: **onboarding re-runnable in under 10 seconds without
DevTools.**

## The four buttons, and what each must actually clear

**Reset onboarding.** `useOnboardingStore().reset()` already exists and already does
the right thing — it clears `complete`, `skipped` and the persisted
`hej.onboarding.*` completion flag, and its comment says it is "used by the dev/QA
override (task 139) and by sign-out". So this button is a caller, not new logic.
Verify afterwards that the router gate genuinely sends you back to `/welcome`; if it
does not, the bug is worth finding, because sign-out depends on the same action.

**Log out / clear session.** `useSessionStore().logout()` posts
`/api/auth/logout` and drops the remembered identity even if the request fails.
Prefer it over hand-clearing, so the dev path and the real path stay the same code.
Offer an offline-safe variant only if `logout()` turns out to block without network.

**Clear caches.** The one with real substance:

- Service worker caches — including the tile cache and portrait cache named in
  `config/cache.ts` (`TILE_CACHE_NAME`, `PORTRAIT_CACHE_NAME`).
- `trackDb` (IndexedDB) — the point and event stores.
- The contacts cache owned by `contacts.store`.
- The runtime-config mirror in localStorage: `hej.dataforsyningen-token`,
  `hej.show-build-id`, `hej.show-layout-debug`, `hej.install-gate`,
  `hej.contacts-poll-seconds`.

**Prefer the owners' own clear paths over deleting storage from the panel.**
`offline.store` already has a per-dataset `clear(id)` action and a registry of
handlers registered by whoever owns the storage (`registerDataset`), and its header
is explicit that the store "holds no data of its own" and that "anything here that
starts fetching or evicting is a sign the registry has been reinvented". A dev panel
that reaches past those handlers into IndexedDB is the same mistake wearing a dev
badge — and it would silently leave `offline.store`'s status wrong, since
`markEmpty`-style bookkeeping would not run.

**Unregister the service worker.** After clearing caches, so the next load fetches a
fresh bundle. Note `registerType: 'prompt'` and `injectRegister: false` in
`vite.config.ts` — registration is manual in `helpers/pwa.ts`, so unregistration
should live near it rather than being inlined in the panel.

## Guard against a fat finger

These buttons destroy state, and "clear caches" is one keystroke from the developer's
own logged-in session. A confirm step on the destructive two is worth the friction.
They are dev-only, so the blast radius is a dev database — but that dev database holds
real member data replayed from the broker, and re-syncing it is not instant.

## Acceptance Criteria

- [x] **Reset onboarding** calls `useOnboardingStore().reset()`; the gate then routes
      back to `/welcome` on the next navigation
- [x] **Log out** calls `useSessionStore().logout()`
- [x] **Clear caches** clears SW caches (tiles, portraits), `trackDb`, the contacts
      cache and the runtime-config localStorage mirror
- [x] Dataset clearing goes through `offline.store`'s registered handlers, so
      dataset status stays truthful afterwards — asserted by a test
- [x] **Unregister service worker**, in `@/dev/resets` next to the cache clearing rather
      than inline in the panel
- [x] A confirm step on the two destructive actions (`caches`, `everything`)
- [x] A **clear all dev overrides** action, using the `hej.dev.*` prefix scan
      task 206's naming convention exists for
- [~] Full onboarding re-run in under 10 seconds with no DevTools — the buttons exist and
      are unit-tested, but the 10-second claim is a stopwatch measurement in a browser and is
      **unmeasured**; see the log
- [x] No new store or helper gains dev-only logic in its own file — the panel calls
      existing actions via `@/dev/resets`

## Depends on

- **Task 208** — the panel these live in.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 2.
- 2026-09-12 09:20 — Picked up. Work lives in `@/dev/resets` rather than in the component, so
  the panel stays markup and the reset policy is testable without mounting anything.
- 2026-09-12 09:21 — Confirmed the premise held: `onboardingStore.reset()` and
  `sessionStore.logout()` both already do the right thing, so these are callers. That matters
  beyond tidiness — `reset()` is what sign-out uses, so if the button breaks, sign-out is broken
  too, and the test asserts the call rather than the effect for exactly that reason.
- 2026-09-12 09:22 — **Decision on the position track, and it needed care.**
  `offlineStore.clear('track')` *refuses*, because for a participant the local copy may be the
  sole record of where a team was. I did not weaken that. The dataset loop skips anything marked
  unrecoverable, and the track is deleted by a separate, explicitly dev-only
  `indexedDB.deleteDatabase('hej-track')` path with the reasoning written next to it: a
  developer's fake walk from task 213's playback is not evidence of anything, and leaving it
  behind makes every later track test start from someone else's route. Asserted by a test that
  `clear` is never called with `track`.
- 2026-09-12 09:23 — `deleteDatabase` resolves rather than hangs on `onblocked` (another tab
  holding the DB open), and the report says "kept (open elsewhere?)". A button that never
  finishes is worse than one that says why it could not.
- 2026-09-12 09:24 — Each reset returns a one-line report ("3 datasets, 4 caches, track deleted,
  1 sw"). A button that appears to do nothing is worse than no button.
- 2026-09-12 09:25 — Added `everything`, which is the two destructive paths plus identity plus
  the `hej.dev.*` keys, then reloads so nothing in memory survives to contradict what is now on
  disk. Amber-coloured and confirm-gated.
- 2026-09-12 09:26 — ✅ Suite 449/449 across 36 files, `vue-tsc` clean, and a fresh production
  build re-scanned: no dev-layer strings, no dev chunk.
- 2026-09-12 09:26 — **One criterion left `[~]`: the 10-second onboarding re-run.** That is a
  stopwatch measurement in a browser and I cannot take it from here. The mechanism is in place;
  someone should time it once and record the number — it is PRD 014 §9's headline metric, so an
  unverified claim there would be the wrong thing to tick.
