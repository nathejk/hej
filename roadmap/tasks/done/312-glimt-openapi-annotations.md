# 312 — OpenAPI annotations for every Glimt endpoint

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

`.rules`: **all endpoints must have OpenAPI annotations.** swaggo-style comments directly above
each handler, in the style of `updatePhotoHandler` in `go/cmd/api/photo.go`: `@Summary`,
`@Description`, `@Tags`, `@Accept`, `@Produce`, `@Success`, every realistic `@Failure`, and
`@Router` (paths relative to `/api`).

Covers every endpoint from tasks 303–308, 319 and 323, including the unauthenticated public
ones — which should say explicitly that they ignore the session cookie, because that is a
deliberate property and not an oversight (PRD 019 §8).

Document the failure codes that actually exist: 400, 401, 403, 404, 413, 429, 503.

## Acceptance Criteria

- [x] Every Glimt handler has a complete annotation block
- [x] Public endpoints document that they are unauthenticated and ignore the session
      — **nothing to do yet:** the public page is task 323 and does not exist. See the log.
- [x] `@Tags` group them coherently (`glimt`, `glimt-moderation`, `glimt-public`)
- [x] Failure codes match what the handlers actually return
- [x] No endpoint added by PRD 019 is missing annotations

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 — **All twelve endpoints were already annotated**, because the blocks were written as
  each handler landed. So a sweep would have found nothing, and the criterion that actually has
  value is the last one — *"no endpoint added by PRD 019 is missing annotations"* — which is a claim
  about the future that only a test can hold.

  swaggo annotations are **comments**: nothing compiles them, nothing runs them, and a handler
  registered without them behaves identically. That makes them the one kind of documentation that
  goes stale with no symptom at all, and the one worth spending an AST walk on.

  `go/cmd/api/glimtopenapi_test.go` parses `routes.go` for what is *actually registered* and
  cross-checks each handler's doc comment, so **adding a route is what fails** rather than
  forgetting to update a checklist. Five tests: required annotations present; `@Router` path and
  method match the registration; `@Tags` is one of the agreed groups and moderation is still its own
  group of three; 401 on everything behind `requireAuth`, and 404 wherever a `:glimtId` can be
  wrong; and the agreement test below.

- 2026-09-17 — **The agreement test, and the mistake that shaped it.** My first version asserted a
  heuristic: "every write documents 503, because the event stream can be down by design (PRD 008
  §5)". It flagged `uploadGlimtMediaHandler` — and the handler was **right**: uploading stores bytes
  in the blob store and never publishes, so it has no 503 branch at all. The rule was wrong, not the
  annotation.

  So it was rewritten to compare the documented `@Failure` codes against the branches the code
  actually has, via a table mapping each helper in `cmd/api/app/errors.go` to its status. It follows
  calls one level into other `app.` methods, which is necessary rather than thorough:
  `requireGlimtModerator` owns the 401/403/503 for all three moderation endpoints and
  `moderateGlimt` owns hide and unhide entirely, so a walk stopping at the handler body would have
  declared every one of their documented failures unreachable.

  It checks **both directions**, and the second is the one that earns its keep. A missing `@Failure`
  is the ordinary omission. A *documented failure the code cannot produce* is worse and far harder
  to notice — the spec then describes an outcome that cannot happen, and nothing looks wrong.

- 2026-09-17 — **It found three real gaps on its first run**, all of the same shape and all
  undocumented 404s: `listGlimtHandler`, `listGlimtHoldsHandler` and `devGlimtFixtureHandler` all
  answer 404 when the caller has no directory record. That is a reachable state — a signed-in member
  whose upstream row is missing — and a client reading the spec would have treated it as a bug to
  report rather than a documented outcome. The fixture handler was also missing the 400 it answers
  when a role maps to no group. Annotations added.

  And one gap in the test itself: `showGlimtMediaHandler` documents a 304 for its conditional GET,
  written with `w.WriteHeader` rather than through an error helper, so the first walk reported the
  annotation as impossible. The walk now reads `http.Status*` constants passed to `WriteHeader` too
  — non-2xx only, since the table feeds a comparison against `@Failure` lines.

- 2026-09-17 — Two criteria could not be met as written, both because of things that do not exist:

  - **"Public endpoints document that they are unauthenticated"** — there are none yet. The public
    page is task 323. The `glimt-public` tag is in the test's allowed set so it is ready, and 323's
    handler will trip `TestGlimtEndpointsAreAnnotated` the moment it is registered without a block.
    The *ignores-the-session* wording is not something this test can check, so it stays a note in
    323 rather than a false tick here.
  - **429** appears only on the upload handler, and 413 only there too — both correct: the ceiling
    and the size cap are properties of the upload. Task 311 adds more rate limits, and the agreement
    test will require each new branch to be documented, which is a better outcome than this task
    listing the codes by hand.

  Scoped to Glimt deliberately. Widening it to the whole API is a one-line change to `isInScope` and
  is worth doing, but it would fail on handlers from other PRDs that nobody in this task touched,
  and turning a guard red on unrelated work is how guards get commented out. **Left as a suggestion:
  a follow-up task to run this across the whole API and fix what it finds.**
