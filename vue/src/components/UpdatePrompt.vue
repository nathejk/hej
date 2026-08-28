<script setup lang="ts">
import { ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { RefreshCw } from '@lucide/vue'
import { useAppStore } from '@/stores/app.store'
import { applyUpdate } from '@/helpers/pwa'

// Non-blocking "new version available" banner. The service worker registration
// (@/helpers/pwa) flips app.store.updateAvailable via onNeedRefresh; reloading
// activates the waiting SW and loads the new build.
const app = useAppStore()
const { updateAvailable } = storeToRefs(app)

const dismissed = ref(false)
const busy = ref(false)

// Re-show if a newer version is announced after a dismissal.
watch(updateAvailable, (available) => {
  if (available) dismissed.value = false
})

async function reload() {
  busy.value = true
  await applyUpdate()
}
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="updateAvailable && !dismissed"
        class="fixed inset-x-0 top-0 z-[60] m-2 flex items-center justify-between gap-3 rounded-xl bg-slate-900 px-4 py-3 text-sm text-white shadow-lg"
        style="margin-top: calc(var(--sat) + 0.5rem)"
        role="status"
      >
        <span>En ny version er tilgængelig.</span>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="flex items-center gap-1 rounded-lg bg-white px-3 py-1.5 font-medium text-slate-900 disabled:opacity-50"
            :disabled="busy"
            @click="reload"
          >
            <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': busy }" aria-hidden="true" />
            Genindlæs
          </button>
          <button type="button" class="px-2 py-1.5 text-slate-300" @click="dismissed = true">
            Senere
          </button>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
