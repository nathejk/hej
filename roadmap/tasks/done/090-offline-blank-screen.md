# 090 — The app renders a blank screen when offline

**Status:** done
**Priority:** high
**Created:** 2026-08-27
**Picked up by:** agent session (Zed)
**Started:** 2026-08-27
**Completed:** 2026-08-27

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

- [x] Offline reload renders the app instead of a blank screen
- [x] A signed-in participant who goes offline stays signed in and reaches the map
- [x] A network failure in `fetchMe()` is distinguished from a 401 and does not
      abort the router guard
- [x] Offline state is visible to the user, not silent — they should be able to
      tell "no signal" from "signed out"
- [x] Cached map tiles are actually reachable offline (this is what task 087 and
      PRD 009 exist for; currently unverifiable because nothing renders)
- [x] Verified in a browser, offline, not just reasoned about — the task 036
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
- 2026-08-27 — **Open question 1 answered: the remembered identity lives in
  localStorage, as provisional state** (`@/helpers/identity`). What is stored is a user
  id and a role — the *answer* to "who am I", never the proof. The proof stays in the
  HttpOnly cookie and the BFF authorizes every protected endpoint independently, so
  editing the value by hand changes which tabs the nav draws and nothing else. It is
  replaced by the BFF's answer the moment one arrives, and cleared on a real 401 or a
  sign-out.
  Rejected the alternative of caching `/api/me` in the service worker: it reads more
  elegantly, but it makes the service worker responsible for authentication state, and a
  cached 200 would resurrect a session the server has already revoked with the app unable
  to tell the answer was stale. It also dies with the cache, which is evictable.
- 2026-08-27 — **Open question 2 answered: every route stays reachable offline.**
  Blocking navigation is precisely what caused this bug, and a page that says "no signal"
  is strictly better than a route that refuses to open. Verified that no view fetches on
  mount (only the session, notifications and scans stores touch the network), so all
  routes render offline from the precache today.
- 2026-08-27 — Implementation:
  * `fetchWrapper` gains **`NetworkError`**, thrown only when `fetch` itself rejects.
    `fetch` resolves for every HTTP status, so that catch means exactly "the request never
    reached the server" — an absence of an answer, which must not be handled like an
    answer.
  * `session.store.fetchMe()` now has three outcomes instead of two: an identity
    (confirmed, remembered), a 401 (genuinely signed out, forget it), or no network
    (unknown → fall back to the remembered identity and mark it `provisional`). A network
    failure is no longer a sign-out.
  * `ensureReady()` **cannot reject**, and the router guard says so in a comment naming
    the failure mode, because that is the part that turns any error here into a white
    screen.
  * `refresh()` re-confirms a provisional identity when the `online` event fires — which
    is also how an expired session gets discovered.
  * `app.store` owns an `online` flag, seeded from `navigator.onLine` but corrected by
    real request failures. `navigator.onLine` alone is useless here: it is true on a
    captive portal, true with one unusable bar, and true on the event's own patchy
    coverage.
  * `OfflineNotice.vue` uses the shadcn **Alert** primitive (generated for this task) in
    the document flow — not an overlay, which would either collide with `UpdatePrompt` at
    the top or cover the map at the bottom.
  * `LoginView` says plainly that signing in needs signal, because it genuinely does: the
    only secret is an SMS PIN. This is the one screen that cannot work offline, so it
    explains rather than failing silently.
- 2026-08-27 — Moved `Role`/`ALL_ROLES`/`Identity` to `@/config/roles`. Validating a role
  read from localStorage needs the list at **runtime**, which a type union cannot provide,
  and putting the array in `session.store` created a real import cycle with
  `helpers/identity`. It happened to work — the array is only read inside a function — but
  that is evaluation order saving us, not a design.
- 2026-08-27 — **A second bug found by looking at the screenshot rather than the check
  results.** Offline, the map claimed *"Kortlag mangler en API-nøgle
  (DATAFORSYNINGEN_TOKEN)"* even though the key was configured: `GET /api/config` fails
  offline, so an empty token was read as an unconfigured deployment. Same mistake as the
  original bug — a network failure reported as something else — and it accuses the
  operator of a misconfiguration the user cannot act on. Fixed both ends: the token is
  remembered in localStorage (it is a public quota key already served to any browser, and
  it is the same value the next online start would fetch), and `missingToken` now also
  requires being online.
- 2026-08-27 — **Verified in a real browser, 17/17 checks**, using the task 036 harness
  method: the `prod` image serving the built bundle, headless Chromium at 390×844, a real
  sign-in through the BFF, then `setOfflineMode`. Results: the app renders offline
  (`#app` has children), is not bounced to login, shows the notice, stays on `/maps`,
  navigates between routes, and raises no uncaught errors. The login screen renders and
  explains itself when signed out *and* offline. The notice clears when connectivity
  returns.
- 2026-08-27 — **Task 087's tile cache got its first real verification as a side effect**
  — it had only ever been checked by reading the generated `sw.js`. 12 tiles cached while
  browsing online, and all 12 still drawing after going offline and reloading.
- 2026-08-27 — Two harness mistakes worth recording, both of which produced a red result
  from correct code:
  * The first run reported 0 cached tiles because I started the container **without**
    `DATAFORSYNINGEN_TOKEN`, so the map showed its missing-key notice and never requested
    a tile. The harness was testing nothing.
  * The signed-out-offline check failed on a race of my own making: toggling
    connectivity back on fires the `online` event, whose `refresh()` re-saved the identity
    concurrently with the harness clearing localStorage. Reordered to sign out first and
    *then* lose signal — which is also the order a real user does it in.
- 2026-08-27 — UI note, accepted: the notice wraps to two lines at 390px. An earlier
  three-line version was honest and unusable — offline is the *normal* state for hours
  during the event, so this has to cost as little of the map as possible. Two lines is the
  compromise; shortening the text further would drop the "kort og regler virker stadig"
  reassurance, which is the useful half.
- 2026-08-27 — `vue-tsc --noEmit` clean and `npm run build` succeeds. ✅ All criteria
  complete. Moving to done.
