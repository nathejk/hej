# 287 — Uniform refreshIfVersionDiffers across the stores

**Status:** open
**Priority:** high
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 1. The sync loop (task 286) needs one uniform way to say "here is the
server's version of your dataset; refresh yourself if it differs from what you hold".

`contacts.store.ts`'s `refreshIfStale` is the model, **minus its private version
request** — it currently asks `/api/contacts/version` itself, and the sync response now
provides that. So the new method takes the version as an argument and makes at most one
request: the payload, and only when the version differs.

Stores to cover: `contacts`, `profile`, `scans`, `handouts`, `checkpoints`, plus the
race-area cache. All six exist already, so this is a uniform addition rather than new
state.

Two things each implementation must get right:

- **Replace, do not merge** (PRD 009 §6). A dataset small enough for one payload is
  replaced wholesale, so a sheet that was deleted or a contact who left the race stops
  existing on the device. A delta needs explicit tombstones or the server's purge is
  decorative.
- **Tolerate being refreshed while a view renders it.** A list being scrolled must not
  reorder mid-refetch (PRD 017 §7) — apply on next idle or preserve scroll anchoring.

The held version must survive a reload, or every cold start refetches everything: it is
persisted alongside the cached payload, not kept in memory only.

## Acceptance Criteria

- [ ] `refreshIfVersionDiffers(version)` on all six stores, same signature and
      semantics.
- [ ] Returns whether it refetched, so the loop can report/log.
- [ ] Same version → no request at all.
- [ ] Different version → payload refetched and **replaced** wholesale; held version
      updated.
- [ ] Nothing held → fetches outright (a first sync is not a special case for callers).
- [ ] A failed refetch keeps the cached copy and records staleness; does not throw.
- [ ] The held version is persisted with the payload and survives a reload.
- [ ] `contacts.store.ts`'s `refreshIfStale` is expressed in terms of the new method
      (or removed if task 288 leaves no callers).
- [ ] Per-store tests: no-request-on-same, replace-on-differ, fetch-when-empty,
      failure-keeps-copy, version-survives-reload.
- [ ] `npm run type-check` and the unit suite green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1.
