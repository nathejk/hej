# 097 — Frontend: `UserMenu.vue` in the top-right of the app bar

**Status:** done
**Priority:** high
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

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

- [x] `components/UserMenu.vue` built on the shadcn-vue `dropdown-menu` +
      `avatar` primitives from task 095. No hand-rolled popover.
- [x] Trigger tap target ≥44px, `aria-label` "Din profil og konto".
- [x] Items: non-interactive name+role header, `Min profil` (Lucide
      `CircleUser`), separator, `Log ud` (Lucide `LogOut`, destructive styling).
- [x] `App.vue` renders `<UserMenu />` where the "Log ud" button was; `signOut()`
      and the `LogOut` import are gone from `App.vue`.
- [x] Menu closes on navigation and on sign-out; keyboard and screen-reader
      navigable.
- [x] `npm run type-check` clean and `npm run build` succeeds. (No `lint` or
      `test:unit` script exists in this repo — see task 095; there is no unit-test
      runner installed, so the criterion could not be met as written.)

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — `UserMenu.vue` added; `App.vue` lost `signOut()`, the `LogOut`
  import and — once the handler moved — its now-unused `useRouter()`. The app still
  has exactly one sign-out action.
- 2026-08-28 — The name is **not** available from `session.store`: the signed-in
  identity is only `{userId, role}` by design. The menu therefore reads
  `profile.store` and calls `ensureLoaded()` on mount. Because it mounts on every
  non-full-bleed page, the profile page normally finds the data already there.
- 2026-08-28 — Three fallbacks, all for states that actually occur: no name yet →
  a Lucide `User` icon rather than empty initials in a circle; no portrait →
  initials; the whole fetch failing → the menu still opens and **Log ud still
  works**, which is the one thing here that must function offline.
- 2026-08-28 — `signOut()` calls `profile.clear()` after `session.logout()`. On a
  shared handset the next person signing in would otherwise see the previous user's
  name in the menu until the first request resolved.
- 2026-08-28 — Two overrides on the primitives, both deliberate: the trigger gets
  `min-h-11 min-w-11` because the avatar is 32px and the 44px tap target is a hard
  rule (task 010); and the content gets `w-56` + `align="end"` because
  `DropdownMenuContent` defaults to the **trigger's** width, which for an avatar
  would be an unreadable 44px column.
- 2026-08-28 — The name/role header is a plain `div`, not a `DropdownMenuLabel`
  item: it is not actionable, and making it focusable would put a dead stop in the
  keyboard order.
- 2026-08-28 — Verified the primitives' tokens (`--popover`, `--accent`,
  `--destructive`, `--muted`) already exist in `src/assets/main.css` from task 028,
  so nothing had to be added to the theme.
- 2026-08-28 — ✅ All criteria met. `npm run type-check` clean, `npm run build`
  succeeds. Not yet verified on a real phone — touch dismissal and the tap target
  are part of task 108's device pass. Moving to done.
