<script setup lang="ts">
import type { NavDestination } from '@/config/navigation'
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle } from '@/components/ui/drawer'

// Bottom sheet listing the overflow destinations (the ones beyond the primary
// nav slots). Controlled by BottomNav via `open`; emits `close` when dismissed
// or after navigation.
//
// Built on the shadcn-vue Drawer (Reka UI) rather than hand-rolled markup, per
// the standard-component-first rule in .rules — that gives us focus trapping,
// escape/outside-press dismissal, scroll locking and swipe-to-dismiss, none of
// which the previous hand-rolled version had. The drawer's default
// `swipeDirection` is "down", i.e. a bottom sheet, which is what we want.
defineProps<{ open: boolean; items: NavDestination[] }>()
const emit = defineEmits<{ close: [] }>()

// Build identity, shown at the foot of the sheet. Deliberately here and not in the
// always-visible chrome: it is a debugging aid, not product copy, and the overflow
// sheet is somewhere you can reach in two taps from any screen without it costing
// anything the rest of the time.
//
// This exists because "which build is the phone actually running?" was repeatedly
// unanswerable while testing on a device — an installed PWA can sit on a stale
// service worker, so the screen in front of you is not necessarily the code you just
// deployed, and a test result against the wrong build proves nothing.
const buildId = __BUILD_ID__

function onOpenChange(value: boolean) {
  if (!value) {
    emit('close')
  }
}
</script>

<template>
  <Drawer :open="open" @update:open="onOpenChange">
    <!-- pb keeps the last row clear of the iOS home indicator. -->
    <DrawerContent class="pb-[env(safe-area-inset-bottom)]">
      <!-- Visually hidden: the sheet is self-evident, but it still needs an
           accessible name, and Drawer requires a title for that. -->
      <DrawerHeader class="sr-only">
        <DrawerTitle>Flere sider</DrawerTitle>
      </DrawerHeader>

      <nav class="pb-2" aria-label="Flere sider">
        <RouterLink
          v-for="item in items"
          :key="item.name"
          :to="{ name: item.name }"
          class="flex min-h-[3.25rem] items-center gap-3 px-5 py-3 text-slate-700"
          active-class="font-medium text-slate-900"
          @click="emit('close')"
        >
          <component :is="item.icon" class="h-5 w-5" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <!-- Low contrast on purpose: recognisable when you go looking for it, quiet
           enough to ignore otherwise. `select-all` makes it one tap to copy when
           reporting which build a problem was seen on. -->
      <p
        class="px-5 pb-2 text-right font-mono text-[10px] leading-none text-slate-300 select-all"
        :aria-label="`Build ${buildId}`"
      >
        {{ buildId }}
      </p>
    </DrawerContent>
  </Drawer>
</template>
