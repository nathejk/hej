# 068 — person projection skeleton: schema, constructor, EnsureColumn

**Status:** open
**Priority:** high
**Created:** 2026-08-25
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `go/nathejk/table/person/` with `table.go`, `table.sql`
- [ ] `New(p cqrs.Publisher, w cqrs.Writer, r cqrs.Reader)` signature
- [ ] `CREATE TABLE IF NOT EXISTS` + `EnsureColumn`/`EnsureIndex` calls
- [ ] No import of `nathejk.dk/internal/...` from the package
- [ ] Registered in `main.go`'s projections slice (still consuming nothing)
- [ ] Build/vet/staticcheck/test green on both workspace and GOWORK=off

## Progress Log

- 2026-08-25 — Task created from PRD 006.
