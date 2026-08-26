# 085 — Update the location consent copy to cover recording and upload

**Status:** open
**Priority:** high
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.1 changed what the app does with a member's location. It is no longer read
locally to draw a marker — it is **recorded, uploaded to the organizers, and retained
indefinitely**. The consent copy has to say so.

The existing pre-prompt (`MapsView.vue`, and PRD 005's onboarding) currently says:

> Vi bruger din placering til at vise dig på kortet under løbet. Du kan altid slå det fra
> igen.

That was accurate when it was written. It is now a narrower description than the actual
use, and asking for a permission under a narrower description than the use is the specific
thing to avoid — particularly here, where most of the people granting it are minors and
the number being recorded is their physical location through the night.

## Scope

Copy and where it appears, not mechanism. Small task, high consequence.

## Acceptance Criteria

- [ ] The location pre-prompt states that the route is recorded and sent to the
      organizers, and roughly why (safety/analysis, and the team can see its own track
      afterwards — which is the part that makes it feel like a feature rather than
      surveillance)
- [ ] Copy is Danish and consistent with the rest of the app's voice
- [ ] Updated in **both** places it is asked for: the map view's pre-prompt and PRD 005's
      onboarding step — they must not drift into two different promises
- [ ] The profile or settings surface makes it discoverable *after* onboarding that
      recording is happening, so consent granted once at 22:00 is not the only time it is
      ever visible
- [ ] Wording reviewed by the maintainer before it ships — this is a promise to
      participants and their parents, not an implementation detail, and it is cheap to get
      right now and expensive to correct later

## Open question for the maintainer

Does the indefinite retention need saying explicitly in the prompt, or is "sent to the
organizers" the right level of detail for a phone screen with the fuller version living
elsewhere? PRD 002 §11.1 records retention as indefinite *for now*, pending analysis, so
promising a specific period would be wrong; saying nothing about duration may also be
wrong.

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
