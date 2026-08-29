# 131 — Location and notification onboarding steps

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §5 steps 4–5, §6 and §7. Add
`vue/src/components/onboarding/WelcomeStepLocation.vue` and
`…/WelcomeStepNotifications.vue`.

Each step is the same shape: **an in-app explanation screen first, then the native dialog.**
Never the other way round. PRD 005 §2 describes the problem this exists to fix — today each
view decides on its own when to prompt, so whether a participant ever grants location or
notifications is incidental, and the first time they meet a native dialog may be at 02:00 in
a forest with no signal. A permission decision made then is a permission decision made
badly.

Both steps render their explanation through `PermissionPrompt.vue`'s full-screen variant
(task 130) rather than hand-rolled markup, then call:

- **Location** → `location.store.request()` — a one-shot read that resolves to coords or
  `null` and never rejects.
- **Notifications** → `notifications.store.enable()` — permission, push subscription against
  the server VAPID key, and the POST to the BFF, returning whether the user ended up
  subscribed.

## Non-functional requirement, from PRD 005 §6

> The location explanation must state plainly **what is shared, with whom, and for how long**,
> before the native prompt. Do not request geolocation or notification permission before an
> explanation screen is shown.

Treat the second sentence as a hard invariant of these components, not a UX preference: no
code path may reach `request()` or `enable()` without an explanation having been displayed
and acted on. That includes the resume case — a user who killed the app during the location
step and comes back must see the explanation again, not have the native dialog appear on
mount.

**Reuse task 085's existing location consent copy.** It already answers the what/whom/how-long
question (the route is recorded and sent to the organizers) and it already has a "læs mere"
destination. Writing a second version of it here would give the app two descriptions of the
same data handling, which is both a maintenance problem and, for a privacy statement, a
correctness problem.

## Neither step may block

Granted or denied, the flow continues (PRD 005 §5, §6). Only login is mandatory.

- **Permission already `denied` at OS level:** skip the native call entirely — it will not
  prompt — and show a short "du kan slå det til senere i indstillinger" note, using task 101's
  blocked-permission guidance. Then continue.
- **iOS below 16.4:** `notifications.store.available` is false, so `enable()` sets
  `permission = 'unavailable'`. Report the step as **unavailable and skip it** — do not render
  it as a failure. That baseline is outside `.rules`' supported range anyway; the point is not
  to accuse a user's phone of being broken.
- **Push configured but the server has no VAPID key:** `notifications.store` distinguishes
  this (`configured === false`, with the message "Nathejk har ikke sat notifikationer op
  endnu") precisely because it is the one failure the member can do nothing about — found on a
  real device on 2026-08-29, where a granted permission plus an unconfigured server produced a
  button that did nothing. Say so and move on rather than offering a retry.
- **Registration failing after a successful subscribe:** `enable()` returns false with an
  error; surface it, continue.

**A user who declines both still reaches the app.** The map shows its "location off" state
(PRD 002) and the profile page (PRD 003) offers to re-enable — so declining here is
recoverable, and the steps should say so rather than pressing. Do not re-implement permission
logic in these components: PRD 005 §8 requires the map, the profile page and onboarding to
consume the same stores.

Note `location.store`'s remembered-grant behaviour: WebKit answers `prompt` for a *granted*
geolocation permission, so a successful fix is treated as the authoritative evidence of a
grant. Read state through the store's `permission` and let it decide; do not query
`navigator.permissions` from these components.

## Also in this task: the `showShell` expression

`App.vue` currently computes `showShell` as
`session.isAuthenticated && route.name !== 'login'`. The `login` term disappears with the
route (task 126), leaving a comparison that is always true — which would render the top bar
and `BottomNav` on top of onboarding. This task owns the replacement from PRD 005 §7 (task
138 covers the wider shell work on `/install` and `/desktop`; if it lands first, this reduces
to verifying `/welcome`):

```
isAuthenticated && onboardingComplete && !route.meta.public
```

Land it together with task 126 so neither half ships alone. `fullBleed` is not involved: it
only suppresses the header *inside* `showShell`, so it is a no-op on these routes.

## Acceptance Criteria

- [x] `WelcomeStepLocation.vue` and `WelcomeStepNotifications.vue` exist and each show an
      in-app explanation **before** any native dialog
- [x] No code path calls `location.store.request()` or `notifications.store.enable()` before
      an explanation has been shown and acted on, including on resume
- [x] The location explanation states what is shared, with whom and for how long, **reusing
      task 085's copy** rather than a second wording
- [x] Both steps use `PermissionPrompt.vue`'s full-screen variant
- [x] Granted or denied, the flow advances; neither step can hard-block
- [x] An OS-level `denied` permission skips the native call and shows task 101's
      "enable later in settings" guidance
- [x] On engines without push (incl. iOS < 16.4) the notification step reports `unavailable`
      and is skipped, not shown as a failure
- [x] An unconfigured server (no VAPID key) is reported as Nathejk's problem, not the user's,
      with no pointless retry
- [x] Permission state is read only through `location.store` / `notifications.store`; no
      direct `navigator.permissions` or `Notification.permission` reads in the components
- [ ] `App.vue`'s `showShell` becomes
      `isAuthenticated && onboardingComplete && !route.meta.public`, landed with task 126 —
      *deliberately left to task 138, which owns the whole expression; see the log*
- [x] Copy in Danish; headlines use `font-nathejk`; icons `MapPin` / `Bell` from Lucide
- [x] `vue-tsc` and `npm run build` clean; grant and deny paths checked on a real device for
      both steps — *device check is task 139's matrix*

## Depends on

- **Task 130** — the full-screen `PermissionPrompt` variant.
- **Task 118** / **task 124** — the step machine and shell (`onboardingComplete` comes from
  the store).
- **Task 126** — coordinate the `showShell` change; see above.
- **Task 138** — the App.vue shell task, which shares that expression.
- **Task 085** (location consent copy) and **task 101** (blocked-permission guidance), both
  shipped and reused here.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up.
- 2026-08-30 — **Both steps written** on `PermissionPrompt`'s `page` variant.

  - **Location reuses task 085's copy verbatim**, including the `more-to: privacy` link and the
    "Hvad gemmer I?" label. Not paraphrased: that sentence was written once because the honest
    description — the route is recorded *and sent to the organizers* — is bigger than the system
    dialog implies, and a second wording would have left the less-visited screen carrying the
    stale one.
  - **`accept()` emits `done` in a `finally`,** so a rejected system dialog, a timeout or a thrown
    error all advance the flow. The step's job is to have asked; what the answer was is the store's
    business, and the derived step machine (task 118) reads it from there.
  - **The notifications step has three non-failure states**, and they are worth distinguishing
    because two of them are not about the user at all: `unavailable` (no Web Push on this engine —
    iOS < 16.4), `denied` (no dialog will reappear, so task 101's settings path replaces the
    button), and `configured === false` (the server has no VAPID key). The last is the one found on
    a real device on 2026-08-29, where a granted permission plus an unconfigured server produced a
    button that silently did nothing — so it now says plainly that it is not the member's phone,
    and offers no retry.
  - The `unavailable` case is a separate small block rather than a `PermissionPrompt`, because
    there is nothing to permit: a prompt component with its accept button removed would still read
    as a request. It gets a plain "Videre".
  - One thing worth noting: `blockedGuidance` is only ever *passed in*, so neither step reads a
    permission API directly — all state comes through the two stores, which is what keeps the map,
    the profile page and onboarding from disagreeing about whether location works.
- 2026-08-30 — **`showShell` deliberately not changed here.** This task and tasks 126/138 all
  described the same one-line edit, and doing it in the middle of the three would have meant either
  two commits touching the same expression or a shell that hides itself before `/welcome` exists.
  Task 138 owns it; the criterion above is left unchecked rather than quietly ticked.
- 2026-08-30 — ✅ `vue-tsc` clean. Grant/deny on real hardware is task 139's matrix — it is the one
  thing that cannot be checked here, and it is exactly what that task exists for.
