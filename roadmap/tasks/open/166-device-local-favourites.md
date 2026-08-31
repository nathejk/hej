# 166 — Device-local favourites

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

## Description

Any person in the directory can be favourited; favourites appear first (PRD 007 §6, §7).

**Device-local** (decision 2026-08-31): stored on the device, no endpoint, and no
server-side record of who is interested in whom — which is more sensitive than it first
sounds. They do not survive a reinstall, and that is accepted.

Implementation notes:

- Store **person ids only**, and resolve them against the synced index at render time.
  Never cache a name or number in the favourites store: a stale copy could display
  someone the user has since lost access to.
- **Re-validate against the manifest on each sync.** A favourite must not outlive the
  user's permission to see that person (role change, reassignment).
- A favourited member who *withdraws* stays visible with their status marking (task 160)
  — that is different from losing access, and must not be conflated.
- Follow the existing local-storage patterns in `vue/src/stores/`, and validate what
  comes back out of storage rather than trusting it (same reasoning as
  `@/helpers/identity` validating a role from localStorage, per `config/roles.ts`).

## Acceptance Criteria

- [ ] Favourite/unfavourite from the row, persisted across reloads.
- [ ] Only ids are stored; names and numbers are resolved from the index.
- [ ] Stored value is validated on read; malformed storage does not break the pane.
- [ ] Favourites out of the permitted set are dropped on sync — asserted by test.
- [ ] Withdrawn favourites remain, with their marking.
- [ ] No favourites request ever reaches the BFF.
- [ ] Favourites section hidden entirely when empty, rather than an empty header.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §11.5.
