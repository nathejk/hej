# 146 — Portrait nudge after onboarding

**Status:** open
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

**A PRD 005 requirement with no implementation.** Found while auditing what is outstanding on
PRD 005 (2026-08-30): §6 requires it, §11 gives the reasoning, §10 listed it as a task — and the
24 tasks created at approval time do not include it. Task 129 (the onboarding portrait step)
explicitly says "No post-onboarding nudge logic in this component", correctly deferring it here,
and here never existed.

PRD 005 §6:

> A user with no portrait is **nudged again after onboarding**, not asked once and forgotten.
> Onboarding is a single moment and the step is skippable, so a one-shot prompt means the
> members most likely to decline are exactly the ones who stay unidentifiable. The nudge must be
> dismissible per session and must stop permanently once a portrait exists — a prompt that
> cannot be silenced trains people to ignore it.

## Why it matters more than a nudge usually would

The portrait is not decorative. PRD 005 §11 settled its purpose as **night-time identification**:
much of the race runs in the dark, and personnel need to know who they are talking to. A member
who skipped the step is a member whom a samarit cannot identify at 03:00.

And the cohort that skips is not random — it is exactly the members least inclined to bother,
plus everyone who was mid-flow when something interrupted them. Asking once is therefore
systematically biased towards leaving the wrong people unphotographed.

## Constraints from the PRD

- **Dismissible per session.** Not permanently: that would recreate the one-shot problem under a
  different name.
- **Silenced for good once a portrait exists.** Driven by `profile.store.hasPhoto` (from
  `has_photo`, shipped by PRD 003) — not a `localStorage` flag, and not derived from verification
  or lifecycle state.
- **Exactly one nudge surface** (PRD 005 §6). There is already a *passive* one: `UserMenu.vue`
  falls back to initials when there is no portrait, which its comment calls "a standing nudge".
  That is not what §6 asks for and does not conflict with it, but whatever is built here must not
  become a second *active* prompt alongside a third somewhere else.
- Must not block anything. Only login is mandatory.

## Open, and worth deciding before building

- **Where does it appear?** Candidates: a dismissible `Alert` at the top of the map (the most
  visited screen, and the one where covering content costs most), a row on Min profil (honest but
  passive — nobody visits the profile page because they were asked to), or a one-time prompt on
  the next cold start after onboarding.
- **Does it survive an event start?** A member already racing without a portrait is precisely the
  person PRD 005 §11 says needs one most, so probably yes — but a prompt during the race competes
  with the map, which is a safety surface.
- **Sequencing against PRD 007.** Portraits have no viewing surface until PRD 007 ships, so
  nudging now collects photographs nothing can display. Same open question as task 129 recorded;
  it belongs to whoever schedules 007.

## Acceptance Criteria

- [ ] A user with no portrait is prompted again after onboarding completes
- [ ] The prompt is dismissible, and the dismissal lasts for the session only
- [ ] It never appears again once a portrait exists, driven by `hasPhoto` rather than a stored
      flag
- [ ] It is the only *active* nudge surface in the app
- [ ] It blocks nothing and can always be ignored
- [ ] It reuses the existing capture path (`PhotoCapture.vue` → `PUT /api/me/photo`), not a
      second one
- [ ] Copy in Danish, stating the purpose (identification during the race, largely at night)
      rather than merely asking for a photo
- [ ] `vue-tsc`, `npm test` and `npm run build` clean

## Depends on

- **PRD 003, shipped** — `PhotoCapture.vue`, `PUT /api/me/photo`, `profile.store.hasPhoto`.
- **Task 129** — the onboarding step this backstops.
- Worth coordinating with **PRD 007**, which is what makes the photos useful.

## Progress Log

- 2026-08-30 — Task created during an audit of what is outstanding on PRD 005. It was in the
  PRD's own §10 breakdown and was missed when the 24 approval-time tasks were created — so the
  board looked complete while a stated requirement had no owner. Recorded plainly because that is
  the failure mode the board exists to prevent.
