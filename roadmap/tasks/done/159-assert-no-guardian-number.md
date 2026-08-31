# 159 — Assert `phoneParent` never appears in contacts responses

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

`.rules` now carries a repo-wide invariant:

> **Guardian phone numbers never enter the PWA**, with exactly one exception: a user
> confirming or approving **their own** guardian's number (PRD 003, PRD 005). BFF
> responses must project the field out rather than relying on the client not to render
> it.

This task is the tripwire. It is test-only, but it is what makes the rule durable:
`PhoneParent` is present on `users.User`, so "we just won't select it" is one careless
change away from being false — someone adds a field to a response struct, or swaps a
projection for a broader one, and a guardian's number ships to a few hundred devices.

Cover every contacts surface: manifest (154), photo (156), patrol lookup (157), profile
(167). The patrol lookup matters most — those are the records that *have* guardian
numbers.

## Implementation

`go/cmd/api/guardiantripwire_test.go`. **Two layers, because they fail differently.**

**1. Body scans** over the marshalled response of every JSON surface (`manifest`,
`version`, `patrols/{number}`), asserting neither the distinctive fixture number nor any
forbidden key appears. Written against the bytes, not the struct: an embedded struct, a
renamed field, or a removed `json:"-"` would each pass a struct-level check and still put
the number on the wire.

**2. Type reflection** over every contacts response type, walking JSON tags recursively
(through nested and embedded structs, slices and pointers). This is the layer that catches
the *likely* accident — a field added but not populated in the fixture, which a body scan
sails straight past.

The forbidden list covers `phoneParent`/`phone_parent`/`guardian`/`parentPhone`, plus
`address`/`postalCode` — not a `.rules` invariant, but excluded from the contacts allow-list
by PRD 007 §11.4, and it belongs in the same tripwire rather than in a second one nobody
remembers to run.

**`TestFixturesActuallyCarryAGuardianNumber`** keeps the body scans honest. Without it, a
change that stopped populating the fixture would leave every assertion here passing while
testing nothing — the classic way a security test rots into decoration.

Also extended the claim to the two **image** surfaces. They return bytes, so there is no
field to leak, but a 404 body is still a body, and asserting it closes the "every contacts
surface" claim instead of leaving two of them implicitly trusted.

Refactored `contactsTestApp` into `newTestAppWithPeople`, since three test files now need to
seed a person stub with more than a listable directory.

## Verified by actually breaking it

Per the acceptance criterion, I added `PhoneParent string \`json:"phoneParent,omitempty"\`` to
`patrolLookupMember` and ran the suite. It failed with:

```
patrolLookupResponse.Members.PhoneParent exposes "phoneParent", which no contacts surface
may carry (see .rules and PRD 007 §11.4)
patrolLookupMember.PhoneParent exposes "phoneParent", ...
```

Note it caught the field through the *nested* type as well as directly, and named the
offending field — which is the difference between a tripwire someone can act on and one they
have to debug. Then reverted.

## Follow-up left behind

When task 167 adds the person profile route, it must be added to the `paths` list in this
file. A surface missing from that list is a surface with no tripwire, so I have added an
acceptance criterion to 167 rather than trusting it to be remembered.

## Acceptance Criteria

- [x] A test asserts the serialised JSON of every contacts response contains no
      `phoneParent` / `phone_parent` key, and no value matching a known guardian number
      from the fixtures.
- [x] The assertion is written against the marshalled bytes, not the Go struct, so an
      embedded or renamed field cannot slip past — plus a reflection layer for fields that
      exist but are unpopulated.
- [x] The test fails if someone adds the field back — verified by temporarily adding it
      (output above).
- [x] A comment in the test names `.rules` as the source of the requirement, so its
      purpose survives a future refactor.

## Progress Log

- 2026-08-31 20:10 — Picked up. Plan: body scans plus a reflection pass, since they catch
  different mistakes.
- 2026-08-31 20:25 — Added the non-vacuity test after realising a body scan alone would pass
  silently if a fixture stopped carrying a guardian number.
- 2026-08-31 20:35 — Verified the tripwire by adding `PhoneParent` to `patrolLookupMember`:
  it failed, naming the field through both the direct and nested type. Reverted.
- 2026-08-31 20:40 — Added the profile-route criterion to task 167 so the new surface cannot
  land without a tripwire.
- 2026-08-31 20:45 — ✅ All criteria met. `go vet`, `go test ./...` pass; `gofmt` clean.
