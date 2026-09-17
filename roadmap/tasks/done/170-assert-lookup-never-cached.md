# 170 — Assert nothing from a patrol lookup is cached

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

The single most important invariant in PRD 007: **no spejder record is ever written to a
device.**

An earlier draft had the patrol lookup working offline, which meant shipping ~557 spejder
thumbnails to every crew device — a deliberate relaxation of "scope the payload, not the
view", and the largest privacy cost in the design. The 2026-08-31 decision to leave the
lookup uncached removed it entirely. This task is what keeps it removed.

The threat is not malice, it is drift: a service worker route added later, a well-meaning
"add offline support to the lookup" change, or a generic PRD 009 dataset registration
would each silently undo it. Enforcement therefore has to be mechanical:

- `Cache-Control: no-store` on the lookup and its photo endpoint (task 157);
- the routes excluded from the service worker's runtime caching in `vite.config.ts` /
  `push-sw.js`;
- the lookup never declared as a cached dataset under PRD 009 (task 161);
- and a test that actually inspects storage after a lookup.

## Acceptance Criteria

- [x] Test: perform a patrol lookup, then assert the Cache Storage API holds no entry
      whose URL matches the lookup routes.
      — **stronger than asked:** the real Workbox matchers are run against the real lookup URLs, so
      the claim is "no rule *could* match" rather than "none did on this run". See the log.
- [x] Test: after a lookup, no spejder id, name, number or image is present in
      localStorage, IndexedDB or any Pinia-persisted store.
      — asserted structurally; the suite has no DOM. See the log for why that is the honest form.
- [x] Test asserts `no-store` is present on both lookup responses.
      — already existed in `go/cmd/api/patrol_test.go`, including the refusal paths. Named from the
      spec file so the third leg is findable.
- [x] Service worker route config asserted to exclude the lookup paths.
- [x] The tests are named so their purpose is obvious (`TestPatrolLookupIsNeverCached`
      or similar), and carry a comment explaining what breaks if they are deleted.
      — `vue/src/stores/patrolLookupNeverCached.spec.ts`, with a "what breaks if this file is
      deleted" header listing the four realistic drift paths.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8.

- 2026-09-17 — Done. `vue/src/stores/patrolLookupNeverCached.spec.ts` (7 tests), plus a small
  refactor of `vue/src/config/cache.ts` and `vue/vite.config.ts` that made the invariant reachable at
  all.

  **Audit first.** One of the four criteria was already met: `go/cmd/api/patrol_test.go` asserts
  `no-store` on both routes *and* on the refusal and not-found paths (a cacheable refusal is still a
  cached answer about a real child). The other three had nothing.

  And the service-worker leg was worse than "untested": `vite.config.ts` mentioned `patrol` and
  `lookup` **nowhere**. The lookup was uncached **by omission** — `runtimeCaching` happens to match
  only the tile host and three specific paths — so the most important invariant in PRD 007 rested on
  nobody ever adding a broad rule. That is exactly the drift the task predicted, and it was one line
  away from happening at any time.

- 2026-09-17 — **The key decision: import the matchers, do not grep for them.**

  A grep for `patrols` in the build config proves today's spelling and nothing else — it would pass
  happily against a `/^\/api\//` rule that caches everything. So the four `urlPattern` values moved
  into `src/config/cache.ts` (which the build config already imports for cache names) and the test
  **runs the real matchers against the real lookup URLs**. The question asked is the one that
  matters: *would any cache claim this URL?*

  `RUNTIME_CACHE_MATCHERS` is a list rather than four separate exports, so **adding a fifth cache
  without adding it here** is the only way to escape the guard — and that is a visible omission in a
  reviewed diff, where a new `urlPattern` buried in `vite.config.ts` is not.

  Two hazards in that refactor, both handled:

  - Workbox's generateSW **stringifies function bodies** into `sw.js`, so an identifier from module
    scope becomes an undefined free variable in the worker — the trap already documented on
    `TILE_CACHE_KEY_IGNORED_PARAMS`, which cost a `ReferenceError` on every tile request once.
    `urlPattern` is evaluated at build time and safely inlined, which is why importing these is fine;
    `PORTRAIT_URL_PATTERN` is itself a function and *is* stringified, which is safe only because its
    body closes over nothing but its own parameters. Noted in the code as a constraint to keep.
  - **Verified output-equivalent rather than assumed.** Built before and after and diffed `sw.js`:
    the `registerRoute` calls are **byte-identical**, including the stringified portrait function.
    The only differences are asset content hashes, which change on every build because `__BUILD_ID__`
    is a timestamp.

- 2026-09-17 — **Each guard was verified to fail when the invariant is broken.** A guard nobody has
  seen go red is not a guard. Four realistic drift scenarios, each reverted after:

  | change | caught by |
  |---|---|
  | a broad `/\/api\//` rule ("cache all API images") | `is claimed by no runtime cache` — all 4 URLs |
  | portrait matcher loosened `people` → `contacts/.*\/photo` | that test **and** the dedicated one |
  | a recent-lookups list written to `localStorage` | `no file that touches the lookup routes also writes to storage` |
  | `patrol` registered as an offline dataset | `is not an offline dataset` |

  The second is the one to expect: `people` and `patrols` look like two spellings of one endpoint,
  and they are not — one serves crew and gøglere who are in the directory, the other serves minors.
  It gets its own test so the failure message can say so, and that test also asserts the rule still
  matches *directory* portraits, so it cannot pass by doing nothing.

- 2026-09-17 — Two criteria met in a different form than written, both recorded rather than quietly
  reinterpreted:

  - **"Assert the Cache Storage API holds no entry after a lookup."** Not done as written, and the
    version built is stronger. This suite runs in node with no DOM and no service worker, so a
    literal Cache Storage assertion would need a fake SW and would only prove *that run*. Running the
    matchers proves no rule **can** match, which is the property that survives a refactor.
  - **"No spejder data in localStorage, IndexedDB or a persisted store."** Asserted structurally: any
    file that references the lookup routes must contain no persistence call. The net is right — a
    file that never names the endpoint cannot persist its result — and it is paired with three
    assertions on `PatrolLookup.vue` itself: it still resets on close, still calls `reset()`, and
    **still touches no Pinia store**.

  Also asserted: the lookup is not a `SyncDataset`. A `patrol` key on `/api/sync` would mean the
  client holds a copy to compare a version against, which is the whole thing this task prevents.

  One test was written and removed: reading `go/cmd/api/patrol_test.go` from the Vue suite to check
  the `no-store` assertion still exists. It failed — the `ui` container mounts only `vue/`, so `go/`
  does not exist from the suite's point of view. That is the right constraint rather than an obstacle:
  a frontend test reaching into backend source to check a header would have passed on a host and
  failed in CI. Replaced with a comment naming where the third leg lives.

  904 Vue tests, `type-check`, `build`, and the full Go chain including `staticcheck` all clean.

  **Nothing here was verified on a device**, and nothing needs to be: every claim is about
  configuration and source, which is the level the invariant actually lives at. What is *not* covered
  is a cache written by something outside this repo's control — a browser HTTP cache honouring a
  header we set wrong, or a proxy. `no-store` is the answer to both and the BFF asserts it.
