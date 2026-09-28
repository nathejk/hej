# 454 — A credit filter in the library

**Status:** done
**Priority:** low
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 025 §6 R7. **Lower priority than the rest of PRD 025, and the reason is worth recording:** its original
justification was repairing misspelled credits already in the 2026 data, and the maintainer confirmed there are
none — the feature has not been used yet.

It stays on the board because "which photographs are credited to X" is a question that will be asked — to
re-credit a batch, or to answer a photographer's own request about their work — and today the library cannot
answer it. The filter composes with the others, like `album`, `location`, `verdict` and `tagged`.

## Acceptance Criteria

- [ ] A credit filter on the library read, composing with the existing ones
- [ ] Reachable from the contact sheet's filter row
- [ ] OpenAPI updated
- [ ] Works for both a picked credit and a typed one

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R7, at low priority: nothing to repair yet.

## What changed

`credit=none|any|<value>` on the library read, composing with the other filters.

**One parameter for both questions**, because a curator asks one of two things and shouldn't have to know which
way a credit was recorded:

- `none` / `any` — without or with a credit of **either** kind. `(p.credit <> "" OR p.creditCrewId <> "")`, and
  testing both columns is what makes "uden fotokredit" mean what a curator means by it: with only the typed
  column it would list every photograph credited by picker, which is the exact question the filter exists to
  answer, answered wrongly.
- `<value>` — a crew id **or** an exact credit line, bound once and compared to both columns. One of the two is
  always empty for a given photograph (the writer clears the other), so it cannot match on the wrong field.

**Words, not `yes`/`no`.** `location=yes|no` is the register elsewhere, and this parameter deviates because it
also carries *values*: `credit=no` would be ambiguous the day somebody is credited as "no". `album=none` already
spells its sentinel the same way for the same reason.

**Exact, not a substring.** `credit=Anne` finds nothing if the line reads "Foto: Anne Sørensen". A substring match
would be a search box, and this library has no search — adding one through the back door of a filter would mean
deciding what search means here without anybody having asked for it.

**One button in the filter row: "Uden fotokredit".** That is the checklist question before an album is published.
"Credited to X" cannot be a button — free text does not enumerate, and a button per photographer would be a roster
in the filter row — so it is reached by URL, having copied the line off a photograph. The guard also checks that
`adminFiltersFor` recognises the preset, since a preset it does not know would land the page back on "Alle" while
the grid showed a narrowed set.

## Two more exceptions, and why these ones were cheap

`photo.Filter.HasCredit` and `.CreditIs` tripped the `credit` needle. Excepted, with a short argument rather than
the long one `credit` itself needed: **a filter is a question, not a stored fact.** `HasCredit` is a bool and could
not carry a name; `CreditIs` carries what the curator is looking for, which is already on the photograph they
copied it from. And `photo.Filter` is the curator's read model — it reaches no public surface, since the public
album page reads `album.Item`, walked separately with its own single exception.

## Acceptance Criteria

- [x] A credit filter on the library read, composing with the existing ones
- [x] Reachable from the contact sheet's filter row
- [x] OpenAPI updated
- [x] Works for both a picked credit and a typed one

## Progress Log

- 2026-09-28 — Picked up last, as planned: its original justification (repair the misspellings already in the
  data) was void, since the feature had not been used.
- 2026-09-28 — Settled the shape on one parameter rather than two. "Which photographs are credited to Anne" is one
  question, and a curator who has to know whether Anne was picked or typed has been handed our data model.
- 2026-09-28 — Wrote down why it is exact and not a substring. The temptation is obvious and the cost is a search
  feature nobody specified.
- 2026-09-28 — ✅ Tests at both levels: the SQL (both columns, negation, bound twice, nothing added when absent)
  and the query string (all four forms, refusals, the filter row and its round trip through `adminFiltersFor`).
  Mutation-checked by dropping `creditCrewId` from the condition.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
