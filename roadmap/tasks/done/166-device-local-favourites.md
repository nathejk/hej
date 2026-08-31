# 166 — Device-local favourites

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

Any person in the directory can be favourited; favourites appear first (PRD 007 §6, §7).

**Device-local** (decision 2026-08-31): stored on the device, no endpoint, and no server-side
record of who is interested in whom — which is more sensitive than it first sounds. They do not
survive a reinstall, and that is accepted.

Store **person ids only**, resolve them against the synced index at render time, and re-validate
against the manifest on each sync so a favourite cannot outlive the user's permission to see that
person.

## Implementation

`vue/src/stores/favourites.store.ts` + spec (16 tests).

**Ids only, asserted.** A test checks the persisted JSON is exactly `["p-1"]` and contains
neither the name nor any `+45`, so a future "cache the name for speed" change fails loudly. A
stale name or number could outlive the user's access to that person, which is the whole reason
for resolving at render time.

**`pruneAgainstDirectory` distinguishes two things that look alike.** A favourite is dropped when
the person is no longer in the manifest — a role change or reassignment put them out of scope. A
favourite is **kept** when the person is merely withdrawn, because a withdrawn member is still in
the manifest with a marking and no number (task 160). Losing them from favourites the moment they
go home would be exactly when a samarit is looking for them. That is why the prune checks
presence in the directory rather than `stillInRace`, and both cases have a test.

**It does nothing while the directory is empty.** An offline start that has not synced yet must
not be read as "you may see nobody" and wipe the list — that would silently destroy user data on
a cold start in a forest.

**A malformed stored array is filtered, not rejected.** One bad element costs that element, not
somebody's whole favourites list.

Storage is the same injected seam as the contacts store, passed in by the caller, so the store
needs no knowledge of how the platform exposes storage and the tests need no DOM.

Also settled a small thing worth naming: a person listed in two populations (a crew bandit) is
one favourite, because `byId` deduplicates. Tested, since the pane renders them as two rows and
"two rows, one favourite" is a reasonable thing to get wrong.

## Acceptance Criteria

- [x] Favourite/unfavourite from the row, persisted across reloads.
- [x] Only ids are stored; names and numbers are resolved from the index.
- [x] Stored value is validated on read; malformed storage does not break the pane.
- [x] Favourites out of the permitted set are dropped on sync — asserted by test.
- [x] Withdrawn favourites remain, with their marking.
- [x] No favourites request ever reaches the BFF — there is no endpoint and no fetch in the
      store.
- [x] Favourites section hidden entirely when empty, rather than an empty header (in
      `ContactsView`, `v-if="favouriteEntries.length > 0"`).

## Progress Log

- 2026-08-31 22:55 — Picked up.
- 2026-08-31 23:05 — Realised pruning must not use `stillInRace`: a withdrawn member is in scope
  and must stay a favourite. Prunes against directory presence instead, with a test for each side.
- 2026-08-31 23:10 — Added the "does nothing while empty" guard after thinking through a cold
  offline start, which would otherwise wipe the list.
- 2026-08-31 23:15 — ✅ All criteria met. 16 tests pass, `vue-tsc --noEmit` clean.
