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

import { devNavigator } from '@/helpers/platform'

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
//
// It remains the only sniff, but it is no longer the only *input*: when a dev device profile
// is active (PRD 014) it sniffs the simulated navigator from @/helpers/platform instead of the
// real one, which is what makes the iOS and Android instructions readable on a laptop. The
// simulated UA is deliberately a real string, so the branch taken here is the same branch the
// device itself would take — and there is one mapping of "what an iPad looks like", over
// there, rather than a second copy here. Inert in production builds.
export function detectPlatform(): Platform {
  const nav = devNavigator() ?? (typeof navigator === 'undefined' ? null : navigator)
  if (!nav) return 'other'
  const ua = nav.userAgent
  const iOSLike =
    /iPad|iPhone|iPod/.test(ua) || (/Macintosh/.test(ua) && (nav.maxTouchPoints ?? 0) > 1)
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

// What we tell someone *before* asking for their location (task 085, task 331).
//
// # Why this string lives here and not in the components
//
// It was a literal in three places — onboarding's location step, the map's prompt and the profile
// page's status row — and the comment in WelcomeStepLocation.vue already warned that two texts for
// the same request would drift, with the one on the less-visited screen going stale. Task 331 proved
// the point: the wording had to change in all three at once, and nothing would have failed if one had
// been missed.
//
// # Why it now mentions the public page
//
// PRD 011 publishes a patrol's route on a page that needs no login (§0b.1). We have the right to do
// that — permission was obtained — so this is not a consent gap. It is a **transparency** obligation:
// the old wording said the route went to the arrangører and stopped there, so a participant reading it
// would be surprised by the public page. Being surprised by a true thing is still a failure of the
// copy, and this is the text someone reads while deciding whether to grant location at all.
//
// Two things it is careful to say, because they are what makes the public page defensible:
//
//   - the route is published **for the patrol, not the person** — the tracks are merged and carry no
//     name, so nobody can be picked out of one;
//   - it appears **afterwards**, never while the patrol is still walking.
//
// Short enough to sit above a system dialog. The full account is the privacy page, which every caller
// links to.
export const locationConsentMessage =
  'Appen viser dig på kortet og gemmer din rute. Ruten sendes til arrangørerne, og efter løbet vises ' +
  'patruljens samlede rute på en offentlig side — uden navne. Du kan altid slå det fra igen.'
