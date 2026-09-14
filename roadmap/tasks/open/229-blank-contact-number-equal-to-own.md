# 229 — Blank the contact number when it equals the member's own

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

Some members registered their own number as their guardian's — observed in the register. Such
a record passes every check we have: well-formed, instantly recognised, verifies perfectly.
And it is worthless in the situation it exists for, because an injured or withdrawing
13-year-old's phone is the phone we are trying not to depend on.

Fast-tracking one of those members through check-in would be **worse than not fast-tracking at
all**, because the counter would then skip the one record that actually needs fixing. So the
collision has to be caught before the number is ever served.

In `go/cmd/api/profile.go`, blank `phone_parent` to `""` when the registered contact number
equals the member's own number after normalisation with `internal/phone`. `""` means
"expected but not registered" — **not** `null`, which means "this population has no contact
number at all" (bandits). The distinction drives `confirmation_required` and the PWA's mode,
so getting it wrong is not cosmetic.

This is a pure comparison of two fields already on the loaded `person.Person`: no query, no
new failure mode, unit-testable without a database.

It must be applied by the BFF in **every** response that could carry the value, never by the
client choosing not to render it (`.rules` — the BFF projects the field out). Note that a
copy already cached on a device is a separate problem, owned by task 233.

`confirmation_required` stays **true** for a blanked number: a spejder with `""` is exactly
the record an organizer wants to hear about (PRD 005 §8, unchanged), and the member is asked
to supply a number as though the register held none.

Independent of the message-contract work (tasks 222–228) and can go in parallel.

## Acceptance Criteria

- [ ] A member whose registered contact number normalises equal to their own number gets
      `phone_parent: ""` from `GET /me/profile`
- [ ] Numbers differing only in formatting (spaces, `+45`, leading `0045`) are treated as
      equal; genuinely different numbers are untouched
- [ ] The rule is a no-op for bandits — `phoneParent` stays `null`, asserted in a test rather
      than assumed
- [ ] `confirmation_required` is still true for a blanked number
- [ ] Every response that could carry the contact number applies the blanking, not just
      `GET /me/profile`
- [ ] The rule has its own unit test, and is covered by the guardian tripwire
      (`go/cmd/api/guardiantripwire_test.go`)
- [ ] The `GET /me/profile` OpenAPI description states the blanking rule, since a client
      author cannot infer it

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
