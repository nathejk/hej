# 100 — `notifications.store.syncSubscription()`

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

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

- [ ] `syncSubscription()` in `notifications.store.ts`, safe to call when push is
      unavailable and when there is no SW registration.
- [ ] `subscribed` is accurate after a reload without calling `enable()`.
- [ ] Granted-but-not-subscribed is representable and distinguishable.
- [ ] Unit test with a faked registration for: no SW, no subscription, live
      subscription.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
