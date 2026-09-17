<script setup lang="ts">
import type { WithClassAsProps } from './interface'
import { cn } from '@/helpers/utils'
import { useCarousel } from './useCarousel'

defineOptions({
  inheritAttrs: false,
})

const props = defineProps<WithClassAsProps>()

const { carouselRef, orientation } = useCarousel()
</script>

<template>
  <div
    ref="carouselRef"
    data-slot="carousel-content"
    class="h-full overflow-hidden"
  >
    <!--
      LOCAL DEVIATION FROM UPSTREAM shadcn-vue: `h-full` added to the viewport div above.

      Upstream only forwards `props.class` to the inner flex track, so a caller who wants a
      fixed-height carousel (`<CarouselContent class="h-full">`) sets the height on the track
      while this wrapper stays auto — and `h-full` against an auto-height parent resolves to
      auto. The chain breaks here, silently: slides then size themselves from their content,
      sit at the top of the frame, and any `object-cover` inside them has no box to cover.

      That is exactly what Glimt's media strip hit (task 316): the container had a correct
      aspect ratio, and the images inside ignored it and top-aligned.

      Harmless in upstream's own usage — `Carousel` is auto-height there, and `h-full` of an
      auto-height parent behaves as auto — so this makes the fixed-height case work without
      changing the default one.
    -->
    <div
      :class="
        cn(
          'flex',
          orientation === 'horizontal' ? '-ml-4' : '-mt-4 flex-col',
          props.class,
        )"
      v-bind="$attrs"
    >
      <slot />
    </div>
  </div>
</template>
