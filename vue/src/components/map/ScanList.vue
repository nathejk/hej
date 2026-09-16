<script setup lang="ts">
import { computed } from 'vue'
import { Flag, Skull, MapPinOff } from '@lucide/vue'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer'
import { Badge } from '@/components/ui/badge'
import type { Scan } from '@/stores/scans.store'
import { verdictBadge } from './scanVerdict'

// Chronological list of the patrol's registrations, in the same Drawer primitive
// the nav overflow uses (see .rules: prefer a standard shadcn component). The map
// stays visible behind it, so a tap on a row can pan the map underneath.
const props = defineProps<{ open: boolean; scans: Scan[] }>()
const emit = defineEmits<{ close: []; select: [id: string] }>()

// Decorate each scan with its verdict badge once, rather than calling the (pure) helper repeatedly from
// the template. `positioned` gates the tap affordance without a second pass over the list.
const rows = computed(() =>
  props.scans.map((scan) => ({
    scan,
    badge: verdictBadge(scan),
    positioned: scan.lat !== null && scan.lng !== null,
  })),
)

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
    <DrawerContent class="pb-[var(--sab)]">
      <DrawerHeader class="pb-2">
        <DrawerTitle>Registreringer</DrawerTitle>
      </DrawerHeader>

      <p v-if="!scans.length" class="px-5 pb-6 text-sm text-slate-500">
        Ingen registreringer endnu.
      </p>

      <ul v-else class="max-h-[50vh] overflow-y-auto pb-2">
        <li v-for="row in rows" :key="row.scan.id">
          <button
            type="button"
            class="flex min-h-[3.25rem] w-full items-center gap-3 px-5 py-3 text-left"
            @click="pick(row.scan)"
          >
            <span
              class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
              :class="row.scan.kind === 'bandit'
                ? 'bg-red-100 text-red-700'
                : 'bg-emerald-600 text-white ring-2 ring-emerald-600/20'"
            >
              <Skull v-if="row.scan.kind === 'bandit'" class="h-4 w-4" aria-hidden="true" />
              <Flag v-else class="h-4 w-4" aria-hidden="true" />
            </span>

            <span class="min-w-0 flex-1">
              <span class="block truncate text-slate-800">{{ row.scan.label }}</span>
              <span class="block text-xs text-slate-500">
                {{ timeFormat.format(row.scan.scannedAt) }}
              </span>
            </span>

            <Badge
              v-if="row.badge"
              :variant="row.badge.tone"
              class="shrink-0"
            >
              {{ row.badge.text }}
            </Badge>

            <MapPinOff
              v-if="!row.positioned"
              class="h-4 w-4 shrink-0 text-slate-400"
              aria-label="Ingen placering"
            />
          </button>
        </li>
      </ul>
    </DrawerContent>
  </Drawer>
</template>
