// Central brand values, referenced by the UI and (via task 014) the PWA
// manifest so name/colours live in one place and are easy to change.
export const APP_NAME = 'Hej Nathejk'
export const APP_SHORT_NAME = 'Hej Nathejk'
export const APP_DESCRIPTION = 'Nathejk in-event companion — maps, contacts, rulebook and updates.'

// Colours also feed the PWA manifest + the login/splash surfaces.
export const THEME_COLOR = '#0f172a' // slate-900

// The launch ('boot') screen colour. Deliberately the same as THEME_COLOR, not
// white: the platform synthesises the splash from this plus the app icon, so a
// white value flashes bright, then cuts to the dark app bar on every cold start
// — and does it in a participant's face at night, which is when this app is
// actually used.
export const BACKGROUND_COLOR = '#0f172a'

// The moon's yellow, from the official logo artwork. See
// @/assets/brand/nathejk-moon.svg for how it was derived and why it is an
// estimate of the source CMYK.
export const BRAND_YELLOW = '#E6EA08'
