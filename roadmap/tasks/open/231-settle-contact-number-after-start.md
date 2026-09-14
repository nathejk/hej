# 231 — Settle the contact number once the member has started

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

Once a member has started, the contact number is settled: staff established it at the counter
and a member replacing it afterwards would leave staff holding a number nobody validated.
`Person.HasStarted()` (`MemberStatusRacing`) is the signal.

`POST /me/profile/guardian` must refuse for a started member. `/me/profile/confirm` already
refuses, because `confirmationRequired` is false once started (PRD 005 §8) — convenient rather
than coincidental, and it means `/guardian` is the only endpoint that has to change.

This **deliberately reverses task 148's ungating of `/guardian`**. That decision was right when
it was made and is wrong now: it was written before check-in became the backstop, so keeping
the correction path permanently open was the only way a wrong number ever got fixed. Under
PRD 015 the correction path stays open right up to the start and closes there. Whoever
implements this should say so in the code, so the reversal reads as a decision and not as a
regression of 148.

The profile response must also **tell the client the number is settled**, so the PWA can render
it read-only instead of offering an edit that will be refused. An edit affordance that always
fails is worse than no affordance.

OpenAPI annotations on `/me/profile/guardian` and `GET /me/profile` must be updated (`.rules`).

Reads best alongside task 230, which supplies the settled number itself.

## Acceptance Criteria

- [ ] `POST /me/profile/guardian` refuses for a member with `MemberStatusRacing`, with a
      response the PWA can distinguish from a validation failure
- [ ] A member who has not started can still correct the number exactly as today
- [ ] The profile response carries a flag saying the contact number is settled, true only once
      the member has started
- [ ] `/me/profile/confirm` behaviour for a started member is confirmed unchanged by a test,
      not assumed from PRD 005
- [ ] A comment at the refusal records that this reverses task 148 and why
- [ ] OpenAPI annotations updated on both affected endpoints

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
