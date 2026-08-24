<script setup lang="ts">
import { computed } from 'vue'
import { Locate, LocateFixed, LocateOff } from '@lucide/vue'
import type { GeoPermission } from '@/stores/location.store'

// Recentre / follow toggle. Reflects three states so the user is never left
// guessing why the map will not find them: following, idle, and blocked.
const props = defineProps<{
  permission: GeoPermission
  following: boolean
  hasPosition: boolean
}>()

const emit = defineEmits<{ locate: [] }>()

const blocked = computed(
  () => props.permission === 'denied' || props.permission === 'unavailable',
)

const label = computed(() => {
  if (blocked.value) {
    return 'Placering er slået fra'
  }
  return props.following ? 'Følger din placering' : 'Vis min placering'
})
</script>

<template>
  <button
    type="button"
    class="flex h-11 w-11 items-center justify-center rounded-xl bg-white/95 shadow-md ring-1 ring-slate-900/10"
    :class="blocked ? 'text-slate-400' : following && hasPosition ? 'text-blue-600' : 'text-slate-700'"
    :aria-label="label"
    :title="label"
    :aria-pressed="following"
    @click="emit('locate')"
  >
    <LocateOff v-if="blocked" class="h-5 w-5" aria-hidden="true" />
    <LocateFixed v-else-if="following && hasPosition" class="h-5 w-5" aria-hidden="true" />
    <Locate v-else class="h-5 w-5" aria-hidden="true" />
  </button>
</template>
