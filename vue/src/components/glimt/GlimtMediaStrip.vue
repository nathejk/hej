<script setup lang="ts">
// The media of one glimt: a single item, or a swipeable strip with dots (PRD 019 §7, task 316).
//
// # One shape for the whole strip
//
// A carousel is a flex row, and a row is as tall as its tallest child. Giving each slide its own
// aspect ratio therefore made a glimt containing a landscape *and* a portrait render as a
// portrait-tall box with the landscape floating at the top and a screen of empty card underneath.
// Reported from a device on 2026-09-18 and immediately obvious in a screenshot.
//
// So the *container* owns the height — one ratio, from the first item, clamped (see
// `stripAspectRatio`) — and every slide fills it. Every level from here down is `h-full` for that
// reason: nothing inside may contribute a height, or the row starts measuring its children again.
//
// # Thumbnail-first, and that is not a detail
//
// The feed and the grid both ask for `variant=thumb`; full media is fetched only when the viewer
// opens (task 318). This is the load peak of the whole feature — a thousand people at the finish line
// on the worst network of the weekend (PRD 019 §0a.3).
//
// # Built on shadcn's carousel
//
// Embla underneath, so it brings pointer/touch handling, keyboard support and correct focus
// behaviour. Hand-rolling a swipe strip is what the skill says not to do, and the accessible version
// of it is much more code than this.
import { computed, ref } from 'vue'
import { Play } from '@lucide/vue'

import { Carousel, CarouselContent, CarouselItem } from '@/components/ui/carousel'
import { cn } from '@/helpers'
import { glimtMediaUrl, type Glimt } from '@/stores/glimt.store'
import { mediaAltText, stripAspectRatio } from '@/components/glimt/glimtPresentation'

const props = defineProps<{
  glimt: Glimt
  class?: string
}>()

const emit = defineEmits<{
  /** Opening an item is the viewer's business, not this component's. */
  open: [ordinal: number]
}>()

// Which slide the strip is on, for the dots.
const current = ref(0)

function onSelect(index: number) {
  current.value = index
}

// One ratio for the whole strip, so the card does not change height as you swipe — which would move
// everything below it in the feed.
const aspect = computed(() => stripAspectRatio(props.glimt.media))

const single = computed(() => props.glimt.media.length === 1)

// A thumbnail when one exists, the full item otherwise. `hasThumb` is false for real reasons — task
// 303 lets a thumbnail fail without failing the upload — and the fallback is what stops a perfectly
// good photo rendering as a broken tile.
function srcFor(ordinal: number, hasThumb: boolean) {
  return glimtMediaUrl(props.glimt.id, ordinal, hasThumb ? 'thumb' : 'full')
}
</script>

<template>
  <div
    v-if="glimt.media.length"
    :class="cn('relative w-full overflow-hidden bg-muted', props.class)"
    :style="{ aspectRatio: aspect }"
  >
    <!-- One item: no carousel at all. A single-item strip would add a scroll container, a swipe
         gesture and two ARIA roles for a photograph that cannot be swiped anywhere. -->
    <button
      v-if="single"
      type="button"
      class="relative block size-full"
      @click="emit('open', glimt.media[0].ordinal)"
    >
      <img
        :src="srcFor(glimt.media[0].ordinal, glimt.media[0].hasThumb)"
        :alt="mediaAltText(glimt, glimt.media[0].ordinal)"
        loading="lazy"
        decoding="async"
        class="size-full object-cover"
      />
      <span
        v-if="glimt.media[0].kind === 'video'"
        class="pointer-events-none absolute inset-0 flex items-center justify-center"
      >
        <Play class="size-10 text-white drop-shadow-lg" aria-hidden="true" />
      </span>
    </button>

    <template v-else>
      <!-- `h-full` all the way down: the container owns the height, and anything inside that sized
           itself would put the flex row back in charge of it. -->
      <Carousel class="size-full" :opts="{ align: 'start' }" @select="onSelect">
        <CarouselContent class="-ml-0 h-full">
          <CarouselItem v-for="item in glimt.media" :key="item.ordinal" class="h-full pl-0">
            <button type="button" class="relative block size-full" @click="emit('open', item.ordinal)">
              <img
                :src="srcFor(item.ordinal, item.hasThumb)"
                :alt="mediaAltText(glimt, item.ordinal)"
                loading="lazy"
                decoding="async"
                class="size-full object-cover"
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
              'size-1.5 rounded-full bg-white transition-opacity',
              i === current ? 'opacity-100' : 'opacity-50',
            )
          "
        />
      </div>
    </template>
  </div>
</template>
