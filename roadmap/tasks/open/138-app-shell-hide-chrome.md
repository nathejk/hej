# 138 — App.vue: hide shell chrome on install, welcome and desktop

**Status:** open
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `showShell` is `isAuthenticated && onboardingComplete && !route.meta.public`
- [ ] The `route.name !== 'login'` term is gone
- [ ] Top bar and `BottomNav` are both absent on `/install`, `/welcome` and `/desktop`
- [ ] `onboardingComplete` is read from the onboarding store, not recomputed in `App.vue`
- [ ] No `fullBleed` added to the three routes, with a comment saying why it would be a
      no-op
- [ ] `UpdatePrompt` does not overlay the install wall
- [ ] Authenticated, onboarded routes look exactly as they do today

## Depends on

- **Task 118** — the onboarding store supplies `onboardingComplete`.
- **Task 123** — `DesktopView`.
- **Task 126** — removal of the `/login` route; this task's expression assumes it is gone.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
