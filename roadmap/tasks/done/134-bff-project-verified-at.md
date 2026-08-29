# 134 — BFF: project verified_at and derive confirmation_required

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

## Description

PRD 005 §8. Consume the `member.verified` event (task 132/133) and project it onto
**PRD 006's `person` read model in this repo** (`go/nathejk/table/person/`), then expose
the result on `GET /api/me/profile`.

## Where the column lives, and why that matters

Deliberately **not** on a shared-go table. The shared-go member type gains the field
only if and when PRD 006's projection is lifted there. PRD 005 §8 flags the earlier
ambiguity — this PRD and PRD 006 named two different homes for the same column — as a
**data-migration risk**, which is why the home is now stated rather than implied. Pick
the other one and the field has to be moved later with data in it.

The field name is **`verified_at`**, one name across PRDs 003, 005 and 006. Do not
introduce `confirmed_at`, `verified` or a boolean alias.

Also store **the acknowledged number** alongside the timestamp, so a later change to the
guardian number can invalidate the verification (PRD 005 §8, §11 and §12 Q4). Without it
the projection can say *that* someone verified but not *what* they verified.

## Why a field and not a MemberStatus

PRD 005 §11 (2026-08-25) considered `MemberStatusVerified = "verified"` and rejected it.
Worth restating here, because the pull towards a status is real — `hq` already renders
status columns and filters, so a new status would appear at the counter "for free":

1. **The type forbids it.** `types.MemberStatus` documents itself as answering one
   question — "what is true of this member right now?" — with *exclusive* states, and
   says outright that anything that is several facts at once belongs in its own field.
   A member is `seated` **and** verified; those are two facts.
2. **It would break seat accounting.** `MemberStatusSeated` is documented as the
   paid-seat count — the count of seated members is what the team has actually paid for.
   If verifying moved a member `seated → verified`, every member who verified would
   silently drop out of billing-adjacent accounting.
3. **A status is overwritten, and this fact must survive.** Statuses advance: the moment
   check-in flips a member to `racing`, a `verified` status is gone. That destroys
   exactly the measurement that justifies the feature ("how many members verified before
   arriving?"), makes it impossible to invalidate verification when a number changes, and
   leaves no audit of who acknowledged what. A timestamp survives every later transition.

## The response, and what the client must not compute

`GET /api/me/profile` (owned by PRD 003, already shipped) gains `confirmation_required`
and `verified_at`.

`confirmation_required` is **derived server-side** from "has verified" **OR** "has
started the event". "Has started" means `types.MemberStatusRacing` onwards, and member
status is already projected onto `person` by task 080 — read that rather than inventing a
second notion of started. The client must not reimplement the rule: it is a two-input
predicate over data the client only partially has, and duplicating it guarantees the two
copies drift, at which point a participant is either re-prompted mid-event or never
prompted at all.

The endpoint keeps returning `phone_parent` in full to its owner, exactly as PRD 003
shipped it — masking is a UI concern (PRD 005 §11, 2026-08-30). `200` / `401` / `404`
are unchanged.

## Acceptance Criteria

- [x] A consumer projects the `member.verified` event onto `person` in
      `go/nathejk/table/person/`, following the consumer/table conventions already there
- [x] The column is named `verified_at`; the acknowledged contact number is stored with it
- [x] No field is added to a shared-go table, and no `MemberStatus` value is added
- [x] `GET /api/me/profile` returns `verified_at` and `confirmation_required`
- [x] `confirmation_required` is computed on the server from "has verified" OR
      "has started the event" (`types.MemberStatusRacing` onwards, task 080's projection)
- [x] `phone_parent` is still returned in full to its owner — PRD 003's contract is
      unchanged by this task
- [x] OpenAPI annotations on `showProfileHandler` updated to document both new fields,
      including that `verified_at` is null when never verified
- [x] Tests cover the derivation: never verified + not started → required; verified →
      not required; not verified but `racing` → not required

## Depends on

- **Task 132** — the message.
- **Task 133** — the publisher; without it the projection has nothing to consume, though
  it can be tested against synthesised events.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — **Most of the storage side was already in place** and is worth recording so nobody
  goes looking for work that is done: the `verifiedAt` and `acknowledgedPhone` columns exist in
  `person/table.sql`, the querier scans them, `Person.IsVerified()` compares the acknowledged number
  against the current one, `Person.HasStarted()` reads the projected member status (task 080), and
  `invalidateVerification` clears a verification when the guardian number changes (task 076). The
  projection handler landed with task 132, since it decodes that task's message.
- 2026-08-30 — **What this task added: the read side.** `GET /api/me/profile` now carries
  `confirmation_required` and `verified_at`, with the OpenAPI description extended to say how each
  is derived — including that `verified_at` goes null once the guardian number changes even though
  an earlier acknowledgement is still on the row.

  Two decisions:

  - **`verified_at` reads through `IsVerified()`, not the raw column.** Reporting the old timestamp
    after a number change would tell the member that the number now on file was confirmed, which is
    the one thing this field must never imply.
  - **Both fields are always present in the JSON**, and there is a test for it. A client that treats
    a missing `confirmation_required` as `true` would ask everybody; one that defaults it to `false`
    would ask nobody. Neither is acceptable, so presence is part of the contract rather than a
    convention.

  `phone_parent` is untouched — still returned in full to its owner, per PRD 005 §11's 2026-08-30
  decision that masking is a UI concern.
- 2026-08-30 — **Ten tests added** across `verification_test.go` and `profile_test.go`. The
  derivation table covers the three cases the brief asked for plus three the brief did not, all of
  which are real states in the data: no guardian number at all (do not ask — and this is not
  "verified"), a guardian number expected but empty (**do** ask, because that record is one an
  organizer wants to hear about), and a verification superseded by a number change (ask again). The
  `""` vs `nil` distinction is the whole reason `PhoneParent` is a pointer, so it is pinned in both
  directions.
- 2026-08-30 — ✅ `go build ./...`, `go test ./cmd/api/` and `go test ./nathejk/table/person/` pass.
  No shared-go table field, no new `MemberStatus` value — see PRD 005 §11 for why the status route
  was rejected (it would silently drop verified members out of the paid-seat count, and a status is
  overwritten by the next transition).
