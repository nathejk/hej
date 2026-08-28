# 099 — Frontend: `PreferenceRow.vue` + `På denne enhed` section

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

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

- [ ] `components/profile/PreferenceRow.vue` — icon (Lucide), label, status text,
      trailing slot for action/guidance.
- [ ] Push, location and camera rows wired to real store state.
- [ ] Re-sync on `visibilitychange`, with the listener removed on unmount.
- [ ] Unavailable (unsupported browser) renders distinctly from denied.
- [ ] No colour-only status; each row states its state in words.
- [ ] Unit test covering the state → row-rendering matrix.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10. Install row explicitly deferred to
  PRD 005.
