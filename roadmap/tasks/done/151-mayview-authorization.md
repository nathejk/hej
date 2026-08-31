# 151 — `mayView(viewer, subject)` authorization for contacts

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

The single server-side authorization function behind every contacts response (PRD 007
§6 access matrix, §8 BFF). One function, used by the manifest, the photo handler, the
profile and the patrol lookup — if this logic exists in two places it will diverge, and
the failure mode is either a privacy breach or a broken game.

Matrix (PRD 007 §6):

| Viewer | spejder | bandit | gøgler | crew |
|---|---|---|---|---|
| spejder | — no pane — | — no pane — | — no pane — | — no pane — |
| bandit | ❌ | ✅ all | ❌ | ✅ all |
| gøgler | ❌ | ❌ | ✅ all | ✅ all |
| crew | ⚠️ patrol lookup only | ✅ all | ✅ all | ✅ all |

Notes that matter:

- `samarit` / `guide` / `postmandskab` are all crew and are not distinguished.
- Spejder is never a viewer, and never appears in a directory listing or search result.
- Crew reach spejdere **only** through the patrol lookup (task 157), never the
  directory — so the function needs to distinguish "may list" from "may look up".
- Placement (which population you are listed in) is task 152 and is *not* this
  function's job. This function answers permission only.

## Implementation

`go/internal/users/contacts.go`, with `go/internal/users/contacts_test.go`.

**Three functions, not one.** The task name says `mayView`, but a single predicate
cannot express this matrix honestly, because crew have *two different* relationships to
a spejder: they may look one up, and they may never list one. Collapsing that into
`mayView(viewer, subject) bool` would have forced every call site to remember which
kind of access it was asking about — exactly the ambiguity that leaks. So:

- `MayUseContacts(viewer)` — does this role get the pane at all;
- `MayList(viewer, subject)` — directory listings, search, favourites, profiles;
- `MayLookUpPatrol(viewer)` — the one path to a spejder.

**A `Population` type, distinct from `Role`.** The matrix's columns are populations, not
roles: a crew member out as a bandit is *listed* among banditter while still *viewing*
as crew. Keeping the two vocabularies apart in the type system is what stops task 152's
placement map from being mistaken for a permission change. Placement itself is task 152;
this file consumes a population as given.

**Placed in `internal/users` rather than `cmd/api`** so it sits next to `Role`,
`AllRoles` and `IsCrew()`, and so it is unit-testable without an HTTP layer.

Also corrected `RoleCrew`'s doc comment in `directory.go`, which still described PRD
007's *superseded* matrix ("identified crew may see every portrait in the event"). It now
records why PRD 007 treats all crew alike without weakening the least-privilege rule for
everything else.

## Acceptance Criteria

- [x] `mayView` (or equivalently named pair, e.g. `mayList` / `mayLookUp`) lives in one
      place in the BFF and is the only authorization path for contacts.
- [x] Table-driven tests cover **every** viewer/subject role pair in both directions,
      including `gøgler` and generic `crew`.
- [x] A test asserts spejder is denied as a viewer on every contacts surface.
- [x] A test asserts the spejder/bandit pair is denied symmetrically — game integrity,
      not just privacy.
- [x] A test asserts crew may look up a spejder but may not list one.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
- 2026-08-31 11:05 — Picked up. Plan: implement in `internal/users` next to `Role`, with
  an exhaustive matrix test.
- 2026-08-31 11:20 — Decided on three functions rather than one `mayView`. A single
  predicate cannot express "crew may look up a spejder but never list one" without a
  flag at every call site, which is the ambiguity that leaks.
- 2026-08-31 11:30 — Introduced `Population` as a separate type from `Role`, so task
  152's placement map cannot be confused for a permission change.
- 2026-08-31 11:45 — ✅ All criteria met. Matrix test fails loudly if a role is added to
  `AllRoles` without a row, rather than silently not testing it.
- 2026-08-31 11:50 — Found and fixed a stale comment: `RoleCrew` in `directory.go` still
  described PRD 007's old matrix, where identified crew saw every portrait. `go test
  ./internal/users/` passes, `go vet` clean, `gofmt` clean.
