# 068 — person projection skeleton: schema, constructor, EnsureColumn

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 006 §8. Create `go/nathejk/table/person/` — deliberately **not** under
`internal/`, because Go forbids importing another module's `internal` tree and
this projection is destined for shared-go once the design settles.

Constructor takes `(cqrs.Publisher, cqrs.Writer, cqrs.Reader)`, never a `*sql.DB`
or a concrete stream. Schema drift via `cqrs.EnsureColumn` / `EnsureIndex` from
`New`, per `go-bff-layout`.

Columns per PRD 006 §6: person id, year, app role, name, normalized phone,
guardian phone (spejder only), address, postal code, city, email, birthday, team
id, team name, member status, `verifiedAt`, acknowledged number, portrait ref.
`UNIQUE (year, phone)` is deferred to task 071 (collision policy).

## Acceptance Criteria

- [x] `go/nathejk/table/person/` with `table.go`, `table.sql`
- [x] `New(p cqrs.Publisher, w cqrs.Writer, r cqrs.Reader)` signature
- [x] `CREATE TABLE IF NOT EXISTS` + `EnsureColumn`/`EnsureIndex` calls
- [x] No import of `nathejk.dk/internal/...` from the package
- [x] Registered in `main.go`'s projections slice (still consuming nothing)
- [x] Build/vet/staticcheck/test green on both workspace and GOWORK=off

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — Picked up. Created `go/nathejk/table/person/` with `table.sql`,
  `table.go`, `consumer.go`, `querier.go` — the shape shared-go entities use, so the
  eventual lift is mechanical.
- 2026-08-25 — Kept the package free of `nathejk.dk/internal/...` imports, which is
  the constraint that makes the lift possible at all (Go forbids importing another
  module's `internal` tree). Recorded that in the package doc so it is not
  accidentally broken by someone reaching for `internal/phone`.
- 2026-08-25 — Schema decision: **`phoneParent` is NULL-able**, not defaulted to `""`.
  Only spejder have a guardian number, so "this population does not have one" has to
  be distinguishable from "should have one and it is missing" — PRD 003's profile page
  and PRD 005's confirmation step render those differently. Pinned by a test.
- 2026-08-25 — Schema decision: `year_phone` is a **plain KEY, not UNIQUE**. Two
  people can share a number, and a UNIQUE constraint would make the *projector* fail
  on real data rather than letting the collision policy decide (task 071). Also
  pinned by a test, including an assertion that it does not become UNIQUE later.
- 2026-08-25 — Schema decision: **soft delete** via a `deleted` flag rather than a
  hard `DELETE`. Under replay this makes ordering harmless — a `deleted` arriving
  before a late `updated` still leaves the person excluded, whereas delete-then-insert
  would resurrect them (task 076 builds on this).
- 2026-08-25 — API decision: `Queries.Lookup` returns a **slice**, not a single
  `Person`. An interface returning one value would bake a collision policy into the
  type signature where nobody can see it; a slice makes task 071 a decision rather
  than an accident.
- 2026-08-25 — Guard added while writing the querier: `Lookup("")` returns nothing
  **without querying**. Otherwise an empty phone matches every row holding the column
  default, so a failed normalization would log someone in as an arbitrary person with
  no number on file. The test passes a nil `Reader` on purpose, so removing the guard
  panics instead of quietly passing.
- 2026-08-25 — Soft-delete filtering lives in the querier, not at call sites: a
  deleted member must lose their login, and leaving that to every caller is how one
  eventually forgets.
- 2026-08-25 — `EnsureColumn`/`EnsureIndex` are wired in with an explanatory comment
  but no calls yet — the table was introduced whole, so there is nothing to migrate
  forward. The comment records the duplication rule (every ensured column also lives
  in `table.sql`) and that narrowing a column needs a real migration.
- 2026-08-25 — Registered in `main.go`'s projections slice, constructed inside the
  broker-connect callback so a database-only run does not create tables nothing will
  fill. A construction failure logs and continues rather than taking the API down.
- 2026-08-25 — `Consumes()` returns nil for now, with a comment that this is
  deliberate: an empty subject list means reads work, return nothing, and nothing
  errors — the exact silence `eventing.go`'s registration comment warns about.
- 2026-08-25 — ✅ All criteria complete. 4 tests; build/test/vet/staticcheck/gofmt
  green on both the workspace and `GOWORK=off` paths (10 packages).
- 2026-08-25 — Verification caveat: **Docker daemon not responding**, so the schema
  has never been executed against MariaDB. The SQL is untested — a syntax error or a
  bad default would only surface on first boot. This is also PRD 008's stated
  acceptance test ("PRD 006's first projection persists and rebuilds"), so **PRD 008
  cannot be closed on this evidence alone**.
- 2026-08-25 — Moving to done.
