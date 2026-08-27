<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LogOut } from '@lucide/vue'
import { useSessionStore } from '@/stores/session.store'
import { useAppStore } from '@/stores/app.store'
import { useLocationStore } from '@/stores/location.store'
import { useTrackStore } from '@/stores/track.store'
import BottomNav from '@/components/BottomNav.vue'
import UpdatePrompt from '@/components/UpdatePrompt.vue'
import OfflineNotice from '@/components/OfflineNotice.vue'
import { APP_NAME } from '@/config/brand'

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
onMounted(() => {
  window.addEventListener('online', handleOnline)
  window.addEventListener('offline', handleOffline)

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

  <div v-if="showShell" class="flex h-full flex-col">
    <header
      v-if="!fullBleed"
      class="flex items-center justify-between border-b border-slate-200 bg-white px-4 pb-3"
      style="padding-top: calc(env(safe-area-inset-top) + 0.75rem)"
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
         full-bleed route there is no header above it, so it carries the top safe-area
         inset itself. -->
    <div v-if="fullBleed" style="padding-top: env(safe-area-inset-top)">
      <OfflineNotice />
    </div>
    <OfflineNotice v-else />

    <main :class="fullBleed ? 'relative min-h-0 flex-1' : 'min-h-0 flex-1 overflow-y-auto'">
      <RouterView />
    </main>

    <BottomNav />
  </div>

  <RouterView v-else />
</template>
