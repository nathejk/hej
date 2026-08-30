import { describe, expect, it } from 'vitest'

import {
  installPlatform,
  isMobileDevice,
  isStandalone,
  type PlatformEnv,
  type PlatformNavigator,
} from '@/helpers/platform'

// The environment is injected precisely so these cases can exist: an iPad claiming to be
// a Mac and a Facebook webview cannot be reproduced by whatever is running the tests.
function env(nav: Partial<PlatformNavigator>, media: Record<string, boolean> = {}): PlatformEnv {
  return {
    navigator: { userAgent: '', maxTouchPoints: 0, ...nav },
    matchMedia: (query) => ({ matches: media[query] ?? false }),
  }
}

const UA = {
  iphone:
    'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
  // iPadOS 13+ requests desktop sites: this is a *Macintosh* UA, from an iPad.
  ipadOS:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
  macSafari:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
  androidChrome:
    'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36',
  androidFirefox: 'Mozilla/5.0 (Android 14; Mobile; rv:121.0) Gecko/121.0 Firefox/121.0',
  facebookIOS:
    'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/440.0.0.32.108]',
  androidWebview:
    'Mozilla/5.0 (Linux; Android 14; Pixel 8 Build/UQ1A; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/120.0.0.0 Mobile Safari/537.36',
  windowsChrome:
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
}

describe('isMobileDevice', () => {
  it('classifies a plain Android Chrome phone as mobile', () => {
    expect(
      isMobileDevice(
        env(
          { userAgent: UA.androidChrome, maxTouchPoints: 5, userAgentData: { mobile: true } },
          { '(pointer: coarse)': true },
        ),
      ),
    ).toBe(true)
  })

  it('classifies an iPhone as mobile', () => {
    expect(
      isMobileDevice(env({ userAgent: UA.iphone, maxTouchPoints: 5 }, { '(pointer: coarse)': true })),
    ).toBe(true)
  })

  // The case the whole file exists for: iPadOS is indistinguishable from macOS Safari by
  // user agent, and a tablet that fails this check has no route into the app.
  it('classifies iPadOS reporting itself as macOS Safari as mobile', () => {
    expect(
      isMobileDevice(env({ userAgent: UA.ipadOS, platform: 'MacIntel', maxTouchPoints: 5 })),
    ).toBe(true)
  })

  it('classifies a mouse-only desktop as desktop', () => {
    expect(
      isMobileDevice(
        env(
          { userAgent: UA.windowsChrome, maxTouchPoints: 0, userAgentData: { mobile: false } },
          { '(pointer: fine)': true },
        ),
      ),
    ).toBe(false)
  })

  it('classifies a real Mac as desktop — MacIntel alone must not imply an iPad', () => {
    expect(isMobileDevice(env({ userAgent: UA.macSafari, platform: 'MacIntel', maxTouchPoints: 0 }))).toBe(
      false,
    )
  })

  // Ambiguous, and PRD 005 §11 says ambiguous resolves to mobile. The cost is one tap on
  // the escape hatch; the cost of the other answer is an iPad user locked out.
  it('classifies a touchscreen desktop as mobile (the deliberate tie-break)', () => {
    expect(
      isMobileDevice(
        env(
          { userAgent: UA.windowsChrome, maxTouchPoints: 10, userAgentData: { mobile: false } },
          { '(pointer: fine)': true, '(any-pointer: coarse)': true },
        ),
      ),
    ).toBe(true)
  })

  // Chrome reports `mobile: false` on Android tablets, so the flag can only ever be
  // trusted when it is true.
  it('does not treat userAgentData.mobile === false as decisive (Android tablet)', () => {
    expect(
      isMobileDevice(
        env(
          {
            userAgent: UA.androidChrome.replace(' Mobile', ''),
            maxTouchPoints: 5,
            userAgentData: { mobile: false },
          },
          { '(pointer: coarse)': true },
        ),
      ),
    ).toBe(true)
  })
})

describe('isStandalone', () => {
  it('is true for display-mode: standalone', () => {
    expect(isStandalone(env({}, { '(display-mode: standalone)': true }))).toBe(true)
  })

  // The iOS branch: this property exists nowhere else, and on the platform where installing
  // matters most (Web Push is home-screen-only) it is the unambiguous signal.
  it('is true for iOS navigator.standalone even when display-mode says nothing', () => {
    expect(isStandalone(env({ userAgent: UA.iphone, standalone: true }))).toBe(true)
  })

  it('is false in a browser tab', () => {
    expect(
      isStandalone(env({ userAgent: UA.androidChrome, standalone: false }, { '(display-mode: browser)': true })),
    ).toBe(false)
  })

  // REGRESSION (task 142). These two used to count as "equally installed", which made a
  // browser tab skip the install wall and land in onboarding — the exact configuration PRD 005
  // exists to prevent. The manifest only ever requests `standalone`, so neither mode can occur
  // on an installed launch; they can only occur on an uninstalled one.
  it('is false for a fullscreen video in a tab', () => {
    expect(
      isStandalone(
        env(
          { userAgent: UA.androidChrome },
          { '(display-mode: fullscreen)': true, '(display-mode: browser)': false },
        ),
      ),
    ).toBe(false)
  })

  it('is false for a browser reporting minimal-ui for its own chrome-less mode', () => {
    expect(
      isStandalone(
        env(
          { userAgent: UA.androidChrome },
          { '(display-mode: minimal-ui)': true, '(display-mode: browser)': false },
        ),
      ),
    ).toBe(false)
  })

  // The veto: only a real tab reports `browser`, so it must win over anything else the engine
  // happens to match at the same time.
  it('lets an explicit display-mode: browser override a matching standalone query', () => {
    expect(
      isStandalone(
        env({}, { '(display-mode: browser)': true, '(display-mode: standalone)': true }),
      ),
    ).toBe(false)
  })

  // ...but iOS's own signal still wins, because a home-screen app there is not in doubt.
  it('trusts navigator.standalone over a stray browser match', () => {
    expect(
      isStandalone(env({ userAgent: UA.iphone, standalone: true }, { '(display-mode: browser)': true })),
    ).toBe(true)
  })
})

describe('installPlatform', () => {
  it('detects the Facebook in-app browser', () => {
    expect(installPlatform(env({ userAgent: UA.facebookIOS, maxTouchPoints: 5 }))).toBe('webview')
  })

  it('detects an Android WebView by its wv token', () => {
    expect(installPlatform(env({ userAgent: UA.androidWebview, maxTouchPoints: 5 }))).toBe('webview')
  })

  it('detects iOS Safari', () => {
    expect(installPlatform(env({ userAgent: UA.iphone, maxTouchPoints: 5 }))).toBe('ios-safari')
  })

  it('detects iPadOS as ios-safari despite the Macintosh user agent', () => {
    expect(
      installPlatform(env({ userAgent: UA.ipadOS, platform: 'MacIntel', maxTouchPoints: 5 })),
    ).toBe('ios-safari')
  })

  it('detects Chromium from the brands list', () => {
    expect(
      installPlatform(
        env({
          userAgent: UA.androidChrome,
          maxTouchPoints: 5,
          userAgentData: { mobile: true, brands: [{ brand: 'Google Chrome' }] },
        }),
      ),
    ).toBe('chromium')
  })

  it('falls back to other for Android Firefox', () => {
    expect(installPlatform(env({ userAgent: UA.androidFirefox, maxTouchPoints: 5 }))).toBe('other')
  })
})
