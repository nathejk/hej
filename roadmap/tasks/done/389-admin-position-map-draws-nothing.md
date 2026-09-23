# 389 — The position map draws nothing: the base layer is looked up by index, not by key

**Status:** done
**Priority:** high
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

Reported by the maintainer using the tool:

> *"There is no map in the leaflet container — same layers and same retry as on the patrulje-page."*

The "Sæt position" panel rendered an empty Leaflet container. One line:

```js
const base = layers && layers.layers && layers.layers[0];
```

`public/maplayers.json` keys `layers` **by layer id** — `dtk25`, `dtk50`, `orto` — not as an array. So `base`
was permanently `undefined`, no tile layer was ever added, and the panel showed a grey rectangle.

Three more things were missing, each of which would have kept it blank on its own:

- **The token.** These are Dataforsyningen WMS endpoints and refuse an unauthenticated request. The token
  comes from `/api/config`, which is where the Vue app reads it.
- **The `layers` and `format` parameters.** A WMS URL without them is not a tile request. The old code looked
  for a `kind: 'wms'` discriminator and a `params` object; neither exists in the file.
- **Retry.** Leaflet has none — one failed image leaves that tile grey until something recreates it, and
  `maplayers.json`'s own comment records that patchy rural data makes that the normal case rather than the
  exception. It ships a retry policy (`limit: 3`, `baseDelayMs: 400`, `jitterMs: 250`) that nothing on this
  page was reading.

## Why no test caught it

This is the interesting part. `TestTheAdminPositionPanelWorksWithoutTheMap` asserts the island degrades
silently when Leaflet is unavailable, and it passed — **because a map that draws nothing degrades perfectly.**
Every test about failing gracefully was satisfied by a feature that never worked at all.

So the new guards assert that it *renders*, not that it fails politely. Those are different claims and only
one of them was being made.

## The fix

`drawPositionMap` now looks the layer up by the file's own `default` key, falling back to the first entry
rather than to a hard-coded id — so a renamed default degrades to "some layer" instead of back to an empty
map. It fetches the token alongside the layer config, builds the WMS layer with the same options the app's
`wmsLayerOptions` produces, applies the file's zoom bounds, and attaches a retry.

`attachTileRetry` is a deliberately faithful port of `EventMap.vue`'s, not a simplification. Each property it
keeps is one a tidier rewrite would drop while appearing to work:

- re-assigning `src` on the **same** `<img>` keeps Leaflet's own load/error handlers attached, so a late
  success still fades the tile in normally. A new image loses that.
- the `&_retry=` suffix defeats negative caching of the failed response by the browser or a proxy.
- jitter stops a whole screen of failed tiles retrying in lockstep and hammering the service.
- `isConnected` guards a tile Leaflet has since discarded by a pan or a layer swap.

**Every number and URL is read from `maplayers.json` at runtime.** This page has no build step so it cannot
import the app's TypeScript, which makes drift the standing risk; reading the shared file means a change to
the layer definitions or the retry policy reaches this map without an edit here.

And a map that cannot be drawn now **says so in Danish** rather than leaving a grey square: *"Kortet kan ikke
hentes lige nu. Du kan stadig vælge en post i listen."* Degrading gracefully is not a licence to degrade
silently — that is precisely how this shipped.

## Verified

**Live, against the real WMS.** Rebuilt the container, read the layer config and token the way the island
does, and issued the GetMap request those options produce:

```
default layer key: dtk25 -> Topografisk 1:25.000
tile -> 200 image/png 105834 bytes
```

So the lookup resolves and the resulting URL returns a real tile. (The island itself needs a browser; this is
the decisive half that a suite can reach.)

**Broken to check the guards**, each reverted:

| Sabotage | Caught |
|---|---|
| the original `cfg.layers[0]` restored | named the bug and the reason, plus the missing `cfg.default` |
| `attachTileRetry(...)` call removed | named it |
| the cache-buster reduced to `tile.src = original` | **not caught at first** — see below |

That last one is worth recording. The assertion was on the needle `_retry=`, which also appears in the regex
that *strips* it (`tile.src.replace(/&_retry=\d+$/, '')`), so the test passed over a broken retry. Tightened
to the whole assignment, then re-broken to confirm. A needle that matches the code removing the thing it is
checking for is not a guard.

## Acceptance Criteria

- [x] The position panel draws a base map, with the same layers the app and the public map use
- [x] The layer is looked up by key, with the file's own default honoured
- [x] The Dataforsyningen token is supplied, and a tile request with these options returns an image
- [x] Tile retry matches the app's — backoff, jitter, cache-buster, same-image reassignment — with the numbers
      read from the shared file
- [x] A map that cannot be drawn is explained in Danish, and the checkpoint picker still works without it
- [x] Guards assert the map *renders* rather than only that it fails gracefully, and each is verified by
      breaking it

## Notes

- **The backtick trap, again.** The explanation of this bug quotes `maplayers.json` field names, and
  backticks inside `adminpage.go`'s raw-string template terminate the literal. Symptom:
  `syntax error: unexpected name layers in argument list`. Third time this file has done it; the fix is
  `sed -i '' "<range> s/\`/'/g"`.
- Deliberately unchanged: the panel still works entirely without the map. The picker is the route a curator
  actually uses — they know "Post 3", not a coordinate — and the map is the aid.
