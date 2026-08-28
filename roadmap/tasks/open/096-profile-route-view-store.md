# 096 — Frontend: `/profile` route, `ProfileView.vue` skeleton, `profile.store.ts`

**Status:** open
**Priority:** high
**Created:** 2026-08-28

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

- [ ] `/profile` route (name `profile`) in `router/index.ts`, auth-guarded, no
      role restriction, not full-bleed.
- [ ] `profile.store.ts`: `details`, `loading`, `error`, `fetch()`; safe to call
      repeatedly; a failed fetch leaves a rendered page with an error state, not
      a blank one.
- [ ] `ProfileView.vue` renders an `<h1>` (`font-nathejk`) plus placeholder
      sections for `Mine oplysninger` and `På denne enhed`.
- [ ] No sign-out control on the page (it lives in the user menu).
- [ ] `npm run type-check` + `npm run lint` clean.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
