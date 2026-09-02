// Building the exact tile URLs the map would request (task 087).
//
// # Why Leaflet generates the URLs and we do not
//
// The bulk download only helps if its URLs are **byte-identical** to the ones the map asks for: the
// service worker matches cache entries on the whole URL, and only `token` and `_retry` are normalised
// away (`TILE_CACHE_KEY_IGNORED_PARAMS`). One extra parameter, one missing one, one different order, and
// the download stores several hundred megabytes the map will never look up — a feature that appears to
// work perfectly until somebody opens the map in a forest.
//
// The map uses **WMS**, which makes this worse than it sounds. There is no `{z}/{x}/{y}` template to
// reuse: Leaflet builds each URL from `wmsParams` in insertion order and appends a `bbox` it computes
// through the map's CRS. Reimplementing that means reimplementing a private method and its parameter
// ordering, and being wrong is silent.
//
// So the layer itself is asked. A detached, never-rendered map exists only to give the WMS layer the CRS
// it needs — `getTileUrl` goes through `map.unproject` — and is thrown away afterwards. That is a little
// odd, and much less odd than a second implementation of Leaflet's URL construction that has to stay in
// step with it forever.
//
// The options come from `wmsLayerOptions`, shared with `EventMap`, so there is one description of what a
// tile request looks like.

import { baseLayers, wmsLayerOptions, type BaseLayerKey } from '@/config/map'
import type { TileCoord } from '@/helpers/offline/tileArea'

export interface TileUrlSource {
  url: (tile: TileCoord) => string
  dispose: () => void
}

/**
 * A URL builder for one base layer.
 *
 * Leaflet is imported dynamically so this does not pull the map library into the app shell — it is
 * already a lazy chunk for `EventMap`, and the readiness view must not become the thing that loads it.
 */
export async function tileUrlSource(
  key: BaseLayerKey,
  token: string,
): Promise<TileUrlSource> {
  const L = (await import('leaflet')).default
  const cfg = baseLayers[key]

  const container = document.createElement('div')
  // Sized and attached, off-screen — not merely detached (task 203).
  //
  // A detached element has `clientWidth`/`clientHeight` of zero, and a zero-size Leaflet map is a hazard:
  // `setView` and the tile layer's `onAdd` both compute a pixel bounds from the container, and a zero or
  // NaN extent is the sort of thing that throws deep inside the library. An exception here would be
  // swallowed by the sync handler and appear to a user as a button that flickers and does nothing — which
  // is the bug this hardening comes from, and which was expensive to diagnose precisely because it was
  // silent.
  //
  // One tile's worth of size is enough; nothing is ever painted, and the element is removed again below.
  container.style.cssText = 'position:absolute;left:-9999px;top:0;width:256px;height:256px'
  document.body.appendChild(container)

  const map = L.map(container, { center: [56, 11], zoom: 12, crs: L.CRS.EPSG3857 })
  const layer = L.tileLayer.wms(cfg.url, { ...wmsLayerOptions(cfg, token) } as L.WMSOptions)
  layer.addTo(map)

  return {
    url: (tile) =>
      // `getTileUrl` is Leaflet's own, which is the entire point of this module.
      (layer as unknown as { getTileUrl: (c: TileCoord) => string }).getTileUrl(tile),
    dispose: () => {
      map.remove()
      container.remove()
    },
  }
}
