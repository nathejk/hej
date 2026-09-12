# 212 — Fake geolocation provider (`helpers/devGeolocation.ts`)

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 3. A dev-only position source, so the map is usable on a laptop.

`location.store` and `track.store` both narrow to a small slice of
`navigator.geolocation` already (`location.store.ts:129` documents "the part of
`navigator.geolocation` this store uses"), so this provider satisfies that interface
and is injected at the same seam. **Do not patch the global** — a global patch is
invisible at the call site and would also affect code paths that are not under test.

## Why a provider rather than Chrome's sensor override

The sensor panel can pin one coordinate. It cannot *move*, and movement is what the
map, the position marker, the tile cache and the track log actually respond to. It
also cannot produce a failure on demand, and the failure states are the ones hardest
to reach deliberately: `track.store` dedupes identical geolocation failures
(`track.store.ts:90`) and the map has a "location off" state, neither of which has
any other trigger on a laptop.

## Scope

- Configurable accuracy, so the map's accuracy circle and the staleness states are
  reachable.
- `getCurrentPosition` and `watchPosition`, with `watchPosition` emitting on an
  interval.
- Failure modes: `PERMISSION_DENIED`, `POSITION_UNAVAILABLE`, and a timeout (i.e. a
  watch that never calls back — `config/track.ts:37` notes hanging is the expensive
  case, so it must be producible).
- A pinned coordinate in the event area as the default.

## Acceptance Criteria

- [x] `vue/src/dev/devGeolocation.ts` implements the slice the stores depend on
      (`GeolocationLike`, declared by `location.store`)
- [x] Configurable accuracy and coordinate
- [x] Each failure mode can be selected, including a never-answering watch (`hang`,
      kept distinct from `timeout` — see the module comment)
- [x] Inert in production: the module is unreachable there, because nothing in `src/dev/`
      is statically imported by product code
- [x] Unit tested (9 tests), injectable, no global patching
- [x] `navigator.geolocation` is not monkey-patched anywhere

## Depends on

- **Task 206** — for the dev-state conventions and `PROD` inertness.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 3.
- 2026-09-12 09:35 — Landed as `src/dev/devGeolocation.ts` rather than `helpers/`, per the
  `src/dev/` rule established in task 208.
- 2026-09-12 09:36 — **Design change from the task's plan: it is a pass-through *bridge*, not "the
  fake when enabled".** `location.store` resolves its geolocation **once**, into state, when the
  store is created (`geo: browserGeolocation()`). A provider that returned `null` while switched off
  would therefore be captured as the real device, and the panel's toggle would do nothing until a
  reload. The bridge decides per call instead, delegating to the device while off.
- 2026-09-12 09:37 — Watch ownership is tracked per id. Routing `clearWatch` by the toggle's current
  position would hand a fake id to the real API (which ignores it, leaking our interval) or a real id
  to ours (leaving the device's watch running — the radio stays on).
- 2026-09-12 09:38 — Switching the fake off stops its watches, or the interval keeps pushing
  simulated positions over the real ones and the app looks like it has two devices.
- 2026-09-12 09:40 — 🐛 **A test caught a real bug of mine.** `stopDevWatches()` also cleared the
  ownership set, so a watch started on the fake and cleared after switching off was routed to the
  real API — which ignores an unknown id, leaving our interval running forever. Ownership now
  outlives the toggle; only `clearWatch` forgets an id.
- 2026-09-12 09:41 — ✅ 9 tests; full suite 467/467; `vue-tsc` clean; production build re-scanned
  and free of the fake coordinates and UA strings.
