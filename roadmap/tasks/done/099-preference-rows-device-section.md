# 099 — Frontend: `PreferenceRow.vue` + `På denne enhed` section

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

PRD 003 §6: a compact status row per device capability — icon, label, status
text, trailing action or guidance — reusing the visual language of
`PermissionPrompt.vue` without forking it.

Rows in scope now:

- **Push notifications** — `notifications.store` (`available`, `permission`,
  `subscribed`). Needs task 100 to be accurate after a reload.
- **Location sharing** — `location.store` (`available`, `permission`).
- **Kamera** — whether the portrait camera is usable on this device.

The **Installed as app** row is deferred: `install.store` is owned by PRD 005 and
does not exist yet. Leave the list shaped so it can be appended without a
redesign (a preferences list, not a bespoke layout) and do not stub the store
here.

Permission states must be re-synced on `visibilitychange` so a change made in
browser settings is reflected on return. A blocked permission shows guidance
(task 101), never a button that cannot work.

Accessibility: status is conveyed in **text**, not colour alone.

## Acceptance Criteria

- [x] `components/profile/PreferenceRow.vue` — icon (Lucide), label, status text,
      trailing slot for action/guidance.
- [x] Push, location and camera rows wired to real store state.
- [x] Re-sync on `visibilitychange`, with the listener removed on unmount.
- [x] Unavailable (unsupported browser) renders distinctly from denied.
- [x] No colour-only status; each row states its state in words.
- [ ] Unit test covering the state → row-rendering matrix — **not done: no
      unit-test runner in this repo** (see task 098). The mapping is expressed as
      three `computed`s returning plain objects, which is at least testable the
      moment a runner exists.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10. Install row explicitly deferred to
  PRD 005.
- 2026-08-28 — `PreferenceRow.vue` takes `status` as **text** and an optional
  `detail`/`action`; it is a sibling of `PermissionPrompt.vue`, not a wrapper. Kept
  separate deliberately: the prompt *asks* for a permission, this row *reports* on
  one, and merging them would produce a card that sometimes has buttons and
  sometimes prose.
- 2026-08-28 — Each capability's state→row mapping is a `computed` returning
  `{status, detail, action?}`, so the matrix lives in one readable place per row
  rather than as `v-if`s in the template.
- 2026-08-28 — Push has **four** states, not three: unsupported, blocked, granted
  but *not subscribed*, and subscribed. The third is the one task 100 exists for —
  it is where push silently does not work — and it gets its own "Ikke tilmeldt"
  status plus a Tilmeld action.
- 2026-08-28 — Camera: no store yet (its capture component, task 106, is blocked on
  the consent decision), so the row reports only what is knowable without opening
  the camera. WebKit cannot be queried for the camera permission, so that path
  stays `unknown` and renders "Klar — du bliver spurgt …" rather than guessing
  "Fra", which would tell an iPhone user their camera is off when it is not.
- 2026-08-28 — Location's copy says the route is stored and sent to the organizers,
  and does **not** say "placering deles": PRD 003 §11 flags exactly this wording
  risk, and "til" alone implies somebody is watching a live dot.
- 2026-08-28 — Re-sync happens on mount and on `visibilitychange` (listener removed
  on unmount), which is the only reliable moment — a permission changed in system
  settings does not notify the page.
- 2026-08-28 — ✅ `npm run type-check` and `npm run build` clean. Real device
  behaviour is task 108. Moving to done.
