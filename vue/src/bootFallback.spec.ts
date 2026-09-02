import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// The boot fallback (task 204).
//
// Found on an iPad mini 2 running iOS 12.5.8: the page rendered **white**. The bundle uses syntax that
// Safari cannot parse, so the module threw before anything rendered — and PRD 005's device gate, which
// would have sent that browser somewhere sensible, is itself JavaScript that never ran. A blank page is the
// worst possible way to tell someone their device is unsupported: it looks like the event's app is broken.
//
// The fix is static markup inside `#app`, which is on screen before the bundle is fetched and survives a
// bundle that cannot run. Vue clears the container on mount, so a working device never keeps it.
//
// These tests guard the two ways it silently stops working: someone "tidying" `index.html` back to an empty
// `<div id="app">`, and someone moving the styling into the app's stylesheet, which the browsers this exists
// for cannot parse either.

const INDEX = fileURLToPath(new URL('../index.html', import.meta.url))

function html(): string {
  return readFileSync(INDEX, 'utf8')
}

describe('the boot fallback', () => {
  // An empty #app is the state that produced a white screen on a real device.
  it('puts content inside #app rather than leaving it empty', () => {
    expect(html()).not.toMatch(/<div id="app"><\/div>/)
    expect(html()).toMatch(/id="boot-fallback"/)
  })

  it('tells the user they can still take part', () => {
    // The thing a participant or parent actually needs to know, and it comes first in the copy.
    expect(html()).toMatch(/stadig være med i løbet/i)
  })

  // Naming the baseline is what makes the message actionable — "unsupported" alone leaves someone guessing
  // whether a different phone would help.
  it('names the baseline it needs', () => {
    expect(html()).toMatch(/16\.4/)
    expect(html()).toMatch(/Chrome 111/)
  })

  // /desktop.html is a plain static file outside the SPA (task 140), so it works on exactly the browsers
  // this fallback is for. Linking anywhere inside the app would be a link to another white page.
  it('links somewhere that works without the app', () => {
    expect(html()).toMatch(/href="\/desktop\.html"/)
  })

  it('says something when JavaScript is switched off entirely', () => {
    expect(html()).toMatch(/<noscript>/)
  })

  describe('its styling', () => {
    // The app's stylesheet is Tailwind v4, which emits `oklch()` and modern at-rules that an old Safari
    // cannot parse. A fallback for browsers that cannot run the app must not depend on CSS they cannot read.
    it('is inline, and free of colour syntax an old browser cannot parse', () => {
      const source = html()
      const style = source.slice(source.indexOf('#boot-fallback'), source.indexOf('</style>'))

      expect(style.length).toBeGreaterThan(0)
      expect(style).not.toMatch(/oklch|color-mix|@layer|light-dark/)
      // Hex colours, which every browser understands.
      expect(style).toMatch(/#[0-9a-f]{6}/i)
    })

    // Without a delay, a slow-but-supported phone would flash "your device is too old" before booting —
    // a worse lie than saying nothing. Vue clears the container within a few hundred ms on a working device.
    it('reveals itself only after a delay', () => {
      expect(html()).toMatch(/animation:\s*boot-fallback-in[^;]*\b2s\b/)
      expect(html()).toMatch(/opacity:\s*0/)
    })
  })
})
