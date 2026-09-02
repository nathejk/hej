import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// Map tiles are cached while the map is used, unconditionally (task 087, task 194).
//
// # Why this is asserted rather than trusted
//
// Maintainer direction, 2026-09-01: *"if users don't want to prefetch the entire map, we should still
// cache it during normal browse as people moves along the route and uses the map feature."*
//
// That is easy to agree with and easy to break, because the plausible next change is a settings
// toggle — "spar data" or "hent kort automatisk" — and wiring the runtime cache to it would look like
// a courtesy while quietly removing the only offline map most participants will ever have. Nobody
// declining a 324 MB bulk download is asking for the tiles they are *looking at* to be thrown away.
//
// The bytes are already being fetched to draw the map, so storing them costs nothing extra. There is
// no user preference this should ever consult, which is why the check below is that the route has no
// condition on it at all.
//
// Structural, following `layout.spec.ts` and `offlineIndicator.spec.ts`: assert the cause, because the
// symptom is a blank map in a forest at 03:00 and nobody sees it here.

const CONFIG = fileURLToPath(new URL('../../../vite.config.ts', import.meta.url))

function tileRouteSource(): string {
  const source = readFileSync(CONFIG, 'utf8')
  const start = source.indexOf('runtimeCaching')
  expect(start, 'no runtimeCaching block in vite.config.ts').toBeGreaterThan(-1)
  // Comments stripped, for the reason `layout.spec.ts` gives about its own guard: the config
  // deliberately *documents* why it does not use `purgeOnQuotaError`, and a naive scan would flag
  // that explanation as the offence it warns against. A guard that fires on its own documentation
  // trains people to delete the documentation.
  return source
    .slice(start)
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/^\s*\/\/.*$/gm, '')
}

describe('browse-time map caching', () => {
  it('is registered as a runtime cache with its own name', () => {
    const routes = tileRouteSource()
    expect(routes).toContain('TILE_CACHE_NAME')
    expect(routes).toContain("handler: 'CacheFirst'")
  })

  // The specific regression this guards. A consent flag, a preference, an env switch — anything that
  // could make the tile route conditional — has no business here.
  it('is not gated on a preference, consent flag or setting', () => {
    const routes = tileRouteSource()

    for (const forbidden of [
      'localStorage',
      'consent',
      'preference',
      'optIn',
      'opt-in',
      'enabled',
      'allowTileCache',
      'saveData',
    ]) {
      expect(routes.toLowerCase(), `the tile route consults "${forbidden}"`).not.toContain(
        forbidden.toLowerCase(),
      )
    }
  })

  // Losing every tile because one write did not fit is unrecoverable in the field: offline, a
  // discarded tile cannot be re-fetched. A full cache that cannot grow is much better than an empty
  // one (PRD 009 §6, task 186).
  it('never purges the whole cache on a quota error', () => {
    expect(tileRouteSource()).not.toContain('purgeOnQuotaError')
  })

  // Opaque responses are padded heavily for quota accounting, so accepting them would turn a 324 MB
  // budget into something far larger for the same tiles.
  it('stores only real, readable responses', () => {
    expect(tileRouteSource()).toContain('cacheableResponse')
    expect(tileRouteSource()).toContain('statuses: [200]')
  })
})
