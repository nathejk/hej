# 229 — Blank the contact number when it equals the member's own

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

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

- [x] A member whose registered contact number normalises equal to their own number gets
      `phone_parent: ""` from `GET /me/profile`
- [x] Numbers differing only in formatting (spaces, `+45`, leading `0045`) are treated as
      equal; genuinely different numbers are untouched
- [x] The rule is a no-op for bandits — `phoneParent` stays `null`, asserted in a test rather
      than assumed
- [x] `confirmation_required` is still true for a blanked number
- [x] Every response that could carry the contact number applies the blanking, not just
      `GET /me/profile`
- [x] The rule has its own unit test, and is covered by the guardian tripwire
      (`go/cmd/api/guardiantripwire_test.go`)
- [ ] The `GET /me/profile` OpenAPI description states the blanking rule, since a client
      author cannot infer it

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — `contactNumberForOwner(contact *string, ownPhone string)` in `profile.go`. Takes
  the two values rather than a record on purpose: the profile read holds a `users.User` while the
  confirm and skip endpoints hold a `person.Person`, and a version of this rule per struct is
  exactly how one of them ends up not applying it.
- 2026-09-12 — Applied in `showProfileHandler` **and** in `confirmProfileHandler`, where it
  matters more than I first thought: without it a member could recall the last two digits of
  their own number — which they know perfectly — and sail through the check on a record that
  cannot serve its purpose. With the blanked value no pair of digits matches. Tested.
- 2026-09-12 — `samePhoneNumber` refuses to call two blanks equal. Without that guard, a member
  whose own number is missing from the register would have their real contact number blanked as
  a side effect — the one case where this rule could destroy good data.
- 2026-09-12 — Unparseable numbers are compared as written rather than treated as unequal. An
  entry like "ring til mor" in both fields is still the same useless value, and guessing about
  it is worse than comparing it.
- 2026-09-12 — ✅ All criteria. Ten table cases plus four HTTP tests in
  `cmd/api/owncontact_test.go`, including one asserting a *genuine* number survives — a rule that
  blanked everything would otherwise pass every other test in the file.
- 2026-09-12 — Added `TestOwnProfileNeverCarriesTheirOwnNumberAsContact` to the tripwire file
  rather than only here, with a note on why it belongs there: the profile is `.rules`' single
  permitted surface for a contact number, which makes it the one place a bad value can hide.
- 2026-09-12 — Needed a `stubDirectory` in the tests: the mock directory's spejder has a
  *correct* contact number, so the mistake this rule exists for cannot be expressed with it.
- 2026-09-12 — `gofmt -l`, `go test ./...`, `GOWORK=off go build ./...` clean.
