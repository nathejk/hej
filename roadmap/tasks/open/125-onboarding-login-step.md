# 125 — Refactor LoginView into an onboarding login step

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §7. Move `vue/src/views/LoginView.vue` to
`vue/src/components/onboarding/WelcomeStepLogin.vue` so the credential step lives *inside*
`/welcome` instead of at its own route.

The reason this is a move and not a new component: the login screen is not a form, it is a
small state machine with three steps — `phone` → `pin` → `choose` — plus a resend cooldown,
an offline branch, and specific error mappings (401 = "Koden passer ikke", 429 = "For mange
forsøg", `NetworkError` = "Ingen forbindelse", and a 401 on `choose` meaning the ~1-minute
token expired rather than a bad code). The `choose` step exists because roughly one number
in eight belongs to more than one person (task 079) and guessing would sign someone in as a
sibling. Reimplementing any of that inside `/welcome` would produce a second, subtly
different login, and the two would drift.

So: **move the file, keep the logic**. It talks to `session.store` `requestPin` / `verify` /
`choose` / `clearChoice` exactly as it does today.

**Reworking the login mechanism is explicitly a non-goal** (PRD 005 §4) — phone + SMS PIN
stays as it is. Do not take this task as licence to change the PIN length, the cooldown, the
anti-enumeration behaviour (the PIN step always follows the phone step whether or not the
number is recognised), or the nødtelefon fallback copy.

What does change is what happens on success. Today `submitPin` and `choose` both
`router.replace({ path: '/' })`, deliberately routed by path so the view does not need to
know the first destination's name. As a step, it must instead hand control back to the
onboarding store and let the flow advance to the next applicable step — a first-time spejder
goes to profile confirmation, not to `/maps`. The `/` redirect is the shell's business at the
end of the flow (task 124), not this component's.

**Login is the only mandatory step** in the flow (PRD 005 §6): every other step is skippable,
and this one is not. It therefore has no skip affordance and no "continue anyway" path — the
escape hatches in this PRD are on the install wall, not here.

Presentation notes: the standalone view centred itself in the viewport with its own safe-area
padding and rendered the app name, the `KeyRound` badge and the version string. Inside the
shell, the version string and the outer layout belong to the shell; keep the step's own
markup to the credential UI so two components are not both padding for `--sat`/`--sab`.

`vue/src/views/LoginView.vue` should be gone at the end of this task. Removing the `/login`
route and repointing its two call sites is task 126 — sequence them together, since deleting
the view without task 126 leaves a route pointing at a missing module.

## Acceptance Criteria

- [ ] `components/onboarding/WelcomeStepLogin.vue` exists and `views/LoginView.vue` is
      removed
- [ ] The phone → PIN → choose state machine, resend cooldown, offline branch and error
      mappings are carried over unchanged in behaviour
- [ ] `session.store` `requestPin` / `verify` / `choose` / `clearChoice` are used as before;
      no change to the login mechanism (PRD 005 §4)
- [ ] The shared-number chooser (task 079) still works, including the expired-token 401 path
      that returns to the phone step
- [ ] On success the step advances the onboarding flow rather than navigating to `/`
- [ ] The step exposes **no skip** affordance — login is mandatory
- [ ] Layout/safe-area padding and the version string are left to the shell; no duplicated
      page chrome
- [ ] All copy in Danish, unchanged in meaning from the shipped view
- [ ] `vue-tsc` and `npm run build` clean, with no remaining imports of `LoginView.vue`

## Depends on

- **Task 118** — the onboarding store, for advancing to the next step.
- **Task 124** — the shell that renders it.
- Sequence with **task 126**, which removes the `/login` route and its call sites.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
