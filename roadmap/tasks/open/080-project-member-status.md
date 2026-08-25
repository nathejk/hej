# 080 — Project member lifecycle status onto the person directory

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

## Description

Deferred out of task 072, which left the `person.memberStatus` column in place but
empty. It does not arrive on the `spejder.*.updated` events that task projects — it
comes from the lifecycle/`spejderstatus` subject family, which is a separate set of
events (`types.MemberStatus`: registered → seated → racing → finished, plus
waiting/transit/sheltered/reunited/released).

Two features read it, so it is not optional:

- **PRD 005's skip rule.** Profile confirmation is skipped for a member who has
  already started the event, which means `racing` or later. Without this column the
  rule is unimplementable and every returning member is asked to re-confirm.
- **The portrait nudge** (PRD 005, clarified 2026-08-25). A member who started the
  event *without* a portrait must still be nudged. That means the nudge is driven by
  "has no portrait", **not** by verification status — but the skip rule beside it
  still needs the status, and the two must not be wired to the same signal by
  accident.

## Where to get it

Do not invent a second notion of member status. `hq` already projects this from the
same events into its `spejderstatus` table, and `shared-go/types/member.go` owns the
vocabulary plus the `CanFinish` / `InOurCare` helpers. Consume the same subjects and
store the same string values, so a status here means exactly what it means in `hq`.

Note `types.MemberStatus` also documents superseded persisted values
(`REGISTERED`/`STARTED`/`active`/`emergency`/`hq`/`out`) that may still appear in
older events; `hq`'s `ParseMemberStatus` maps them. Reuse that mapping rather than
matching raw strings, or older members will read as having no status at all.

## Acceptance Criteria

- [ ] Lifecycle subjects added to the person projection's `Consumes()`
- [ ] `memberStatus` populated with `types.MemberStatus` values
- [ ] Legacy/superseded status strings mapped, not dropped
- [ ] An unknown status value is logged and stored as empty rather than failing the
      statement (a bad status must not cost a member their row — see task 072's
      birthday lesson)
- [ ] A derived "has started the event" helper, so PRD 005's skip rule does not
      re-implement the comparison at the call site
- [ ] Idempotent under replay; tested with `cqrstest` fakes
- [ ] Verified against the shared stream: how many members land in each status, and
      zero dead letters

## Progress Log

- 2026-08-25 — Task created, deferred out of task 072 and made urgent by the portrait-nudge clarification in PRD 005.
