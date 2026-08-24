<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import {
  baseLayers,
  FALLBACK_BOUNDS,
  FALLBACK_CENTER,
  FALLBACK_ZOOM,
  LOCATE_ZOOM,
  MAX_ZOOM,
  MIN_ZOOM,
  TILE_RETRY_BASE_DELAY_MS,
  TILE_RETRY_LIMIT,
  type BaseLayerKey,
} from '@/config/map'
import { dataforsyningenToken } from '@/config/runtime'
import type { Coords } from '@/stores/location.store'
import type { Scan } from '@/stores/scans.store'

// Leaflet owns its DOM and mutates it imperatively, so the map is deliberately
// kept outside Vue's reactivity: props are watched and translated into Leaflet
// calls rather than rendered into a template.
const props = defineProps<{
  baseLayer: BaseLayerKey
  position: Coords | null
  following: boolean
  scans: Scan[]
}>()

const emit = defineEmits<{
  /** User panned/zoomed by hand — stop chasing the position. */
  userInteracted: []
  /** A tile gave up after all retries (offline, quota, missing/bad token). */
  tileError: []
  /** All visible tiles loaded — clears any earlier failure notice. */
  tilesOk: []
}>()

const container = ref<HTMLDivElement | null>(null)

let map: L.Map | null = null
let currentBase: L.TileLayer.WMS | null = null
let positionMarker: L.CircleMarker | null = null
let accuracyCircle: L.Circle | null = null
let scanLayer: L.LayerGroup | null = null
const scanMarkers = new Map<string, L.Marker>()

// Set while we move the map ourselves, so our own setView() calls are not
// mistaken for the user panning away (which would cancel follow mode).
let selfMoving = false

function moveTo(latlng: L.LatLngExpression, zoom?: number) {
  if (!map) {
    return
  }
  selfMoving = true
  map.setView(latlng, zoom ?? map.getZoom(), { animate: true })
  // Cleared on the next tick after Leaflet has emitted its move events.
  window.setTimeout(() => {
    selfMoving = false
  }, 350)
}

// Tiles that are mid-retry, so a layer swap or unmount can cancel them.
const pendingRetries = new Set<number>()

// Leaflet has no tile retry: when an image request fails the tile simply stays
// grey until something forces it to be recreated. That is a poor deal on patchy
// rural mobile data, so retry each failed tile a few times with exponential
// backoff before surfacing the error.
//
// Re-assigning `src` on the *same* <img> keeps Leaflet's own load/error handlers
// attached, so a late success still marks the tile loaded and fades it in
// normally. The cache-buster defeats any negative caching of the failed response
// by the browser or an intermediary.
function attachTileRetry(layer: L.TileLayer.WMS) {
  layer.on('tileerror', (event: L.TileErrorEvent) => {
    const tile = event.tile as HTMLImageElement & { _hejRetries?: number }
    if (!tile) {
      emit('tileError')
      return
    }

    const attempt = (tile._hejRetries ?? 0) + 1
    if (attempt > TILE_RETRY_LIMIT) {
      emit('tileError')
      return
    }
    tile._hejRetries = attempt

    // Exponential backoff with jitter, so a whole screen of failed tiles does not
    // retry in lockstep and hammer the service.
    const delay = TILE_RETRY_BASE_DELAY_MS * 2 ** (attempt - 1) + Math.random() * 250
    const original = tile.src.replace(/&_retry=\d+$/, '')

    const timer = window.setTimeout(() => {
      pendingRetries.delete(timer)
      // The tile may have been discarded meanwhile (layer swap, pan, unmount).
      if (!tile.isConnected) {
        return
      }
      tile.src = `${original}&_retry=${attempt}`
    }, delay)
    pendingRetries.add(timer)
  })
}

function cancelRetries() {
  for (const timer of pendingRetries) {
    window.clearTimeout(timer)
  }
  pendingRetries.clear()
}

function buildBaseLayer(key: BaseLayerKey): L.TileLayer.WMS {
  const cfg = baseLayers[key]
  // Leaflet copies any option it does not recognise into the WMS query string,
  // which is how `token` reaches Dataforsyningen.
  const layer = L.tileLayer.wms(cfg.url, {
    layers: cfg.layer,
    format: 'image/png',
    transparent: false,
    attribution: cfg.attribution,
    token: dataforsyningenToken.value,
    maxZoom: MAX_ZOOM,
  } as L.WMSOptions)
  attachTileRetry(layer)
  // Once a full screen of tiles has loaded, any earlier failure is stale.
  layer.on('load', () => emit('tilesOk'))
  return layer
}

// divIcon rather than Leaflet's default image marker: no asset wrangling through
// Vite, and the two kinds stay visually distinct on both topo and aerial.
function scanIcon(kind: Scan['kind']): L.DivIcon {
  const isBandit = kind === 'bandit'
  const bg = isBandit ? '#b91c1c' : '#0f172a'
  const glyph = isBandit ? '&#9760;' : '&#9873;'
  return L.divIcon({
    className: '',
    html:
      `<span style="display:flex;align-items:center;justify-content:center;` +
      `width:28px;height:28px;border-radius:9999px;background:${bg};color:#fff;` +
      `border:2px solid #fff;box-shadow:0 1px 3px rgb(0 0 0 / .4);font-size:15px;` +
      `line-height:1">${glyph}</span>`,
    iconSize: [28, 28],
    iconAnchor: [14, 14],
    popupAnchor: [0, -16],
  })
}

const timeFormat = new Intl.DateTimeFormat('da-DK', {
  weekday: 'short',
  hour: '2-digit',
  minute: '2-digit',
})

function renderScans() {
  if (!map || !scanLayer) {
    return
  }
  scanLayer.clearLayers()
  scanMarkers.clear()

  for (const scan of props.scans) {
    if (scan.lat === null || scan.lng === null) {
      continue
    }
    const marker = L.marker([scan.lat, scan.lng], {
      icon: scanIcon(scan.kind),
      title: scan.label,
    }).bindPopup(
      `<strong>${scan.label}</strong><br>${
        scan.kind === 'bandit' ? 'Fanget af bandit' : 'Post'
      } &middot; ${timeFormat.format(scan.scannedAt)}`,
    )
    marker.addTo(scanLayer)
    scanMarkers.set(scan.id, marker)
  }
}

function renderPosition() {
  if (!map || !props.position) {
    return
  }
  const { lat, lng, accuracy } = props.position
  const latlng: L.LatLngExpression = [lat, lng]

  if (!positionMarker) {
    accuracyCircle = L.circle(latlng, {
      radius: accuracy,
      color: '#2563eb',
      weight: 1,
      fillColor: '#3b82f6',
      fillOpacity: 0.15,
      interactive: false,
    }).addTo(map)
    positionMarker = L.circleMarker(latlng, {
      radius: 7,
      color: '#ffffff',
      weight: 3,
      fillColor: '#2563eb',
      fillOpacity: 1,
    })
      .addTo(map)
      .bindPopup('Din placering')
  } else {
    positionMarker.setLatLng(latlng)
    accuracyCircle?.setLatLng(latlng)
    accuracyCircle?.setRadius(accuracy)
  }

  if (props.following) {
    moveTo(latlng, Math.max(map.getZoom(), LOCATE_ZOOM))
  }
}

/** Pan to a scan and open its popup — called by the parent from the list. */
function focusScan(id: string) {
  const marker = scanMarkers.get(id)
  if (!marker || !map) {
    return
  }
  moveTo(marker.getLatLng(), Math.max(map.getZoom(), LOCATE_ZOOM))
  marker.openPopup()
}

/** Recentre on the current position (locate button). */
function recenter() {
  if (props.position && map) {
    moveTo([props.position.lat, props.position.lng], LOCATE_ZOOM)
  }
}

defineExpose({ focusScan, recenter })

onMounted(() => {
  if (!container.value) {
    return
  }

  map = L.map(container.value, {
    center: FALLBACK_CENTER,
    zoom: FALLBACK_ZOOM,
    minZoom: MIN_ZOOM,
    maxZoom: MAX_ZOOM,
    // Our controls float over the map; Leaflet's own are desktop-sized.
    zoomControl: false,
  })

  // Opening view: the user's own position when we have it, otherwise Sjælland.
  // The event area is deliberately not used as a default — it is not fully known
  // to participants, and the map should not reveal it.
  if (props.position) {
    map.setView([props.position.lat, props.position.lng], LOCATE_ZOOM)
  } else {
    map.fitBounds(FALLBACK_BOUNDS)
  }

  currentBase = buildBaseLayer(props.baseLayer).addTo(map)
  scanLayer = L.layerGroup().addTo(map)

  renderScans()
  renderPosition()

  // Hand gestures cancel follow mode; our own moveTo() calls must not.
  map.on('dragstart', () => {
    if (!selfMoving) {
      emit('userInteracted')
    }
  })
  map.on('zoomstart', () => {
    if (!selfMoving) {
      emit('userInteracted')
    }
  })
})

onBeforeUnmount(() => {
  cancelRetries()
  map?.remove()
  map = null
  currentBase = null
  positionMarker = null
  accuracyCircle = null
  scanLayer = null
  scanMarkers.clear()
})

watch(
  () => props.baseLayer,
  (key) => {
    if (!map) {
      return
    }
    // Pending retries belong to the outgoing layer's tiles.
    cancelRetries()
    // Swap in place so centre, zoom and overlays survive the change.
    const next = buildBaseLayer(key).addTo(map)
    if (currentBase) {
      map.removeLayer(currentBase)
    }
    currentBase = next
  },
)

watch(() => props.position, renderPosition, { deep: true })
watch(() => props.scans, renderScans, { deep: true })
watch(
  () => props.following,
  (following) => {
    if (following) {
      recenter()
    }
  },
)
</script>

<template>
  <!-- Leaflet needs a sized element; the shell hands us everything above the
       bottom nav (see App.vue's fullBleed handling).

       `isolate` is load-bearing: Leaflet gives its internal panes z-index 200-800,
       and without a stacking context here those compete with the app's own UI in
       the body context — which silently buries the floating map controls and the
       registrations drawer behind the tiles. -->
  <div ref="container" class="isolate h-full w-full" />
</template>

<style>
/* Leaflet ships its own font stack and a light background; match the app and
   keep the attribution legible but unobtrusive at night. */
.leaflet-container {
  font-family: inherit;
  background: #e2e8f0;
}

.leaflet-control-attribution {
  font-size: 10px;
  background: rgb(255 255 255 / 0.75);
}
</style>
