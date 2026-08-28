<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LogOut } from '@lucide/vue'
import { useSessionStore } from '@/stores/session.store'
import { useAppStore } from '@/stores/app.store'
import { useLocationStore } from '@/stores/location.store'
import { useTrackStore } from '@/stores/track.store'
import { logEvent } from '@/helpers/trackDb'
import BottomNav from '@/components/BottomNav.vue'
import UpdatePrompt from '@/components/UpdatePrompt.vue'
import OfflineNotice from '@/components/OfflineNotice.vue'
import LayoutDebug from '@/components/LayoutDebug.vue'
import { APP_NAME } from '@/config/brand'
import { showLayoutDebug } from '@/config/runtime'

const session = useSessionStore()
const app = useAppStore()
const location = useLocationStore()
const track = useTrackStore()
const route = useRoute()
const router = useRouter()

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
  if (!document.hidden) void track.sample()
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

// The app shell (top bar + bottom nav) frames authenticated pages. The login
// screen renders bare.
const showShell = computed(() => session.isAuthenticated && route.name !== 'login')

// Full-bleed routes (the map) get everything above the bottom nav: no top bar,
// no scroll container. Sign-out stays reachable from every other page — and, once
// PRD 003 lands, from the profile page.
const fullBleed = computed(() => route.meta.fullBleed === true)

async function signOut() {
  await session.logout()
  await router.replace({ name: 'login' })
}
</script>

<template>
  <UpdatePrompt />
  <!-- Diagnostic only, and teleported+fixed so it never affects the layout it reports
       on. Rendered outside the shell so it is present on the login screen too. -->
  <LayoutDebug v-if="showLayoutDebug" />

  <!-- `.app-shell` is a fixed, out-of-flow layer sized to the physical screen rather than
       the reported viewport; see main.css for why that is not `h-full`. -->
  <div v-if="showShell" class="app-shell flex flex-col">
    <header
      v-if="!fullBleed"
      class="flex items-center justify-between border-b border-slate-200 bg-white px-4 pb-3"
      style="padding-top: calc(var(--sat) + 0.75rem)"
    >
      <span class="font-nathejk text-lg tracking-wide">{{ APP_NAME }}</span>
      <button
        type="button"
        class="flex items-center gap-1 text-sm text-slate-500"
        @click="signOut"
      >
        <LogOut class="h-4 w-4" aria-hidden="true" />
        Log ud
      </button>
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
