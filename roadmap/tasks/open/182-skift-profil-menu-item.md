# 182 — "Skift profil" in the user menu

**Status:** open
**Priority:** high
**Created:** 2026-09-01

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

## Acceptance Criteria

- [ ] Menu item present, above Log ud, hidden when the caller has one profile.
- [ ] Opens the candidate list; choosing issues the new session and lands on `/maps`.
- [ ] Role-scoped state is gone afterwards — nav, router guard and the contacts pane all reflect the
      new profile's role (rests on task 180).
- [ ] A failed switch leaves the user signed in as they were, with a visible message; offline says
      it needs a connection.
- [ ] Not offered on `/maps`, which has no app bar — the accepted limitation Min profil and Log ud
      already share.
- [ ] Tests: the item's visibility follows the profile count; a failed switch does not clear the
      session.

## Progress Log

- 2026-09-01 — Task created from PRD 012 §10.
