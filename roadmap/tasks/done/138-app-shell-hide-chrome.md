# 138 — App.vue: hide shell chrome on install, welcome and desktop

**Status:** done
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §7. Both the top bar **and** `BottomNav` are hidden on `/install`, `/welcome` and
`/desktop`. The install wall, the onboarding flow and the desktop placeholder are each a
full-page surface with a single job; framing them in app chrome offers navigation into an
app the user has not yet been let into.

`showShell` becomes:

```ts
isAuthenticated && onboardingComplete && !route.meta.public
```

Spelled out as a full expression because today it reads
`session.isAuthenticated && route.name !== 'login'` — and the `login` term **disappears
with the route** (task 126). Replacing one name with three would also be wrong: the
condition is not "which routes are special", it is "has this user finished getting in".
`route.meta.public` already marks all three new routes (PRD 005 §7), so no route-name list
is needed and a fourth public route added later needs no change here.

**`fullBleed` is deliberately not used** on these routes. It only suppresses the header
*inside* `showShell`, so on a route where `showShell` is already false it is a no-op —
setting it would look like it was doing the work and mislead the next reader.

Note `UpdatePrompt` and `LayoutDebug` render **outside** the shell today, which is
correct and should stay that way — with one caveat from PRD 005 §8: `UpdatePrompt` must
not overlay the install wall.

## Acceptance Criteria

- [x] `showShell` is `isAuthenticated && onboardingComplete && !route.meta.public`
- [x] The `route.name !== 'login'` term is gone
- [x] Top bar and `BottomNav` are both absent on `/install`, `/welcome` and `/desktop`
- [x] `onboardingComplete` is read from the onboarding store, not recomputed in `App.vue`
- [x] No `fullBleed` added to the three routes, with a comment saying why it would be a
      no-op
- [x] `UpdatePrompt` does not overlay the install wall
- [x] Authenticated, onboarded routes look exactly as they do today

## Depends on

- **Task 118** — the onboarding store supplies `onboardingComplete`.
- **Task 123** — `DesktopView`.
- **Task 126** — removal of the `/login` route; this task's expression assumes it is gone.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — **The `showShell` expression itself landed in task 126**, deliberately: the `login`
  term it tested died in that commit, so leaving it for this one would have shipped an intermediate
  state with the top bar and bottom nav drawn over onboarding. It reads
  `session.isAuthenticated && onboarding.complete && !route.meta.public`, with `complete` coming
  from the onboarding store rather than being recomputed here. `BottomNav` needed no separate
  change — it is inside the `v-if="showShell"` subtree, so all three routes lose it with the header.
- 2026-08-30 — **`UpdatePrompt` suppressed on `/install`.** It is `Teleport`ed to the body, fixed to
  the top of the viewport at `z-60`, so on the wall it would land squarely over the explanation and
  the install button — the one screen where the user has exactly one thing to do and no way to do
  anything else. Suppressed rather than restyled: a waiting build is not urgent for a tab whose only
  job is to get the app onto the home screen, and the new version is picked up the moment they open
  it from there. Left visible on `/welcome` and `/desktop`, where reloading costs nothing.
- 2026-08-30 — No `fullBleed` on any of the three routes; the reason (a no-op outside `showShell`,
  which would then mislead whoever next edits that expression) is recorded in `WelcomeView.vue` and
  restated in `App.vue`'s comment.
- 2026-08-30 — ✅ `vue-tsc` and `npm run build` clean. Authenticated app routes are untouched: the
  only change in their path is that `showShell` now also requires `onboarding.complete`, which is
  true for anyone who has been through the flow.
