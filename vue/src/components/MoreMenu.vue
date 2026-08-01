<script setup lang="ts">
import type { NavDestination } from '@/config/navigation'

// Bottom sheet listing the overflow destinations (the ones beyond the 4 primary
// nav slots). Controlled by BottomNav via `open`; emits `close` on backdrop tap
// or after navigation.
defineProps<{ open: boolean; items: NavDestination[] }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="open"
        class="fixed inset-0 z-40 bg-black/40"
        aria-hidden="true"
        @click="emit('close')"
      />
    </Transition>

    <Transition name="slide-up">
      <div
        v-if="open"
        class="fixed inset-x-0 bottom-0 z-50 rounded-t-2xl bg-white pb-[env(safe-area-inset-bottom)] shadow-2xl"
        role="dialog"
        aria-label="Flere sider"
      >
        <div class="mx-auto my-3 h-1 w-10 rounded-full bg-slate-300" />
        <nav class="pb-2">
          <RouterLink
            v-for="item in items"
            :key="item.name"
            :to="{ name: item.name }"
            class="flex items-center gap-3 px-5 py-3 text-slate-700"
            active-class="font-medium text-slate-900"
            @click="emit('close')"
          >
            <component :is="item.icon" class="h-5 w-5" aria-hidden="true" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </nav>
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
.slide-up-enter-active,
.slide-up-leave-active {
  transition: transform 0.2s ease;
}
.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
}
</style>
