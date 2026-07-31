import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import { useSessionStore } from '@/stores/session.store'
import type { Role } from '@/stores/session.store'

// Route meta used by the global guard.
declare module 'vue-router' {
  interface RouteMeta {
    // public routes (e.g. login) skip the auth check.
    public?: boolean
    // when set, only these roles may enter the route.
    roles?: Role[]
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { public: true },
    },
  ],
})

// Global gate: the app is unusable until signed in. Unauthenticated users are
// sent to /login; signed-in users are kept out of /login; role-scoped routes
// redirect disallowed roles home. Client-side gating is UX only — the BFF
// authorizes every protected endpoint independently.
router.beforeEach(async (to) => {
  const session = useSessionStore()
  await session.ensureReady()

  if (!to.meta.public && !session.isAuthenticated) {
    return { name: 'login' }
  }
  if (to.name === 'login' && session.isAuthenticated) {
    return { name: 'home' }
  }
  if (to.meta.roles && session.role && !to.meta.roles.includes(session.role)) {
    return { name: 'home' }
  }
  return true
})

export default router
