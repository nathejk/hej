<script setup lang="ts">
import { computed, ref } from 'vue'
import { Menu } from 'lucide-vue-next'
import { useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session.store'
import { visibleDestinations } from '@/config/navigation'
import MoreMenu from '@/components/MoreMenu.vue'

// Bottom navigation, filtered by the signed-in role. At most MAX_SLOTS slots:
// when the role sees more than that, the last slot becomes a "More" (burger)
// entry that reveals the remaining destinations. Task 012 turns the overflow
// list into a polished bottom sheet.
const MAX_SLOTS = 5

const session = useSessionStore()
const route = useRoute()

const all = computed(() => visibleDestinations(session.role))
const hasOverflow = computed(() => all.value.length > MAX_SLOTS)
// When overflowing: show the first (MAX_SLOTS - 1) items + a "More" slot.
const primary = computed(() =>
  hasOverflow.value ? all.value.slice(0, MAX_SLOTS - 1) : all.value,
)
const overflow = computed(() => (hasOverflow.value ? all.value.slice(MAX_SLOTS - 1) : []))

const overflowOpen = ref(false)
const overflowActive = computed(() => overflow.value.some((d) => d.name === route.name))

function toggleOverflow() {
  overflowOpen.value = !overflowOpen.value
}
function closeOverflow() {
  overflowOpen.value = false
}
</script>

<template>
  <div>
    <!-- Overflow destinations live in a bottom sheet. -->
    <MoreMenu :open="overflowOpen" :items="overflow" @close="closeOverflow" />

    <nav
      class="flex items-stretch border-t border-slate-200 bg-white"
      style="padding-bottom: env(safe-area-inset-bottom)"
      aria-label="Hovednavigation"
    >
      <RouterLink
        v-for="item in primary"
        :key="item.name"
        :to="{ name: item.name }"
        class="flex min-h-[3.25rem] flex-1 flex-col items-center justify-center gap-1 py-2 text-xs text-slate-500"
        active-class="text-slate-900 font-medium"
        @click="closeOverflow"
      >
        <component :is="item.icon" class="h-5 w-5" aria-hidden="true" />
        <span>{{ item.label }}</span>
      </RouterLink>

      <button
        v-if="hasOverflow"
        type="button"
        class="flex min-h-[3.25rem] flex-1 flex-col items-center justify-center gap-1 py-2 text-xs"
        :class="overflowActive || overflowOpen ? 'text-slate-900 font-medium' : 'text-slate-500'"
        :aria-expanded="overflowOpen"
        aria-label="Mere"
        @click="toggleOverflow"
      >
        <Menu class="h-5 w-5" aria-hidden="true" />
        <span>Mere</span>
      </button>
    </nav>
  </div>
</template>
