<script setup lang="ts">
import { computed } from 'vue'
import { Flag, Skull, MapPinOff, Map, Smartphone } from '@lucide/vue'
import {
  Drawer,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer'
import { Badge } from '@/components/ui/badge'
import SyncRefreshButton from '@/components/SyncRefreshButton.vue'
import type { Scan } from '@/stores/scans.store'
import type { Handout } from '@/stores/handouts.store'
import { verdictBadge } from './scanVerdict'
import { scanVariant } from './scanAppearance'
import { handoutStatus, handoutSticker } from './handoutPresentation'
import { weekdayClock } from '@/helpers/eventTime'

// The patrol's registrations and the map sheets it has been handed, in one Drawer — the same primitive the
// nav overflow uses (see .rules: prefer a standard shadcn component). The map stays visible behind it, so a
// tap on a registration row can pan the map underneath.
const props = defineProps<{ open: boolean; scans: Scan[]; handouts: Handout[] }>()
const emit = defineEmits<{ close: []; select: [id: string] }>()

// Decorate each scan with its verdict badge once, rather than calling the (pure) helper repeatedly from
// the template. `positioned` gates the tap affordance without a second pass over the list.
const scanRows = computed(() =>
  props.scans.map((scan) => ({
    scan,
    variant: scanVariant(scan),
    badge: verdictBadge(scan),
    positioned: scan.lat !== null && scan.lng !== null,
  })),
)

// Handouts are keyed by qr number where there is one, falling back to name+time for a synthesised sheet
// (which has no sticker) so two skitser handed out at different posts stay distinct rows.
const handoutRows = computed(() =>
  props.handouts.map((handout, index) => ({
    key: handout.qrId || `${handout.name}-${handout.handedOut.getTime()}-${index}`,
    handout,
    sticker: handoutSticker(handout),
    status: handoutStatus(handout),
  })),
)

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
        <!-- The map route is full-bleed, so it has no app bar to carry the app's refresh control
             (PRD 017 §7, task 282). It goes here rather than into the floating map controls because
             this is the surface whose content it refreshes — the drawer is where a patrol looks for
             the scan they just made. -->
        <div class="flex items-center justify-between gap-2">
          <DrawerTitle>Din patrulje</DrawerTitle>
          <SyncRefreshButton />
        </div>
      </DrawerHeader>

      <div class="max-h-[60vh] overflow-y-auto pb-2">
        <!-- Registrations -->
        <h3 class="px-5 pt-1 pb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
          Registreringer
        </h3>

        <p v-if="!scans.length" class="px-5 pb-4 text-sm text-slate-500">
          Ingen registreringer endnu.
        </p>

        <ul v-else class="pb-2">
          <li v-for="row in scanRows" :key="row.scan.id">
            <button
              type="button"
              class="flex min-h-[3.25rem] w-full items-center gap-3 px-5 py-3 text-left"
              @click="pick(row.scan)"
            >
              <span
                class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
                :class="{
                  'bg-red-100 text-red-700': row.variant === 'bandit',
                  'bg-emerald-600 text-white ring-2 ring-emerald-600/20': row.variant === 'checkpoint',
                  'bg-slate-100 text-slate-500': row.variant === 'plain',
                }"
              >
                <Skull v-if="row.variant === 'bandit'" class="h-4 w-4" aria-hidden="true" />
                <Flag v-else-if="row.variant === 'checkpoint'" class="h-4 w-4" aria-hidden="true" />
                <!-- Neither a catch nor a placeable post visit: a bare registration from a device.
                     Lucide's Smartphone (the catalogue's nearest thing to a handheld) and a grey,
                     unemphasised treatment, so it reads as "this happened" and not as progress. -->
                <Smartphone v-else class="h-4 w-4" aria-hidden="true" />
              </span>

              <span class="min-w-0 flex-1">
                <span class="block truncate text-slate-800">{{ row.scan.label }}</span>
                <span class="block text-xs text-slate-500">
                  {{ weekdayClock(row.scan.scannedAt) }}
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

        <!-- Map sheets handed out -->
        <h3 class="px-5 pt-3 pb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">
          Kort udleveret
        </h3>

        <p v-if="!handouts.length" class="px-5 pb-4 text-sm text-slate-500">
          Ingen kort udleveret.
        </p>

        <ul v-else class="pb-2">
          <li v-for="row in handoutRows" :key="row.key">
            <!-- Not a button: a handout has nothing to pan to. A sheet no longer held is dimmed and
                 tagged "afleveret" — never named against another team (see handoutPresentation). -->
            <div
              class="flex min-h-[3.25rem] w-full items-center gap-3 px-5 py-3"
              :class="{ 'opacity-60': row.status !== null }"
            >
              <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-100 text-slate-700">
                <Map class="h-4 w-4" aria-hidden="true" />
              </span>

              <span class="min-w-0 flex-1">
                <span class="block truncate text-slate-800">{{ row.handout.name }}</span>
                <span class="block text-xs text-slate-500">
                  <!-- Sticker line only when there is one, so a synthesised (QR-less) row leaves no gap. -->
                  <template v-if="row.sticker">Nr. {{ row.sticker }} · </template>
                  {{ weekdayClock(row.handout.handedOut) }}
                </span>
              </span>

              <Badge v-if="row.status" variant="secondary" class="shrink-0">
                {{ row.status }}
              </Badge>
            </div>
          </li>
        </ul>
      </div>
    </DrawerContent>
  </Drawer>
</template>
