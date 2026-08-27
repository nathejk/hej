# 090 — The app renders a blank screen when offline

**Status:** open
**Priority:** high
**Created:** 2026-08-27

## Description

Found while verifying task 036's service-worker update flow. The service worker is
working correctly — it is the app that fails.

Offline, a reload serves the precached shell (`index.html`, 939 bytes, no
navigation error) and the JS chunk loads from the precache. The only failed
request is `GET /api/me`. But `#app` has **0 children**: the user sees a blank
white screen.

Cause, read from the code and consistent with the observation:

1. `session.store`'s `fetchMe()` treats a 401 as "not signed in" (correct) but
   **rethrows everything else** — including a network failure.
2. `ensureReady()` awaits it inside `router.beforeEach`.
3. A rejected navigation guard **aborts the navigation**, so no route component
   ever mounts, and nothing renders.

There is a second, deeper problem behind the first. Even if the guard tolerated
the failure, `ready` flips to `true` in a `finally` with `user` still `null`, so
the guard would redirect to `/login` — and offline you cannot log in, because
login needs an SMS round-trip. The identity is not persisted anywhere, so an
offline start has no way to know who you are.

This matters more than a normal bug: offline is not an edge case for this app. It
is an overnight walking event in North Zealand forest. Task 087 caches map tiles
and PRD 009 plans a ~360 MB bulk download of the whole race area — all of which is
unreachable if the app cannot render without a network.

## Acceptance Criteria

- [ ] Offline reload renders the app instead of a blank screen
- [ ] A signed-in participant who goes offline stays signed in and reaches the map
- [ ] A network failure in `fetchMe()` is distinguished from a 401 and does not
      abort the router guard
- [ ] Offline state is visible to the user, not silent — they should be able to
      tell "no signal" from "signed out"
- [ ] Cached map tiles are actually reachable offline (this is what task 087 and
      PRD 009 exist for; currently unverifiable because nothing renders)
- [ ] Verified in a browser, offline, not just reasoned about — the task 036
      harness method applies

## Open Questions

1. **Where does the offline identity live?** The session cookie is HMAC-signed and
   survives offline, but the *app* cannot read it (HttpOnly) and cannot ask the BFF
   who it belongs to. Options: persist the last known identity in
   localStorage/IndexedDB and treat it as provisional until the BFF confirms; or
   have the service worker cache the `/api/me` response and replay it offline. The
   second keeps one source of truth; the first is simpler and survives a cache
   eviction. Either way the BFF still authorizes every request, so a stale local
   identity grants nothing — it only decides what the UI draws.
2. Should an offline start be allowed to reach *every* route, or only the ones that
   work without a network (map, rulebook, contacts) with the rest showing an
   offline notice?

## Progress Log

- 2026-08-27 — Task created from the task 036 verification run. Diagnosis above was
  confirmed by probing rather than assumed: the offline navigation succeeded and the
  script tag was served from the precache, which rules out the service worker and
  points at the router guard.
