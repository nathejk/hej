<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/helpers/utils'

const props = defineProps<{
  class?: HTMLAttributes['class']
}>()
</script>

<template>
  <!--
    LOCAL DEVIATION FROM UPSTREAM shadcn-vue: height bumped h-9 -> h-11 to match
    this repo's `Input`, which is itself bumped for a >=44px touch target on
    phones (see input/Input.vue). Keep in mind when re-running the CLI.
  -->
  <div
    data-slot="input-group"
    role="group"
    :class="cn(
      'border-input dark:bg-input/30 has-[[data-slot=input-group-control]:focus-visible]:border-ring has-[[data-slot=input-group-control]:focus-visible]:ring-ring/50 has-[[data-slot][aria-invalid=true]]:ring-destructive/20 has-[[data-slot][aria-invalid=true]]:border-destructive dark:has-[[data-slot][aria-invalid=true]]:ring-destructive/40 h-11 rounded-md border shadow-xs transition-[color,box-shadow] has-[[data-slot=input-group-control]:focus-visible]:ring-3 has-[[data-slot][aria-invalid=true]]:ring-3 has-[>[data-align=block-end]]:h-auto has-[>[data-align=block-end]]:flex-col has-[>[data-align=block-start]]:h-auto has-[>[data-align=block-start]]:flex-col has-[>[data-align=block-end]]:[&>input]:pt-3 has-[>[data-align=block-start]]:[&>input]:pb-3 has-[>[data-align=inline-end]]:[&>input]:pr-1.5 has-[>[data-align=inline-start]]:[&>input]:pl-1.5 group/input-group relative flex w-full min-w-0 items-center outline-none',
      props.class,
    )"
  >
    <slot />
  </div>
</template>
