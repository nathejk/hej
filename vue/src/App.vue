<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session.store'
import { useAppStore } from '@/stores/app.store'
import { useLocationStore } from '@/stores/location.store'
import { useTrackStore } from '@/stores/track.store'
import { useOnboardingStore } from '@/stores/onboarding.store'
import { logEvent } from '@/helpers/trackDb'
import BottomNav from '@/components/BottomNav.vue'
import UpdatePrompt from '@/components/UpdatePrompt.vue'
import OfflineNotice from '@/components/OfflineNotice.vue'
import UserMenu from '@/components/UserMenu.vue'
import LayoutDebug from '@/components/LayoutDebug.vue'
import { APP_NAME } from '@/config/brand'
import { showLayoutDebug } from '@/config/runtime'

const session = useSessionStore()
const app = useAppStore()
const location = useLocationStore()
const track = useTrackStore()
const onboarding = useOnboardingStore()
const route = useRoute()

// Connectivity (task 090). The browser events are a hint, not the truth —
// navigator.onLine is true on a captive portal and with one unusable bar — so
// `offline` is only ever optimistic here: going online triggers a real request, and
// if that fails session.store puts the flag back.
function handleOffline() {
  app.setOnline(false)
}
async function handleOnline() {
  app.setOnline(true)
  // Confirm a provisional identity, or discover the session actually expired while
  // we were away.
  if (session.provisional) await session.refresh()
  // Signal is back: ship the backlog now rather than at the next interval tick.
  void track.flush()
}
// Records when the document is suspended and resumed. This is the measurement task 082
// actually needs from a real phone: a web app cannot record while backgrounded, and the
// pair of hidden/visible timestamps either side of a gap in the points is the evidence of
// what that costs in practice. Written to IndexedDB rather than kept in memory because
// iOS may kill the app in between, taking any in-memory log with it.
function onVisibility() {
  void logEvent(document.hidden ? 'hidden' : 'visible')
  // Coming back from a suspend, the interval may have been throttled or the app
  // restarted; sample promptly rather than waiting out the rest of the period. sample()
  // refuses if a point was recorded within the last interval, so this cannot oversample.
  if (!document.hidden) {
    void track.sample()
    // And ship whatever accumulated. On iOS the app does not run while backgrounded, so
    // being foregrounded is the only moment a backlog can move (task 082's measurement) —
    // waiting out a fresh 2-minute interval would waste the window the user just gave us.
    void track.flush()
  }
}

onMounted(() => {
  window.addEventListener('online', handleOnline)
  window.addEventListener('offline', handleOffline)
  document.addEventListener('visibilitychange', onVisibility)
  void logEvent('load', document.visibilityState)

  // Position track (task 082). Deliberately started HERE and not in MapsView: that
  // view stops its geolocation watch on `document.hidden` and on unmount, which is
  // right for a map marker and wrong for a recorder — the track would stop the moment
  // the user navigated away from the map. At app level it records whenever the app is
  // open at all, which is the most coverage a web app can get (it cannot record while
  // backgrounded on any platform).
  void location.syncPermission().then(() => void track.start())
})
onUnmounted(() => {
  window.removeEventListener('online', handleOnline)
  window.removeEventListener('offline', handleOffline)
  document.removeEventListener('visibilitychange', onVisibility)
  track.stop()
  track.stopUploading()
})

// React to permission being granted or revoked later, and to signing in or out.
// `start()` is safe to call repeatedly and refuses unless there is both a signed-in
// person and a granted permission; `stop()` is what a revocation must reach.
watch(
  () => [location.permission, session.user?.userId] as const,
  ([permission, userId]) => {
    if (permission === 'granted' && userId) void track.start()
    else track.stop()
  },
)

// The uploader follows the SESSION, not the permission (task 083). Points can be pending
// with recording stopped — permission revoked, storage full, or a previous session that
// recorded and never found signal — and in every one of those cases the backlog still has to
// ship. Tying it to the recorder would strand exactly the data that is hardest to reproduce.
watch(
  () => session.user?.userId,
  (userId) => {
    if (userId) track.startUploading()
    else track.stopUploading()
  },
  { immediate: true },
)

// The app shell (top bar + bottom nav) frames authenticated pages. Onboarding, the install
// wall and the desktop placeholder render bare.
//
// Spelled out as three conditions because the old form was
// `isAuthenticated && route.name !== 'login'`, and the `login` term died with that route
// (task 126): comparing against a route name that no longer exists is always true, which
// would have put the top bar and bottom nav on top of `/welcome`.
//
// `onboardingComplete` is separate from `isAuthenticated` on purpose — login is only the
// *first* step of onboarding, so a signed-in user part-way through the flow must still see
// no chrome (PRD 005 §7).
const showShell = computed(
  () => session.isAuthenticated && onboarding.complete && !route.meta.public,
)

// Full-bleed routes (the map) get everything above the bottom nav: no top bar,
// no scroll container. The top bar's trailing user menu (PRD 003: profile +
// sign-out) is therefore absent here; the bottom nav is the way out.
const fullBleed = computed(() => route.meta.fullBleed === true)

// The update banner is fixed to the top of the viewport at z-60, so on the install wall it
// would sit directly over the explanation and the install button — the one screen where the
// user has exactly one thing to do and cannot yet do anything else (PRD 005 §8). Suppressed
// there rather than restyled: an update is not urgent for a tab whose only purpose is to get
// the app onto the home screen, and the new build will be picked up the moment they open it
// from there.
//
// Left visible everywhere else, including onboarding, where reloading is harmless.
const showUpdatePrompt = computed(() => route.name !== 'install')
</script>

<template>
  <UpdatePrompt v-if="showUpdatePrompt" />
  <!-- Diagnostic only, and teleported+fixed so it never affects the layout it reports
       on. Rendered outside the shell so it is present on the onboarding routes too. -->
  <LayoutDebug v-if="showLayoutDebug" />

  <div v-if="showShell" class="flex h-full flex-col">
    <header
      v-if="!fullBleed"
      class="flex items-center justify-between border-b border-slate-200 bg-white px-4 pb-3"
      style="padding-top: calc(var(--sat) + 0.75rem)"
    >
      <span class="font-nathejk text-lg tracking-wide">{{ APP_NAME }}</span>
      <!-- Profile + sign-out (PRD 003). Owns signOut() — there is exactly one
           sign-out action in the app. -->
      <UserMenu />
    </header>

    <!-- In flow, so it never covers the map or collides with UpdatePrompt. On a
         full-bleed route there is no header above it to clear the status bar, so it
         carries the top safe-area inset itself — inside its own `v-if`, so nothing is
         reserved while online. -->
    <OfflineNotice :inset-top="fullBleed" />

    <!-- `overscroll-behavior: contain` on the scrolling variant: without it a swipe that
         reaches the end of this container (or starts on a page shorter than the viewport)
         chains to the document and drags the whole shell. -->
    <main
      :class="
        fullBleed
          ? 'relative min-h-0 flex-1'
          : 'min-h-0 flex-1 overflow-y-auto [overscroll-behavior:contain]'
      "
    >
      <RouterView />
    </main>

    <BottomNav />
  </div>

  <RouterView v-else />
</template>
