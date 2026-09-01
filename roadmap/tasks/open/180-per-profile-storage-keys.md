# 180 — Per-profile storage keys for the contacts cache and favourites

**Status:** open
**Priority:** high
**Created:** 2026-09-01

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

## Acceptance Criteria

- [ ] Both stores key their persisted payload by the signed-in user id.
- [ ] A store with no signed-in user does not read or write anything under another profile's key.
- [ ] Switching profile (or sign out → sign in as someone else) shows the new profile's own cache,
      never the previous one's.
- [ ] Switching back finds the earlier cache still present.
- [ ] Old device-wide keys (`hej.contacts.v1`, `hej.contacts.favourites.v1`) are cleaned up on
      first run, so an upgrading device does not leave a readable directory behind under the old
      key.
- [ ] Tests: two user ids do not see each other's entries or favourites; the legacy key is removed;
      a missing user id is handled without throwing.

## Progress Log

- 2026-09-01 — Task created from PRD 012 §8, which found the latent per-device keying while
  designing the switch.
