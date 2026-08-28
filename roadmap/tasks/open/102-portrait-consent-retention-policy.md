# 102 — Decide + document portrait consent, retention and access policy

**Status:** open
**Priority:** high
**Created:** 2026-08-28

## Description

**Blocker for every photo task below (103–108, and PRD 005's onboarding portrait
step, and PRD 007).** Participants are frequently minors and the portrait is
sensitive personal data used for night-time identification during the race.

Needs a human decision, not an agent's: legal basis, whether parental consent is
required and where it is captured, how long photos are kept (PRD 003 proposes
deletion after the event), and who besides the owner may see them (PRD 007's
access matrix answers the audience; the legal basis for that audience is what is
missing).

Output is a written decision, recorded in PRD 003 §6 Non-Functional and §11 with
a date, so the implementing tasks have something unambiguous to build against.

## Acceptance Criteria

- [ ] Legal basis documented.
- [ ] Parental-consent position decided, with the capture point named if needed.
- [ ] Retention period decided.
- [ ] Audience confirmed against PRD 007's access matrix.
- [ ] PRD 003 updated (§6, §11) and the answer cross-referenced from PRD 005 and
      PRD 007.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10. Not agent-decidable; awaiting the
  product/legal call.
