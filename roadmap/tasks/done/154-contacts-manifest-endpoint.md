# 154 — `GET /api/contacts/manifest`

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

The cached directory's data source (PRD 007 §8). Returns the people the caller may
**list** — never a spejder — with everything the pane needs offline.

Per person: id, name, group id + label, own-group flag, phone, still-in-race flag,
portrait version/etag, whether a portrait exists.

Requirements that are easy to miss:

- **`phoneParent` must never appear.** `.rules` invariant; project it out server-side
  rather than trusting the client. Task 159 is the tripwire test.
- **No postal address**, no guardian number: the allow-list is avatar, name, group,
  phone, and function/section for crew (PRD 007 §11.4).
- **Delta support** via `If-None-Match` / version, and it must be able to express
  **removal of a field** — a withdrawn member's phone number has to disappear from a
  device that already synced it, or the purge in task 160 is decorative. Simplest
  approach: re-issue the record and have the client replace rather than merge.
- Spejder callers get `403`.
- OpenAPI annotations are mandatory (`.rules`).

Depends on task 150 (where status comes from), 151 (authorization), 152 (placement),
153 (grouping).

## Implementation

`go/cmd/api/contacts.go` + `contacts_test.go`, route in `routes.go`, and
`person.Queries.ListByAppRoles` in `go/nathejk/table/person/querier.go`.

**One entry per (person, group), not per person.** A crew bandit legitimately appears
twice for a crew viewer — among banditter and among crew — so the payload is a flat list
of entries carrying their population and group path. Favourites key on person id, so a
duplicate entry is not a duplicate favourite. Documented at the type, because "why is this
person listed twice" is otherwise a bug report waiting to happen.

**The query fetches by app role, not by population.** Placement is a property of the
person (section slug), so the roles a viewer may see are derived from the matrix, fetched,
and then each row is placed. `ListByAppRoles` returns nothing for an empty role slice —
failing closed, since the caller derives that slice from a permission check.

**Version is a per-viewer content hash.** No natural version column exists, and hashing the
payload means "did anything *I* can see change?" is answerable without a global counter —
one person's edit does not invalidate everybody's cache. It doubles as the ETag. Entries are
sorted deterministically for exactly this reason: an unstable order would make every poll
look like a change.

**403 for a spejder, and a new `ForbiddenResponse` helper.** The app package had no 403.
Added one, with a comment marking the distinction this feature depends on: 403 is right
here (a spejder knows the pane exists), while the patrol lookup must answer 404 for
refusals so it cannot be used to discover which patrols exist.

**`stillInRace` derived in one place** from `types.MemberStatus`, per task 150's interim.
`finished` is deliberately not a withdrawal — tested, because marking a finisher as a
dropout is the mistake shared-go's own docs warn about.

Also: adding a method to `person.Queries` broke three test doubles (`fakeQueries`,
`yearRecorder`, `stubPeople`); each now implements it, with `stubPeople` recording the role
sets it was asked for so a test can assert spejder rows are never even queried.

## Acceptance Criteria

- [x] `GET /api/contacts/manifest` behind `app.requireAuth`, in a new
      `go/cmd/api/contacts.go`.
- [x] Returns only listable people for the caller's role, via task 151's function.
- [x] Carries id, name, group id + label, own-group flag, phone, still-in-race flag,
      portrait etag, has-portrait (as `portraitVersion`, empty when absent — one field
      rather than two, since a version implies existence).
- [x] `phoneParent`, address, postal code and city are absent from the response type
      itself — not merely unset. Asserted against the marshalled body.
- [x] `If-None-Match` returns `304` when unchanged.
- [x] Field removal propagates: `TestContactsManifest_ClearedFieldDisappears` syncs,
      clears the number, re-syncs, and asserts both that it is gone and that the version
      changed.
- [x] Spejder gets `403`; unauthenticated gets `401`.
- [x] OpenAPI annotations present.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8.
- 2026-08-31 14:15 — Picked up. Plan: query method first, then handler, then tests.
- 2026-08-31 14:30 — Added `ListByAppRoles` to `person.Queries`; ordered by name+personId so
  the content hash is stable.
- 2026-08-31 14:50 — Settled the payload shape: one entry per (person, group). A crew bandit
  must appear twice, which a person-keyed map cannot express.
- 2026-08-31 15:05 — Chose a per-viewer content hash for the version, so an edit only
  invalidates the caches of people who can see it.
- 2026-08-31 15:20 — Interface change broke three test doubles; implemented the method on
  each rather than loosening the interface.
- 2026-08-31 15:35 — One genuine test failure: own-klan was not marked. Cause was my
  fixture inventing a klan id while the mock viewer has `MockBanditPatrolID`, so the test
  was exercising the "different klan" path while claiming otherwise. Fixtures now use the
  mock's exported constants.
- 2026-08-31 15:45 — Added `ForbiddenResponse` to `cmd/api/app/errors.go` (none existed) so
  the spejder refusal is a 403 as specified, with a comment on why the patrol lookup must
  not follow suit.
- 2026-08-31 15:55 — ✅ All criteria met. `go build ./...`, `go vet ./...`, `go test ./...`
  all pass; `gofmt` clean on touched files.
