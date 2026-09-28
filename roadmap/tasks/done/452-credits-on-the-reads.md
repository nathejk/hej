# 452 — The public and admin reads render the resolved credit

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 025 §6 R5/R6. Needs task 451.

The public album page and the curator's reads show the credit whether it came from the picker or from typed
text — they are the same credit line by two routes, and the page must not be able to tell.

- **The public response carries the name, never the id** (R6). The id is a handle to a person record and has no
  business on the open web, even though it names nobody by itself. `isPersonShaped` still flags it, so a public
  type that grows one fails — see task 451.
- **An unresolvable id renders nothing** (R5), which is what a photograph with no credit already does.
- The album page is cached for 60 seconds, so a deleted crew member stops being named within that window. That
  is the erasure path working, and it is worth a line in the page's own comments.

## Acceptance Criteria

- [ ] The public album page credits a picked photographer identically to a typed one
- [ ] No public response carries `creditPersonId`
- [ ] A photograph whose credit id resolves to nothing renders no credit line
- [ ] The curator's reads show the resolved name, so the tool shows what the public will see

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R5/R6.

## What changed

`cmd/api/photocredit.go` — `app.creditNames(year, ids)` and `resolvedCredit(typed, crewID, names)`. Two small
functions and a file that exists to hold the reasoning next to them.

**Resolution is in the handlers, not in a SQL join**, and that was the design decision. `photo` and `person` are
projections folded from independent streams; a join across them in a read's statement would put the credit's four
bounds into *every* statement that renders a credit, which is how they come to differ. `person.CreditNames` owns
the bounds, the reads fetch references, the handlers merge names in.

- **The public album page** resolves **only the window it is about to render** — an album may hold hundreds of
  photographs and the page shows a side of them. This is the one read in the service that touches the person
  projection on a public path, so it does as little of it as the page needs.
- **The library read** resolves too, so the tool shows what the public will show: not the reference, which is
  meaningless to read, and not the typed field, which is empty on a photograph credited by picker. It is also how
  a curator discovers a credit has stopped resolving because the crew member asked to be deleted.

`resolvedCredit` prefers the reference when both are set. Not a tie-break: the writer clears one when it sets the
other, in the handler *and* the fold, so a row with both should not exist — and preferring the reference means
such a row resolves to the name that can still be erased.

## The privacy guard needed a *type*-scoped exception

`TestPublicAlbumReadModelHasNowhereToPutAPerson` failed on `album.Item.CreditCrewID`, correctly. The exception is
scoped to **the type, not the field name**, and that distinction is the whole value of it:

- `album.Item` may carry the reference — it is a read model whose consumer is a handler that resolves it;
- **`publicAlbumItem` may not**, and is now walked with no exception at all. It is the type the template renders,
  so the moment somebody carries the reference one step further — where a template could print it — the guard
  fails.

A read model holding an id the handler consumes is ordinary. A rendered type holding a handle to a person record
is not. Field-scoping the exception would have erased that difference.

## Acceptance Criteria

- [x] The public album page credits a picked photographer identically to a typed one
- [x] No public response carries `creditCrewId` — asserted on the rendered type *and* on the page's bytes
- [x] A photograph whose credit resolves to nothing renders no credit line
- [x] The curator's reads show the resolved name

## Progress Log

- 2026-09-28 — Picked up. Chose handler-side resolution over a SQL join; see above.
- 2026-09-28 — The public privacy walk refused `album.Item`. Scoped the exception to the type and added the
  rendered type to the walk with no exception, which is a stronger guarantee than the file had before.
- 2026-09-28 — ✅ Five behavioural tests, including the erasure path (no name, no placeholder, photograph intact)
  and an unavailable person projection — a public album must not go offline because a *credit line* could not be
  read. Mutation-checked by making `resolvedCredit` ignore the reference.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
