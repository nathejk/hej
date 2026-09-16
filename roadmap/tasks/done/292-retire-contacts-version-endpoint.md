# 292 — Retire /api/contacts/version

**Status:** done
**Priority:** low
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 017 phase 2 (cleanup). `GET /api/contacts/version` was the reference implementation
for the freshness convention and is superseded by `/api/sync`, which returns the same
contacts version among five others.

**It stays for one release after the client stops calling it**, then goes. An installed
PWA can be running an old service-worker-cached bundle, and removing the endpoint the
moment the new client ships would break freshness for anyone who has not updated — the
users least likely to notice and most likely to be mid-event.

Its comment documents itself as the shape other panes should copy. That documentation
duty passes to `/api/sync` (task 285 does the client side), so removing it must not
delete the reasoning — the two traps (wrongly-stable is silent, wrongly-unstable is
expensive) live on in `contacts.go` and `mapversion.go` and must survive.

`contactsVersionFor` itself **stays** — `/api/sync` depends on it. This task removes the
HTTP route and handler only.

## Acceptance Criteria

- [x] Confirmed no client code path calls `/api/contacts/version`.
- [x] At least one release has shipped with the sync loop before removal. **Moot, and that is
      the maintainer's call, not an inference:** the app is not launched and there are no field
      devices outside their control, so the one risk this gate protects against — an installed
      PWA serving an older cached bundle to somebody unreachable — does not exist yet.
- [x] Route and handler removed from `routes.go` / `contacts.go`.
- [x] `contactsVersionFor` and its `versionCache` retained and still tested.
- [x] OpenAPI annotations for the removed endpoint deleted.
- [x] `contactsversion_test.go` reduced to the derivation's own tests.
- [x] The convention's reasoning still documented somewhere it will be found.
- [x] `/api/config`'s `contacts_poll_seconds` removed with it, along with the now-dead
      `contactsPollSeconds` in `vue/src/config/runtime.ts` and its remembered-value key.
- [x] All four Go gates green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 2.
- 2026-09-17 01:00 — Unblocked: the maintainer confirmed the app is **not launched** and there
  are **no field devices** outside their control. The wait-a-release gate existed to protect
  users on an older cached bundle who cannot be reached; with nobody in that position the
  endpoint can go now rather than lingering as a second freshness path nobody calls.
- 2026-09-17 01:10 — Removed: the route, `contactsVersionHandler`, `contactsVersionResponse`
  and its OpenAPI block; `/api/config`'s `contacts_poll_seconds` and the `contactsPollSeconds`
  flag; the client's `refreshIfStale`, its `VersionResponse` type, and the whole
  `contactsPoll` plumbing in `runtime.ts` including the remembered key. `contactsVersionFor`
  and `versionCache` stay — `/api/sync` composes them.
- 2026-09-17 01:15 — **The reasoning was moved, not deleted.** Three things lived only in the
  retired endpoint's comments and are load-bearing for whoever touches freshness next:
  *versions travel in the body, not only the ETag* (because `fetchWrapper` does not expose
  response headers) is now on `syncResponse.Versions`; *push cannot be used for invalidation*
  (iOS requires every push to raise a notification) is now in `sync.go`'s header; and the
  "do not copy this shape, add a key" instruction is on the route in `routes.go`.
- 2026-09-17 01:20 — **The guardian tripwire needed a replacement, not a deletion.** Its own
  comment says "a surface missing from this list is a surface with no tripwire", so
  `/api/contacts/version` was swapped for **`/api/sync`** in the paths list and
  `contactsVersionResponse{}` for `syncResponse{}` in the response-type walk. That is a
  strengthening rather than a like-for-like: `/api/sync` is hit by every device on every
  foreground, so a forbidden field added there would be the most widely leaked field in the
  app — and the profile version is *derived from* a guardian number, which is exactly the kind
  of proximity worth a tripwire.
- 2026-09-17 01:25 — **Six endpoint tests rewritten rather than dropped.** `MatchesManifest`,
  `ChangesWithData`, `CoversEveryExposedField`, `CachesAcrossRequests` and
  `CacheIsKeyedByPermittedSet` were HTTP tests of a route, but they assert properties of the
  *derivation*, which still exists — so they now call `contactsVersionFor` directly, which is a
  better home for them anyway. `MatchesManifest` got better in the process: it now compares the
  `contacts` key in `/api/sync` against the manifest's own `version`, which is literally the
  pairing the client's freshness mechanism depends on. Only the two genuinely route-specific
  tests went (auth/spejder-refusal and the ETag 304), both already covered by `sync_test.go`.
  Same on the client: the four `refreshIfStale` tests were replaced by two additions to
  `syncVersions.spec.ts` — that an unchanged answer does not move `syncedAt`, and that a failed
  refetch keeps both the copy and the old version.
- 2026-09-17 01:30 — `config_test.go`'s two poll-interval tests were deleted, with a comment in
  their place naming the equivalents (`TestSync_ServesTheIntervalAndDebounce`,
  `TestSync_ZeroIntervalIsServedAsZero`) — the lever moved, so its tests moved.
- 2026-09-17 01:35 — Completed. Frontend 733 tests + type-check + production build green; all
  four Go gates green; the load test still passes.
