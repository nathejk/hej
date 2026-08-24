<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LogOut } from '@lucide/vue'
import { useSessionStore } from '@/stores/session.store'
import BottomNav from '@/components/BottomNav.vue'
import UpdatePrompt from '@/components/UpdatePrompt.vue'
import { APP_NAME } from '@/config/brand'

const session = useSessionStore()
const route = useRoute()
const router = useRouter()

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

    <main :class="fullBleed ? 'relative min-h-0 flex-1' : 'min-h-0 flex-1 overflow-y-auto'">
      <RouterView />
    </main>

    <BottomNav />
  </div>

  <RouterView v-else />
</template>
