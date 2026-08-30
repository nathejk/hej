import type { RouteLocationNormalized, RouteLocationRaw } from 'vue-router'

import { isMobileDevice, isStandalone } from '@/helpers/platform'
import { useInstallStore } from '@/stores/install.store'
import { useOnboardingStore } from '@/stores/onboarding.store'

// Steps 2-4 of the router guard, in their own module.
//
// Separated from `router/index.ts` so they can be tested without it: that module calls
// `createWebHistory()` at import time, which needs a `window` — so importing it into a unit
// test drags in a DOM the gates themselves have no use for. The gates are pure decisions over
// two stores and two predicates, and this is what keeps them testable as such.

// The desktop placeholder is a **static page outside this app** (task 140):
// `public/desktop.html`, no Vue, no bundle. So leaving for it is a full-page navigation, not
// a route change — routing to it inside the SPA is precisely what that task removed.
//
// The URL is the file's own path rather than a tidier `/desktop`, and that is load-bearing:
// a path the SPA fallback would answer with `index.html` would boot the app, which would
// redirect here again — a loop. A real file cannot be caught by the fallback. The service
// worker's navigation fallback excludes it for the same reason (see vite.config.ts).
export const DESKTOP_PAGE = '/desktop.html'

/**
 * "Leave the SPA for the placeholder page." Distinct from a redirect, because it is not a
 * route: the guard turns it into a full-page navigation.
 */
export const LEAVE_APP = Symbol('leave-app')

// Steps 2–4 of the guard order, factored out so the guard body reads as the six numbered
// steps rather than as one long chain of returns.
//
// Returns `true` to fall through to auth, `LEAVE_APP` when the browser should be sent out of
// the app entirely, or a redirect target.
//
// It returns that decision rather than performing it: a function that calls
// `window.location.replace` cannot be tested without a DOM, and the desktop branch is one of
// the two the tests most need to cover. Acting on it is the guard's job.
//
// **Every path must terminate.** These redirects compose with the auth redirect below, and a
// pair of rules that each bounce the other's destination is an infinite navigation — which
// vue-router aborts, leaving a page that never finishes loading and nothing rendered. That is
// not a hypothetical: see `router.spec.ts`, which drives every state combination to a fixpoint
// precisely because reading this function is not enough to be sure.
//
// Exported for that test. Not used anywhere else.
//
// Wrapped in try/catch by the caller: these read `navigator`/`matchMedia`, and the one thing
// this guard must never do is throw (task 090).
export function deviceAndInstallGates(
  to: RouteLocationNormalized,
): true | typeof LEAVE_APP | RouteLocationRaw {
  const install = useInstallStore()
  const onboarding = useOnboardingStore()

  // 2. Device class. A desktop computer is not an app user: it gets the plain placeholder
  //    page, which is not part of this application. In particular it never sees /install or
  //    /welcome — showing a laptop how to add a phone app to its home screen is worse than
  //    the placeholder.
  if (!isMobileDevice()) {
    return LEAVE_APP
  }

  // 3. Standalone. The app is not usable in a browser tab, so every route leads to the wall
  //    until it is installed — unless the user took the escape hatch, which unblocks *this*
  //    gate only (task 121).
  if (!isStandalone() && !install.continueInBrowser) {
    return to.name === 'install' ? true : { name: 'install' }
  }

  // Installed (or overridden): the wall has nothing left to say.
  if (to.name === 'install') {
    return { name: onboarding.complete ? 'maps' : 'welcome' }
  }

  // 4. Onboarding. Not "done" merely because the user is signed in — login is only its first
  //    step — so this asks the onboarding store, not the session store.
  if (!onboarding.complete && to.name !== 'welcome') {
    return { name: 'welcome' }
  }

  // Deliberately NO rule sending a finished user *away* from /welcome.
  //
  // There was one — `complete && to.name === 'welcome' → maps` — and it was an infinite
  // redirect: an onboarded device whose session had expired went `maps → (auth) → welcome →
  // (this rule) → maps → …`, so the app never rendered and the page simply never finished
  // loading. A 7-day session and a per-device completion flag make that state ordinary rather
  // than exotic: complete the flow once, come back next week, and the app is bricked.
  //
  // Leaving /welcome is `WelcomeView`'s job instead, and it is the right owner: it redirects
  // when there is no unsettled step left **and** the user is authenticated, so it cannot fire
  // for the case that caused the loop. The gate should not need to know about the session,
  // which is the whole reason it runs before auth (§11).

  return true
}
