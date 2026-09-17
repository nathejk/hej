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
// # It loops
//
// `loop: true`, so the last photograph continues to the first (maintainer, 2026-09-18). Two reasons
// it is the right default here rather than a preference:
//
//   - A glimt is a handful of photographs from one moment, not an ordered document. There is no "end"
//     to arrive at, so stopping dead at the last one just reads as the control having broken.
//   - It removes a dead end from the desktop arrows in particular. Without looping, the right-hand
//     arrow disables itself on the final slide — and a disabled control on a photograph gives no clue
//     that the way onward is the *other* arrow.
//
// Note what this changes about the arrows: `canScrollPrev`/`canScrollNext` are now always true, so
// neither is ever rendered disabled. That is the intended consequence, not an oversight.
//
// # Filling the frame, centred
//
// Every slide is `object-cover object-center` inside a box the container sizes. `object-cover` scales
// the photograph until it fills the box and crops whatever overflows — which is the "zoom a little and
// crop the long direction" behaviour, and it is why the clamps in `stripAspectRatio` are set to keep
// that crop small for ordinary shapes.
//
// `object-center` is Tailwind's default and is written out anyway: this is the line that decides *which
// part* of a photograph survives a crop, and leaving it implicit invites somebody to assume the top is
// kept. Centre is right for a scene; for a face it would not be, but a glimt is a scene.
//
// This only works if every level from the aspect container down has a height. It did not, at first —
// `CarouselContent`'s viewport div is auto-height upstream, so `h-full` on the track resolved against
// auto and the whole chain collapsed: slides sized themselves from their content, sat at the top of
// the frame, and `object-cover` had no box to cover. Fixed in `ui/carousel/CarouselContent.vue` with a
// LOCAL DEVIATION note. Single-item glimt never had the bug, because they skip the carousel entirely —
// which is what pointed at it.
//
// # Getting between slides without a swipe
//
// A swipe is the right control on a phone and it is the *only* control on a phone — but it is not the
// only place this runs. Two cases make arrows necessary rather than decorative:
//
//   - **Desktop.** The public page (PRD 019 §7, task 323) is explicitly for a parent on a laptop, and
//     there is no gesture there. A trackpad can sometimes drag an Embla carousel and a mouse
//     generally cannot, so "swipe" reduces to "you cannot see the second photo".
//   - **Keyboard and assistive tech.** shadcn's `Carousel` already wires ←/→ and takes focus, which is
//     more than the first version of this comment credited it with — but nobody discovers a keyboard
//     shortcut on a photograph. A visible control is what makes an existing capability usable.
//
// So the prev/next buttons are rendered and shown **only where a pointer is fine** (`pointer-coarse`
// hides them). On a phone they would sit on top of the photograph to duplicate a gesture that already
// works; on a laptop they are the only way through. The dots stay in both cases — they say how many
// there are, which is a different job.
//
// Note the public page cannot use this component at all: it is server-rendered with no bundle
// (PRD 019 §7), so it needs its own no-JavaScript answer. Recorded in task 323 rather than solved
// here.
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

import { Carousel, CarouselContent, CarouselItem, CarouselNext, CarouselPrevious } from '@/components/ui/carousel'
import type { CarouselApi } from '@/components/ui/carousel'
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
//
// # This is driven by `init-api`, and it has to be
//
// The obvious spelling — `<Carousel @select="...">` — is what this component had, and **it never
// fired**: `CarouselEmits` in the shadcn primitive declares exactly one event, `init-api`. A binding
// for an event a component does not emit is not an error in Vue; it falls through to the root element
// as a native DOM listener, and a `div` never emits `select`. So `current` sat at 0 for the life of
// the card and the lit dot never moved, reported from a device on 2026-09-17.
//
// Nothing could have caught that except looking: it is a silent no-op at every level — type-check,
// lint and build are all happy, and the unit suite never mounts a component.
//
// So the Embla instance is taken from `init-api` (which is what that event is for) and its own
// `select` event is subscribed to directly. `reInit` too, because Embla re-initialises when the slide
// list changes and would otherwise leave the dot pointing at a slide that has moved.
//
// No teardown: `embla-carousel-vue` destroys the instance on unmount, and its listeners go with it.
// Calling `destroy()` here would be a second destroy of something we do not own.
const current = ref(0)

function onInitApi(api: CarouselApi) {
  if (!api) return
  const sync = () => {
    current.value = api.selectedScrollSnap()
  }
  // Called once immediately: `loop` plus `align: 'start'` means the initial snap is not always 0, and
  // waiting for the first `select` would show the wrong dot until the user swiped.
  sync()
  api.on('select', sync)
  api.on('reInit', sync)
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
        class="size-full object-cover object-center"
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
      <Carousel class="size-full" :opts="{ align: 'start', loop: true }" @init-api="onInitApi">
        <CarouselContent class="-ml-0 h-full">
          <CarouselItem v-for="item in glimt.media" :key="item.ordinal" class="h-full pl-0">
            <button type="button" class="relative block size-full" @click="emit('open', item.ordinal)">
              <img
                :src="srcFor(item.ordinal, item.hasThumb)"
                :alt="mediaAltText(glimt, item.ordinal)"
                loading="lazy"
                decoding="async"
                class="size-full object-cover object-center"
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

        <!-- Inside `<Carousel>` because these consume its provider via `useCarousel()`.

             Shown only for a fine pointer: on a phone they would cover the photograph to duplicate a
             swipe that already works, and on a laptop they are the only way to the second photo.
             Positioned inside the frame, unlike shadcn's default `-left-12`, which assumes a carousel
             with margin around it — this one is full-bleed inside a card. -->
        <CarouselPrevious
          class="left-2 size-9 border-0 bg-black/45 text-white hover:bg-black/65 hover:text-white pointer-coarse:hidden"
        />
        <CarouselNext
          class="right-2 size-9 border-0 bg-black/45 text-white hover:bg-black/65 hover:text-white pointer-coarse:hidden"
        />
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
