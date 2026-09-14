# 224 — Project own-phone and contact verification into separate columns

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

The reshaped message (task 222) carries two independent facts, and the projection in
`go/nathejk/table/person/` currently has one column for them. Merging them would invert the
whole point of PRD 015.

Today `verifiedAt` means "the contact number was confirmed", and `Person.IsVerified` gates
`confirmation_required` on it (`querier.go`). A skip or exhaustion event carries a
`VerifiedAt` too — it is a real verification, of the member's *own* number, proven by the
PIN. So writing that timestamp into the existing column would mark **every member who gave
up** as verified, silence the question on their next login, and fast-track at check-in
exactly the members whose records still need a contact number. That is the opposite of what
this PRD exists to do.

So: add a separate own-phone column (`phoneVerifiedAt` or similar) and leave the contact
column meaning only what it means today. `IsVerified` and `confirmationRequired` keep
reading the contact column and nothing else.

Second change in the same handler: `handleMemberVerified` currently **returns an error**
when there is no acknowledged phone number. Under the new shape that would dead-letter every
login event and every skip event, because neither carries a `PhoneContact`. The rule it was
protecting — never record a contact verification that names no number — survives as: an
event without `PhoneContact` writes **only** the own-phone column.

Comment the sharp edge at the consume end (PRD 015 §6): an absent `PhoneContact` from a
login says nothing, while the guardian-check endpoints are the only publishers that may
assert the field. A login must never be able to un-verify a contact number.

Depends on tasks 222 and 223.

## Acceptance Criteria

- [ ] Own-phone verification is stored in its own column; the existing contact-verification
      column keeps its current meaning
- [ ] `Person.IsVerified` and `confirmation_required` are unchanged for a member who has
      only an own-phone verification — a skip does not silence the check
- [ ] `handleMemberVerified` no longer errors on an event with no `PhoneContact`; it writes
      only the own-phone column and returns success
- [ ] An event carrying both numbers writes both columns
- [ ] A login-shaped event (no `PhoneContact`) cannot clear an existing contact verification
- [ ] Consumer tests cover all three event shapes: own-phone only, both numbers, and a
      guardian-check event with an empty `PhoneContact`

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
