# 184 — `offline.store` — aggregate cache status for the whole app

**Status:** open
**Priority:** high
**Created:** 2026-09-01

## Description

PRD 009 §8. The one new piece of machinery the PRD asks for: a Pinia store holding the
*aggregate* picture of what this device has cached, so the readiness view (task 187) and the
shell indicator (task 188) have a single source instead of interrogating each feature.

**It holds no data of its own.** Features keep owning their storage — tiles in the Cache API,
the directory in `localStorage`, the track in IndexedDB — and report status into this store.
That asymmetry is deliberate and is the whole shape of PRD 009 after its rescope (§4, §11.2):
shared *policy and reporting*, not shared storage.

Per dataset: last sync, item count, bytes, staleness, and whether it is complete. Globally:
`navigator.storage.estimate()`, whether `persist()` was granted (task 185), and whether the
app is currently running on cached data.

**Model "never synced", "synced", "stale" and "evicted" as distinct states.** They read
differently to a user and conflating them is what produces an app that looks empty (task 090's
lesson, and PRD 009's honesty requirement). `contacts.store.ts` already distinguishes
`hydrated` from empty for the same reason — follow it.

## Acceptance Criteria

- [ ] `vue/src/stores/offline.store.ts` with per-dataset status keyed off task 183's
      declaration, plus the global aggregate.
- [ ] A reporting seam features call, rather than the store reaching into feature storage.
- [ ] `navigator.storage.estimate()` is read through a guard — absent in some browsers, and
      `TrackStatusView.vue` already shows the pattern.
- [ ] The four states above are representable and tested, including "evicted": present
      metadata, absent payload.
- [ ] Unit tests run in the `node` environment like the other stores: browser APIs are taken
      as arguments, not read as globals (see `vitest.config.ts` and `contacts.store.ts`'s
      storage seam).

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
