<script setup lang="ts">
// The media of one glimt: a single item, or a swipeable strip with dots (PRD 019 §7, task 316).
//
// # Thumbnail-first, and that is not a detail
//
// The grid and the feed both ask for `variant=thumb`. Full-size media is fetched only when the
// viewer opens (task 318). This is the load peak of the whole feature — a thousand people at the
// finish line on the worst network of the weekend (PRD 019 §0a.3) — and "just render the full
// image, it is only one per card" is how that becomes unusable.
//
// # Lazy, and never eager
//
// `loading="lazy"` plus `decoding="async"`: a cold feed open must not fetch every image. The card
// also fixes an aspect ratio from the stored dimensions so the layout does not jump as images
// arrive, which on a slow link is the difference between scrolling and chasing.
//
// # Built on shadcn's carousel
//
// Which is Embla underneath, so it brings pointer/touch handling, keyboard support and correct
// focus behaviour. Hand-rolling a swipe strip is exactly the thing the skill says not to do — and
// the accessible version of it is much more code than this.
import { ref } from 'vue'
import { Play } from '@lucide/vue'

import {
  Carousel,
  CarouselContent,
  CarouselItem,
} from '@/components/ui/carousel'
import { cn } from '@/helpers'
import { glimtMediaUrl, type Glimt } from '@/stores/glimt.store'
import { mediaAltText } from '@/components/glimt/glimtPresentation'

const props = defineProps<{
  glimt: Glimt
  /** Full-bleed inside a card: no rounding on the sides the card already rounds. */
  class?: string
}>()

const emit = defineEmits<{
  /** Opening an item is the viewer's business, not this component's. */
  open: [ordinal: number]
}>()

// Which item the strip is on, for the dots. `undefined` until Embla reports, so the dots do not
// flash a wrong active state on first paint.
const current = ref(0)

function onSelect(index: number) {
  current.value = index
}

// A thumbnail when one exists, the full item otherwise. `hasThumb` is false for real reasons —
// task 303 lets a thumbnail fail without failing the upload — and the fallback is what stops a
// perfectly good photo rendering as a broken tile.
function srcFor(ordinal: number, hasThumb: boolean) {
  return glimtMediaUrl(props.glimt.id, ordinal, hasThumb ? 'thumb' : 'full')
}

// Aspect ratio from the stored dimensions, so the card reserves the right space before the bytes
// arrive. Falls back to 4/3 when the projection has no dimensions (an old row, or an upload whose
// decode did not report them).
function aspect(width: number, height: number) {
  return width > 0 && height > 0 ? `${width} / ${height}` : '4 / 3'
}
</script>

<template>
  <!-- One item: no carousel at all. A single-item strip would add a scroll container, a swipe
       gesture and two ARIA roles for a photograph that cannot be swiped anywhere. -->
  <button
    v-if="glimt.media.length === 1"
    type="button"
    :class="cn('block w-full overflow-hidden bg-muted', props.class)"
    :style="{ aspectRatio: aspect(glimt.media[0].width, glimt.media[0].height) }"
    @click="emit('open', glimt.media[0].ordinal)"
  >
    <img
      :src="srcFor(glimt.media[0].ordinal, glimt.media[0].hasThumb)"
      :alt="mediaAltText(glimt, glimt.media[0].ordinal)"
      loading="lazy"
      decoding="async"
      class="h-full w-full object-cover"
    />
    <span
      v-if="glimt.media[0].kind === 'video'"
      class="pointer-events-none absolute inset-0 flex items-center justify-center"
    >
      <Play class="size-10 drop-shadow-lg" aria-hidden="true" />
    </span>
  </button>

  <div v-else-if="glimt.media.length > 1" :class="cn('relative', props.class)">
    <Carousel
      class="w-full"
      :opts="{ align: 'start' }"
      @select="onSelect"
    >
      <CarouselContent class="-ml-0">
        <CarouselItem
          v-for="item in glimt.media"
          :key="item.ordinal"
          class="pl-0"
        >
          <button
            type="button"
            class="relative block w-full overflow-hidden bg-muted"
            :style="{ aspectRatio: aspect(item.width, item.height) }"
            @click="emit('open', item.ordinal)"
          >
            <img
              :src="srcFor(item.ordinal, item.hasThumb)"
              :alt="mediaAltText(glimt, item.ordinal)"
              loading="lazy"
              decoding="async"
              class="h-full w-full object-cover"
            />
            <span
              v-if="item.kind === 'video'"
              class="pointer-events-none absolute inset-0 flex items-center justify-center"
            >
              <Play class="size-10 text-white drop-shadow-lg" aria-hidden="true" />
            </span>
          </button>
        </CarouselItem>
      </CarouselContent>
    </Carousel>

    <!-- Dots rather than arrows. Arrows are a desktop affordance; on a phone the gesture is the
         control, and the dots exist only to say how many there are. `aria-hidden` because the
         count is already in each image's alt text ("billede 2 af 4") — announcing it twice is
         noise. -->
    <div
      class="pointer-events-none absolute inset-x-0 bottom-2 flex justify-center gap-1.5"
      aria-hidden="true"
    >
      <span
        v-for="(item, i) in glimt.media"
        :key="item.ordinal"
        :class="
          cn(
            'size-1.5 rounded-full transition-opacity',
            i === current ? 'bg-white opacity-100' : 'bg-white opacity-50',
          )
        "
      />
    </div>
  </div>
</template>
