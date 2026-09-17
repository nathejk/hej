# 328 — Run the OpenAPI annotation guard across the whole API

**Status:** open
**Priority:** low
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

Task 312 built an AST-based guard for OpenAPI annotations
(`go/cmd/api/glimtopenapi_test.go`). It parses `routes.go` for what is actually registered,
cross-checks each handler's doc comment, and — the useful part — asserts that the documented
`@Failure` codes **agree with the branches the code actually has**, following one level into
delegated guards.

It is deliberately scoped to Glimt via a one-line `isInScope` predicate. Widening it to the
whole API is that one line, but it will fail on handlers from earlier PRDs, and turning a guard
red on work nobody in the current task touched is how a guard gets commented out.

So this task is: widen the scope, see what it says, and fix what it finds. `.rules` already
requires annotations on **all** endpoints, so anything it reports is a rule violation rather
than a new standard being imposed.

Expect two kinds of finding, based on what it turned up for Glimt alone:

- **Undocumented reachable failures.** All three Glimt findings were the same shape — a 404 when
  the caller has no directory record. That pattern is likely repo-wide, since most handlers do a
  `app.models.Users.Get(s.UserID)`.
- **Documented failures the code cannot produce.** Harder to spot by reading and worse to leave:
  the spec describes an outcome that cannot happen and nothing looks wrong.

If the count is large, splitting by file is fine — the guard can stay scoped and be widened a
directory at a time, as long as the end state is the whole API.

While there: rename the file (`openapi_test.go`) and drop the `Glimt` prefixes from the test
names, since they would no longer be about Glimt.

## Dependencies

- Task 312 (the guard) — done

## Acceptance Criteria

- [ ] `isInScope` covers every registered route
- [ ] Every finding either fixed or, where the guard is wrong, the guard corrected with the
      reasoning recorded (task 312's log has one worked example of the guard being wrong)
- [ ] File and tests renamed to drop the Glimt scoping
- [ ] `gofmt`, `go build ./...` and `go test ./...` clean

## Progress Log

- 2026-09-17 00:00 — Task created out of task 312, which built the guard and deliberately left it
  scoped. See that task's log for why the agreement test is shaped the way it is, and for the one
  case where the guard itself turned out to be wrong rather than the annotation.
