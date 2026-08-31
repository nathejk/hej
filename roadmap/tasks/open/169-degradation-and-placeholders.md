# 169 — Names-only degradation and missing-portrait placeholder

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

## Description

Two distinct failure states that must never look like breakage (PRD 007 §5, §6):

**1. Portrait missing.** Skippable at onboarding, so many portraits will be absent
early on. Show a neutral placeholder with initials — visibly "no photo", not a failed
image load.

**2. Images unavailable entirely.** iOS evicts service-worker caches for web apps that
go unused, within days. This interacts badly with PRD 005 pushing users to install
*early*: someone who installs three weeks out and never reopens may arrive with an empty
cache. The pane must degrade to **names-only** — search, groups and favourites keep
working — rather than appearing empty. This is why task 161 keeps the metadata index
separable from the images.

The distinction has to be legible to the user: **"no photo" is not the same as "not
synced yet"**, and a user who believes they have portraits but does not is worse off than
one who knows they do not.

Mitigations for eviction: re-sync on launch when a network is present (task 162), prompt
a sync at check-in over venue wifi, and treat "synced" as a state the app reports rather
than assumes.

## Acceptance Criteria

- [ ] Initials placeholder for a person with no portrait.
- [ ] Names-only mode when images are absent or evicted: list, groups, search and
      favourites all still work.
- [ ] The UI distinguishes "no photo" from "not synced yet".
- [ ] Sync state is prominent enough that silent failure is not possible.
- [ ] Opportunistic re-sync when a network appears.
- [ ] Tests: index present + images absent renders a usable pane.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §5 / §6.
