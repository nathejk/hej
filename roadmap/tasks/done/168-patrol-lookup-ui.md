# 168 — Patrol lookup UI

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-09-01

## Description

The crew-only **"Slå patrulje op"** surface (PRD 007 §7), driving task 157's endpoint.

Deliberate constraints — each one is what keeps this from becoming the browsable index of minors'
faces the PRD exists to avoid: a distinct secondary entry (not merged into the main search),
numeric exact-match input, results in a transient panel, no recent-lookups list, nothing
persisted, crew-only. Offline is a first-class state: **"kræver forbindelse"** with a pointer to
the radio, never an empty or stale patrol.

## Implementation

- `vue/src/components/contacts/PatrolLookup.vue` — the panel.
- `vue/src/config/memberStatus.ts` + spec — Danish labels and status predicates.
- `vue/src/config/roles.ts` — `isCrewRole`, mirroring `Role.IsCrew()` in the BFF.
- `vue/src/views/ContactsView.vue` — the entry point, crew only.
- `go/cmd/api/guardiantripwire_test.go` — lookup path added to the shared tripwire.

**Results live in component state and nowhere else.** No store, no localStorage, no cache entry.
Closing the panel clears them, and a `watch` on `open` does the clearing — a panel that reopened
with the previous patrol still in it would be a recent-lookups list with extra steps.

**Status is shown in full, and colour-coded by meaning rather than decoratively.** `memberStatus.ts`
maps shared-go's vocabulary to Danish and exposes three predicates. `isInOurCare` (waiting /
transit / sheltered) is highlighted because it is exactly what a samarit needs *before* setting
off: somebody else already has the member. `hasLeftRace` deliberately excludes `finished` —
marking a finisher as a withdrawal turns an achievement into a dropout, which shared-go's own docs
are careful about.

**Unknown statuses render as-is.** A status this build does not recognise still means something to
the crew member reading it during an incident; only the *marking* logic is conservative about
unknown values, and both halves are tested.

**403 and 404 produce one message.** The BFF makes them indistinguishable so the endpoint cannot
map the numbering; phrasing them differently in the UI would undo that server-side care.

**A plain overlay rather than the `drawer` primitive.** The panel must be dismissible with no
ceremony and must not animate content in from off-screen while somebody is reading a face in the
dark. Noted in the component, per the `.rules` requirement to say why a standard primitive was not
used.

The component header states, in order, why there is no history, no prefix search, no suggestions
and no shared field — the four changes most likely to be proposed as improvements.

## Blocker, resolved

`go build`/`go test` failed for a reason **outside this repo**:

```
../../shared-go/types/section.go:1:1: expected 'package', found 'EOF'
```

`/Users/knj/Development/nathejk/shared-go/types/section.go` was a zero-byte, untracked file, and
`hej` resolves `shared-go` through a local path, so an empty file in that package stopped this
repo compiling. Left untouched deliberately — deleting a file in a sibling repo is not a call to
make silently. The maintainer saved it with content on 2026-09-01, and the Go side was then
verified: `go build ./...`, `go vet ./...`, `go test ./...` all pass.

## Acceptance Criteria

- [x] Crew-only entry point, visually distinct from the directory search.
- [x] Exact numeric input; no partial matching or suggestions.
- [x] Results transient — leaving the panel discards them; nothing in storage or caches.
- [x] No recent-lookups history anywhere in the UI.
- [x] Offline shows "kræver forbindelse" and points at the radio.
- [x] A miss shows one neutral "ingen patrulje med det nummer".
- [x] Rows show face, name, status, phone; withdrawn members marked.
- [x] A comment in the component records *why* there is no history and no prefix search.
- [x] Lookup endpoints added to `guardiantripwire_test.go` — verified 2026-09-01:
      `TestContactsSurfacesNeverCarryAGuardianNumber` runs against `/api/contacts/patrols/138`
      alongside the manifest and version surfaces, and passes.

## Progress Log

- 2026-09-01 00:30 — Picked up. Extracted status labels and `isCrewRole` as pure modules first, so
  they are testable without a DOM.
- 2026-09-01 00:40 — Chose a plain overlay over the `drawer` primitive and recorded why in the
  component, per `.rules`.
- 2026-09-01 00:45 — Added `isInOurCare` highlighting: a member already in a car or at HQ is the
  thing a samarit most needs to know before setting off.
- 2026-09-01 00:50 — Frontend green: 158 tests, `vue-tsc --noEmit` clean.
- 2026-09-01 00:55 — **Blocked on the Go side** by an empty untracked `types/section.go` in the
  sibling `shared-go` checkout, which stops this repo compiling. Left it alone and reported it
  rather than deleting another repo's file. Task stays in `doing/` until the Go test is verified.
- 2026-09-01 01:05 — Maintainer saved `section.go` with content. Go side verified: build, vet and
  the full test suite pass, and the tripwire now demonstrably covers all three JSON contacts
  surfaces. ✅ All criteria met.
