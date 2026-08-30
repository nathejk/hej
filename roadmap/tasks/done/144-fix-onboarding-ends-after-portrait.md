# 144 — Fix: onboarding ended itself after the portrait step

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

Reported by the maintainer, 2026-08-30: logging into the installed app and completing the
profile photo ended the flow — the remaining steps were skipped and the app opened, instead of
continuing to step 3.

Two distinct causes, one a correctness bug and one a design flaw in how the view consumed the
step machine. Both had to be fixed; either alone would have left the symptom reachable.

## Cause 1: a granted permission is not a working subscription

The notifications step counted as settled when `permission === 'granted'`:

```ts
notificationsSettled:
  notifications.permission === 'granted' || … 'denied' || … 'unavailable'
```

But **permission and subscription are independent**, which `notifications.store` documents in
its own comments and which task 115 exists because of. A member can have granted notifications
weeks ago and have **no subscription registered with the BFF** — in which case nothing will ever
be delivered to them. The step whose entire job is to create that subscription (`enable()` =
permission + subscribe + POST) was being skipped on the strength of the permission alone.

Silently, and for exactly the people who look most set up. On a device that has been through
push testing, the grant is already there, so the step vanished.

Now a granted permission settles the step only once there is a subscription behind it;
`denied` and `unavailable` still settle outright, because nothing further can be done in either.
`WelcomeStepNotifications` also syncs the live subscription on mount, so a member who genuinely
has one is not asked again.

## Cause 2: the view let the machine yank a mounted step away

`WelcomeView` rendered `onboarding.currentStep` directly. That reads well and is wrong in one
specific way: the machine is *derived*, so its answer changes the instant any underlying state
changes — including while a step is on screen.

The notifications step syncs permission and subscription when it mounts. So with cause 1 in
place, the sequence was: photo uploaded → `currentStep` becomes `notifications` → step mounts →
sync discovers the existing grant → step settles → `currentStep` becomes `null` → the completion
watcher fires → onboarding marked complete → redirect to the app. All within a frame or two, so
from the user's side the photo step was simply the last thing they saw.

The derived machine is still right — it should stay the source of truth for *order* and for what
is unsettled (that is what makes the flow resumable, task 118). What was wrong is that the view
had no notion of "the step the user is currently looking at". It does now:

- `shown` is latched. The machine decides the order; `shown` decides what is on screen, and it
  only moves when a step hands control back (`done`/`skip`).
- `undefined` ("not decided yet") is deliberately distinct from `null` ("nothing left to do"),
  because the second one completes onboarding and the first must not.
- The completion watcher watches `shown`, not `currentStep`: completion means *the user finished
  the last step*, not *the state happens to look finished*.

## Also fixed: a race that could skip profile confirmation

`profile.ensureLoaded()` returned immediately when a fetch was already in flight — telling the
second caller "done" while the data was still missing. Harmless for the user menu, which renders
reactively, but not for a flow that decides *which step is next* from `confirmation_required` and
`has_photo`: deciding that against an unloaded profile skips the confirmation step. It now awaits
the in-flight request, and `advance()` awaits it before asking for the next step.

## Acceptance Criteria

- [x] A granted notification permission with no subscription still runs the notifications step
- [x] A granted permission *with* a subscription does not
- [x] `denied` / `unavailable` still settle without needing a subscription
- [x] A step on screen is not replaced when state resolves underneath it
- [x] Onboarding completes only after the user finishes the last step
- [x] `ensureLoaded()` awaits an in-flight fetch, so the next step is chosen against a loaded
      profile
- [x] Tests cover the granted-but-unsubscribed case, and were confirmed to fail before the fix
- [x] `vue-tsc`, `npm test` and `npm run build` clean

## Progress Log

- 2026-08-30 — Reported. Traced by asking what could make `currentStep` go null right after the
  photo: the only candidates were the location and notifications steps settling, and
  notifications was settling on permission alone while its own store documents that permission
  and subscription are independent.
- 2026-08-30 — Fixed the predicate, then realised that alone only makes the symptom rarer: any
  step that settles itself on mount can still tear the flow down, so the latch in `WelcomeView`
  is the general fix. Kept both.
- 2026-08-30 — **Verified the regression test fails without the fix**: restoring the old
  predicate fails "still asks for notifications when permission is granted but nothing is
  subscribed", and nothing else. 40 tests pass with it in place.
- 2026-08-30 — ✅ `vue-tsc`, 40 tests, `npm run build` clean.

  Worth knowing: on a device that has already granted location, the location step is still
  skipped — correctly, since a granted permission means the map and the track recorder work and
  there is nothing to decide. If the intent is that the flow should *show* every step once
  regardless, that is a different change (a per-step "seen" flag) and a product decision, not a
  bug.
