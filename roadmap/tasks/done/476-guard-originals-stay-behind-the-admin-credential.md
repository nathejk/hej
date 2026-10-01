# 476 — Guard: nothing outside the admin surface may resolve an original

**Status:** done
**Priority:** high
**Created:** 2026-10-01
**Picked up by:** agent session (PRD 027 rollout)
**Started:** 2026-10-01
**Completed:** 2026-10-01

## Description

First task of **PRD 027**, and it is first on purpose: it must land **before** anything stores an original.

PRD 027 reverses a rule `cmd/api/albummedia.go` states absolutely — *"**Never** change the pipeline to preserve EXIF
because this feature wants a coordinate"* — and keeps the photographer's file byte for byte, metadata and GPS
included. That is the maintainer's decision (2026-10-01) and it is safe for exactly one reason: **an original is a
download behind `requireAdmin`, never a thing a reader is served.** Every rendition stays re-encoded and stripped.

So the whole EXIF decision rests on an invariant, and this task is that invariant:

- **R10** — no route outside `requireAdmin` may resolve `originalRef`.
- **R11** — no viewer, `srcset`, grid, public page or PWA surface may name an original variant.

Written now, while `originalRef` does not exist, for two reasons. It is cheap: the guard can be written against the
field's *name* and will simply find nothing to complain about until task 477 adds it. And it is honest: a guard added
after the data exists is a guard written to pass against whatever was built, which is how this repo got the scars
documented in the `go-server-rendered-pages` skill ("a needle matching the comment that explained the rule instead
of the code implementing it, three times in one session").

The failure mode being guarded is silent. Nobody notices a public route gaining a `variant=original` branch; what
they notice, years later, is that a photograph of a child has been served with the coordinates of where it was taken
inside it.

### How to build it

Follow the existing source-walking guards rather than inventing a shape:

- `cmd/api/publicprivacy_test.go` — enumerates routes by parsing `routes.go` rather than from a hand-written list,
  precisely so a route added later is covered without anybody remembering. Reuse `allRegisteredRoutes` /
  `wrapsRequireAdmin` from `cmd/api/admin_test.go` and `glimtopenapi_test.go`.
- `cmd/api/curatorboundary_test.go` — the per-file allowlist pattern, where each entry *is* the justification. The
  same shape fits here: the files permitted to resolve `originalRef` are the admin-owned ones, named with reasons.

Two standing hazards in this repo, both of which have bitten before and both of which apply directly:

1. **A guard will match the comment explaining it.** Strip comment lines before searching — see `foldBody` in
   `nathejk/table/album/membershipsafety_test.go`. This test's own prose will contain `originalRef` many times.
2. **A needle can match the code that removes the thing you are checking for.** Assert whole statements, not
   fragments.

For R11 the viewer's candidates are asserted via the assembled page, using `adminPageSource(t)` / the viewer's own
source, and the rule is that `800w`/`1600w` are the only widths named (PRD 023 §7.9, task 410). A 24 MP file in an
`srcset` is both a privacy leak and a 10 MB page, so this is worth pinning independently of R10 — a route can be
admin-only and still be referenced from markup that a non-admin surface shares.

The guard must pass **today**, before `originalRef` exists, and keep passing after task 477. A guard that only
becomes meaningful later is fine; a guard that is red for three tasks gets commented out.

## Acceptance Criteria

- [x] A test enumerates every registered route by parsing `routes.go` (not a hand-written list) and fails if a route
      that is **not** wrapped in `requireAdmin` reaches code resolving `originalRef`.
- [x] A test fails if any file outside a named, reasoned allowlist resolves `originalRef`; each allowlist entry
      carries the reason that file is permitted to.
- [x] A test fails if any viewer/`srcset`/grid/public/PWA markup names an original variant; the viewer's width
      candidates remain exactly `800w` and `1600w`.
- [x] Each guard's failure message says *why* the rule exists, not just that it was broken — a future reader has to
      be able to tell this from an arbitrary assertion, or they will delete it.
- [x] The guards ignore comment text, so the prose explaining them cannot satisfy or trip them.
- [x] The whole suite is green with `originalRef` not yet existing anywhere.
- [x] Task 477 is noted as the follow-on that makes these guards load-bearing.
- [x] **Added during the work:** each guard was verified to actually fail when violated. A source-reading guard that
      has never been seen red is an assertion nobody has tested.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-10-01 — Task created from PRD 027.
- 2026-10-01 — Picked up. Plan: read the three existing guard tests for their shape, then add
  `cmd/api/originalboundary_test.go` holding all three rules, written against `originalRef` before the field exists.
- 2026-10-01 — Added `cmd/api/originalboundary_test.go` with three guards rather than one, because they catch
  different mistakes. `TestOnlyTheAdminSurfaceResolvesAnOriginal` is the file allowlist (shape borrowed from
  `curatorboundary_test.go`). `TestNoRouteOutsideTheAdminCredentialResolvesAnOriginal` resolves every registration in
  `routes.go` to the **file its handler is declared in** — which the allowlist cannot do, and which is the mistake
  that actually worries me: a permitted file growing a second handler that somebody registers without
  `requireAdmin`. `TestNoReaderSurfaceNamesAnOriginal` is R11.
- 2026-10-01 — Reused `withoutComments` from `viewer_test.go` instead of adding a third comment-stripper; the package
  already had two (`stripGoComments` for `//` only) and a fourth spelling of "ignore the prose" is how they drift.
- 2026-10-01 — **The guard caught something on its first run, and it was not what it was looking for:**
  `portrait.go` already has an `originalRef`. That is the *portrait* original (task 111), which is
  metadata-**stripped** and belongs to PRD 003/007 — the opposite rule, a different feature, nothing to do with the
  library. Resolved with a second map, `filesWithAnUnrelatedOriginal`, rather than by adding it to the permitted
  list: "may hand over the photographer's file" and "happens to use the same identifier" must not become the same
  claim. This is the clearest argument for PRD 027 sequencing the guard first — the needle `originalRef` is less
  specific than it looks, and finding that out while the column does not yet exist cost nothing.
- 2026-10-01 — ✅ All criteria met, and then verified the guards are not vacuous, which the criteria had not asked
  for. Injected `var _ = "variant=original"` into `publicsite.go`: guards 1 and 3 fired. Registered a non-admin
  `/api/public/probe` route whose handler sits in `glimtfeed.go` beside an `originalRef`: guard 2 fired and named the
  route, the handler and the file — and also flagged the two legitimate glimt routes in that file, which is the
  file-granularity of this check being honest about itself. Both injections reverted; `git diff` confirms only the
  intended zip route remains.
- 2026-10-01 — `gofmt` clean, `go vet` clean, full `go test ./...` green. Moving to done. Next: task 477 adds
  `photo.Uploaded.Original` and the five columns, at which point these guards start protecting real data.
