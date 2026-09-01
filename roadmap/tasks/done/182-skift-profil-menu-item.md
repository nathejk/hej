# 182 — "Skift profil" in the user menu

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

From **PRD 012**. The user-facing half: a **Skift profil** item in the app bar's user menu, which
ends the current session and starts one as the chosen profile.

Placement and behaviour, from PRD 012 §7:

- In the existing shadcn-vue `dropdown-menu` in `UserMenu.vue`, above **Log ud**.
- **Shown only when the number carries more than one profile** (`profile_count` from `/api/me`), so
  it is invisible to the majority. A control that answers "you have nothing to switch to" is worse
  than no control.
- Candidates in a `dialog`, using task 181's shared component — not by routing through onboarding to
  reach a list the user is already looking at.
- After switching, land on **`/maps`**: the current route may be role-gated and no longer permitted
  for the new profile, and the guard bouncing the user elsewhere would read as a glitch.
- Offline or on failure, the current session is left untouched and the error is explicit. Nothing
  half-switched.

Note there is no signed-out gap: `/auth/choose` issues a session cookie that **replaces** the
previous one, so the switch is a single request rather than a sign-out followed by a sign-in.

## Implementation

`UserMenu.vue` (item + dialog), `session.store.ts` (`profileCount`, `canSwitchProfile`,
`startProfileSwitch`), tests in `stores/switchprofile.spec.ts`.

**A full page load after switching, not a router push.** Every store in memory belongs to the profile
that just stopped being signed in — the contacts directory, favourites, profile details — and while
their *persisted* copies are keyed per profile (task 180), the in-memory ones are not. Reloading is the
one move that cannot leave a stale store behind through a path somebody forgets to reset, and on a PWA
the assets come from cache so it costs little. Landing on `/maps` because the current route may be
role-gated and no longer permitted — a guard bouncing the user would read as a glitch.

**The dialog lives outside the dropdown.** The menu closes on select, and a dialog nested inside it
would be torn down with it.

**`profileCount` is set by the login chooser too**, from the candidate count — which *is* the number of
profiles on that number. Without that the switcher would be missing for the rest of the session that
just used the chooser, until the next `/api/me`.

**Not persisted with the remembered identity**, deliberately: on an offline start the count is unknown,
and a switch cannot complete without the network anyway. So the control is absent offline rather than
present and broken.

**`startProfileSwitch` throws**, unlike `fetchMe`, which deliberately never does. Here there is an
explicit user action to report on, and silently doing nothing after a tap is worse than a message. The
dialog distinguishes "kræver forbindelse" from a generic failure, since only one of those is
actionable.

## Acceptance Criteria

- [x] Menu item present, above Log ud, hidden when the caller has one profile.
- [x] Opens the candidate list; choosing issues the new session and lands on `/maps`.
- [x] Role-scoped state is gone afterwards — persisted state by task 180's keys, in-memory state by
      the full reload.
- [x] A failed switch leaves the user signed in as they were, with a visible message; offline says it
      needs a connection. Both asserted.
- [x] Not offered on `/maps`, which has no app bar — inherited from `UserMenu`'s placement.
- [x] Tests: the item's visibility follows the profile count (including absent, one, several, and
      after sign-out); a failed switch does not clear the session.

## Progress Log

- 2026-09-01 — Task created from PRD 012 §10.
- 2026-09-01 16:20 — Chose a full page load over a router push after weighing it: resetting each store
  by hand would work until the next store is added and forgotten.
- 2026-09-01 16:30 — Added `profileCount` from the login chooser's candidate count, so the switcher is
  not missing for the session that just disambiguated.
- 2026-09-01 16:35 — ✅ All criteria met. 186 frontend tests pass, `vue-tsc` clean, production build
  clean, Go gates clean under the pinned resolution.
