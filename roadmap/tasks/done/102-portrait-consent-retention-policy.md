# 102 — Decide + document portrait consent, retention and access policy

**Status:** done
**Priority:** high
**Created:** 2026-08-28
**Picked up by:** maintainer decision, recorded by agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

**Was the blocker for tasks 103–109, PRD 005's onboarding portrait step and PRD
007.** Participants are frequently minors and the portrait is used for
identification during the race, much of which happens at night.

The question was: legal basis, whether parental consent is required and where it
is captured, retention, and who besides the owner may see a portrait.

## Decision (maintainer, 2026-08-28)

> "We have all the parental consents we need from the sign up, this is an in-race
> app that is all approved. The photo is a security feature."

Three things follow, and they are what the implementing tasks build against:

1. **Consent is already held.** It is captured at **sign-up**, by the guardian,
   outside the app — which is the only place it could work, since guardians do not
   use the app. The app therefore does **not** need its own consent gate, and the
   portrait step must not be built as one.
2. **The purpose is safety, not decoration.** The portrait exists so that staff can
   identify a member in the dark, including when something has gone wrong. That is
   the basis on which it is held, and it is why the capture copy should say
   *identification during the race*, in those terms.
3. **Retention: the photo does not outlive the event.** An in-race safety feature
   has no purpose the day after the race, so portraits are purged after the event
   (task 109, where the window is configuration rather than a literal).

## What this decision does NOT settle

Recorded so nobody reads more into it than was said:

- **The exact purge window** — "after the event" is the rule; the number of days is
  a config value in task 109 and still wants a maintainer number. It does not block
  capture.
- **Participant-to-participant viewing.** "It is a security feature" justifies
  *staff* seeing a member's face. It says nothing about whether spejdere and
  banditter may see each other, which is a race-dynamics question as much as a
  privacy one. That stays **PRD 007's access matrix** to decide.
- **The technical safeguards are unaffected.** Consent being in hand is not a
  reason to relax any of them: authenticated-only serving, EXIF/GPS stripping,
  server-side re-encode, hard size limit, non-enumerable storage (tasks 103/105).

## Acceptance Criteria

- [x] Legal basis documented — safety/identification during the event.
- [x] Parental-consent position decided — already held from sign-up; no in-app
      consent gate.
- [x] Retention decided — purged after the event; window is configuration
      (task 109).
- [x] Audience: staff viewing is covered; the participant-to-participant question
      is explicitly left with PRD 007's matrix.
- [x] PRD 003 updated (§6, §10, §11) and the answer cross-referenced from PRD 005
      (§8, §11, §12) and PRD 007 (§11).

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10. Not agent-decidable; awaiting the
  product/legal call.
- 2026-08-28 — Maintainer answered (quoted above). Recorded rather than
  interpreted: the quote is kept verbatim in this file so a later reader can judge
  the reasoning for themselves rather than inheriting my paraphrase of it.
- 2026-08-28 — Wrote the "what this does not settle" section deliberately. The
  answer unblocks capture and storage, but it would be easy to read it as also
  authorising participants to browse each other's faces, which is not what was
  said and is the one part of the audience question with a race-integrity angle.
- 2026-08-28 — ✅ Decision propagated to PRDs 003, 005 and 007; the "Blocked by
  task 102" lines removed from tasks 103–109. Moving to done.
