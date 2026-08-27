<script setup lang="ts">
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref } from 'vue'
import { MapPin, List } from '@lucide/vue'
import PermissionPrompt from '@/components/PermissionPrompt.vue'
import LayerSwitcher from '@/components/map/LayerSwitcher.vue'
import LocateButton from '@/components/map/LocateButton.vue'
import ScanList from '@/components/map/ScanList.vue'
import { useLocationStore } from '@/stores/location.store'
import { useScansStore } from '@/stores/scans.store'
import { useAppStore } from '@/stores/app.store'
import {
  BASE_LAYER_STORAGE_KEY,
  baseLayers,
  DEFAULT_BASE_LAYER,
  type BaseLayerKey,
} from '@/config/map'
import { dataforsyningenToken, loadRuntimeConfig } from '@/config/runtime'

// Leaflet is a sizeable dependency and only this page needs it, so the map is
// loaded on demand rather than from the app-shell bundle.
const EventMap = defineAsyncComponent(() => import('@/components/map/EventMap.vue'))

const location = useLocationStore()
const scans = useScansStore()
const app = useAppStore()

const mapRef = ref<{ focusScan: (id: string) => void; recenter: () => void } | null>(null)

// Base layer choice survives navigation and reloads.
const stored = localStorage.getItem(BASE_LAYER_STORAGE_KEY)
const baseLayer = ref<BaseLayerKey>(
  stored && stored in baseLayers ? (stored as BaseLayerKey) : DEFAULT_BASE_LAYER,
)
function onBaseLayerChange(key: BaseLayerKey) {
  baseLayer.value = key
  localStorage.setItem(BASE_LAYER_STORAGE_KEY, key)
}

const DISMISS_KEY = 'hej.prompt.location.dismissed'
const dismissed = ref(localStorage.getItem(DISMISS_KEY) === '1')
const tileError = ref(false)
const listOpen = ref(false)

// Leaflet reads the WMS token once, when it builds the base layer, so the map
// must not mount before /api/config has answered — otherwise the first tiles go
// out unauthenticated.
const configLoaded = ref(false)

// Show the soft prompt only when we could still gain permission and the user
// hasn't dismissed it before.
const showPrompt = computed(
  () =>
    location.available &&
    !dismissed.value &&
    (location.permission === 'unknown' || location.permission === 'prompt'),
)

// Only a genuinely absent key is worth reporting; don't flash the notice while
// the config request is still in flight. And only when we actually got an answer
// from the BFF: offline there is no answer, and blaming the deployment for something
// the user cannot act on — while the offline notice already explains the real
// situation — is worse than saying nothing (task 090).
const missingToken = computed(
  () => configLoaded.value && dataforsyningenToken.value === '' && app.online,
)

async function accept() {
  const coords = await location.request()
  if (coords) {
    location.setFollowing(true)
    location.watch()
  }
}

function dismiss() {
  dismissed.value = true
  localStorage.setItem(DISMISS_KEY, '1')
}

function onLocate() {
  if (location.permission === 'denied' || location.permission === 'unavailable') {
    return
  }
  location.setFollowing(true)
  if (location.position) {
    mapRef.value?.recenter()
  } else {
    void accept()
  }
}

function onSelectScan(id: string) {
  // Selecting from the list means the user wants to look elsewhere.
  location.setFollowing(false)
  mapRef.value?.focusScan(id)
}

// A high-accuracy watch is expensive; only run it while the page is visible.
function onVisibilityChange() {
  if (document.hidden) {
    location.stopWatch()
  } else if (location.permission === 'granted') {
    location.watch()
  }
}

onMounted(async () => {
  // Deliberately not awaited alongside the geolocation work below: the map
  // should appear as soon as the config lands, independent of the permission
  // round-trip.
  void loadRuntimeConfig().then(() => {
    configLoaded.value = true
  })
  await location.syncPermission()
  if (location.permission === 'granted') {
    location.watch()
  }
  document.addEventListener('visibilitychange', onVisibilityChange)
  void scans.fetch()
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
  location.stopWatch()
})
</script>

<template>
  <!-- Full-bleed: the shell gives this view everything above the bottom nav
       (route meta `fullBleed`), so the map fills it and controls float on top. -->
  <div class="absolute inset-0 overflow-hidden">
    <EventMap
      v-if="configLoaded"
      ref="mapRef"
      :base-layer="baseLayer"
      :position="location.position"
      :following="location.following"
      :scans="scans.scans"
      @user-interacted="location.setFollowing(false)"
      @tile-error="tileError = true"
      @tiles-ok="tileError = false"
    />

    <!-- Controls, top-right, clear of the notch. z-10 keeps them above the map
         element (which is isolated, so Leaflet's own z-indexes stay inside it). -->
    <div
      class="pointer-events-none absolute right-3 z-10 flex flex-col gap-2"
      style="top: calc(env(safe-area-inset-top) + 0.75rem)"
    >
      <div class="pointer-events-auto">
        <LayerSwitcher :model-value="baseLayer" @update:model-value="onBaseLayerChange" />
      </div>
      <div class="pointer-events-auto">
        <LocateButton
          :permission="location.permission"
          :following="location.following"
          :has-position="location.position !== null"
          @locate="onLocate"
        />
      </div>
    </div>

    <!-- Notices, top-left, so they never cover the controls. -->
    <div
      class="absolute left-3 right-16 z-10 space-y-2"
      style="top: calc(env(safe-area-inset-top) + 0.75rem)"
    >
      <PermissionPrompt
        v-if="showPrompt"
        title="Vis din placering"
        message="Appen viser dig på kortet og gemmer din rute, som sendes til arrangørerne. Du kan altid slå det fra igen."
        cta="Slå placering til"
        :icon="MapPin"
        :more-to="{ name: 'privacy' }"
        more-label="Hvad gemmer I?"
        @accept="accept"
        @dismiss="dismiss"
      />

      <p
        v-if="missingToken"
        class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-900 shadow-xs ring-1 ring-amber-200"
      >
        Kortlag mangler en API-nøgle (<code>DATAFORSYNINGEN_TOKEN</code>).
      </p>
      <p
        v-else-if="tileError"
        class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-900 shadow-xs ring-1 ring-amber-200"
      >
        Kortbilleder kunne ikke hentes. Din placering virker stadig.
      </p>
      <p
        v-if="location.error && location.permission !== 'denied'"
        class="rounded-lg bg-white/95 px-3 py-2 text-xs text-slate-600 shadow-xs ring-1 ring-slate-900/10"
      >
        Kunne ikke finde din placering.
      </p>
    </div>

    <!-- Registrations handle, bottom-centre, above the nav. Hidden when the user
         has no patrol (the BFF returns an empty list for personnel roles). -->
    <div
      v-if="scans.hasAny"
      class="absolute inset-x-0 bottom-3 z-10 flex justify-center"
    >
      <button
        type="button"
        class="flex min-h-11 items-center gap-2 rounded-full bg-white/95 px-4 text-sm font-medium text-slate-800 shadow-md ring-1 ring-slate-900/10"
        @click="listOpen = true"
      >
        <List class="h-4 w-4" aria-hidden="true" />
        Registreringer ({{ scans.scans.length }})
      </button>
    </div>

    <ScanList
      :open="listOpen"
      :scans="scans.scans"
      @close="listOpen = false"
      @select="onSelectScan"
    />
  </div>
</template>
