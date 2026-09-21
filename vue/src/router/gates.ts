import type { RouteLocationNormalized, RouteLocationRaw } from 'vue-router'

import { isMobileDevice, isStandalone } from '@/helpers/platform'
import type { Role } from '@/config/roles'
import { useOnboardingStore } from '@/stores/onboarding.store'

// Steps 2-4 of the router guard, in their own module.
//
// Separated from `router/index.ts` so they can be tested without it: that module calls
// `createWebHistory()` at import time, which needs a `window` — so importing it into a unit
// test drags in a DOM the gates themselves have no use for. The gates are pure decisions over
// two stores and two predicates, and this is what keeps them testable as such.

// The public website is **outside this app** (tasks 140/143, PRD 011): server-rendered by the BFF,
// no Vue, no bundle, and **anonymous — there is no login on it**. So leaving for it is a full-page
// navigation, not a route change; routing to it inside the SPA is precisely what task 140 removed.
//
// # Why the year is computed here (task 351)
//
// The public site lives under the **event year** (`/2026`), and this bundle has no way to be told which
// year the server is serving: that value is runtime configuration on the BFF, and this decision is taken
// during the router's first navigation, before anything has been fetched. Waiting for `/api/config`
// would make the one navigation a desktop visitor gets depend on a request that can fail — and would
// reopen the boot-ordering race `main.ts` documents at length.
//
// So the year is derived from the calendar, which is **the same default the server uses**
// (`currentYear()` in env.go). The two agree unless somebody overrides `EVENT_YEAR` to serve a past
// event, and the degradation in that case is bounded and self-correcting: the visitor gets the public
// site's own "not found" page, which links to the frontpage of the year actually being served. A wrong
// guess cannot loop back into the app, because every year-shaped path is answered by the public site.
//
// An earlier version sent visitors to `/desktop.html`, a placeholder reading "more to come…", and then
// briefly to `/offentligt`, the site's first address. Both are gone.
export const WEBSITE_PAGE = `/${new Date().getFullYear()}`

/**
 * "Leave the SPA for the website." Distinct from a redirect, because it is not a route: the
 * guard turns it into a full-page navigation.
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
  const onboarding = useOnboardingStore()

  // 2. Device class. A desktop computer is not an app user: it gets the website, which is not
  //    part of this application. In particular it never sees /install or /welcome — showing a
  //    laptop how to add a phone app to its home screen is worse than the website is.
  if (!isMobileDevice()) {
    return LEAVE_APP
  }

  // 3. Standalone. **There is no login outside the installed app** (task 143), and every page of
  //    this app is install-only — so a browser tab asking for one gets the install instructions.
  //    No override, no exceptions.
  //
  //    **Except the front door** (task 356). The public website is for every device, and `/` is the
  //    address people type and share; answering it with an add-to-home-screen wall pushes the app
  //    at a visitor who only wanted to read the site. So a browser that arrived at the root leaves
  //    for the website, exactly as a desktop does, and the wall is reserved for someone who asked
  //    for an app page by name — or who tapped the website's own install invitation.
  //
  //    `/` is a redirect to `maps` in the route table, so by the time the guard runs the root is
  //    only visible as `redirectedFrom`. Checking both keeps it correct if that ever becomes a
  //    component route instead. Note this cannot move to the server: the root is also the installed
  //    app's `start_url`, and no request tells the BFF whether it came from a home screen.
  if (!isStandalone()) {
    if (to.path === '/' || to.redirectedFrom?.path === '/') {
      return LEAVE_APP
    }
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

/**
 * Step 6 of the guard order: role gating.
 *
 * Returns `true` when the route may be entered, or a redirect target when it may not.
 *
 * Extracted from the guard body for the same reason as the gates above — so it can be tested
 * without importing `router/index.ts`, which calls `createWebHistory()` at import time — and
 * because PRD 007 made this gate load-bearing rather than cosmetic: it is what refuses
 * `/contacts` to a spejder who types the URL, follows a stale link, or restores a tab.
 *
 * # This is not the security boundary
 *
 * The BFF authorizes every contacts endpoint independently and answers 403 for a spejder. This
 * gate exists so the app does not render a pane it cannot fill; a reader must not conclude
 * that data is protected because this returns a redirect. Same rule as the install gate above.
 *
 * # An unknown role falls through
 *
 * When `role` is null the route is allowed, matching the previous inline behaviour. That is
 * deliberate rather than an oversight: `role` is null while the session is still resolving and
 * on a cold offline start, and redirecting then would bounce a legitimate user off a page they
 * are entitled to — the blank-screen class of bug task 090 was about. The endpoint still
 * refuses, so the cost of falling through is an empty pane, not a disclosure.
 */
export function roleGate(
  to: RouteLocationNormalized,
  role: Role | null,
): true | RouteLocationRaw {
  if (!to.meta.roles) return true
  if (role === null) return true
  if (to.meta.roles.includes(role)) return true

  // Somewhere sensible, never an error page: the map is every role's landing route.
  return { name: 'maps' }
}

/**
 * The moderation gate: `meta.moderator` routes need the Team-section assignment (PRD 019, task 309).
 *
 * A separate gate rather than a `Role`, because moderation is **not** a role: it comes from the
 * member's `sectionSlug`, and every Team member is `crew` as far as roles are concerned — as are the
 * kitchen and PR. Expressing it as `meta.roles: ['crew']` would hand the queue to every unclassified
 * crew account in the event.
 *
 * # Unlike `roleGate`, an unknown answer refuses
 *
 * `roleGate` lets a null role through, because bouncing a legitimate user off a page while the session
 * resolves is the blank-screen class of bug. The trade is inverted here: `moderatesGlimt` is false
 * both when the caller does not moderate and when we could not ask (an offline cold start, where the
 * remembered identity carries no such flag). Falling *through* would then render a queue that cannot
 * load — the view needs the network by definition — so refusing costs a moderator one reconnect and
 * saves everyone else a screen they should not see.
 *
 * Not the security boundary, and nothing here protects data: all three moderation endpoints re-read
 * the assignment per request, so a caller who reaches this route by any means still gets 403s. Same
 * rule as the gates above — do not let a later change lean on it.
 */
export function moderatorGate(
  to: RouteLocationNormalized,
  moderates: boolean,
): true | RouteLocationRaw {
  if (!to.meta.moderator) return true
  if (moderates) return true
  // The feed, not the map: somebody following a stale moderation link was on their way to Glimt.
  return { name: 'glimt' }
}
