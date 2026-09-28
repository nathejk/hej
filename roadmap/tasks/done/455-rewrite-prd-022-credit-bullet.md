# 455 — Rewrite PRD 022 §6's credit bullet

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 025 reverses a decision PRD 022 §6 states at length, and **the reversal has to be recorded where the
decision was made**, not only in the newer document. Anybody reading §6 today is told the opposite of what the
code will do.

What §6 currently says, and what changes:

- *"free text a curator typed. Never derived, never looked up, never joined to the `person` projection"* — no
  longer true. It is now either that, or a reference to a crew member resolved at read time.
- *"emphatically not a `creditPersonId`"* — reversed by name.
- *"There is no photographer roster and no picker — that would mean holding a list of volunteers' names in this
  service"* — overstated, and the maintainer said so: the roster already exists, and the tool only reads it.

The reasons for the reversal, all three from the maintainer on 2026-09-28 and each one removing a premise the
rule stood on:

1. **Erasure.** A name copied into `photo.credit` is also on the append-only event log, so "delete me" could
   never be fully honoured. A reference can be: one place to delete, and the name is gone from every photograph
   and every public page.
2. **Consent.** Crew are adults who consented to be registered. The rule's phrase "a consenting adult volunteer
   in a professional capacity" already covers them.
3. **Retention.** Crew names are not purged after the event; the same id is reissued the following year. The
   purge objection applied to participants.

What must **stay** in §6, because it is what the exception still rests on: the credit names a consenting adult
photographer and nobody else, and PRD 025 §6 R3 is the new bound — crew only, name only, within the year, and
"" when absent.

## Acceptance Criteria

- [ ] §6's credit bullet rewritten, with the reversal and its three reasons
- [ ] The narrower claim stated, so the next reader knows what is still guaranteed
- [ ] `publicprivacy_test.go`'s header, which cites §6, updated to match
- [ ] No claim left in either PRD that the code contradicts

## Progress Log

- 2026-09-28 — Task created from PRD 025 §10.

## What changed

PRD 022 §6's credit bullet rewritten. The structure is deliberate: the **old wording is quoted** before it is
overturned, because the original reasoning was sound and is still instructive — somebody reading the new rule
should see what it replaced and why that was not enough, rather than finding a clean sentence with no history.

Three sections now: the bounds that still hold; "the reference was forbidden here until 2026-09-28, and why that
changed" with the three corrected premises; and what is guaranteed instead, naming the four bounds and where they
live.

The **residue is stated rather than hidden**: the public site now renders something derived from the person
projection, which it did not before. What keeps that from becoming "the public site can render any person's name"
is one narrow function with its own tests, and the discipline of not giving it a second caller.

`publicprivacy_test.go`'s header had the same problem in the same way — its second bullet was "**a credit is
typed, never derived**", described in its own prose as "the one doing the work". Rewritten to the two-route claim,
with the erasure argument, and with the part that survives restated: this service does not take names out of its
person records and put them on public pages *except* through one function whose bounds make "any person"
impossible.

## Acceptance Criteria

- [x] §6's credit bullet rewritten, with the reversal and its three reasons
- [x] The narrower claim stated, so the next reader knows what is still guaranteed
- [x] `publicprivacy_test.go`'s header updated to match
- [x] No claim left in either PRD that the code contradicts

## Progress Log

- 2026-09-28 — Done with task 452, because a PRD that tells the reader the opposite of the code is worse the
  longer it sits — and §6 is the document the privacy guards cite by number.
- 2026-09-28 — Quoted the old wording rather than replacing it silently. A reversal without its history reads as
  though nobody had thought about it the first time.
