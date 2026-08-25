# 067 — Extend the app-role enum with gøgler and generic crew

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent session (Zed)
**Started:** 2026-08-25
**Completed:** 2026-08-25

## Description

PRD 006 §6. The app-role enum is `spejder | bandit | postmandskab | guide |
samarit` in both `go/internal/users/directory.go` and
`vue/src/stores/session.store.ts`. PRD 006 must cover **gøgler** and a generic
**crew** fallback, and neither value exists — a gøgler login is currently
untypeable, and an unclassifiable crew member has no role to be given.

Both new values need a navigation answer too (`vue/src/config/navigation.ts`).
Generic `crew` must be **least-privileged**: it exists because classification
failed, so it must not inherit what identified crew functions get (PRD 007's
access matrix depends on this).

## Acceptance Criteria

- [x] `users.Role` gains `gøgler` and a generic crew value
- [x] `session.store.ts` `Role` type matches exactly
- [x] `navigation.ts` states what each new role sees
- [x] Generic crew is documented as least-privileged, not "unrestricted"
- [x] Existing role-gated routes still behave for the five original roles
- [x] Go and frontend builds/tests green (frontend: see the caveat below)

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-25 — Picked up. Added `RoleGoegler` (`"gøgler"`) and `RoleCrew` (`"crew"`)
  to `users.Role`, and the same two members to the TypeScript union.
- 2026-08-25 — Naming decision: the Go constant is ASCII (`RoleGoegler`) while the
  **value** keeps the Danish spelling `gøgler`, because the value is what travels on
  the wire, appears in `GET /api/me`, and matches the upstream subject vocabulary
  (`NATHEJK.*.gøgler.*`). Anglicising the value would have created a second
  translation to maintain.
- 2026-08-25 — Added `AllRoles`, `Role.Valid()` and `Role.IsCrew()`. `AllRoles`
  exists so anything enumerating roles stops keeping its own list — which is exactly
  how a new role gets silently missed (see below).
- 2026-08-25 — `IsCrew()` deliberately includes `RoleCrew` but its doc comment says
  it must **not** be used for access decisions: `RoleCrew` means "we do not know what
  this person does", so gating on "is crew" would let an unrecognised section slug
  widen access. PRD 007's portrait matrix depends on that distinction, since
  identified crew may see every portrait in the event.
- 2026-08-25 — Navigation: both new roles see the ungated content pages and **not**
  the SOS page. Recorded the reasoning at the gate itself — gøglere staff posts but
  are not in the medical/guide response chain, and `crew` is a classification failure.
  Also noted on the `roles` field that "all signed-in roles" now includes the
  fallback, so anything sensitive must gate explicitly rather than rely on being
  unlisted.
- 2026-08-25 — Seeded both new roles in the mock directory. Without a seed entry a
  role-gated page can only be exercised in production, and the fallback path in
  particular would first appear the day an organizer renames a section.
- 2026-08-25 — **Found a quietly-broken test:** `TestMockDirectory_SeedsAllRoles`
  hardcoded five roles, so it kept passing while no longer covering "all roles" as
  its name claims. Extended it and added an assertion that the seed count equals
  `len(users.AllRoles)`, so the next added role fails the test instead of slipping
  through.
- 2026-08-25 — Added `role_test.go` pinning the role **string values**. They are a
  wire format — compared in the frontend router guard and returned by `GET /api/me` —
  so a rename is a breaking change, not a refactor.
- 2026-08-25 — ✅ Go side fully verified: build, `go test ./...`, vet, staticcheck,
  gofmt clean on both the workspace and `GOWORK=off` paths.
- 2026-08-25 — **Verification caveat (frontend):** there is **no Node runtime on this
  host** (`npm` not found, `node_modules/.bin` absent) and the Docker daemon is not
  responding, so `npm run type-check` (`vue-tsc`) could not be run. Substituted a
  manual audit: `Role` is referenced in only three files and used exclusively as
  `Role[]`, `Role | null`, and a field type — there are **no exhaustive maps or
  switches keyed by `Role`**, so widening the union cannot make existing code
  non-exhaustive. Low risk, but genuinely unverified by a compiler.
- 2026-08-25 — Moving to done.
