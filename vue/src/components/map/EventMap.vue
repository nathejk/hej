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
  wmsLayerOptions,
  type BaseLayerKey,
} from '@/config/map'
import { dataforsyningenToken } from '@/config/runtime'
import type { Coords } from '@/stores/location.store'
import type { Scan } from '@/stores/scans.store'
import type { Checkpoint } from '@/stores/checkpoints.store'
import {
  checkpointMarkerStyle,
  checkpointState,
  type CheckpointState,
  checkpointPopupHtml,
} from '@/components/map/checkpointPresentation'

// Leaflet owns its DOM and mutates it imperatively, so the map is deliberately
// kept outside Vue's reactivity: props are watched and translated into Leaflet
// calls rather than rendered into a template.
const props = defineProps<{
  baseLayer: BaseLayerKey
  position: Coords | null
  following: boolean
  scans: Scan[]
  /**
   * The checkpoints this patrol has earned sight of (PRD 016).
   *
   * Already patrol-scoped by the BFF, so there is nothing to filter here — everything in this list may
   * be drawn. Positions of posts the patrol has not been shown never reach the client at all.
   */
  checkpoints: Checkpoint[]
  /** Ids of checkpoints the patrol has already scanned, so the map doubles as a progress view. */
  scannedCheckpointIds: string[]
  /**
   * Checkgroups the patrol has cleared — those holding at least one scanned post.
   *
   * Passed in rather than derived here because it needs the checkpoint lookup the view already holds, and
   * because a post in a cleared line renders differently from one still ahead: reaching either post in a
   * postlinje clears it, so the other one no longer needs walking to.
   */
  clearedCheckgroups: string[]
}>()

const emit = defineEmits<{
  /** User panned/zoomed by hand — stop chasing the position. */
  userInteracted: []
  /** A tile gave up after all retries (offline, quota, missing/bad token). */
  tileError: []
  /** All visible tiles loaded — clears any earlier failure notice. */
  tilesOk: []
  /**
   * The map moved, so anything positioned over it has to be recomputed.
   *
   * A bare signal rather than a payload of coordinates: the overlay needs to *project* points, which only
   * this component can do, so it calls back through `project()` instead. Emitting the viewport as data would
   * mean reimplementing Leaflet's projection outside Leaflet.
   */
  viewportChanged: []
}>()

const container = ref<HTMLDivElement | null>(null)

let map: L.Map | null = null
let currentBase: L.TileLayer.WMS | null = null
let positionMarker: L.CircleMarker | null = null
let accuracyCircle: L.Circle | null = null
let scanLayer: L.LayerGroup | null = null
const scanMarkers = new Map<string, L.Marker>()
let checkpointLayer: L.LayerGroup | null = null
const checkpointMarkers = new Map<string, L.Marker>()

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
  // Options come from `wmsLayerOptions` rather than being written here, so the offline downloader
  // (task 087) produces byte-identical URLs. If the two ever differ, the bulk download fills the
  // cache with entries this map never looks up — which looks like a working feature right up until
  // somebody opens the map in a forest. The reasoning for each option lives with the function.
  const layer = L.tileLayer.wms(cfg.url, {
    ...wmsLayerOptions(cfg, dataforsyningenToken.value),
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

// The window a post is open, for the marker popup. Date-less on purpose: a patrol reading this at 02:00
// knows what night it is, and "fre 22:40–23:40" is quicker to read than a full timestamp.

// Checkpoint markers, distinct from scan markers on purpose.
//
// The map shows both at once, and "a post we must reach" versus "something we registered" must not read as
// the same object — so the shape differs (a pin with a point, rather than the scans' round badge) as well as
// the colour. Shape matters more than colour here: this is used at night, one-handed, by people whose
// screen brightness is turned down, and some of whom will not distinguish orange from red.
//
// What the marker *says* — the colours, the glyphs, the popup text — lives in `checkpointPresentation.ts`,
// so those decisions can be tested in node. This function is only the Leaflet call.
function checkpointIcon(state: CheckpointState): L.DivIcon {
  const style = checkpointMarkerStyle(state)
  return L.divIcon({
    className: '',
    html:
      `<span style="display:flex;align-items:center;justify-content:center;` +
      `width:30px;height:30px;background:${style.background};color:${style.foreground};` +
      `border:2px solid ${style.border};box-shadow:0 1px 4px rgb(0 0 0 / .45);font-size:16px;` +
      // A rounded square with one pointed corner: a pin, without an image asset.
      `line-height:1;border-radius:9999px 9999px 2px 9999px;transform:rotate(45deg)">` +
      `<span style="transform:rotate(-45deg)">${style.glyph}</span></span>`,
    iconSize: [30, 30],
    // Anchored at the point, not the centre, so the pin sits *on* the post.
    iconAnchor: [15, 28],
    popupAnchor: [0, -28],
  })
}

function renderCheckpoints() {
  if (!map || !checkpointLayer) {
    return
  }
  checkpointLayer.clearLayers()
  checkpointMarkers.clear()

  const scanned = new Set(props.scannedCheckpointIds)
  const cleared = new Set(props.clearedCheckgroups)

  for (const cp of props.checkpoints) {
    const state = checkpointState(cp, scanned, cleared)
    const marker = L.marker([cp.lat, cp.lng], {
      icon: checkpointIcon(state),
      title: cp.name,
      // Below the scan markers: where a patrol has scanned the post it is standing at, the registration
      // is the newer fact and should be the one on top.
      zIndexOffset: -100,
    }).bindPopup(checkpointPopupHtml(cp, state))
    marker.addTo(checkpointLayer)
    checkpointMarkers.set(cp.id, marker)
  }
}

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

/** Pan to a checkpoint and open its popup — called from the arrow overlay and the drawer. */
function focusCheckpoint(id: string) {
  const marker = checkpointMarkers.get(id)
  if (!marker || !map) {
    return
  }
  moveTo(marker.getLatLng(), Math.max(map.getZoom(), LOCATE_ZOOM))
  marker.openPopup()
}

/**
 * Where a ground position falls in the container's pixel space, or null before the map exists.
 *
 * Exposed rather than emitted because projection is Leaflet's job and depends on the live centre, zoom and
 * container size. An overlay that reimplemented it would drift out of step on every animated pan — and would
 * have to know about Web Mercator, which is exactly the knowledge this component exists to contain.
 */
function project(lat: number, lng: number): { x: number; y: number } | null {
  if (!map) {
    return null
  }
  const p = map.latLngToContainerPoint([lat, lng])
  return { x: p.x, y: p.y }
}

/** The map container's pixel size, or null before it exists. */
function viewportSize(): { width: number; height: number } | null {
  if (!map) {
    return null
  }
  const size = map.getSize()
  return { width: size.x, height: size.y }
}

/** Recentre on the current position (locate button). */
function recenter() {
  if (props.position && map) {
    moveTo([props.position.lat, props.position.lng], LOCATE_ZOOM)
  }
}

defineExpose({ focusScan, focusCheckpoint, recenter, project, viewportSize })

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
  // Checkpoints below scans: where a patrol has scanned the post it is standing at, the registration is the
  // newer fact and should be the one on top.
  checkpointLayer = L.layerGroup().addTo(map)
  scanLayer = L.layerGroup().addTo(map)

  renderCheckpoints()
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

  // Anything drawn over the map has to follow it. `move` fires continuously during a pan and a zoom
  // animation, which is what keeps an edge arrow attached to its direction rather than jumping when the
  // gesture ends. `resize` matters on a phone because the browser chrome appears and disappears as the page
  // scrolls, changing the container height without any map interaction at all.
  map.on('move zoom resize', () => {
    emit('viewportChanged')
  })

  // One emit after mount so an overlay can place itself without waiting for the first gesture.
  emit('viewportChanged')
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
  checkpointLayer = null
  checkpointMarkers.clear()
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
// Both the list and the scanned set change what a marker looks like, so both are watched. A new scan at a
// post turns its pin from a flag into a tick without the checkpoint list itself changing at all.
watch(() => props.checkpoints, renderCheckpoints, { deep: true })
watch(() => props.scannedCheckpointIds, renderCheckpoints, { deep: true })
// A new scan can clear a line without touching either list above — the other post in the postlinje changes
// appearance because of a scan at its sibling — so this is watched separately rather than assumed.
watch(() => props.clearedCheckgroups, renderCheckpoints, { deep: true })
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
