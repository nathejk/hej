<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LogOut } from 'lucide-vue-next'
import { useSessionStore } from '@/stores/session.store'
import BottomNav from '@/components/BottomNav.vue'

const session = useSessionStore()
const route = useRoute()
const router = useRouter()

// The app shell (top bar + bottom nav) frames authenticated pages. The login
// screen renders bare.
const showShell = computed(() => session.isAuthenticated && route.name !== 'login')

async function signOut() {
  await session.logout()
  await router.replace({ name: 'login' })
}
</script>

<template>
  <div v-if="showShell" class="flex h-full flex-col">
    <header
      class="flex items-center justify-between border-b border-slate-200 bg-white px-4 pb-3"
      style="padding-top: calc(env(safe-area-inset-top) + 0.75rem)"
    >
      <span class="font-bold">Hej Nathejk</span>
      <button
        type="button"
        class="flex items-center gap-1 text-sm text-slate-500"
        @click="signOut"
      >
        <LogOut class="h-4 w-4" aria-hidden="true" />
        Log ud
      </button>
    </header>

    <main class="min-h-0 flex-1 overflow-y-auto">
      <RouterView />
    </main>

    <BottomNav />
  </div>

  <RouterView v-else />
</template>
