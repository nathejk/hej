# 212 — Fake geolocation provider (`helpers/devGeolocation.ts`)

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `vue/src/helpers/devGeolocation.ts` implements the slice the stores depend on
- [ ] Configurable accuracy and coordinate
- [ ] Each failure mode can be selected, including a never-answering watch
- [ ] Inert in production (`import.meta.env.PROD`), consistent with task 206
- [ ] Unit tested, injectable, no global patching
- [ ] `navigator.geolocation` is not monkey-patched anywhere

## Depends on

- **Task 206** — for the dev-state conventions and `PROD` inertness.

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 3.
