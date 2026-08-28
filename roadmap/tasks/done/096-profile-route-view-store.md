# 096 — Frontend: `/profile` route, `ProfileView.vue` skeleton, `profile.store.ts`

**Status:** done
**Priority:** high
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

The page itself, empty of content: an auth-guarded `/profile` route for **all**
roles, a `ProfileView.vue` with the section scaffolding from PRD 003 §7, and a
`profile.store.ts` fetching `GET /api/me/profile` (task 094) through
`fetchWrapper`.

Explicitly **not** a nav destination: PRD 003 §7 (decided 2026-08-28) reaches
the page from the top-bar user menu, so `config/navigation.ts` is untouched.

Headline uses `font-nathejk` per `.rules`; body text stays on the system sans
stack. All copy in Danish.

## Acceptance Criteria

- [x] `/profile` route (name `profile`) in `router/index.ts`, auth-guarded, no
      role restriction, not full-bleed.
- [x] `profile.store.ts`: `details`, `loading`, `error`, `fetch()`; safe to call
      repeatedly; a failed fetch leaves a rendered page with an error state, not
      a blank one.
- [x] `ProfileView.vue` renders an `<h1>` (`font-nathejk`) plus placeholder
      sections for `Mine oplysninger` and `På denne enhed`.
- [x] No sign-out control on the page (it lives in the user menu).
- [x] `npm run type-check` clean. (No `lint` script exists in this repo — see
      task 095.)

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Path is `/profil`, not `/profile`: every other user-visible path in
  this app is Danish (`/sporing`). The route *name* stays `profile`, since that is
  what code refers to.
- 2026-08-28 — Route is auth-guarded for free by the global `beforeEach` and
  carries no `meta.roles`, which is how "all roles" is expressed here.
- 2026-08-28 — `profile.store.ts` kept separate from `session.store` on purpose,
  and the store's doc says why: the session store is consulted by the router guard
  on every navigation and must not grow a dependency on a request that is allowed
  to fail. `fetch()` never throws, mirroring `scans.store`.
- 2026-08-28 — Added `ensureLoaded()` and `clear()` beyond the task's list, both
  forced by the user menu (task 097): the menu mounts on every page, so it needs a
  fetch-once entry point, and on a **shared handset** the next person to sign in
  would otherwise see the previous user's name in the menu until the first request
  resolved. Also an `initials` getter for the avatar — it takes the first and
  **last** name parts, so "Anne Sofie Jensen" reads AJ rather than AS.
- 2026-08-28 — Added `ROLE_LABELS` to `config/roles.ts` (typed `Record<Role,
  string>`, so a new role without a label is a type error, not a blank badge). No
  such map existed; putting it in the component would have guaranteed three
  spellings of "gøgler".
- 2026-08-28 — The view renders its error banner *above* the sections rather than
  instead of them: task 099's permission rows read local device state and are
  useful precisely when the network is not.
- 2026-08-28 — ✅ All criteria met. `npm run type-check` clean. Moving to done.
