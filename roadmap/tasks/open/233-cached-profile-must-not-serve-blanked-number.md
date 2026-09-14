# 233 — A cached profile must not serve a blanked-away contact number

**Status:** open
**Priority:** low
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

## Description

This app is offline-first (PRD 009), so a profile response fetched before task 229 shipped may
still be sitting on a device holding a contact number that is really the member's own number —
the number the BFF now blanks.

Projecting the field out server-side does nothing about a copy already written to a device
(`.rules`). Until the cached copy is dealt with, the member sees the collided number, recognises
it, verifies it, and gets fast-tracked through check-in — which is the single outcome PRD 015 §3
sets out to prevent.

Two acceptable fixes: bump the sync version so cached profiles are refetched, or apply the
blanking on read as well as on write. The second is cheap — it is the same pure comparison as
task 229 and the client already holds both numbers — and it also covers a device that is offline
at the moment the version changes. Pick one deliberately and record why.

Depends on task 229.

## Acceptance Criteria

- [ ] A device holding a pre-blanking cached profile does not present the collided contact
      number after the change, verified against a seeded cache rather than by reasoning
- [ ] The chosen approach (sync-version bump or read-time blanking) is recorded with its reason
- [ ] A device that is offline when the change ships still does not serve the number once it
      reads the cached profile
- [ ] The member lands in correction mode, matching the fresh-fetch behaviour from task 232
- [ ] No other cached response carries the contact number; if one does, it is covered too

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
