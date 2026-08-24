<script setup lang="ts">
import { Flag, Skull, MapPinOff } from '@lucide/vue'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer'
import type { Scan } from '@/stores/scans.store'

// Chronological list of the patrol's registrations, in the same Drawer primitive
// the nav overflow uses (see .rules: prefer a standard shadcn component). The map
// stays visible behind it, so a tap on a row can pan the map underneath.
const props = defineProps<{ open: boolean; scans: Scan[] }>()
const emit = defineEmits<{ close: []; select: [id: string] }>()

const timeFormat = new Intl.DateTimeFormat('da-DK', {
  weekday: 'short',
  hour: '2-digit',
  minute: '2-digit',
})

function onOpenChange(value: boolean) {
  if (!value) {
    emit('close')
  }
}

function pick(scan: Scan) {
  // Un-positioned registrations exist (a scan can be registered manually); they
  // are listed but there is nothing to pan to.
  if (scan.lat !== null && scan.lng !== null) {
    emit('select', scan.id)
  }
  emit('close')
}
</script>

<template>
  <Drawer :open="props.open" @update:open="onOpenChange">
    <DrawerContent class="pb-[env(safe-area-inset-bottom)]">
      <DrawerHeader class="pb-2">
        <DrawerTitle>Registreringer</DrawerTitle>
      </DrawerHeader>

      <p v-if="!scans.length" class="px-5 pb-6 text-sm text-slate-500">
        Ingen registreringer endnu.
      </p>

      <ul v-else class="max-h-[50vh] overflow-y-auto pb-2">
        <li v-for="scan in scans" :key="scan.id">
          <button
            type="button"
            class="flex min-h-[3.25rem] w-full items-center gap-3 px-5 py-3 text-left"
            @click="pick(scan)"
          >
            <span
              class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
              :class="scan.kind === 'bandit' ? 'bg-red-100 text-red-700' : 'bg-slate-100 text-slate-700'"
            >
              <Skull v-if="scan.kind === 'bandit'" class="h-4 w-4" aria-hidden="true" />
              <Flag v-else class="h-4 w-4" aria-hidden="true" />
            </span>

            <span class="min-w-0 flex-1">
              <span class="block truncate text-slate-800">{{ scan.label }}</span>
              <span class="block text-xs text-slate-500">
                {{ timeFormat.format(scan.scannedAt) }}
              </span>
            </span>

            <MapPinOff
              v-if="scan.lat === null || scan.lng === null"
              class="h-4 w-4 shrink-0 text-slate-400"
              aria-label="Ingen placering"
            />
          </button>
        </li>
      </ul>
    </DrawerContent>
  </Drawer>
</template>
