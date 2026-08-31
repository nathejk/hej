# 175 — Add an "left the race" predicate to `shared-go`'s `MemberStatus`

**Status:** open
**Priority:** low
**Created:** 2026-08-31

## Description

Follow-up from task 150. The contacts directory needs one bit per person: **is this
member still in the race?** Withdrawn members keep their name and portrait but lose their
phone number and gain a status marking (PRD 007 §6, task 160).

That predicate belongs in `shared-go/types/member.go`, next to the existing
`Valid()`, `CanFinish()` and `InOurCare()` — for the same reason `InOurCare()` is a
method rather than a comparison spelled out at each call site: the moment two repos each
decide what "left the race" means, they will eventually disagree, and this one is used to
decide whether to show a phone number.

Proposed shape, to be confirmed against how the organisers actually talk about it:

- `Ended()` — `finished`, `reunited`, `released`. The member's Nathejk is over.
- or `LeftTheRace()` — `reunited`, `released` only, keeping `finished` separate, since
  finishing is not a withdrawal and the existing docs are careful about that distinction.

PRD 007 needs the second meaning: someone who *finished* is not a withdrawal and should
not be marked as one. Worth naming precisely, because `finished` vs. `reunited` is
exactly the distinction `MemberStatusFinished`'s documentation goes out of its way to
protect.

Until this lands, `hej` derives the flag in one place with a comment pointing here
(task 150's interim).

## Acceptance Criteria

- [ ] Predicate added to `shared-go` with documentation in the style of the existing
      helpers, stating what it deliberately excludes.
- [ ] Name chosen to keep `finished` and `reunited` distinct.
- [ ] `hej`'s interim local derivation replaced with a call to it, and the interim
      comment removed.
- [ ] Tests cover every `MemberStatus` value.

## Progress Log

- 2026-08-31 — Task created as a follow-up from task 150's decision.
