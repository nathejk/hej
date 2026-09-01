# 184 — `offline.store` — aggregate cache status for the whole app

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

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

- [x] `vue/src/stores/offline.store.ts` with per-dataset status keyed off task 183's
      declaration, plus the global aggregate.
- [x] A reporting seam features call, rather than the store reaching into feature storage.
- [x] `navigator.storage.estimate()` is read through a guard — absent in some browsers, and
      `TrackStatusView.vue` already shows the pattern.
- [x] The four states above are representable and tested, including "evicted": present
      metadata, absent payload. *Six in the end — `syncing` and `stale` earned their place; see
      the log.*
- [x] Unit tests run in the `node` environment like the other stores: browser APIs are taken
      as arguments, not read as globals.
      as arguments, not read as globals (see `vitest.config.ts` and `contacts.store.ts`'s
      storage seam).

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Plan: options-style Pinia store like contacts/track, four explicit states, browser APIs taken as arguments so the spec runs in node.
- 2026-09-01 — **Six states, not four.** `unknown / empty / synced / stale / evicted` as planned,
  plus `syncing` — without it, "is anything fetching right now?" would be a second parallel flag
  and the one combined progress view PRD 009 asks for would have two sources of truth.
- 2026-09-01 — **`complete` is separate from `synced`.** A half-fetched tile set is stored and
  current and *incomplete*; collapsing the two would let the readiness view call it ready, which
  is precisely the dishonesty the PRD forbids. `ready` therefore requires `complete` **and** a
  stored state, and treats `unknown` as not ready — a readiness answer that defaults to yes
  before anything has reported is the one a user acts on before walking into a forest.
- 2026-09-01 — **`markEvicted` is its own action, not `report({ state: 'evicted' })`**, because
  it must *retain* `syncedAt`. "You had this an hour ago and the phone removed it" is a different
  sentence from "you never had this", and on iOS it is the one we are forced to say. `markCleared`
  keeps it too, for the same reason after a post-event purge.
- 2026-09-01 — `headroomBytes()` takes the **smaller** of the browser's quota and our ~1 GB
  ceiling. Ours is the conservative iOS 16.4 floor, but a device with a nearly full disk reports
  less, and trusting our own number there would plan a download the phone cannot hold.
- 2026-09-01 — ✅ All criteria complete. 19 new tests; full suite 229 passing across 18 files;
  `type-check` clean.
- 2026-09-01 — Done. Nothing reports into it yet — task 192 wires the four existing caches, 185
  supplies `persisted`, 186 supplies evictions. Deliberately inert until then rather than
  guessing on features' behalf.
