# 115 — Stale push subscriptions: detect a rotated VAPID key, survive a server restart

**Status:** done
**Priority:** high
**Created:** 2026-08-29
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-29
**Completed:** 2026-08-29

## Description

Maintainer's request: *"can I have a VAPID_SEQUENCE=1 or something identifying the vapid
key in case of rotation, to help clients drop stale subscriptions"*.

The goal is right and the mechanism is already available without a new setting — and
while implementing it, a **larger** staleness problem turned up.

## Why no VAPID_SEQUENCE

A `PushSubscription` is bound to the `applicationServerKey` it was created with, and the
browser exposes it: `subscription.options.applicationServerKey`. So the client can compare
the key its subscription actually uses against the key the server currently advertises.

**The key identifies itself.** A sequence number would be a second source of truth about
the same fact, and it can drift from what it describes in both directions:

- rotate the pair, forget to bump the counter → every client keeps a dead subscription,
  which is precisely the failure this was meant to prevent;
- bump the counter without rotating → every client re-subscribes for nothing.

A comparison of the key cannot drift, needs no deploy coordination, and works for
rotations that happened before anyone thought to add a counter.

Two engines' worth of caution: `options.applicationServerKey` is not guaranteed to be
exposed everywhere we run, so the key used is **also** remembered in `localStorage` at
subscribe time. Between the two, "which key is this subscription bound to?" is always
answerable.

## The bigger problem, found while looking

`cmd/api/main.go` wires `pushStore: push.NewMemoryStore()`. The subscription store is **in
memory**, so *every restart forgets every subscriber* — while their browsers keep a
perfectly valid subscription and every client keeps displaying "Til".

That is not a rare event like a key rotation; it is **every deploy**. A rotation-only
mechanism would not have touched it.

Fixed by re-registering on load: if a live subscription exists, POST it to the BFF again.
`push.Store.Save` is documented idempotent per (user, endpoint), so this is free, and it
means a restarted server relearns its subscribers as members open the app.

## What was implemented

- `notifications.serverKey` \u2014 the key the server advertises, kept (not just a
  configured/not-configured boolean) so it can be compared.
- `syncSubscription()` now: no subscription \u2192 not subscribed; subscription bound to a
  **different** key \u2192 unsubscribe and re-subscribe; otherwise subscribed, and
  re-registered with the BFF once per page load.
- **The rotation heals itself invisibly.** Permission is already granted, so re-subscribing
  raises no prompt \u2014 the member never learns anything happened.
- `registeredEndpoint` guards the re-POST so returning to the page does not re-register on
  every `visibilitychange`; it is in memory on purpose, since a reload is exactly when
  re-registering is wanted.
- `enable()` remembers the key it subscribed with, and now **fails** if the BFF did not
  accept the registration \u2014 previously it reported success when the browser had a
  subscription the server knew nothing about, i.e. push that could never arrive.

## Ordering, which is load-bearing

`syncConfigured()` must complete **before** `syncSubscription()`, or the key is unknown and
the staleness check silently no-ops \u2014 on the first visit after a rotation, the one moment
it exists for. Both call sites (`ProfileView`, `UpdatesView`) await it first, with the
reason written next to them, because a future tidy-up to `Promise.all` would look like an
improvement.

## Acceptance Criteria

- [x] A subscription created with a superseded key is detected and replaced, with no
      prompt and no user action.
- [x] No new configuration setting; nothing to remember to bump.
- [x] A restarted server relearns subscriptions as members open the app.
- [x] `enable()` no longer reports success when the server did not record the
      subscription.
- [x] Works where `options.applicationServerKey` is unavailable (localStorage fallback).
- [x] `npm run type-check` and `npm run build` clean.

## Still worth doing separately

**Persist the subscription store.** Re-registration makes an in-memory store *recover*,
but recovery depends on members opening the app; a member whose phone is in a pocket is
unreachable until they do. PRD 008 gave this service a database and an event log, so the
natural fix is a projection like every other read model. Not filed as part of this task
because it belongs with the delivery work (a later PRD) that will actually read the store.

If you still want a manual "force everyone to re-subscribe" lever \u2014 distinct from
rotation, e.g. after wiping the store deliberately \u2014 that is a small addition and worth its
own task; say so and I will add it.
