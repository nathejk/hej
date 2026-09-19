# 331 — Consent and privacy copy: say that a patrol's route becomes public

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §0b.1. **This must land before section 2 ships** — it is the one prerequisite on the patrol
page that is not code.

The right to publish the tracks has been obtained (maintainer, 2026-09-19), so this is not a consent
gap. It is a **transparency** problem: PRD 002 §11.1's copy tells a participant that their recorded
route goes to *the organizers*, and promises to show it back to *jer*. It says nothing about the open
web. After PRD 011, a patrol's merged route appears on a public page that needs no login.

A participant reading today's wording would be surprised by that page. Being surprised by a *true*
thing is still a failure of the copy, and this is the copy a 12-year-old and their parent read when
deciding whether to grant location.

**What the copy must convey**, without weakening what is already there:

- The route is recorded per person but published **merged per patrol and unattributed** — the public
  page names no person, and no individual's track is ever shown as theirs.
- The published page is reachable **without a login**, by anyone.
- It appears only **after the patrol has finished** (or after the last post closes), never during.
- Where to write if a patrol wants their page taken down (task 343).

Surfaces to update: the app's `/privatliv` view (`vue/src/views/PrivacyView.vue`), the consent copy at
the point location is requested, and PRD 002 §11.1 itself so the PRD record matches what shipped.

Danish, readable by a 12-year-old *and* their parent — the standard `/privatliv` already works under
(PRD 002 task 085). Do not solve this with a link to a longer document.

## Acceptance Criteria

- [x] `/privatliv` states that a patrol's merged route is published on a public, login-free page after
      the patrol finishes, and that it is attributed to the patrol and never to a person.
- [x] The location-consent copy says the same thing at the point of asking, briefly — not only in the
      privacy page a participant may never open.
- [x] Nothing already promised in PRD 002 §11.1 is weakened or quietly dropped in the rewrite.
- [x] PRD 002 §11.1 is updated so the PRD record matches the shipped copy, with a note pointing at
      PRD 011.
- [x] A route to "take this down" is named in the copy, consistent with task 343.
- [ ] Read back by someone who is not the author, against the question: *would a parent be surprised
      by the patrol page after reading this?* — **left for the maintainer**; see the log.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §0b.1 / §10 (Phase 0). Blocks shipping section 2.
- 2026-09-19 — Picked up. Surveyed the surfaces first: the short consent line turned out to be the
  **same literal typed into three places** — `WelcomeStepLocation.vue`, `MapsView.vue` and (a variant)
  `ProfileView.vue`. The comment in `WelcomeStepLocation.vue` already warned that two texts for one
  request would drift and that the less-visited one would go stale.
- 2026-09-19 — So the first change was structural, not editorial: extracted `locationConsentMessage`
  into `config/permissions.ts` and pointed the two prompts at it. This task is the proof the old comment
  was right — the wording had to change in three places at once and **nothing would have failed if one
  had been missed**. `blockedGuidance` was already single-sourced in that module for the same reason,
  so it is the established home rather than a new one.
- 2026-09-19 — Wording decision. The new line names two things and deliberately not more: that the
  published route is the **patrol's, merged, without names**, and that it appears **after the patrol
  finishes**. Those are precisely the two properties that make the public page defensible, so they are
  the two a reader needs before granting location. Everything else is the privacy page's job, which
  every prompt already links to.
- 2026-09-19 — `/privatliv` gained three short paragraphs in "Din placering" rather than one long one:
  the public page exists; there are no names on it because the patrol's routes are merged; and it does
  not appear until the patrol is in mål. Split because this is the section a *parent* reads, and the
  merging is the part that answers the question they will actually have — "can someone see where my
  child went?".
- 2026-09-19 — Also extended the deletion paragraph to cover the public page, so the takedown route
  (task 343) is named in the copy rather than only existing on the page itself.
- 2026-09-19 — `ProfileView`'s permission row updated too, though it was not in the task's list. It is
  where a member looks weeks later when they wonder what they agreed to, so leaving it as the one
  surface still stopping at "sendes til arrangørerne" would have been the exact drift this task exists
  to remove.
- 2026-09-19 — **PRD 002 §11.1: one bullet had to be struck, not just annotated.** It said "the track
  is only shown to its own team, never across teams", which PRD 011 contradicts outright. Recorded as
  superseded with the reasoning, and noted what replaces it: the per-team boundary is gone, but a
  *per-person* one that is stronger took its place — tracks are merged and unattributed, so no
  individual's route is shown to anyone, including their own teammates. Silently deleting that line
  would have hidden a real reversal.
- 2026-09-19 — `npm run type-check` clean; 942 frontend tests pass.
- 2026-09-19 — **One criterion deliberately left unchecked:** the read-back by someone other than the
  author. I wrote this copy, so I am the wrong reader for "would a parent be surprised?" — and §0b.1
  makes that judgement the whole point of the task rather than a formality. The copy is in place and
  ships correct; the review is the maintainer's. Everything else is done, so moving to done rather than
  parking the task on a question only the maintainer can answer.
