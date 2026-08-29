# 129 — WelcomeStepPortrait: self-portrait capture

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §5 step 3, §6 and §7. Add
`vue/src/components/onboarding/WelcomeStepPortrait.vue`: an explanation screen followed by
the camera, so members arrive at the event with a face on file.

**Wrap PRD 003's shipped `components/profile/PhotoCapture.vue` — do not fork it.** That
component is deliberately upload-free (it emits a `captured: [blob]` and knows nothing about
the profile store), which is exactly what makes it reusable here; the upload is
`profile.store.uploadPhoto()` against `PUT /api/me/photo`, also shipped by PRD 003 (tasks
105–107). The face guide, the retake affordance and the **explicit confirm-before-upload**
are requirements *on that component*, specified in PRD 003 §7, because people will not accept
a photo they did not get to approve. If something is missing from those behaviours, fix it in
`PhotoCapture.vue` where every consumer benefits — a second capture implementation is the one
outcome PRD 005 §6 names explicitly as forbidden.

## What the step says, and what it does not ask

Explain the **purpose**: the photo is used to identify people during the race, much of which
runs at night when faces are hard to see. A samarit or guide at 03:00 needs to know who they
are talking to without asking a tired teenager to spell their name (PRD 005 §5).

**Consent is not a gate.** PRD 005 §11 and task 102 (recorded in PRD 003 §6) settled this on
2026-08-28: the parental consent is already held from sign-up, the basis is
safety/identification, and portraits are purged after the event. The step therefore has **no
in-app consent gate, no checkbox and no consent text** — what remains is explaining the
purpose, not obtaining permission. Earlier PRD drafts treated consent as blocking; that is
resolved, and re-adding a consent tick here would both duplicate a permission already held
and imply the sign-up consent was insufficient.

## Skippable, and degrading

**Skippable** (PRD 005 §6): a member who declines still gets into the app, with the profile
page (PRD 003) as the place to add one later. Only login is mandatory.

Degradation ladder, per PRD 005 §5:

1. `getUserMedia` via `PhotoCapture.vue`.
2. Camera denied or unavailable → `<input type="file" accept="image/*" capture="user">`.
3. That also failing → **continue without a photo**. Never block.

Note `PhotoCapture.vue` already distinguishes "no secure context" from "permission denied",
which matters on a dev stack reached over plain http where camera, service worker and
geolocation all vanish at once; do not paper over that distinction with a generic message
here. Task 101's blocked-permission guidance (`config/permissions.ts`) is the place a
hard-denied camera sends the user.

## It runs for every user with no portrait

Including — especially — users whose profile confirmation was skipped because they had
already started the event (PRD 005 §6, §11 2026-08-25). Verification status and portrait
status are unrelated facts: one says something about the guardian number, the other about
whether there is a face on file. A member already on the trail without a portrait is exactly
the person personnel will fail to identify at 03:00, so letting the skip cascade would remove
the nudge from the cohort that needs it most.

Eligibility is `profile.store.hasPhoto` (from `has_photo`, shipped by PRD 003) — not a
`localStorage` flag, and not derived from confirmation state.

The **post-onboarding re-nudge** for users who skipped (dismissible per session, silenced
permanently once a portrait exists) is PRD 005 §6's separate requirement and a separate task;
this step must not try to be that surface as well. PRD 005 is clear there must be exactly one
nudge surface.

## Sequencing risk worth naming

PRD 005 §8: **portraits have no viewing surface until PRD 007 ships.** Capturing before that
means asking every member for a photo that nothing consumes, which is worth knowing before
this goes live during an event — and PRD 007 also constrains sizing, since the identification
thumbnail is generated at upload (tasks 104/110 already do that server-side). Not a blocker
for building the step; a blocker for claiming the feature delivers its stated value. Record
the state of PRD 007 in this task's log when picking it up.

## Acceptance Criteria

- [ ] `components/onboarding/WelcomeStepPortrait.vue` exists and **reuses**
      `components/profile/PhotoCapture.vue` — no second capture implementation, no changes
      that only serve onboarding
- [ ] Upload goes through `profile.store.uploadPhoto()` / `PUT /api/me/photo`
- [ ] The user confirms the photo before it uploads, and can retake (behaviour owned by
      `PhotoCapture.vue`)
- [ ] Explanation copy states the purpose — identifying people during the race, largely at
      night — and asks for **no consent**
- [ ] The step is skippable, and skipping reaches the next step
- [ ] Camera denied/unavailable falls back to `<input type="file" accept="image/*"
      capture="user">`; if that fails too, the flow continues without a photo
- [ ] The step runs for every user where `hasPhoto` is false, including those whose profile
      confirmation was skipped for having started the event
- [ ] A hard-denied camera points at task 101's blocked-permission guidance rather than a
      dead end
- [ ] Upload failure (PRD 003's 503/400) is surfaced with a retry, and does not leave the
      user believing a photo is on file
- [ ] No post-onboarding nudge logic in this component
- [ ] Copy in Danish; headline uses `font-nathejk`
- [ ] `vue-tsc` and `npm run build` clean

## Depends on

- **PRD 003, shipped** — `PhotoCapture.vue` (task 106), `PUT /api/me/photo` (task 105),
  `profile.store.hasPhoto`.
- **Task 118** / **task 124** — the step machine and shell.
- **PRD 007** — not a build blocker, but it is what makes the portraits useful; see above.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
