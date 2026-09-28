# 451 — The credit resolver: crew only, name only, within the year

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 025 §6 R3, and **the load-bearing task of that PRD.** Everything else is plumbing; this is the part that
decides whether "the public site can credit a photographer" stays distinct from "the public site can render any
person's name".

One function, in one place: *given a photograph's year and a credit id, return a crew member's name or `""`.*

- **`person.RoleCrew` only** (`classify.go`). A spejder's or bandit's id — mistyped, stale or malicious —
  resolves to nothing.
- **The name field only.** Nothing else on that row is read, ever.
- **The photograph's own year**, with no fallback to another. Somebody who was crew in 2026 and is not in 2027
  still has their 2026 photographs credited, because those resolve against 2026.
- **`""` when it finds nothing**, and PRD 025 §6 R5: that renders as no credit line at all — no placeholder, no
  empty label, no gap.

## The guards this replaces

`TestACreditIsOnlyEverTypedNeverDerived` asserts the opposite of this feature and must be **rewritten, not
deleted**. PRD 022 §6's claim was "the credit is never derived from person records"; the new claim is narrower
and still worth enforcing:

- the resolver reads `name` and no other column;
- it yields "" for anybody who is not `RoleCrew`;
- it is scoped to one year;
- it is the **only** path from the person projection to a public page, and it has no second caller.

Also: except `creditpersonid` in `libraryPersonShapedExceptions` (task 448's pattern) so the id may exist in the
library while a **public** response carrying it still fails.

## Acceptance Criteria

- [ ] One resolver, one place, with the four bounds above
- [ ] A test proving a participant's id resolves to ""
- [ ] A test proving no column but `name` is read
- [ ] `TestACreditIsOnlyEverTypedNeverDerived` rewritten to the new claim, not removed
- [ ] `creditpersonid` excepted for the library scope only, with its argument written where the exception lives
- [ ] Mutation-checked: dropping the `RoleCrew` check must fail a test

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R3, §8.

## What changed

**`person.CreditNames(year, ids) (map[string]string, error)`** — one function, four bounds, batched because the
album page resolves a page of photographs at once and a query per photograph would be the obvious way to make
this correct and unusable.

1. **`CrewRoles` only** — `appRole IN (?, ?, ?, ?)`, bound, never spliced.
2. **`personId, name` and nothing else** in the SELECT. The row holds a phone number, a guardian's number, an
   email, an address and a birthday.
3. **One year**, the photograph's own. An empty year short-circuits without a query, because `year = ""` would
   match the column default rather than "every year".
4. **Absent is absent** — `deleted = 0` and `name <> ""`, and an unresolvable id is simply missing from the map,
   never present as `""`. That is the erasure path: delete the person and the credit is gone from every
   photograph, with nothing to find and nothing to rewrite.

## The union, named once

`person.CrewRoles` and `person.IsCrew` in `classify.go`. The set already existed twice — as prose in
`internal/users/contacts.go` (*"Crew is one role, not three"*) and as `crewRoles` in `cmd/api/scansource.go` —
and two copies of a set that decides **whose name may be published** is one too many. `scansource.go` now uses
the shared one; its own reason for wanting that set is unrelated and is preserved at the use site.

**Widened from `RoleCrew` alone**, on the maintainer's instruction: a photographer registered under `guider` or
`samarit` classifies to that role and would have been invisible to the picker, which is exactly the
curator-types-it-by-hand case the feature exists to remove. The safety property is unchanged under either
reading — participants are excluded by both.

**Gøglere are excluded, deliberately**, and that is the part worth remembering: staff-adjacent and
participant-side here. `MayLookUpPatrol` refuses them because "the lookup exists for a safety task, not a game
one", and the credit picker inherits that reasoning rather than re-deciding it.

## The guard that asserted the opposite of the feature

`TestACreditIsOnlyEverTypedNeverDerived` is now
**`TestACreditNamesAPhotographerAndNobodyElse`** — rewritten, not deleted. A guard that asserts the opposite of
the feature is worse than no guard; deleting it would have thrown away a property still worth holding. The new
claim: the typed path still takes its value from the request body and nothing else, the fold still never joins
`person`, and the resolver is the **only** path from the person projection to a credit — asserted by counting
its definitions, so a second one fails.

**Four code comments still named the old test.** Updated. A dangling test name in a comment about a privacy rule
is exactly the rot that makes these guards untrustworthy: the next reader looks for the guarantee, cannot find
it, and concludes there isn't one.

## Acceptance Criteria

- [x] One resolver, one place, with the four bounds above
- [x] A test proving a participant's id resolves to "" (via `IsCrew` and the `appRole IN` guard)
- [x] A test proving no column but `name` is read — asserted against the projection's whole column list, so a
      column added tomorrow is covered without anybody remembering the file
- [x] `TestACreditIsOnlyEverTypedNeverDerived` rewritten to the new claim, not removed
- [x] `creditcrewid` excepted for the library scope only (landed with task 450, which needed it to be green)
- [x] Mutation-checked: dropping the `appRole` filter fails, and so does selecting one more column

## Progress Log

- 2026-09-28 — Picked up. Started by naming the union, because the picker and the resolver disagreeing about
  "who is crew" would mean offering a name the reader then refuses to publish — the worst of both behaviours.
- 2026-09-28 — Widened the roster read from `RoleCrew` to the union, per the maintainer. Its comment records
  that it was the narrow reading for one commit and why that was wrong, so the next reader does not "fix" it back.
- 2026-09-28 — ✅ Seven tests in `creditnames_test.go`, each saying what would happen without the bound it
  guards. Reused `crewMock` rather than a second harness: the two reads have the same one risk.
- 2026-09-28 — ✅ Rewrote the old guard and the four comments that named it.
- 2026-09-28 — Mutation-checked both halves of the load-bearing bound.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
