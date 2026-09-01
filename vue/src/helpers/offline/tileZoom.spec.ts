import { describe, expect, it } from 'vitest'

import { tileZoomFromUrl } from '@/helpers/offline/tileZoom'

// The bbox spans below are the real thing: 40075016.686 / 2^z, which is what Leaflet sends for a
// 256 px tile in EPSG:3857.
function wmsUrl(zoom: number, minX = 1_252_344): string {
  const span = 40_075_016.686 / 2 ** zoom
  const minY = 7_514_065
  return (
    'https://api.dataforsyningen.dk/dtk_25_DAF?service=WMS&request=GetMap' +
    `&bbox=${minX},${minY},${minX + span},${minY + span}&width=256&height=256`
  )
}

describe('tileZoomFromUrl', () => {
  it('derives the zoom from a WMS bbox', () => {
    for (const zoom of [12, 13, 14, 15, 16, 17]) {
      expect(tileZoomFromUrl(wmsUrl(zoom)), `z${zoom}`).toBe(zoom)
    }
  })

  it('accepts an uppercase BBOX parameter', () => {
    const url = wmsUrl(14).replace('bbox=', 'BBOX=')
    expect(tileZoomFromUrl(url)).toBe(14)
  })

  // A server that snaps to its own grid, and floating-point bboxes, both put the computed value a
  // hair off an integer. Flooring would silently reclassify a whole layer one level down.
  it('rounds a bbox that is slightly off the exact grid', () => {
    const span = 40_075_016.686 / 2 ** 16 + 0.4
    const url = `https://api.dataforsyningen.dk/dtk_25_DAF?bbox=0,0,${span},${span}`
    expect(tileZoomFromUrl(url)).toBe(16)
  })

  // Better to leave an unrecognised entry alone than delete it believing it is a tile.
  it('refuses a bbox that is not close to any zoom level', () => {
    const url = 'https://api.dataforsyningen.dk/dtk_25_DAF?bbox=0,0,900000,900000'
    expect(tileZoomFromUrl(url)).toBeNull()
  })

  it('returns null for anything without a usable bbox', () => {
    expect(tileZoomFromUrl('https://api.dataforsyningen.dk/dtk_25_DAF?service=WMS')).toBeNull()
    expect(tileZoomFromUrl('https://example.com/style.css')).toBeNull()
    expect(tileZoomFromUrl('not a url')).toBeNull()
    expect(tileZoomFromUrl('https://x.dk/?bbox=1,2,3')).toBeNull()
    expect(tileZoomFromUrl('https://x.dk/?bbox=a,b,c,d')).toBeNull()
  })

  it('returns null for a zero-width bbox rather than dividing by it', () => {
    expect(tileZoomFromUrl('https://x.dk/?bbox=10,10,10,20')).toBeNull()
  })
})
