# 183 — Encode PRD 009's priority order and storage ceiling as data

**Status:** open
**Priority:** high
**Created:** 2026-09-01

## Description

PRD 009 §6 decided the cross-dataset priority order and the global ceiling (confirmed by the
maintainer 2026-09-01). This task puts it in the codebase as data, so the eviction policy
(task 186) and the readiness view (task 187) read one declaration rather than each restating
it.

| rank | dataset | size | evictable |
|---|---|---|---|
| 1 | unshipped local writes (location track) | ~195 KB | **never** — unrecoverable |
| 2 | app shell | ~1 MB | only by the OS |
| 3 | directory index | < 1 MB | yes, last |
| 4 | portrait thumbnails | ~1 MB | yes |
| 5 | map tiles, **by descending zoom** (z16 first) | ~324 MB | yes, first |

Ceiling: plan the whole origin inside **~1 GB** (the iOS 16.4 baseline; 17+ and Chrome give
60% of disk, so the old floor binds). Budget **~500 MB for tiles** to leave headroom for a
larger race area than 2026's 428 km².

**This is transcription, not negotiation.** The order is agreed; do not relitigate it here. If
a reason to change it appears, that is a PRD 009 edit first.

`helpers/offline/` is where it goes (PRD 009 §8), as a plain declaration — no registry, no
plugin points. PRD 009 cut the generic layer deliberately (§4, §11.2); this file is policy the
existing caches consult, not a framework they are rewritten onto.

**Why `unrecoverable` is a flag and not a comment.** The location track exists only on the
device until a batch is accepted (tasks 082/083). For every other dataset eviction is a
performance problem; for that one it is data loss. Flagging it means the eviction policy and
any future schema-change handling treat it correctly by construction rather than by each
feature remembering to. `vue/vite.config.ts` already cites this principle to justify refusing
Workbox's `purgeOnQuotaError`.

## Acceptance Criteria

- [ ] The order, sizes, ceiling and per-dataset allocations live in one module under
      `vue/src/helpers/offline/`, typed, with no feature-specific logic.
- [ ] Each dataset declares `unrecoverable: boolean`; the location track is the only `true`.
- [ ] A unit test asserts the invariant that matters: **no recoverable dataset ever ranks
      above an unrecoverable one**, so a careless future edit fails the build rather than the
      race.
- [ ] The ceiling and the tile allocation are named constants with the reasoning (iOS 16.4
      ~1 GB, headroom for a larger area) in a comment, not bare numbers.
- [ ] Nothing imports this from inside a Workbox `generateSW` callback — those are
      stringified verbatim into `sw.js` and module-scope identifiers become undefined free
      variables at runtime. Task 087's log has the incident.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
