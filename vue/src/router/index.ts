import { createRouter, createWebHistory } from 'vue-router'
import type { Component } from 'vue'
import type { RouteLocationRaw, RouteRecordRaw } from 'vue-router'
import { useSessionStore } from '@/stores/session.store'
import { gatesEnabled } from '@/config/gates'
import { DESKTOP_PAGE, LEAVE_APP, deviceAndInstallGates } from '@/router/gates'
import type { Role } from '@/stores/session.store'
import { destinations } from '@/config/navigation'

// Route meta used by the global guard.
declare module 'vue-router' {
  interface RouteMeta {
    // public routes (e.g. login) skip the auth check.
    public?: boolean
    // when set, only these roles may enter the route.
    roles?: Role[]
    // render edge-to-edge: no top bar, no scroll wrapper (see App.vue).
    fullBleed?: boolean
  }
}

// Lazy view loaders keyed by destination name (single source: navigation.ts
// owns the label/icon/roles; the router owns the component + path).
const viewLoaders: Record<string, () => Promise<Component>> = {
  maps: () => import('@/views/MapsView.vue'),
  contacts: () => import('@/views/ContactsView.vue'),
  rulebook: () => import('@/views/RulebookView.vue'),
  updates: () => import('@/views/UpdatesView.vue'),
  schedule: () => import('@/views/ScheduleView.vue'),
  sos: () => import('@/views/SosView.vue'),
  faq: () => import('@/views/FaqView.vue'),
  privacy: () => import('@/views/PrivacyView.vue'),
}

const destinationRoutes: RouteRecordRaw[] = destinations.map((d) => ({
  path: d.path,
  name: d.name,
  component: viewLoaders[d.name],
  meta: { roles: d.roles, fullBleed: d.fullBleed },
}))

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    // Landing → first destination.
    { path: '/', redirect: { name: 'maps' } },
    // Onboarding (PRD 005): login lives inside this flow, so the route is public — an
    // The install wall (PRD 005). Public: it is what an unauthenticated mobile visitor
    // sees before any login form exists for them. The gate that redirects here is the
    // router guard (task 137); this entry only makes the route reachable.
    {
      path: '/install',
      name: 'install',
      component: () => import('@/views/InstallView.vue'),
      meta: { public: true },
    },
    // Onboarding (PRD 005): login lives inside this flow, so the route is public — an
    // unauthenticated user must be able to reach it. `fullBleed` is deliberately absent:
    // see the comment in WelcomeView.vue.
    {
      path: '/welcome',
      name: 'welcome',
      component: () => import('@/views/WelcomeView.vue'),
      meta: { public: true },
    },
    ...destinationRoutes,
    // Min profil (PRD 003). Deliberately NOT a `destination`: it is reached from the
    // user menu in the top bar, so it takes no bottom-nav slot — which matters,
    // because the service roles are already close to the five visible slots.
    // Open to every role, hence no `meta.roles`.
    {
      path: '/profil',
      name: 'profile',
      component: () => import('@/views/ProfileView.vue'),
    },
    // Diagnostic page for the position track (task 082). Deliberately NOT a
    // `destination`: it is a measurement tool for the device tests, not a feature, so it
    // stays out of the nav and is reached from the privacy page. If the recorder turns
    // out to need watching during the event, promoting it is one line.
    {
      path: '/sporing',
      name: 'track-status',
      component: () => import('@/views/TrackStatusView.vue'),
    },
  ],
})

// Global gate (PRD 005 §6/§8). Order is fixed, and the order is the design:
//
//   1. dev override / runtime flag — nothing else can be debugged if this is not first
//   2. device class      — phone/tablet vs desktop computer
//   3. standalone        — installed vs a browser tab
//   4. onboarding        — the flow at /welcome
//   5. auth
//   6. roles
//
// **The device gate runs before `session.ensureReady()`,** which it can only do because it
// is session-independent by design: PRD 005 §11 decided there is no desktop login for any
// role, so the gate needs to know nothing about who is asking. That saves every desktop
// visitor a pointless /api/me round-trip.
//
// The consequence is worth naming rather than discovering: because the gate precedes auth,
// **there is no role-based bypass** if an organizer needs laptop access mid-event. The
// dev/QA override would be the only way in and is not intended for that. If organizer
// desktop access is ever wanted, it is a new PRD revisiting this gate — not a flag added
// here.
//
// **The install gate is UX, never security.** Nothing in this guard protects data: the BFF
// authorizes every protected endpoint independently, and a user who reaches an app route by
// any means still gets nothing they are not entitled to. Do not let a later change lean on
// it.
//
// NOTHING IN HERE MAY REJECT (task 090). A rejected guard aborts the navigation, so
// no route component mounts and the user is left looking at a blank white screen —
// which is what happened offline, when `ensureReady()` propagated the failure of
// GET /api/me. `ensureReady()` is now explicitly non-throwing; keep it that way, and
// do not add an `await` here that can reject. The gates added above it are synchronous and
// dependency-free, which is also what keeps them from adding a redirect flash on a cold
// start.
router.beforeEach(async (to) => {
  if (gatesEnabled()) {
    let outcome: true | typeof LEAVE_APP | RouteLocationRaw = true
    try {
      outcome = deviceAndInstallGates(to)
    } catch {
      // A missing `navigator`/`matchMedia`, or anything else unexpected, must not abort the
      // navigation — that is a blank white screen (task 090). Falling through means a
      // degraded gate, which is the right way to fail for a gate that is UX only.
      outcome = true
    }
    if (outcome === LEAVE_APP) {
      // A desktop visitor: out of the app entirely, to a static file that is not part of it.
      window.location.replace(DESKTOP_PAGE)
      // Aborts the in-app navigation so nothing renders in the moment before the browser
      // leaves. Safe here, unlike the blank-screen case of task 090: the page is on its way
      // out, and the destination is a file rather than something that has to boot.
      return false
    }
    if (outcome !== true) return outcome
  }

  // 5. Auth.
  const session = useSessionStore()
  await session.ensureReady()

  if (!to.meta.public && !session.isAuthenticated) {
    return { name: 'welcome' }
  }

  // 6. Roles.
  if (to.meta.roles && session.role && !to.meta.roles.includes(session.role)) {
    return { name: 'maps' }
  }
  return true
})

export default router
