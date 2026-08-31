# 146 — Portrait nudge after onboarding

**Status:** done
**Priority:** medium
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

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

## Decided before building

- **Where:** an in-flow banner in the app shell, on the ordinary content routes. Not an overlay
  anywhere — it pushes content down and cannot cover anything, which is the same rule
  `OfflineNotice` follows and the reason it is placed beside it.

  Off the **map**, structurally rather than by name: the map is the one full-bleed route, it is an
  operational surface during the race, and it already carries the location pre-prompt and the offline
  notice. Off **SOS**, because nothing may compete for attention on an emergency page and a member
  opening it is not about to take a selfie. Off **Min profil**, because the real photo control is
  already there and a banner above it asking for a photo is noise next to the affordance it points
  at.

  The rejected options and why: an alert on the map is the most-seen place and the worst one to
  occupy; a row on Min profil is honest but passive, and nobody visits that page because they were
  asked to; a one-shot prompt on the next cold start is the one-shot problem again with extra steps.
- **Does it survive an event start:** yes, and there is deliberately no lifecycle condition to find.
  PRD 005 §11 says a racing member without a portrait is precisely who needs one most. The concern
  about competing with the race surface is answered by the placement rather than by a rule — it never
  appears on the map.
- **PRD 007 sequencing:** unchanged and still open. This collects photographs nothing can display
  until 007 ships. Worth a product decision before an event, not a code one.

## Acceptance Criteria

- [x] A user with no portrait is prompted again after onboarding completes
- [x] The prompt is dismissible, and the dismissal lasts for the session only
- [x] It never appears again once a portrait exists, driven by `hasPhoto` rather than a stored
      flag
- [x] It is the only *active* nudge surface in the app
- [x] It blocks nothing and can always be ignored
- [x] It reuses the existing capture path (`PhotoCapture.vue` → `PUT /api/me/photo`), not a
      second one
- [x] Copy in Danish, stating the purpose (identification during the race, largely at night)
      rather than merely asking for a photo
- [x] `vue-tsc`, `npm test` and `npm run build` clean

## Depends on

- **PRD 003, shipped** — `PhotoCapture.vue`, `PUT /api/me/photo`, `profile.store.hasPhoto`.
- **Task 129** — the onboarding step this backstops.
- Worth coordinating with **PRD 007**, which is what makes the photos useful.

## Progress Log

- 2026-08-30 — Task created during an audit of what is outstanding on PRD 005. It was in the
  PRD's own §10 breakdown and was missed when the 24 approval-time tasks were created — so the
  board looked complete while a stated requirement had no owner. Recorded plainly because that is
  the failure mode the board exists to prevent.

## Progress Log (continued)

- 2026-08-31 — **Built.** Four pieces:

  - `config/nudge.ts` — `showPortraitNudge(ctx)` plus the route exclusions, as data with the
    reasoning attached. Pure, so the policy is unit-tested without a DOM; same split as
    `router/gates.ts`.
  - `components/PortraitNudge.vue` — the banner, rendered once in the shell beside `OfflineNotice`.
    It decides for itself whether the current route should show it, so there is no condition in
    `App.vue` to keep in sync.
  - `app.store.portraitNudgeDismissed` — **in memory, not localStorage.** Persisting it would turn
    "not now" into "never", which is the one-shot behaviour this task exists to fix, and the members
    who dismiss are the ones most likely to stay unidentifiable. A restart asking again is the
    feature, and it needs no reset on sign-out because a new session is a new store.
  - `composables/usePortraitCapture.ts` — capture/upload/retry, now shared with the onboarding step
    (task 129) instead of duplicated. The interesting part is `pending`: a retry re-sends *that
    photo* rather than making a tired teenager retake it, since the thing that failed was the upload
    and this runs over rural mobile data at night.

  `ProfilePhoto.vue` deliberately keeps its own copy: it is PRD 003's shipped component with a
  different contract (it renders the current portrait, and its errors belong to a page rather than a
  step), so folding it in would mean editing shipped code to serve a refactor rather than a
  requirement. Noted in the composable as worth revisiting if a fourth caller appears.
- 2026-08-31 — Two details worth recording because they are the difference between a nudge and
  nagging: **there is no completion state** — a successful upload sets `hasPhoto`, which makes the
  banner false, so it removes itself because the thing it asked for now exists; and the **dismiss
  control is a labelled 44px button**, not a bare glyph, because it is the affordance that keeps
  this acceptable and must be obvious.
- 2026-08-31 — The visibility rule also has a `profileLoaded` guard, which is less obvious than it
  looks: without it the banner flashes on every cold start before `GET /api/me/profile` resolves,
  because `hasPhoto` defaults to `false` — so it would appear most reliably for the members who have
  *already* complied, which is the fastest possible way to teach everyone to dismiss it on sight.
- 2026-08-31 — ✅ `vue-tsc`, 55 frontend tests (9 new for the visibility rule) and `npm run build`
  clean. Not verified from here: how the banner looks on a phone and that the capture sheet opens
  from it — on task 139's matrix.
