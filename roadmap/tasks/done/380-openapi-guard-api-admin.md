# 380 — Extend the OpenAPI guard to `/api/admin`, and annotate every endpoint

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

Every admin endpoint needs OpenAPI annotations — `@Summary`, `@Description`, `@Tags`, `@Produce`,
`@Success`, `@Router` and a documented `@Failure 401` — which `.rules` already requires of all
endpoints. The problem PRD 022 §8.9 identifies is that **nothing currently enforces it here**:
`isInScope` in `go/cmd/api/glimtopenapi_test.go` matches `/api/glimt*`, anything containing `glimt`,
`/api/public/` and year-prefixed paths. `/api/admin/*` is none of those, so annotations could be
forgotten, half-written or wrong and the suite would stay green.

So: extend `isInScope` to `/api/admin/`, and add a new tag `admin` to `allowedGlimtTags`. A separate
tag rather than reusing `public-site` for the reason that list already records about keeping
`glimt-public` apart from `public-site` — folding the curator's tool in with the public site would put
two things with entirely different rules in one section of the rendered spec.

Two mechanical constraints the AST-based guards impose, worth knowing before writing handlers rather
than after: **handler names must end in `Handler`**, and **route paths must be plain string literals**.
A path built by concatenation is invisible to the guard, which is the same as not being covered.

This is a narrowing of the broader `roadmap/tasks/open/328-openapi-guard-whole-api.md`, not a
replacement for it. Task 328 widens the guard to the whole API; this task does the slice PRD 022 needs
and should leave 328's case intact rather than closing it.

## Acceptance Criteria

- [x] `isInScope` covers `/api/admin/` and `admin` is in `allowedGlimtTags`
- [x] Every admin endpoint carries the full annotation set including `@Failure 401`
- [x] Deleting an annotation from any admin handler fails the suite — verified by breaking it
- [x] Every admin handler name ends in `Handler` and every admin route path is a plain string literal
- [x] The rendered spec groups the admin endpoints under their own `admin` tag
- [x] Task 328 is noted as still open and still broader than this change

## What was done

The scope change is two lines. The interesting part is what turning the guard on revealed.

**`isInScope` now matches `/api/admin/`, and `admin` is an allowed tag** — its own group rather than
folded into `public-site`, for the same reason `glimt-public` is kept apart from it: these endpoints are
behind a credential, they write, and several of them delete. A reader who opens the `admin` section is
asking a different question from one reading about the frontpage.

### The guard only knew about `requireAuth`

With the admin routes in scope, all fifteen failed twice over — and both failures were the guard being
wrong, not the annotations:

1. `TestGlimtEndpointsDocumentAuthFailure` read `authenticated` (is it wrapped in `requireAuth`?) and so
   reported every documented 401 as *implying the handler reads a session, and the public routes must
   not*. On an admin route that is precisely backwards.
2. `TestGlimtFailureCodesMatchTheHandlers` walked the **handler body**, which contains no 401 and no 429
   branch, because `requireAdmin` owns both.

So `registeredRoute` now carries **`gates`** — every `app.` middleware the registration wraps the handler
in — and the failure-code walk unions the statuses of the handler *and* its gates. A route is what a
client calls, so the route's whole chain is what the annotation describes. `credentialed()` replaces the
bare `authenticated` check in the 401 test: two different proofs (a participant's session, the curator's
shared credential), deliberately never combined, but the same question for "must this document a 401?".

Gates are identified by **exclusion** — anything on `app` that is not the handler — rather than by an
allowlist, so the next wrapper is followed by default. Missing one produces a guard that quietly checks
less than it claims, which is the failure mode this whole task is about.

### Two statuses nobody had noticed

Following the gates surfaced real, undocumented outcomes:

- **421.** `adminTransportOK` refuses plain HTTP outside development before looking at the credential —
  because the credential is *in the request being refused*. `http.StatusMisdirectedRequest` was not in
  `statusConstant` and is now, and all fifteen endpoints document it.
- **304** on `showAdminPhotoMediaHandler`. It answers conditional GETs like its glimt counterpart, and
  said nothing about it.

`statusesWritten` also had to learn **`http.Error`**: `requireAdmin` writes its 401 and
`adminTransportOK` its 421 that way, since both need a plain-text body and a `WWW-Authenticate`
challenge rather than the JSON envelope the app helpers produce. Recognising only `WriteHeader` meant the
one status the entire admin surface is built around read as unreachable. The admin `@Failure 401` lines
were corrected to drop `{object} map[string]string` for the same reason — they do not return JSON, and a
spec that says otherwise is the exact drift this guard exists to catch.

### Guarding the guard

`TestTheAdminSurfaceIsInsideTheAnnotationGuard` asserts the guard *sees* at least ten admin routes and
that every one is credentialed. `isInScope` is a predicate over path strings, so the way it stops
covering this tool is not by being deleted — it is by the paths moving. Every test in the file would stay
green over an empty list, which is a worse state than having no guard, because the green tick gets read
as an answer. That is this task one level up: fifteen credentialed endpoints were unchecked and nothing
said so. The tag assertion is likewise "every `/api/admin/` route is tagged `admin`" rather than a count,
since a count that must be edited per endpoint is a count somebody edits without reading.

## Verified by breaking it

- Deleted the `@Failure 401` line from `listAdminAlbumsHandler` → named, with its method and path.
- Removed `/api/admin/` from `isInScope` → `the annotation guard sees only 0 admin routes`.
- Incidental discovery while breaking it: replacing an annotation line with a *blank* line rather than
  deleting it splits the Go doc comment group, so `fn.Doc` loses everything above the gap and the
  handler reports as missing `@Summary` too. Noisy, but it fails, which is what matters.

## Verified against the rendered spec

`swag init` is not part of any build here — the annotations are comments and nothing in the pipeline
compiles them, which is the whole argument for the AST guard. Rendered ad hoc via the pinned
`github.com/swaggo/swag/cmd/swag` tool dependency to check the last criterion for real:

```
admin 15   auth 6   glimt 8   glimt-moderation 3   glimt-public 4   public-site 13   …
```

All fifteen admin operations in one `admin` section, and nothing else in it.

## Notes

**Task 328 is still open and still broader than this.** It widens the guard to the whole API; this is the
third slice taken off it (after glimt and the public site) and closes none of it. The file header now
says so explicitly rather than describing the glimt-only scope it had.
