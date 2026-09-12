# 209 — Dev panel resets: onboarding, session, caches, service worker

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] **Reset onboarding** calls `useOnboardingStore().reset()` and the gate then
      routes back to `/welcome`
- [ ] **Log out** calls `useSessionStore().logout()`; the app returns to the login
      step
- [ ] **Clear caches** clears SW caches (tiles, portraits), `trackDb`, the contacts
      cache and the runtime-config localStorage mirror
- [ ] Dataset clearing goes through `offline.store`'s registered handlers, so
      dataset status stays truthful afterwards — verified on the readiness view
- [ ] **Unregister service worker**, implemented next to `helpers/pwa.ts`'s
      registration rather than inline in the panel
- [ ] A confirm step on the two destructive actions
- [ ] A **clear all dev overrides** action, using the `hej.dev.*` prefix scan
      task 206's naming convention exists for
- [ ] Full onboarding re-run, from complete back to `/welcome` and through again, in
      under 10 seconds with no DevTools (PRD 014 §9)
- [ ] No new store or helper gains dev-only logic in its own file — the panel calls
      existing actions

## Depends on

- **Task 208** — the panel these live in.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 2.
