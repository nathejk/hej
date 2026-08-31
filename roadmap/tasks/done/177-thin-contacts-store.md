# 177 — Thin contacts store (interim local data layer)

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

The local data layer the contacts pane reads from: the synced directory index, persisted so
the pane works offline, with search, groups and favourites built on top (tasks 163–167).

**Deliberately thin, and deliberately not a sync engine.** Task 161 asks for the directory to
be registered as a **PRD 009 dataset**, but PRD 009 is still an unapproved draft — there is no
engine to register against. Building a generic sync layer here would compete with the one 009
exists to provide, which is the exact duplication 009 was written to prevent (see task 171).

So the rule for this task: **no abstraction that 009 would have to displace.** One store, one
dataset, no plugin points, no generic "register a dataset" API, no eviction policy, no storage
budget.

## Implementation

`vue/src/stores/contacts.store.ts` + `contacts.store.spec.ts` (24 tests).

One Pinia store: `entries`, `version`, `syncedAt`, `loading`, `hydrated`, `forbidden`,
`error`. Actions `hydrate()`, `fetch()`, `refreshIfStale()`. Getters `groupViews` and `byId`.

**No ETag plumbing needed.** Both endpoints return `version` in the body, so the store never
touches response headers — which `fetchWrapper` does not expose. The BFF's ETags remain useful
for the browser's own conditional requests; the store simply compares two strings. That
removed the one piece of infrastructure I expected to have to add.

**`refreshIfStale()` does not touch `syncedAt` when the version matches.** "We checked" and
"we refetched" are different facts, and the UI reports the second one — if a version check
bumped the timestamp, the pane would claim to be freshly synced after a poll that transferred
nothing.

**`forbidden` is state, not an error.** A spejder gets 403; there is nothing to retry and
nothing to apologise for. It also clears the stored copy, because a role can change mid-event
and anything held is then out of scope.

**Storage is an injected seam, not a global.** `vitest.config.ts` runs tests in `node` on the
explicit grounds that "the modules under test take their browser environment as an argument
rather than reading globals", and jsdom is not a dependency. My first draft called
`localStorage` directly and the tests failed with `ReferenceError` — the right fix was to
follow the house rule rather than add jsdom, so the store holds a `ContactsStorage | null`
resolved once at construction. Tests assign a fake and assert on what was persisted.

That seam also made the null case explicit and testable: a platform with no storage at all
still works for the session, it just cannot survive a reload. Note the resolver guards even
*reading* the global, because Safari throws on access in some privacy modes rather than on
use.

**Replace, never merge** — the property task 160 depends on. Tested directly: a member whose
number is purged loses it from state *and* from the stored payload, and a person who leaves
the permitted set disappears.

`groupViews` groups on the **innermost** group of the path, so when lok arrives as a second
tier the rows still resolve to their klan. Own group sorts first; labels sort with
`localeCompare(…, 'da')`, which is tested with Æ vs. B because a codepoint sort gets Danish
wrong.

## Acceptance Criteria

- [x] A Pinia store holding the directory entries, the version, and the last-synced time.
- [x] Hydrates from persistent storage on first use, so a cold offline start renders.
- [x] `fetch()` replaces the entries wholesale and never throws.
- [x] Stored payload carries a schema version, and an unrecognised or malformed one is
      discarded rather than crashing the pane (five malformed shapes tested, plus a throwing
      storage and no storage at all).
- [x] A spejder (403) is handled as "not for you" rather than as an error to retry.
- [x] Getters for grouped rendering and lookup by id — `byId` deduplicates, so a crew bandit
      listed twice cannot become two favourites.
- [x] Tests: hydrate, replace-not-merge, malformed storage, offline fetch failure keeps the
      previous copy, 403 handling.
- [x] No generic dataset/sync abstraction — one file, reviewable end to end.

## Progress Log

- 2026-08-31 21:35 — Picked up. Confirmed no header plumbing is needed: `version` is in both
  response bodies.
- 2026-08-31 21:40 — Tests failed with `ReferenceError: localStorage is not defined`. The repo
  runs vitest in `node` deliberately and has no jsdom. Refactored persistence onto an injected
  `ContactsStorage` seam instead of adding a dependency — the house pattern per
  `vitest.config.ts` and `helpers/platform.ts`.
- 2026-08-31 21:45 — Decided `refreshIfStale` must not update `syncedAt` on an unchanged
  version, or the pane would report a sync that never happened.
- 2026-08-31 21:50 — ✅ All criteria met. 95 frontend tests pass, `vue-tsc --noEmit` clean.
