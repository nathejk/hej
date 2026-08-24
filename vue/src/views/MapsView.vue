<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Map, MapPin } from '@lucide/vue'
import PagePlaceholder from '@/components/PagePlaceholder.vue'
import PermissionPrompt from '@/components/PermissionPrompt.vue'
import { useLocationStore } from '@/stores/location.store'

const location = useLocationStore()
const DISMISS_KEY = 'hej.prompt.location.dismissed'
const dismissed = ref(localStorage.getItem(DISMISS_KEY) === '1')

onMounted(() => location.syncPermission())

// Show the soft prompt only when we could still gain permission and the user
// hasn't dismissed it before.
const showPrompt = computed(
  () =>
    location.available &&
    !dismissed.value &&
    (location.permission === 'unknown' || location.permission === 'prompt'),
)

async function accept() {
  await location.request()
}
function dismiss() {
  dismissed.value = true
  localStorage.setItem(DISMISS_KEY, '1')
}
</script>

<template>
  <div class="flex h-full flex-col">
    <PermissionPrompt
      v-if="showPrompt"
      class="m-4"
      title="Vis din placering"
      message="Vi bruger din placering til at vise dig på kortet under løbet. Du kan altid slå det fra igen."
      cta="Slå placering til"
      :icon="MapPin"
      @accept="accept"
      @dismiss="dismiss"
    />
    <PagePlaceholder class="flex-1" title="Kort" :icon="Map" />
  </div>
</template>
