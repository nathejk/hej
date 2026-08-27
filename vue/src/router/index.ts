import { createRouter, createWebHistory } from 'vue-router'
import type { Component } from 'vue'
import type { RouteRecordRaw } from 'vue-router'
import { useSessionStore } from '@/stores/session.store'
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
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { public: true },
    },
    ...destinationRoutes,
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

// Global gate: the app is unusable until signed in. Unauthenticated users are
// sent to /login; signed-in users are kept out of /login; role-scoped routes
// redirect disallowed roles home. Client-side gating is UX only — the BFF
// authorizes every protected endpoint independently.
//
// NOTHING IN HERE MAY REJECT (task 090). A rejected guard aborts the navigation, so
// no route component mounts and the user is left looking at a blank white screen —
// which is what happened offline, when `ensureReady()` propagated the failure of
// GET /api/me. `ensureReady()` is now explicitly non-throwing; keep it that way, and
// do not add an `await` here that can reject.
router.beforeEach(async (to) => {
  const session = useSessionStore()
  await session.ensureReady()

  if (!to.meta.public && !session.isAuthenticated) {
    return { name: 'login' }
  }
  if (to.name === 'login' && session.isAuthenticated) {
    return { name: 'maps' }
  }
  if (to.meta.roles && session.role && !to.meta.roles.includes(session.role)) {
    return { name: 'maps' }
  }
  return true
})

export default router
