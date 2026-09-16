<script setup lang="ts">
// Hand-rolled: shadcn-vue has no map-overlay primitive, and the catalogue has nothing for "elements pinned to
// the edge of a viewport, positioned by geometry". Composes plain elements plus a Lucide chevron; see PRD 016
// §7 and the skill's allowance for hand-rolled map controls.
import { computed } from 'vue'
import { ChevronUp } from '@lucide/vue'

import { computeArrows, type KeepOutZone } from '@/components/map/arrowPlacement'
import type { Point } from '@/components/map/arrowGeometry'
import type { Checkpoint } from '@/stores/checkpoints.store'
import type { Coords } from '@/stores/location.store'

// Edge arrows towards the next posts (PRD 016).
//
// # What this is for
//
// Zoomed in on their own position — which is how the map is actually used while walking — a patrol cannot see
// where they are going. An arrow at the edge of the screen, in the right direction, with a distance, is the
// one thing this app can tell them that the paper in their hand cannot.
//
// # Where the thinking lives
//
// In `arrowPlacement.ts` and `arrowGeometry.ts`, both pure and both tested in node. This file is markup: the
// rules about when an arrow appears and where it may go are not verifiable from a screenshot, so they are not
// kept here.

const props = defineProps<{
  /** Every revealed post in the line the patrol is heading for — one arrow each. */
  targets: Checkpoint[]
  /** The patrol's own position. No position means no arrows — a bearing needs an origin. */
  position: Coords | null
  /**
   * Bumped by the parent whenever the map moves, so the computed below re-runs.
   *
   * A counter rather than a watcher on the map: Leaflet lives outside Vue's reactivity by design (see
   * `EventMap.vue`), so there is nothing reactive to watch. This is the seam between the two worlds, and it is
   * what makes the arrows track a pan continuously instead of snapping when the finger lifts.
   */
  revision: number
  /** Projects a ground position into container pixels. Supplied by the map, which owns the projection. */
  project: (lat: number, lng: number) => Point | null
  /** The map container's pixel size. */
  viewportSize: () => { width: number; height: number } | null
  /** Rectangles an arrow must not sit on, because a floating control does (task 264). */
  keepOut?: KeepOutZone[]
}>()

const emit = defineEmits<{
  /** The user tapped an arrow — pan to that post. */
  select: [id: string]
}>()

const arrows = computed(() => {
  // Referenced so this re-runs when the map moves: Leaflet's centre and zoom are not reactive state.
  void props.revision

  return computeArrows({
    targets: props.targets,
    position: props.position,
    project: props.project,
    viewportSize: props.viewportSize,
    keepOut: props.keepOut,
  })
})
</script>

<template>
  <!-- pointer-events-none on the layer, auto on each arrow: the map underneath must stay draggable
       everywhere except on the arrows themselves. -->
  <div class="pointer-events-none absolute inset-0 z-10 overflow-hidden">
    <button
      v-for="arrow in arrows"
      :key="arrow.id"
      type="button"
      class="pointer-events-auto absolute flex h-12 w-12 -translate-x-1/2 -translate-y-1/2
             flex-col items-center justify-center gap-0.5 rounded-full bg-white/95 shadow-md
             ring-1 ring-slate-900/10"
      :style="{ left: `${arrow.x}px`, top: `${arrow.y}px` }"
      :aria-label="arrow.label"
      @click="emit('select', arrow.id)"
    >
      <!-- The chevron carries the direction and is hidden from assistive tech: the button's label already
           says "mod nordøst", which is the usable form of the same fact. -->
      <ChevronUp
        class="h-4 w-4 text-orange-600"
        :style="{ transform: `rotate(${arrow.bearing}deg)` }"
        aria-hidden="true"
      />
      <span class="text-[10px] font-medium leading-none text-slate-700">{{ arrow.distance }}</span>
    </button>
  </div>
</template>
