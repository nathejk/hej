# 159 — Assert `phoneParent` never appears in contacts responses

**Status:** open
**Priority:** high
**Created:** 2026-08-31

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

## Acceptance Criteria

- [ ] A test asserts the serialised JSON of every contacts response contains no
      `phoneParent` / `phone_parent` key, and no value matching a known guardian number
      from the fixtures.
- [ ] The assertion is written against the marshalled bytes, not the Go struct, so an
      embedded or renamed field cannot slip past.
- [ ] The test fails if someone adds the field back — verified by temporarily adding it.
- [ ] A comment in the test names `.rules` as the source of the requirement, so its
      purpose survives a future refactor.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §11.4 and the `.rules` invariant.
