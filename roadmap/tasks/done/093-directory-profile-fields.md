# 093 — Directory: profile fields on `users.User`

**Status:** done
**Priority:** high
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

PRD 003 §6 needs name, address, own phone and guardian phone for the signed-in
user. `internal/users.User` today carries `{ID, Role, Name, PatrolID,
PatrolName, Section}`. Extend it *behind the existing `Directory` interface* —
task 043 established that new consumers get a field on `User`, not a lookup path
of their own, and PRD 006's `person` projection already stores every field
needed (`person.Person`: `Phone`, `PhoneParent *string`, `Address`,
`PostalCode`, `City`).

Keep `PhoneParent` a pointer through to `users.User`: `person`'s doc explains
that nil means "this population has no guardian number" (bandit, crew, gøgler)
while a pointer to `""` means "should have one and it is missing", and the
profile page renders those differently.

The mock (`internal/users/mock.go`) is a test double only — seed plausible
values, do not grow it into a data source.

## Acceptance Criteria

- [x] `users.User` carries `Phone`, `PhoneParent *string`, `Address`,
      `PostalCode`, `City`.
- [x] `personDirectory.toUser` maps them from `person.Person`.
- [x] `Directory` interface is unchanged (no new method).
- [x] Mock seeds the new fields, incl. at least one role with a nil
      `PhoneParent` and one spejder with a guardian number.
- [x] `go test ./...`, `go vet ./...`, `staticcheck ./...` green.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Picked up and completed in one pass (small, single-commit task;
  the board's separate pick-up commit would have carried no information).
- 2026-08-28 — Five fields added to `users.User`, grouped under a comment saying
  what makes them different from `Name`/`Section`: these are shown only to their
  owner, whereas `Name`/`Section` are shown to *other* holders of a shared number
  by the login chooser. Worth stating, because the chooser is one careless field
  addition away from leaking an address.
- 2026-08-28 — `personDirectory.toUser` passes `PhoneParent` straight through as a
  pointer. Deliberately not dereferenced with a default — that would erase the
  nil-vs-empty distinction two lines from the comment explaining why it exists.
- 2026-08-28 — Mock gained addresses/phones, plus names for the spejder and bandit
  entries that previously had none. Seeded all three guardian-number states,
  including pointer-to-empty on `mock-sibling-b`; two new tests pin them so the
  distinction cannot be flattened silently. Crew are seeded without an address on
  purpose (the app has no reason to show one), and the test asserts accordingly
  rather than demanding a blanket non-empty.
- 2026-08-28 — ✅ All criteria met. `go test ./...`, `go vet ./...` and
  `staticcheck ./...` green in the `api` container. Moving to done.
