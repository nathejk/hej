# 183 — Encode PRD 009's priority order and storage ceiling as data

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

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

- [x] The order, sizes, ceiling and per-dataset allocations live in one module —
      `vue/src/config/offline.ts` — typed, with no feature-specific logic. *Landed in
      `config/` rather than `helpers/offline/`; see the log for why, and PRD 009 §8 is updated.*
- [x] Each dataset declares `unrecoverable: boolean`; the location track is the only `true`.
- [x] A unit test asserts the invariant that matters: **no recoverable dataset ever ranks
      above an unrecoverable one**, so a careless future edit fails the build rather than the
      race.
- [x] The ceiling and the tile allocation are named constants with the reasoning (iOS 16.4
      ~1 GB, headroom for a larger area) in a comment, not bare numbers.
- [x] Nothing imports this from inside a Workbox `generateSW` callback — those are
      stringified verbatim into `sw.js` and module-scope identifiers become undefined free
      variables at runtime. Task 087's log has the incident.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Plan: one declaration module, ordered array as the single source of rank, plus the invariant test.
- 2026-09-01 — **Put it in `config/offline.ts`, not `helpers/offline/` as the PRD said.**
  `config/cache.ts` already holds tile budget numbers and is imported by `vite.config.ts`, so it
  must stay free of browser-only imports; the dataset table needs the same property, and
  splitting one budget across two halves of the tree would guarantee the halves drift. Logic
  (eviction, adapters) still goes to `helpers/offline/` in task 186. PRD 009 §8 updated to match
  rather than left disagreeing with the code.
- 2026-09-01 — **No `rank` field: array order is the order.** Two sources for one fact drift,
  and a numeric rank invites gaps and ties that mean nothing. `evictionOrder()` reverses the
  list and *filters out* anything unrecoverable rather than merely ranking it last — so a caller
  that walks one step too far cannot reach the track at all.
- 2026-09-01 — **The track is budgeted against its hard ceiling, not its expected size.**
  ~195 KB is a normal 12-hour race, but `TRACK_MAX_POINTS` allows 20,000 points ≈ 2.7 MB, so it
  gets 3 MB. Everything else is budgeted for what it normally holds, because overshooting there
  only costs a re-download; this one is never evicted, so its plan has to cover the worst case it
  is permitted to reach.
- 2026-09-01 — Sizes: track 3 MB, shell 5 MB (planning figure, task 192 measures it),
  directory index 1 MB, portraits 4 MB (~4.5 KB × event-wide, task 104), tiles 500 MB. Total
  513 MB against a ~1 GB ceiling. Also added `TILE_EVICTION_ZOOM_ORDER` — z16 first, z17 absent
  because it is never cached.
- 2026-09-01 — ✅ All criteria complete. 11 tests pass; `npm run type-check` clean. Tests run
  via `docker compose exec ui` — there is no host-level node in this environment.
- 2026-09-01 — Done. The declaration is inert until something reads it: task 184 (`offline.store`)
  and task 186 (eviction) are the consumers, and task 192 is what makes the four existing caches
  actually observe it. Until 192, this file is a decision recorded, not a decision enforced.
