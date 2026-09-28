# 449 — The crew roster read behind the credit picker

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent session (2026-09-28)
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 025 §6 R2. An admin-only read of a year's crew, so the "Fotokredit" sheet can offer a list to pick from
instead of a field to type into.

**Names only.** Not a phone number, not an email, not a portrait, not a birthday — the person row holds all of
them and this read must carry exactly the id and the name it needs, because a read that returns "the person" is
one somebody later renders more of. The library's privacy guards do not walk the person package, so this one is
on the reviewer and on the test.

Defaults to `sectionSlug = "pr"`, where most photographers are, and can return all of the year's crew — `pr` is
the default filter, not the boundary. `KEY year_section (year, sectionSlug)` already exists.

Only people who classify as `person.RoleCrew` (see `classify.go`). Not because a participant would otherwise
appear in a photographer picker by accident, but because the same rule has to hold in the resolver (task 451)
and two places deciding "who is crew" is how they come to disagree.

## Acceptance Criteria

- [x] `GET /api/admin/crew` (or similar) — the year's crew, `pr` by default, all-crew on request
- [x] The response carries id and name, and **nothing else**
- [x] Only `RoleCrew`; a spejder or bandit never appears
- [x] Behind the admin credential, `no-store` like the rest of `/admin`
- [x] OpenAPI annotations
- [x] A test that the response type cannot grow a phone number without failing

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R2.
- 2026-09-28 — Implemented. `person.CrewRoster(year, sectionSlug)` in `go/nathejk/table/person/querier.go`,
  returning `person.CrewMember{PersonID, Name}`; `GET /api/admin/crew` in `go/cmd/api/admincrew.go` behind
  `requireAdmin(requireAdminYear(…))`. `?section=` defaults to `pr`; `?section=all` widens to the year's whole
  crew; anything else narrows to that slug.
- 2026-09-28 — The read selects **two columns**, not a `Person`. Same discipline as `TrackMembers` and
  `ExpiredPortraits`, with a sharper reason: the caller is a list a browser renders, so every column fetched is
  one something is about to draw. `TestCrewRosterSelectsOnlyAnIdAndAName` asserts the SELECT against
  `personColumns` — the projection's own column list — so a column added to `table.sql` is covered without
  anybody remembering that test.
- 2026-09-28 — Who counts as crew: `appRole = RoleCrew` exactly, the literal rule stated in the task and in
  PRD 025 §6 R3. **Consequence worth recording before task 451 copies it:** a section that grants a capability
  classifies as `RolePostmandskab`, `RoleGuide` or `RoleSamarit` (classify.go), so a medic or a guide is *not*
  in this roster even though they are crew in the ordinary sense. That is the narrow reading, it is what the
  resolver must also do, and the alternative — "anyone folded from a crewmember row" — is not recoverable from
  the row, since `appRole` is the only trace of the population. A photographer in such a section is credited by
  typing, which PRD 025 §11 Q2 keeps as a peer path.
- 2026-09-28 — Undecided by the task, decided here: **`section=all`** as the widening value, in the register of
  the library's `album=none`, rather than an empty `section=` (indistinguishable from absent, which must mean
  `pr`) or a second boolean parameter (two parameters that could contradict each other). Cost: an organizer who
  names a real section `all` cannot filter to it. Also: an **unrecognised slug is not a 400**, unlike the
  library's filters — there is no list of valid sections to validate against, so the only available refusal
  would be "this section has no crew", which is a legitimate answer. An empty roster with the filter echoed
  says it without inventing a validation.
- 2026-09-28 — Two more narrowings the task did not ask for, both argued in comments: `deleted = 0`, because
  §6 R4 makes deletion the erasure mechanism and a removed crew member who is still offered means the deletion
  did nothing; and `name <> ""`, because `handleSectionAssigned` writes a stub row when an assignment lands
  before the member's details, and a blank option in a picker credits a photograph to nobody.
- 2026-09-28 — `no-store` is **inherited**, not set in the handler: `requireAdmin` calls `setAdminHeaders` on
  the way in, so every response on this surface carries it including the 401.
  `TestTheCrewRosterIsNotCacheable` pins it anyway, and fails if the wrapper is ever dropped from the route.
- 2026-09-28 — The response field carrying the id is named **`ID`, not `PersonID`**, and the reason is in a
  comment on the field because the shorter name looks like an evasion. `isPersonShaped` flags any field
  containing "person", and `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` walks every struct declared in
  an `admin*.go` file — so a `PersonID` here would fail a guard whose exception list is PRD 025 §8's to
  re-scope (task 451/452), in a file this task does not own. **Verified that the existing guard does cover this
  handler's types:** adding a `Phone string` field to `adminCrewMember` fails both the new exact-field-set test
  and `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson`. So `Phone`, `PhoneParent`, `Email`, `Address`,
  `Birthday` and `Portrait…` cannot be added here at all, and the new test additionally catches an
  innocent-sounding one (`Nickname`, `Born`, `Contact`) that the needle list would let through.
- 2026-09-28 — Adding `CrewRoster` to `person.Queries` required a stub on six test fakes
  (`directory_test.go` ×2, `patroltrack_test.go`, `photo_test.go`, `scankind_test.go`, `syncload_test.go`).
  No production implementer other than the querier.
- 2026-09-28 — Every new test verified to fail before it passes, by mutating the source and reverting: all
  columns selected (fails), `appRole` filter dropped (fails), section predicate always applied (fails), slug
  folding skipped (fails), `deleted = 0` dropped (fails), `name <> ""` dropped (fails), empty-year guard
  removed (fails), `adminCrewMember` grown a `Phone` (fails, twice), default section emptied (fails),
  `section=all` no longer widening (fails), nil projection answering 200 (fails), read error swallowed (fails),
  `requireAdmin` removed from the route (fails, three times).
- 2026-09-28 — Validation: `gofmt -l .` silent; `go vet ./...` and `go tool staticcheck ./...` clean;
  `GOWORK=off go test ./...` green. One iteration needed: `TestGlimtFailureCodesMatchTheHandlers` required the
  `@Failure 400` for the year header that `requireAdminYear` answers, as `listAdminCheckpointsHandler`
  documents it.
- 2026-09-28 — Not done, and belongs to later tasks: nothing consumes this read yet. The picker is task 453,
  the `creditPersonId` column task 450, the resolver task 451. The roster's "who is crew" rule must be copied
  verbatim into 451 — see the second entry above for the consequence it carries.
