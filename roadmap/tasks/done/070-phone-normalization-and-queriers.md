# 070 — GetByPhone / GetByID queriers and phone-normalization consistency

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 006 §2/§6. The survey found **no phone lookup exists anywhere** across
shared-go, hq or tilmelding: no `ByPhone` query, no `Filter` accepting a phone, no
index on a phone column. This task adds the one the login path needs.

The correctness risk is normalization: a number stored by the projector and a
number typed at login must normalize **identically**, or lookups silently miss.
`go/internal/phone` already exists and is used by the login handler — the
projector must use the same function, not a second implementation.

## Acceptance Criteria

- [x] `GetByPhone(ctx, year, phone)` and `GetByID(ctx, id)` on the person querier
      — shipped as `Lookup` / `Get` (see log for the naming)
- [x] Index-backed lookup (index declared in the schema)
- [x] Projector and login path share one normalization implementation
- [x] A test proving a number stored via the projector is found by a login-shaped
      lookup, including a messy input form (spaces, +45, leading zeros)
- [x] Not-found returns a clear zero value, not an error

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — Picked up. The querier itself landed with task 068; this task is
  really about the **normalization consistency**, which is where the correctness risk
  is.
- 2026-08-25 — Hit the design tension head-on: the projector must normalize phone
  numbers, but this package **cannot import `internal/phone`** (it is bound for
  shared-go, and Go forbids importing another module's internal tree). The tempting
  shortcut — re-implement the rules here — is exactly the bug PRD 006 §2 warns about.
- 2026-08-25 — Resolved with the pattern `go-bff-layout` prescribes: declared a
  `PhoneNormalizer` port in `person/interfaces.go` and satisfied it in
  `cmd/api/phonenormalizer.go` with a thin adapter over `internal/phone`. There is now
  provably **one** implementation behind both the write and the read.
- 2026-08-25 — Why that matters more than it looks: a normalization mismatch does not
  error. The lookup just finds nothing, and the user is told their number is not
  recognised, with nothing in the logs to explain why. Injecting one implementation
  makes that class of bug unrepresentable rather than merely unlikely.
- 2026-08-25 — Decision: **`Lookup` normalizes its own input** rather than documenting
  "pass a normalized number". A comment is a request; normalizing inside is a
  guarantee. It also means the caller cannot get it wrong by passing raw user input,
  which is precisely what a login handler has.
- 2026-08-25 — `New` now **fails** without a normalizer instead of defaulting to
  storing raw input. The degraded version of that mistake is a directory that looks
  fully populated and never matches a login.
- 2026-08-25 — Naming: kept `Lookup`/`Get` rather than the task's `GetByPhone`/`GetByID`,
  to match the `users.Directory` vocabulary the adapter (task 077) has to satisfy.
  Same behaviour, one less translation.
- 2026-08-25 — Added real query-level tests with `go-sqlmock` (already in the module
  graph; `go mod tidy` promoted it from indirect). The central test asserts on the
  **query argument**: whatever messy form the user types, the SQL must go out with the
  canonical form the projector would have written. Seven input forms, including
  `"30 11 22 33"`, `"+45 30 11 22 33"`, `"0045 30112233"` and `"(30) 11-22-33"`.
- 2026-08-25 — That test caught a bug in **my own fixture**: the test normalizer did
  not handle the `00`-prefix that `internal/phone` accepts, so `0045 30112233` failed.
  Fixed the fixture. Worth noting the test was doing its job — it flagged a real
  divergence between two normalizers, which is the exact failure mode it exists for.
- 2026-08-25 — Also tested: an unparseable input **does not reach the database** (the
  sqlmock has no expected query, so any query fails the test), all owners of a shared
  number come back for task 071's flow, `Get` returns `found=false` rather than an
  error, and a NULL guardian phone scans as `nil` so "not applicable" stays
  distinguishable from "missing".
- 2026-08-25 — ✅ All criteria complete. 5 test functions / 11 cases in the querier
  alone. Green on both the workspace and `GOWORK=off` paths.
- 2026-08-25 — Verification caveat: `go-sqlmock` verifies the SQL *text and
  arguments*, not that MariaDB accepts the query or that the index is used. **Docker
  daemon not responding**, so no query has run against a real database; task 078 is
  where that gets proven.
- 2026-08-25 — Moving to done.
