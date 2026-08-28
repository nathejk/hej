// Guidance for a permission the user has already denied (PRD 003, task 101).
//
// Once a permission is `denied`, the browser will not prompt again — an "enable"
// button is a dead end that makes the app look broken. The only thing that helps is
// telling the user where the switch actually lives, which differs per platform.
//
// This lives in one module on purpose: the profile page's status rows, PRD 005's
// onboarding pre-prompts and PRD 002's "location off" state all need the same
// instructions, and three copies would drift into three different sets of steps —
// at least two of which would be wrong.
//
// Baseline per .rules is iOS/iPadOS Safari 16.4+ and Chrome 111+, so there is no
// legacy branch here.

export type Capability = 'notifications' | 'location' | 'camera'

export type Platform = 'ios' | 'android' | 'other'

// detectPlatform is the ONLY user-agent sniff in the app, and it is contained here
// so components never do their own.
//
// Note it deliberately identifies the *platform*, not the browser: on iOS every
// browser is WebKit and the Settings path is the same regardless, so branching on
// "Safari vs Chrome" would produce two identical answers and one wrong one.
//
// iPadOS reports a desktop-macOS user agent, hence the touch check — without it an
// iPad user gets the generic text.
export function detectPlatform(): Platform {
  if (typeof navigator === 'undefined') return 'other'
  const ua = navigator.userAgent
  const iOSLike =
    /iPad|iPhone|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1)
  if (iOSLike) return 'ios'
  if (/Android/.test(ua)) return 'android'
  return 'other'
}

// Danish, plain, and phrased as "where to go" rather than "you denied this".
const guidance: Record<Capability, Record<Platform, string>> = {
  notifications: {
    // Home-screen web apps get their own entry under Notifications on iOS, which is
    // why this says "Hej Nathejk" and not "Safari".
    ios: 'Åbn Indstillinger → Notifikationer → Hej Nathejk, og slå "Tillad notifikationer" til.',
    android:
      'Åbn Indstillinger → Apps → Hej Nathejk → Notifikationer, og slå notifikationer til.',
    other: 'Slå notifikationer til for hej.nathejk.dk i din browsers indstillinger for websteder.',
  },
  location: {
    ios: 'Åbn Indstillinger → Safari → Placering, og vælg "Spørg" eller "Tillad". Genstart derefter appen.',
    android:
      'Tryk på låsen i adresselinjen → Tilladelser, og slå Placering til. Tjek også, at Placering er slået til for browseren under Indstillinger → Apps.',
    other: 'Tillad adgang til din placering for hej.nathejk.dk i din browsers indstillinger.',
  },
  camera: {
    ios: 'Åbn Indstillinger → Safari → Kamera, og vælg "Spørg" eller "Tillad". Genstart derefter appen.',
    android: 'Tryk på låsen i adresselinjen → Tilladelser, og slå Kamera til.',
    other: 'Tillad adgang til kameraet for hej.nathejk.dk i din browsers indstillinger.',
  },
}

// blockedGuidance returns what to tell a user whose permission is denied.
//
// `platform` is injectable so a caller (or a future test) can ask for a specific
// platform's text without faking a user agent.
export function blockedGuidance(
  capability: Capability,
  platform: Platform = detectPlatform(),
): string {
  return guidance[capability][platform]
}
