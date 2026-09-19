# 331 — Consent and privacy copy: say that a patrol's route becomes public

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `/privatliv` states that a patrol's merged route is published on a public, login-free page after
      the patrol finishes, and that it is attributed to the patrol and never to a person.
- [ ] The location-consent copy says the same thing at the point of asking, briefly — not only in the
      privacy page a participant may never open.
- [ ] Nothing already promised in PRD 002 §11.1 is weakened or quietly dropped in the rewrite.
- [ ] PRD 002 §11.1 is updated so the PRD record matches the shipped copy, with a note pointing at
      PRD 011.
- [ ] A route to "take this down" is named in the copy, consistent with task 343.
- [ ] Read back by someone who is not the author, against the question: *would a parent be surprised
      by the patrol page after reading this?*

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §0b.1 / §10 (Phase 0). Blocks shipping section 2.
