# 353 — The public map draws the app's layers, and retries broken tiles

**Status:** done
**Priority:** medium
**Created:** 2026-09-21
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

Maintainer instruction, 2026-09-21:

> *"the map should have the same layers as the map in the pwa, remember to include retry mechanism to refetch
> broken tiles"*

The public map island (task 342) carried a hand-copied WMS URL with a comment admitting the duplication and
saying the judgement should be revisited *"if a second layer ever appears here"*. Two things were wrong with
that, and neither was the number of layers:

1. **The copy had drifted to a different service.** The island pointed at `dkskaermkort_DAF` /
   `dtk_skaermkort_daempet`, which is not one of the app's three layers — so the public map showed a base map
   no participant had ever seen on their own phone.
2. **It had no tile retry.** Leaflet has none built in: one failed image request leaves that tile grey until
   the visitor pans away and back. The app has carried a retry since PRD 002 because on rural mobile data a
   failed tile is the normal case — and the morning after the event this page is opened by a hundred people at
   once, which is the same problem from the other end.

## Acceptance Criteria

- [x] The public map offers the same base layers as the app: `dtk25`, `dtk50`, `orto`, with the app's labels,
      formats and attribution.
- [x] A layer switcher, so a family can look at the aerial photograph of the field their patrol crossed.
- [x] Broken tiles are retried with the app's policy — bounded attempts, exponential backoff with jitter, the
      same cache-busting parameter — and the policy is defined in one place for both maps.
- [x] The layer definitions are **not duplicated**: one source, read by the bundle at build time and by the
      island at runtime.
- [x] A failed fetch of the shared config costs the switcher, not the map.
- [x] A test fails if the island ever points at a service the app does not use.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-21 — **Done.** `public/maplayers.json` is now the single source: `src/config/map.ts` imports it at
  build time, `public/publicmap.js` fetches it at runtime.

  ### Why a JSON file rather than a generator

  The two maps cannot share a module — one is TypeScript in a bundle, the other a plain script on a page that
  deliberately loads no bundle. Task 342 considered a build-time generator (the `generate-icons.sh` pattern)
  and judged it too much machinery for one URL, which is how the drift happened. A plain data file needs no
  machinery at all: Vite imports JSON, and `fetch` reads it. The cost is that the `satisfies
  Record<BaseLayerKey, BaseLayerConfig>` compile-time check is gone, so `mapLayers.spec.ts` asserts every key
  exists instead — a runtime check for what was a type check, stated rather than hidden.

  The file carries the *reasoning* as well as the values (`_comment` arrays): which service answers to which
  layer name, and why the aerial layer is JPEG while the topographic ones are PNG (~15× the bytes per tile).
  Those notes were in `map.ts`, and moving the values without them would have left the explanations orphaned.

  ### What moved into it

  Layers, attribution, the default layer, `minZoom`/`maxZoom`, and the retry policy (`limit`, `baseDelayMs`,
  `jitterMs`). `EventMap.vue`'s jitter was a literal `250` inline; it now reads `TILE_RETRY_JITTER_MS`, so the
  two maps cannot back off differently.

  ### The retry, ported rather than reinvented

  Faithfully, including the parts that look incidental and are not: the **same `<img>` is reused** so
  Leaflet's own load/error handlers stay attached and a late success still fades the tile in; `_retry=N` busts
  negative caching (and the app's service worker strips it from its cache key, so URLs stay identical across
  the two surfaces); backoff has jitter so a screen of failures does not retry in lockstep; a tile no longer
  in the document is dropped. The island skips the app's timer bookkeeping, because this map lives until the
  page is navigated away from, which cancels everything anyway.

  ### Deliberately not shared: the layer choice

  The app persists it under `hej.map.baseLayer`. The island does not persist anything — this page is read by
  people who are not members, and letting a stranger's browsing change what a member's app opens with would be
  a strange coupling. A test asserts the island touches neither that key nor `localStorage`.

  ### Two self-inflicted problems worth recording

  **I reformatted three tracked files with the wrong tool.** `npx prettier --write` installed prettier 3.9.8 on
  the fly — it is not a project dependency and there is no config — so it applied *defaults* (double quotes,
  semicolons) against a hand-maintained style, rewriting 335 lines of `EventMap.vue` where my change was 3.
  Reverted with `git checkout` and redone by hand; the final diff of that file is the four lines it should be.
  **This repo has no formatter: match the surrounding style by hand.**

  **The first version of the spec failed on its own documentation.** It asserted the island does not contain
  `hej.map.baseLayer` — but the island's comments *name* that key, precisely so the next reader knows why it is
  avoided. The assertion now runs against the file with line comments stripped, which is the honest way to
  assert about what code does.

  ### Verified

  `/maplayers.json` answers 200 as `application/json` through the dev stack, and both it and `publicmap.js`
  land in `dist/` so the Go binary serves them in production. Type-check clean, 952 frontend tests pass, Go
  suite unaffected. The island's behaviour on a real map — the switcher, and a tile actually failing and being
  refetched — is a browser check that belongs with task 348's device pass.
