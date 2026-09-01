# 161 — Declare the directory's datasets under PRD 009's budget

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Completed:** 2026-09-01

> **Closed 2026-09-01 as superseded — folded into task 192.** Not because it was done, but
> because PRD 009's approval turned it into a subset of another task. Its substance moved into
> **192** (reconcile the existing caches with the agreed budget), which does the same job for
> all four caches at once, and **183** (the order as data). Its acceptance criteria were
> carried across verbatim rather than summarised, so nothing is lost. Kept here for the
> reasoning below, which 192 references.

## Description

The contacts directory is PRD 009's most demanding consumer, not its owner (PRD 007 §8).
Two datasets:

- **person index** — structured, searchable, high priority, survives image eviction;
- **portrait thumbnails** — cache-first binary, high priority, server-issued expiry,
  purged after the event.

Explicitly **excluded**: the patrol lookup. It is live, `no-store`, and stores
nothing (task 157). A generic "make it available offline" treatment applied to it
would silently undo the central privacy property of this feature — see task 170.

**Reframed 2026-09-01.** PRD 009 was rescoped: there is **no dataset registry and no generic
sync engine** to register with (009 §4, §11.2), because three consumers — tiles, this
directory, the location track — had already shipped deliberately different storage. So this
task is no longer "call 009's registration API". It is: keep the storage this feature already
has (`localStorage`, via `vue/src/stores/contacts.store.ts`), and make it observe 009's
shared policy — a declared size, a place in the priority order, a server-issued expiry, and
reporting into 009's `offline.store` so the readiness view can show it.

Sizing numbers this feature owes 009's global budget (counts from task 078, thumbnail size
measured in task 104): ~151 banditter ≈ 0.7 MB, ~99 gøglere ≈ 0.4 MB, ~20 crew ≈ 0.1 MB,
so under ~1 MB for the largest role. The budget itself and the priority order against
PRD 002's map tiles (~324 MB, i.e. essentially all of it) are 009's to set — 009 §11.1.

**Unblocked 2026-09-01** — PRD 009 was approved (`doing/`), and the priority order and
ceiling it owed this task are confirmed (009 §6): the index and portraits sit at ranks 3 and 4,
above map tiles, and **tiles are what gets evicted first** — so the "competing with map tiles"
worry is settled in this feature's favour. Neither dataset is `unrecoverable`: both can be
re-fetched, so eviction here is a performance problem, not data loss.

Much of what this task was going to do now lives in **task 192** (reconcile the existing caches
with the budget) and **task 183** (the order as data). Check there before starting, and reduce
this task to whatever 192 does not cover — or close it into 192 if nothing is left.

## Acceptance Criteria

- [ ] Both datasets appear in 009's priority order with declared sizes, and neither is
      marked `unrecoverable` — both can be re-fetched, so eviction is a performance problem
      here, not data loss.
- [ ] The index and the portrait cache report into `offline.store`, so the readiness view
      shows last sync, size and staleness for them.
- [ ] The metadata index is independently usable when images are absent or evicted.
- [ ] The patrol lookup is absent from all of the above, with a comment saying why.
- [ ] The directory's expiry is **server-issued**, so a wrong device clock cannot defeat the
      post-event purge (with task 173).
- [ ] Bulk image sync stays pre-race and user-initiated; **metadata deltas are exempt** and
      may run during the race on mobile data (PRD 007 §6, now also PRD 009 §6). Note
      "wifi-only" is not implementable on iOS — the restriction is *user-initiated with a
      size estimate*.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8.
- 2026-09-01 — Reframed after PRD 009's review: the registry and generic sync engine were cut,
  so this becomes "observe the shared policy and report into the shared store" rather than
  "register with the engine". Blocker narrowed to 009 §11.1's priority order — and then removed
  the same day: the order was confirmed and 009 approved, placing this feature's data above map
  tiles. Overlaps tasks 183 and 192, created on that approval.
- 2026-09-01 — **Closed as superseded, folded into task 192.** Splitting "declare this feature's
  two datasets" from "declare the other two" would have meant two passes over the same
  declaration and the same reporting seam, with the second inevitably tidying the first.
  Criteria carried into 192 verbatim, including the two that were specific to this feature: the
  index staying usable when images are gone, and the bulk/metadata sync-class split.
