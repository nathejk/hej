import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// The public Glimt page must not be swallowed by the service worker (PRD 019 §8, task 323).
//
// # Why this needs a test at all
//
// A single-page app's service worker answers *every* navigation with the app shell. That is what makes
// an installed PWA work offline, and it is exactly wrong for `/offentligt/glimt`, which is a real page
// rendered by the BFF.
//
// The reason it needs guarding rather than just fixing is that **it fails asymmetrically**. A parent
// following a shared link has no service worker and gets the real page, so the bug is invisible to
// everyone who would naturally test it. The only people who see it are **installed members** — who
// would tap a public link and get the app shell — and they are the population least likely to be the
// ones checking whether the public page works.
//
// Nothing else in the build would notice: the denylist is one array in `vite.config.ts` and removing
// it produces a service worker that works perfectly for every other route.

const VITE_CONFIG = fileURLToPath(new URL('../vite.config.ts', import.meta.url))
const BUILT_SW = fileURLToPath(new URL('../dist/sw.js', import.meta.url))

describe('the service worker navigation fallback', () => {
  it('excludes the public glimt page in the config', () => {
    const config = readFileSync(VITE_CONFIG, 'utf8')

    const denylist = config.match(/navigateFallbackDenylist:\s*\[[^\]]*\]/)?.[0]
    expect(denylist, 'navigateFallbackDenylist is gone — every navigation now gets the app shell')
      .toBeTruthy()
    expect(denylist).toContain('offentligt')
  })

  it('still excludes the desktop placeholder', () => {
    // The other half of the same array, and the original reason it exists (task 140): without it an
    // installed client asking for /desktop.html boots the app, which redirects back — a loop.
    const config = readFileSync(VITE_CONFIG, 'utf8')
    const denylist = config.match(/navigateFallbackDenylist:\s*\[[^\]]*\]/)?.[0] ?? ''
    expect(denylist).toContain('desktop')
  })

  // The criterion task 323 actually asks for: verified **in the built `sw.js`**, not just in the
  // config that is supposed to produce it. Those are different claims — a Workbox option can be
  // renamed or silently ignored, and the config would still read correctly.
  //
  // Skipped rather than failed when `dist/` is absent, because the unit suite runs without a build
  // and a test that demands one would be turned off rather than satisfied. It is not decoration: the
  // build runs in the same command as these tests before every commit.
  it.skipIf(!existsSync(BUILT_SW))('is present in the built service worker', () => {
    const sw = readFileSync(BUILT_SW, 'utf8')
    expect(sw, 'the built service worker has no denylist entry for /offentligt/, so an installed ' +
      'member following a public link gets the app shell instead of the page').toContain('offentligt')
    expect(sw).toContain('desktop')
  })
})
