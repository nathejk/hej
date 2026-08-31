# 157 — Crew patrol lookup endpoint

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

`GET /api/contacts/patrols/{number}` — one patrol's members with portrait ref, current
status and phone number. **Crew only.** This is the only path by which a spejder's
details are reachable in the PWA (PRD 007 §6, §8).

The design is deliberately narrow, and each constraint is load-bearing:

- **Exact match on the full patrol number.** No prefix matching, no listing, no
  "recent lookups", no patrol picker.
- **`403` and `404` indistinguishable**, including for a bandit klan number.
- **`Cache-Control: no-store`**, and excluded from the service worker's routes (task 170).
- **Every call is logged server-side** (PRD 007 §11.7) — a log line on the handler, since
  the lookup is online by definition.
- **All crew**, including the generic `crew` fallback (2026-08-31, deliberate).
- `phoneParent` is absent, as everywhere (`.rules`).

Companion endpoint for the images:
`GET /api/contacts/patrols/{number}/photo/{personId}?size=thumb`, also `no-store`.

## Blocked — split

**2026-08-31: blocked on task 176**, which was split out of this one — the person
projection had no patrol number to match against. Task 176 landed, unblocking this.

## Implementation

`go/cmd/api/patrol.go` + `patrol_test.go`, `person.Queries.ListPatrolByNumber`, routes.

**Its own file, not `contacts.go`.** The rules are the opposite of the directory's in the
dimension that matters: nothing here may be stored on a device. Keeping them apart means a
future "add offline support" change has to walk past a file whose header explains why it must
not.

**The query resolves number → teamId → members**, in one statement with a subquery. This is
the mitigation task 176 promised: a member who signed up after their patrulje was numbered
carries no number, and matching on `teamId` still finds them.

**Empty number is refused in the querier as well as the handler.** Without it, an empty
number matches the column default and returns every unnumbered person in the event — a
mistyped lookup becoming a bulk disclosure of spejder records. Same trap `Lookup` guards
against with an empty phone, and now covered by
`TestListPatrolByNumberRefusesEmptyNumber`, which asserts *no query is issued at all*.

**`no-store` is set first, before authorization**, so no early return can produce a
cacheable response — and asserted for success, refusal, absence *and* a spejder caller. Also
asserted: no `ETag` on the lookup, since a validator invites conditional caching of something
that must not be stored.

**Refactored `streamPortrait` to take `cacheControl` explicitly.** My first attempt was a
`streamPatrolPortrait` wrapper that set `no-store` *after* calling the shared helper — which
would not have worked, because the helper sets `Cache-Control` itself before writing the
body, so the directory's `private, max-age=3600` would have won and spejder faces would have
been cached by every crew browser. Caught while writing it. The parameter is now required at
every call site, with a comment on why the answer differs per surface: getting it wrong is a
privacy bug, not a performance one.

**Photos are scoped to a lookup, not to a person id.** `/patrols/{number}/photo/{personId}`
requires the person to be in that patrol, so a caller must already know which patrol somebody
is in — the same thing the lookup itself requires. A person-id-only route would have been a
general "any spejder's face" endpoint.

## Acceptance Criteria

- [x] Both endpoints behind `app.requireAuth`, crew-only via task 151's function.
- [x] Exact-match number only; no prefix/partial/list behaviour exists (asserted for
      `13`, `1`, `1380`, and that the querier only ever receives a trimmed number).
- [x] `403` and `404` byte-identical, including for a valid klan number (a bandit asking
      for a real patrol vs. crew asking for a nonexistent one — same status, same body).
- [x] `Cache-Control: no-store` on both responses, on every path.
- [x] Members carry name, status and phone; `phoneParent` absent (asserted against the
      body, on records that *do* have a guardian number).
- [x] Each lookup writes an audit log line (caller, role, patrol, member count); refusals
      are logged too, since a bandit probing this is worth seeing.
- [x] Tests: crew succeeds for all four crew roles; bandit, gøgler and spejder are refused
      indistinguishably; probing a klan number reveals nothing.
- [x] OpenAPI annotations present.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
- 2026-08-31 18:20 — Picked up, then blocked: no patrol number exists in the person
  projection. Found `NathejkPatrolNumberAssigned` upstream in shared-go, already projected
  by its `patrulje` table, so this is a local gap rather than a cross-repo lift. Split it
  out as task 176 and left this task in `open/` rather than parking it in `doing/` while
  blocked.
- 2026-08-31 19:20 — Task 176 done; resumed. Query resolves number → teamId → members, which
  also covers 176's documented ordering gap.
- 2026-08-31 19:35 — Caught a bug in my own first draft: a `streamPatrolPortrait` wrapper
  that set `no-store` after calling `streamPortrait` would have been overridden by the
  helper's own `private, max-age=3600`, caching minors' faces in every crew browser.
  Refactored the helper to require `cacheControl` at each call site instead.
- 2026-08-31 19:45 — Added the querier-level empty-number guard and a test asserting no
  query is issued, since an empty number would otherwise match the column default and
  return most of the event.
- 2026-08-31 19:55 — ✅ All criteria met. `go vet ./...` and `go test ./...` pass; `gofmt`
  clean.
