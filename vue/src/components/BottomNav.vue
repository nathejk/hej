<script setup lang="ts">
import { computed } from 'vue'
import { useSessionStore } from '@/stores/session.store'
import { visibleDestinations } from '@/config/navigation'

// Bottom navigation. Shows the destinations the signed-in role may see. The
// ≤5 / "More" burger overflow rule is layered on in task 011; task 012 turns
// the overflow into a bottom sheet.
const session = useSessionStore()
const items = computed(() => visibleDestinations(session.role))
</script>

<template>
  <nav
    class="flex items-stretch border-t border-slate-200 bg-white"
    style="padding-bottom: env(safe-area-inset-bottom)"
    aria-label="Hovednavigation"
  >
    <RouterLink
      v-for="item in items"
      :key="item.name"
      :to="{ name: item.name }"
      class="flex min-h-[3.25rem] flex-1 flex-col items-center justify-center gap-1 py-2 text-xs text-slate-500"
      active-class="text-slate-900 font-medium"
    >
      <component :is="item.icon" class="h-5 w-5" aria-hidden="true" />
      <span>{{ item.label }}</span>
    </RouterLink>
  </nav>
</template>
