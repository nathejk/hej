# 455 — Rewrite PRD 022 §6's credit bullet

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

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
