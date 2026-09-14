# 231 — Settle the contact number once the member has started

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

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

- [x] `POST /me/profile/guardian` refuses for a member with `MemberStatusRacing`, with a
      response the PWA can distinguish from a validation failure
- [x] A member who has not started can still correct the number exactly as today
- [x] The profile response carries a flag saying the contact number is settled, true only once
      the member has started
- [x] `/me/profile/confirm` behaviour for a started member is confirmed unchanged by a test,
      not assumed from PRD 005
- [x] A comment at the refusal records that this reverses task 148 and why
- [x] OpenAPI annotations updated on both affected endpoints

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — 409 for the refusal, not 403: nothing about the caller is wrong, the resource is
  simply in a state where the act no longer applies — the same reading /confirm's 409 already has,
  and distinguishable from the 400 a bad number gets.
- 2026-09-12 — The message points somewhere: "sig det til din leder". A bare refusal sends a
  member to the nødtelefon or nowhere, and this one is read by somebody who has just been told
  they cannot fix something they believe is wrong.
- 2026-09-12 — Wrote the task 148 reversal into a comment at the guard, with the reason it is now
  right rather than just "PRD 015 says so": task 148 kept the endpoint open because the app was
  the *only* place a number could be fixed, and PRD 015 gives it a backstop at the counter.
- 2026-09-12 — `contact_settled` added to the profile response, derived from `HasStarted()` and
  false when the projection cannot answer — an outage must not lock a member out of correcting
  their record. Both branches tested.
- 2026-09-12 — Asserted /confirm's existing refusal for a started member rather than trusting PRD
  005's derivation. It was already true, and "already true" is exactly how a regression hides.
- 2026-09-12 — Took the opportunity task 222 set up: `storeVerification`'s unused
  `registeredPhone` parameter is gone, which is one reviewable change now instead of noise inside
  the contract commit.
- 2026-09-12 — Annotations updated on both endpoints, including that `phone_parent` is check-in's
  number after the start and that it is blanked when it equals the member's own — neither is
  inferable from a schema. ✅ All criteria.
- 2026-09-12 — `gofmt -l`, `go test ./...`, `GOWORK=off go build ./...` clean.
