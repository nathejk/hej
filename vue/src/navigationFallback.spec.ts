import { existsSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// What the service worker's navigation fallback must **not** answer for (PRD 019 §8, tasks 323/318).
//
// # The bug class this exists to stop
//
// A single-page app's service worker answers *every* navigation with the app shell. That is what makes
// an installed PWA work offline, and it is exactly wrong for anything the server renders itself.
//
// It has now cost two real bugs, and the reason it keeps happening is that **it fails
// asymmetrically** — never for the developer, always for the installed member:
//
//  1. **The public glimt page** (task 323). A parent following a shared link has no service worker and
//     gets the real page, so the bug is invisible to everyone who would naturally test it. Only
//     installed members would have got the app shell instead of the page they clicked.
//  2. **`/api/`** (task 318, found on an iPhone 2026-09-17). Tapping *Gem* in the viewer downloaded a
//     14 kB file called `nathejk-….jpg.html`: Safari treats an `<a download>` click as a navigation,
//     the fallback answered with `index.html`, and the member got the app shell renamed as a
//     photograph. Nothing about that is visible in a desktop browser.
//
// Nothing else in the build would notice either one. The denylist is a single array, and removing an
// entry produces a service worker that works perfectly for every other route.

const VITE_CONFIG = fileURLToPath(new URL('../vite.config.ts', import.meta.url))
const BUILT_SW = fileURLToPath(new URL('../dist/sw.js', import.meta.url))

describe('the service worker navigation fallback', () => {
  function denylist(): string {
    const config = readFileSync(VITE_CONFIG, 'utf8')
    const found = config.match(/navigateFallbackDenylist:\s*\[[^\]]*\]/)?.[0]
    expect(found, 'navigateFallbackDenylist is gone — every navigation now gets the app shell')
      .toBeTruthy()
    return found ?? ''
  }

  // The public site is denied the navigation fallback by the **shape** of its prefix — a four-digit
  // first segment — rather than by this year's number (task 351), so next year's deployment needs no
  // frontend change to stay out of the way.
  it('excludes the public site under any event-year prefix', () => {
    expect(
      denylist(),
      'the year-prefixed public pages are not denied the navigation fallback, so an installed member ' +
        'following a link to a patrol page gets the app shell (the shape of bug task 332 shipped)',
    ).toContain('\\d{4}')
  })

  it('excludes the desktop placeholder', () => {
    // The original reason the array exists (task 140): without it an installed client asking for
    // /desktop.html boots the app, which redirects back — a loop.
    expect(denylist()).toContain('desktop')
  })

  // `/api/` is never a navigation destination. A browser that navigates to one — a download click, a
  // pasted URL, a form target — should get the API's own answer, bytes or a JSON 404, never the shell.
  it('excludes the API entirely', () => {
    expect(
      denylist(),
      'the API is not denied the navigation fallback, so an <a download> on a media URL will ' +
        'download index.html renamed as a photograph (task 318)',
    ).toContain('api')
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
    expect(
      sw,
      'the built service worker has no denylist entry for the year-prefixed public pages, so an ' +
        'installed member following a public link gets the app shell instead of the page',
    ).toContain('d{4}')
    expect(sw).toContain('desktop')
    expect(sw, 'the built service worker does not deny /api/ the navigation fallback').toMatch(
      /\\\/api\\\/|\/api\\\//,
    )
  })
})

// The other half of task 318's bug: the save action must not *be* a navigation in the first place.
//
// The denylist fix above stops the shell being served, but a save that navigates still routes through
// the service worker and still lands in **Files** rather than Photos on iOS. The viewer now fetches
// the bytes and hands them to `navigator.share`, so there is no navigation to intercept.
describe('the viewer saves without navigating', () => {
  const VIEWER = fileURLToPath(new URL('./components/glimt/GlimtViewer.vue', import.meta.url))

  function code(): string {
    return readFileSync(VIEWER, 'utf8')
      .replace(/<!--[\s\S]*?-->/g, '')
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/^\s*(\/\/|\*).*$/gm, '')
  }

  it('has no download anchor', () => {
    expect(
      code(),
      'the viewer is back to <a download>, which Safari treats as a navigation — that downloaded ' +
        'index.html as a .jpg.html, and even when it works it saves to Files rather than Photos',
    ).not.toMatch(/:download=|\sdownload="/)
  })

  it('goes through the Web Share API, which is what reaches Photos on iOS', () => {
    const source = code()
    expect(source).toContain('navigator.canShare')
    expect(source).toContain('navigator.share')
  })

  // The guard against the exact failure: if what came back is not media, saving it under a .jpg name
  // would put the app shell in somebody's camera roll again.
  it('refuses to save a response that is not an image or a video', () => {
    const source = code()
    expect(source).toMatch(/image\//)
    expect(source).toMatch(/video\//)
  })
})
