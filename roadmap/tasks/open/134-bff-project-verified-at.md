# 134 — BFF: project verified_at and derive confirmation_required

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A consumer projects the `member.verified` event onto `person` in
      `go/nathejk/table/person/`, following the consumer/table conventions already there
- [ ] The column is named `verified_at`; the acknowledged contact number is stored with it
- [ ] No field is added to a shared-go table, and no `MemberStatus` value is added
- [ ] `GET /api/me/profile` returns `verified_at` and `confirmation_required`
- [ ] `confirmation_required` is computed on the server from "has verified" OR
      "has started the event" (`types.MemberStatusRacing` onwards, task 080's projection)
- [ ] `phone_parent` is still returned in full to its owner — PRD 003's contract is
      unchanged by this task
- [ ] OpenAPI annotations on `showProfileHandler` updated to document both new fields,
      including that `verified_at` is null when never verified
- [ ] Tests cover the derivation: never verified + not started → required; verified →
      not required; not verified but `racing` → not required

## Depends on

- **Task 132** — the message.
- **Task 133** — the publisher; without it the projection has nothing to consume, though
  it can be tested against synthesised events.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
