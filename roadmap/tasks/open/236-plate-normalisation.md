# 236 — Licence plate normalisation

**Status:** open
**Priority:** high
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `internal/plate` with a `Normalize`-style function returning the prefixed form
- [ ] `ab 12 345`, `AB12345`, `ab-12-345` and `dk+ab12345` all normalise to one value
- [ ] An explicit non-Danish prefix is preserved (e.g. `SE+ABC123`)
- [ ] No Danish-format enforcement: a plausible foreign plate is accepted
- [ ] Empty and implausible input is rejected with a usable error
- [ ] Table-driven tests covering every rule above, including the idempotence of
      normalising an already-normalised plate
- [ ] `go test ./...` green

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Task created from PRD 010 (approved today).
