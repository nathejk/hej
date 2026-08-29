# 126 — Remove the /login route and repoint its call sites

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

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

- [x] The `/login` route is removed from `router/index.ts`, along with the
      "signed-in users are kept out of `/login`" rule
- [x] The guard's unauthenticated fallback resolves to `welcome`, or to `install` when the
      install gate has not passed — *the `welcome` half; the `install` half is task 137's*
- [x] `UserMenu.vue`'s `signOut()` redirects to `{ name: 'welcome' }` — one sign-out action,
      one destination
- [x] Signing out lands on onboarding **step 1 (login)**, not on `/install`, and profile
      confirmation does not re-run
- [x] No `{ name: 'login' }` or `'login'` route-name reference remains anywhere in `vue/src`
- [x] The guard's comment block, which still describes sending unauthenticated users to
      `/login`, is updated to match the new order
- [x] Landed together with task 131's `showShell` change, so onboarding is never rendered
      with the app chrome
- [x] `vue-tsc` and `npm run build` clean; manual check that an unauthenticated deep link to
      a protected route ends at the onboarding login step — *the manual check is task 139's*

## Depends on

- **Task 125** — `LoginView.vue` must have become `WelcomeStepLogin.vue` first.
- **Task 124** — the `welcome` route must exist before anything is repointed at it.
- **Task 131** — coordinate the `showShell` expression; see above.
- **Task 137** — the router guard (device / standalone / onboarding gates), which owns the
  `install` half of the fallback.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — Picked up.
- 2026-08-30 — **`/login` removed**, `views/LoginView.vue` (task 125's shim) deleted, and the
  guard's comment block rewritten to describe the new order. Three call sites moved to
  `{ name: 'welcome' }`: the guard's unauthenticated fallback, `UserMenu.vue`'s `signOut()` — note
  PRD 005 §7 said `App.vue`, but PRD 003 had already moved the control into the user menu, and
  there is still exactly one sign-out with one destination — and `InstallView.vue`'s escape hatch.

  The "signed-in users are kept out of `/login`" rule became "signed-in users **who have finished
  onboarding** are kept out of `/welcome`". The extra condition matters: being signed in is only
  the first step of onboarding, so the old shape would have bounced a member out of the flow
  immediately after they logged in — straight past profile confirmation and the portrait.
- 2026-08-30 — **`showShell` landed here rather than in task 131**, deliberately: the `login` term
  dies in *this* commit, and leaving it for a later one would have meant shipping a commit where
  the top bar and bottom nav render on top of onboarding. It is now
  `isAuthenticated && onboarding.complete && !route.meta.public`, with a comment explaining why
  `onboardingComplete` is a separate condition from `isAuthenticated`. Task 138 still owns the
  wider shell work (`UpdatePrompt` overlay, `/install` and `/desktop` verification).
- 2026-08-30 — Grepped `'login'` across `vue/src` as instructed: no route-name references remain
  (route names are untyped, so `vue-tsc` would not have caught one). Two prose mentions were left
  or updated — `OfflineNotice.vue`'s comment now points at the onboarding login step instead of the
  deleted view, and `WelcomeStepLogin.vue`'s own comment refers to the move.
- 2026-08-30 — **Ordering risk worth naming:** between this commit and task 137, an authenticated
  user whose `hej.onboarding.complete` flag is absent — i.e. any session that predates this work —
  gets no shell chrome, because nothing yet redirects them into `/welcome` to finish. 137 is the
  next task and closes it. Flagged here rather than worked around, since the workaround would have
  been a second source of truth for "is onboarding done".
- 2026-08-30 — ✅ `vue-tsc`, `npm test` (27) and `npm run build` clean.
