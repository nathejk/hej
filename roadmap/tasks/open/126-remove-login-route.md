# 126 — Remove the /login route and repoint its call sites

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §7. `/login` disappears as a standalone route. The credential step lives inside
`/welcome` as `WelcomeStepLogin.vue` (task 125), so a route that renders login on its own
would be a second entry point into the app that bypasses the install gate and the rest of
onboarding — precisely what this PRD exists to prevent.

PRD 005 §7 made this explicit on 2026-08-25 because an earlier wording said `/login` was
"retained … inside `/welcome`", which left it ambiguous whether the route still existed and
what its callers should do. It does not exist. PRD 004's route inventory should drop it too.

**Two shipped call sites hard-code `{ name: 'login' }` and must move:**

1. `router/index.ts` — the guard's unauthenticated fallback,
   `if (!to.meta.public && !session.isAuthenticated) return { name: 'login' }`. It becomes
   `{ name: 'welcome' }`, or `{ name: 'install' }` when the install gate has not passed.
   The paired rule below it — `if (to.name === 'login' && session.isAuthenticated)` keeping
   signed-in users out of the login screen — goes away with the route; `/welcome` handles a
   signed-in visitor by resuming at the first unsettled step rather than by redirecting.
2. The sign-out action. PRD 005 §7 names `App.vue`, and that is where it lived when the PRD
   was written; **PRD 003 (task 097) has since moved the control into
   `components/UserMenu.vue`**, whose `signOut()` calls `session.logout()`, `profile.clear()`
   and then `router.replace({ name: 'login' })`. PRD 003 moved *where the control sits*, not
   *where it goes*. There is one sign-out action in the app with one destination, and that
   destination becomes `{ name: 'welcome' }`.

**Logout lands on onboarding step 1, never on the install wall** (PRD 005 §5, edge cases).
The app is still installed — sending a user who just signed out back through an install wall
would be a lie about the state of their device and, on iOS, an instruction they cannot even
act on twice. Profile confirmation does not run again either; that is server-side per-user
state (PRD 005 §6), so it does not resurface on sign-in.

Also note, without owning it: `App.vue`'s `showShell` is currently
`session.isAuthenticated && route.name !== 'login'`. The `login` term becomes a comparison
against a route name that no longer exists — always true — so the shell would render over
`/welcome`. The replacement expression
`isAuthenticated && onboardingComplete && !route.meta.public` (PRD 005 §7) is **task 131's**
to land. This task must not leave the two half-done independently: if `/login` is removed
before the shell expression changes, onboarding renders with a top bar and bottom nav.

Grep for `'login'` across `vue/src` before finishing — the two above are the ones known
today, and a stale string reference will not be caught by `vue-tsc` since route names are
untyped.

## Acceptance Criteria

- [ ] The `/login` route is removed from `router/index.ts`, along with the
      "signed-in users are kept out of `/login`" rule
- [ ] The guard's unauthenticated fallback resolves to `welcome`, or to `install` when the
      install gate has not passed
- [ ] `UserMenu.vue`'s `signOut()` redirects to `{ name: 'welcome' }` — one sign-out action,
      one destination
- [ ] Signing out lands on onboarding **step 1 (login)**, not on `/install`, and profile
      confirmation does not re-run
- [ ] No `{ name: 'login' }` or `'login'` route-name reference remains anywhere in `vue/src`
- [ ] The guard's comment block, which still describes sending unauthenticated users to
      `/login`, is updated to match the new order
- [ ] Landed together with task 131's `showShell` change, so onboarding is never rendered
      with the app chrome
- [ ] `vue-tsc` and `npm run build` clean; manual check that an unauthenticated deep link to
      a protected route ends at the onboarding login step

## Depends on

- **Task 125** — `LoginView.vue` must have become `WelcomeStepLogin.vue` first.
- **Task 124** — the `welcome` route must exist before anything is repointed at it.
- **Task 131** — coordinate the `showShell` expression; see above.
- **Task 137** — the router guard (device / standalone / onboarding gates), which owns the
  `install` half of the fallback.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
