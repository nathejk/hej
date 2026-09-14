# 236 — Licence plate normalisation

**Status:** done
**Priority:** high
**Created:** 2026-09-14
**Picked up by:** agent session (Zed)
**Started:** 2026-09-14
**Completed:** 2026-09-14

## Description

shared-go stores a plate with a country prefix — `"DK+AB12345"` — and users will type
`ab 12 345`, `AB 12.345`, `ab12345`. Normalise once, in `go/internal/plate/`, next to
`internal/phone` and for the same reason: PRD 006 §2's lesson is that a value with
several spellings and no single normaliser cannot be compared, and comparison is exactly
what task 238's duplicate detection needs. Two spellings of one plate are two rows for
the coordinator to reconcile.

Rules:

- Upper-case, strip spaces, dots and hyphens.
- Default the country prefix to `DK` when the input has none, since that is who the
  overwhelming majority are — but accept an explicit prefix and keep it.
- **Do not enforce a Danish plate format.** Nathejk draws Danish participants but not
  only Danish ones (PRD 010 §5), so validation is "plausible plate" — a length band and
  an alphanumeric character set — not a national regex. A rejected plate on a car that is
  physically in the field is worse than an oddly-formatted one in the inventory.
- Reject only what cannot be a plate: empty, too short, too long, or containing
  characters no plate carries.

Return the normalised value and an error, like `phone.Normalize` does, so the caller
decides whether to 400.

## Acceptance Criteria

- [x] `internal/plate` with a `Normalize`-style function returning the prefixed form
- [x] `ab 12 345`, `AB12345`, `ab-12-345` and `dk+ab12345` all normalise to one value
- [x] An explicit non-Danish prefix is preserved (e.g. `SE+ABC123`)
- [x] No Danish-format enforcement: a plausible foreign plate is accepted
- [x] Empty and implausible input is rejected with a usable error
- [x] Table-driven tests covering every rule above, including the idempotence of
      normalising an already-normalised plate
- [x] `go test ./...` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
- 2026-09-14 — Picked up. Plan: mirror `internal/phone`'s shape — one `Normalize(string)
  (string, error)` with `ErrInvalid` — so the two normalisers in this codebase read the same
  way at their call sites.
- 2026-09-14 — `internal/plate/normalize.go`: `Normalize(string) (string, error)` with
  `ErrInvalid`, a `DefaultCountry` of `DK`, separators stripped (space, dot, hyphen,
  underscore), and a length band of 2–10 alphanumerics instead of a national format.
- 2026-09-14 — The one real design decision: **a country prefix is recognised only when
  written with the plus**, never inferred from a bare leading letter pair. Two letters
  followed by digits *is* the Danish plate shape, so reading "DK 12345" as country DK plus
  "12345" would file a genuine plate under a different string — reintroducing the
  two-spellings-one-car problem the package exists to prevent, in the one case it would be
  hardest to spot. Pinned by `TestBareLetterPairIsNotACountryCode`.
- 2026-09-14 — A test of mine failed and the test was wrong, not the code: I had asserted
  `AB+123` invalid on the grounds that `AB` is not a country code — it is two letters, so it
  is well-formed. Replaced it with a test that states the actual decision: country codes are
  **not** validated against a real ISO list. The list would need maintaining, and the two
  failure modes are wildly asymmetric — an odd code on a car that is genuinely on site costs
  nothing (the plate is the identity), while refusing it leaves that car out of the
  inventory, which is the problem this PRD exists to solve.
- 2026-09-14 — ✅ All criteria. Tests, `go vet` and `staticcheck` green in the api container.
