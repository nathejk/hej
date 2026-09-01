<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import type { BadgeVariants } from '.'
import { cn } from '@/helpers/utils'
import { badgeVariants } from '.'

const props = defineProps<{
  class?: HTMLAttributes['class']
  variant?: BadgeVariants['variant']
}>()
</script>

<template>
  <span data-slot="badge" :class="cn(badgeVariants({ variant }), props.class)">
    <slot />
  </span>
</template>
