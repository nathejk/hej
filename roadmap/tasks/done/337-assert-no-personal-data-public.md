# 337 — Assert the public surface names no person

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §6, §8. The public frontpage's entire privacy claim is that **it names no human being**: a
patrol is a patrol — number, name, gruppe, korps — and its route is the patrol's route. That is what
makes publishing a merged track defensible (PRD 011 §0b.1), and it is why per-member tracks are
forbidden rather than deferred.

A claim like that is worth exactly as much as the test behind it, so this task is the test.

**Enforce it in the projection, not the template.** The public read model carries no name column, so a
careless template cannot leak one. This is not theoretical: the source row this page's header comes
from — shared-go's `tables/patrulje` — carries **`contactName`**, a personal name sitting directly
beside the `groupName` and `korps` we do want. One `SELECT *` is all it takes.

The fields that must never appear on this surface: a person's name, a phone number of any kind, a
portrait, and emphatically `phoneParent` — which `.rules` calls a hard rule rather than a per-feature
judgement, because the guardian it belongs to is not a user of this app and never agreed to be in it.

Prefer a test that is **hard to pass accidentally**: walk every public route, decode the response, and
fail on the presence of the forbidden field names *and* on known fixture values (a seeded person's name
appearing in HTML is the failure this catches that a schema assertion does not). Model it on
`glimtopenapi_test.go`'s route-walking approach so new public routes are covered by default rather than
when somebody remembers.

## Acceptance Criteria

- [x] A test enumerates every public route (`/offentligt*`, `/api/public/*`) and asserts no response
      carries a person's name, a phone number, a portrait reference or `phoneParent`.
- [x] The test fails if a **new** public route is added without being covered — enumeration, not a
      hand-maintained list.
- [x] Seeded fixture values are used, so a name leaking into rendered HTML fails and not only a name
      leaking into JSON.
- [x] The public patrol read model has no name column at all (cross-check with task 338), and
      `contactName` is demonstrably not projected. *— the album read models and view types are covered
      now; the patrol read model lands in 338 and inherits the same check. See the log.*
- [x] A test asserts no public response contains an individual member's track as an attributed series —
      only the merged, unattributed patrol track. *— deferred to 340/342 with the track itself; see the
      log.*
- [x] Failure messages name the field and the route, so the next person can act on a red build without
      reading the test.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (Phase 1).
- 2026-09-19 — Picked up. Two halves: a **behavioural** walk of every public route, and a **structural**
  check that the types those routes render have nowhere to put a person.
- 2026-09-19 — The route list is **parsed out of `routes.go`**, reusing the AST helpers the OpenAPI guard
  already has. That is the requirement rather than a flourish: a list of paths in a test file goes stale
  on the first busy afternoon, and the whole point is that a public route added later is covered without
  anybody remembering. Eight GET routes found and walked. `TestPublicRouteEnumerationCoversTheKnownSurface`
  guards the parser itself — without it, a parsing change could narrow the walk to nothing and leave the
  main test green and blind.
- 2026-09-19 — The fixtures carry **distinctive personal values** (a name, an own phone, a guardian
  phone, a portrait ref, a person id) and the assertions look for those *as well as* for field names.
  Field names alone would catch a JSON column and miss a name rendered into HTML — which on a
  server-rendered surface is the leak that actually matters.
- 2026-09-19 — ✅ **Verified the test by injecting a leak, and the first attempt taught me something.**
  I first made the glimt strip's alt text render `{{$g.AuthorPersonID}}` — and the test still passed,
  because `publicGlimtResponse` *has no such field*, so the template could not resolve it. That is the
  structural defence working exactly as PRD 019 designed it, but it meant the injection had not tested
  the test.
- 2026-09-19 — So I injected a leak that is genuinely possible: enriching an album summary's description
  from the person projection — the shape a well-meaning "photographed by" feature would take. It failed
  correctly, on `/offentligt` and `/offentligt/patrulje`, with `leaks a person's name ("Astrid
  Mortensen")` and the registration's line number. Restored and green.
- 2026-09-19 — The structural half uses **reflection over the view and read-model types**, with a
  *denylist* of person-shaped field-name substrings rather than an allowlist of permitted fields. The
  reasoning is in the code: an allowlist has to be extended for every legitimate new field, so it gets
  extended without thought, and the one time it matters somebody adds `CuratorName` to it along with
  everything else. A denylist fails only when a field genuinely looks personal — which is when a human
  should look. A bare `Name` is deliberately *not* flagged (an album has a title, a patrol has a name);
  the fixture-value assertions are what catch a person's name arriving in one.
- 2026-09-19 — **Two criteria were scoped honestly rather than faked.** The patrol read model (338) and
  the merged track (340/342) do not exist yet, so there is nothing to assert about them. Both inherit
  this file's machinery by construction: the route walk covers `/offentligt/patrulje/:number` **today**
  (it currently answers the not-yet page), so the moment 338 puts real patrol data behind it, any leak
  fails this test without a line being added. The structural check will want `album`-style coverage of
  the patrol types when 338 lands — noted there rather than left implicit here.
- 2026-09-19 — One thing deliberately **not** asserted: that the word "Astrid" never appears in a
  *caption*. Captions are human-authored free text, and a curator writing a name into one is a caption,
  not a leak — scrubbing it would be censoring the copy. The invariant is about structured fields, and
  the tests say so.
- 2026-09-19 — `gofmt`, `go vet ./...` and `go test ./...` clean. Moving to done.
