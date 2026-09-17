<script setup lang="ts">
import { computed, defineAsyncComponent, ref } from 'vue'
import { Menu } from '@lucide/vue'
import { useRoute } from 'vue-router'
import { useSessionStore } from '@/stores/session.store'
import { visibleDestinations } from '@/config/navigation'
import { splitNavSlots } from '@/config/navSlots'
import { showBuildId } from '@/config/runtime'

// MoreMenu is loaded on demand: it pulls in the Reka UI Drawer, which is a
// meaningful chunk, and most sessions never open the overflow sheet. Keeping it
// out of the app-shell bundle matters on mobile data.
const MoreMenu = defineAsyncComponent(() => import('@/components/MoreMenu.vue'))

// Bottom navigation, filtered by the signed-in role. The bar/overflow split lives in
// @/config/navSlots so it can be tested against every role (task 320) — the ordering it
// encodes is load-bearing now that PRD 019 requires Glimt in the bar for spejdere, and a
// rule kept inside a component cannot be asserted in a DOM-less test run.

const session = useSessionStore()
const route = useRoute()

const all = computed(() => visibleDestinations(session.role))
const slots = computed(() => splitNavSlots(all.value))
const hasOverflow = computed(() => slots.value.hasOverflow)
const primary = computed(() => slots.value.primary)
const overflow = computed(() => slots.value.overflow)

const overflowOpen = ref(false)
// Stays true after the first open so the drawer keeps its close animation
// instead of being unmounted mid-transition.
const overflowMounted = ref(false)
const overflowActive = computed(() => overflow.value.some((d) => d.name === route.name))

function toggleOverflow() {
  overflowOpen.value = !overflowOpen.value
  if (overflowOpen.value) {
    overflowMounted.value = true
  }
}
function closeOverflow() {
  overflowOpen.value = false
}

// Build id, overlaid on the nav so it lands in every screenshot of every screen — the
// nav is the only chrome present on all of them.
//
// Toggled by the BFF's `show_build_id` rather than a VITE_ variable, so one published
// image can have it on for a test deployment and off for the event (see
// @/config/runtime, and the no-VITE_-app-config note in env.d.ts).
//
// It exists at all because an installed PWA can sit on a stale service worker: without
// it a screenshot cannot be attributed to a build, and a test against the wrong build
// proves nothing.
const buildId = __BUILD_ID__
</script>

<template>
  <div>
    <!-- Overflow destinations live in a bottom sheet (lazy: see above). -->
    <MoreMenu
      v-if="overflowMounted"
      :open="overflowOpen"
      :items="overflow"
      @close="closeOverflow"
    />

    <nav
      class="relative flex items-stretch border-t border-slate-200 bg-white"
      style="padding-bottom: var(--sab)"
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

      <!-- Diagnostic overlay, NOT a layout participant.

           `absolute` + `pointer-events-none` is the whole point: this must never move
           or resize anything else, whether it is shown or hidden, so toggling it can
           never be the explanation for a UI difference between two screenshots. It is
           pinned to the nav's bottom-right, which on a device with a home indicator
           falls inside the safe-area strip the nav already reserves — dead space that
           costs nothing. `aria-hidden` keeps it out of the accessibility tree; the
           privacy page exposes the same value as real content.

           Low contrast on purpose: recognisable when you look for it, ignorable
           otherwise. -->
      <span
        v-if="showBuildId"
        class="pointer-events-none absolute right-1 bottom-0 font-mono text-[9px] leading-[1.4] whitespace-nowrap text-slate-300 select-none"
        aria-hidden="true"
        >{{ buildId }}</span
      >
    </nav>
  </div>
</template>
