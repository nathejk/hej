# 093 — Directory: profile fields on `users.User`

**Status:** open
**Priority:** high
**Created:** 2026-08-28

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

- [ ] `users.User` carries `Phone`, `PhoneParent *string`, `Address`,
      `PostalCode`, `City`.
- [ ] `personDirectory.toUser` maps them from `person.Person`.
- [ ] `Directory` interface is unchanged (no new method).
- [ ] Mock seeds the new fields, incl. at least one role with a nil
      `PhoneParent` and one spejder with a guardian number.
- [ ] `go test ./...`, `go vet ./...`, `staticcheck ./...` green.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
