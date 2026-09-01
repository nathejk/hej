# 180 — Per-profile storage keys for the contacts cache and favourites

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

From **PRD 012** §8. Fixes a latent bug that the profile switcher would turn into a routine one.

The contacts directory (`hej.contacts.v1`) and favourites (`hej.contacts.favourites.v1`) are keyed
per **device**. That is already wrong: sign out, sign in as a sibling on the same handset, and the
previous profile's cached directory and favourites are what you see. Rare enough to have gone
unnoticed — and the cache holds colleagues' names, phone numbers and portrait references.

A switcher makes that path ordinary, so both stores become **per profile**:
`hej.contacts.v1.<userId>`, `hej.contacts.favourites.v1.<userId>`.

**Keying rather than clearing, deliberately.** Clearing on switch would work only as long as every
future path remembers to call it; a key that includes the profile holds regardless. It also means
switching *back* finds that profile's own cache intact rather than an empty pane, which matters for
the parent-with-two-children case where switching is frequent.

Note `favourites.pruneAgainstDirectory()` would eventually drop out-of-scope favourites anyway — but
only after a sync, and only for people the new profile cannot see. Not a substitute.

## Implementation

`vue/src/helpers/profileStorage.ts` (`profileKey`, `dropLegacyKey`), applied in
`contacts.store.ts` and `favourites.store.ts`; tests in `stores/profileStorage.spec.ts`.

Both stores expose a `storageKey` getter reading the session store, so call sites are unchanged —
`hydrate()` and `fetch()` keep their signatures and the scoping is not something a caller can forget
to pass.

**Null means "do not touch storage", with no device-wide fallback.** That is the whole point: a
fallback would write a profile's directory back under a key the next profile reads, which is the bug
being fixed. A signed-out store still works in memory for the session; it simply has nowhere
legitimate to persist.

**The legacy key is deleted, not ignored.** An upgrading device has `hej.contacts.v1` sitting there
with the last profile's directory in it — names, phone numbers, portrait references. Ignoring it
leaves it readable until the origin is cleared, so both stores remove it on first hydrate.

Eight existing store tests failed on the key change, which was the correct signal rather than a
nuisance: they asserted on persisted contents, so they now say which profile is signed in. That makes
them more realistic — previously they exercised a state (cache with no owner) that cannot occur in
the app.

## Acceptance Criteria

- [x] Both stores key their persisted payload by the signed-in user id.
- [x] A store with no signed-in user does not read or write anything under another profile's key —
      asserted for both stores.
- [x] Switching profile (or sign out → sign in as someone else) shows the new profile's own cache,
      never the previous one's.
- [x] Switching back finds the earlier cache still present — and both caches coexist, asserted on the
      key set.
- [x] Old device-wide keys are cleaned up on first run.
- [x] Tests: two user ids do not see each other's entries or favourites; the legacy key is removed; a
      missing user id is handled without throwing.

## Progress Log

- 2026-09-01 — Task created from PRD 012 §8, which found the latent per-device keying while
  designing the switch.
- 2026-09-01 15:40 — Implemented as a `storageKey` getter on each store rather than a parameter, so
  scoping cannot be forgotten at a call site.
- 2026-09-01 15:50 — Eight existing tests failed on the key change; updated them to sign a profile in,
  which also removed an unreachable state from the fixtures.
- 2026-09-01 15:55 — ✅ All criteria met. 174 frontend tests pass, `vue-tsc` clean.
