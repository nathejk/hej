# 097 — Frontend: `UserMenu.vue` in the top-right of the app bar

**Status:** open
**Priority:** high
**Created:** 2026-08-28

## Description

PRD 003 §7 (decided 2026-08-28): the shell's top bar carries an **avatar button**
at its trailing edge, replacing the standalone "Log ud" button. It opens a
dropdown with a name+role header, **Min profil** → `/profile`, a separator, and
**Log ud**.

`signOut()` moves out of `App.vue` into this component. There must remain exactly
**one** sign-out action in the app with one destination (PRD 005 §7 owns where it
goes — today `{ name: 'login' }`).

The avatar shows the portrait once one exists (task 107) and initials until then,
which doubles as a standing nudge. `session.store`'s `Identity` carries only
`{userId, role}`, so the name comes from `profile.store` — the menu must render
sensibly before that resolves (fall back to a role label / a generic icon rather
than flashing empty initials).

`/maps` is full-bleed and has no top bar, so the menu is simply absent there —
accepted, not worked around (the map's top-right is the layer switcher).

## Acceptance Criteria

- [ ] `components/UserMenu.vue` built on the shadcn-vue `dropdown-menu` +
      `avatar` primitives from task 095. No hand-rolled popover.
- [ ] Trigger tap target ≥44px, `aria-label` "Din profil og konto".
- [ ] Items: non-interactive name+role header, `Min profil` (Lucide
      `CircleUser`), separator, `Log ud` (Lucide `LogOut`, destructive styling).
- [ ] `App.vue` renders `<UserMenu />` where the "Log ud" button was; `signOut()`
      and the `LogOut` import are gone from `App.vue`.
- [ ] Menu closes on navigation and on sign-out; keyboard and screen-reader
      navigable.
- [ ] `npm run type-check` + `npm run lint` clean; `npm run test:unit` green.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
