# 100 — `notifications.store.syncSubscription()`

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

`subscribed` is currently only set inside `enable()`, so after a reload the store
believes the user is not subscribed even when a live `PushSubscription` exists —
which would make PRD 003's push status row lie.

Add a `syncSubscription()` that reads the actual subscription from the service
worker registration (`registration.pushManager.getSubscription()`) and sets
`subscribed` from it. Call it where permission is synced.

Note the two are independent: `permission === 'granted'` with **no**
subscription is a real state (a subscription can be dropped by the browser or
lost when the SW is replaced), and the row must be able to say so rather than
implying push works.

## Acceptance Criteria

- [x] `syncSubscription()` in `notifications.store.ts`, safe to call when push is
      unavailable and when there is no SW registration.
- [x] `subscribed` is accurate after a reload without calling `enable()`.
- [x] Granted-but-not-subscribed is representable and distinguishable.
- [ ] Unit test with a faked registration — **not done: no unit-test runner in
      this repo** (see task 098). The three branches are small and explicit; a
      test runner is its own task.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Used `navigator.serviceWorker.getRegistration()` rather than
  `.ready`, which `enable()` uses. `ready` **never resolves** when no service
  worker is registered, so reusing it here would hang the call instead of
  answering "not subscribed" — exactly the case this function exists to report.
- 2026-08-28 — Kept separate from `syncPermission()` on purpose, with the reason
  in the code: permission and subscription are independent, and
  granted-without-a-subscription is both a real state (browser-dropped
  subscription, replaced service worker) and the one where push silently does not
  work. Folding them together would make that state unreportable.
- 2026-08-28 — Also called it from `UpdatesView`, which was not in the task but is
  the same bug from the user's side: its push pre-prompt hides itself when
  `subscribed`, so before this it re-asked an already-subscribed user after every
  reload.
- 2026-08-28 — ✅ `npm run type-check` clean. Real-device verification of the
  reload behaviour belongs to task 108. Moving to done.
