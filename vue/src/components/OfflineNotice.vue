<script setup lang="ts">
import { computed } from 'vue'
import { WifiOff } from '@lucide/vue'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { useAppStore } from '@/stores/app.store'
import { useSessionStore } from '@/stores/session.store'

// Tells the user they have no signal, so "nothing is loading" is never mistaken for
// "the app is broken" or "I have been signed out" (task 090).
//
// In the document flow rather than pinned to the viewport: an overlay would either
// collide with UpdatePrompt at the top or cover the map at the bottom, and this
// notice can be on screen for hours at a time — a participant walking through the
// forest is offline as the normal case, not as an incident.
const app = useAppStore()
const session = useSessionStore()

const props = defineProps<{
  // Set on full-bleed routes (the map), where there is no top bar above this to
  // clear the status bar. The inset lives here, on the element that is only in the
  // DOM when the notice actually shows, rather than on a wrapper in App.vue: an
  // always-rendered wrapper reserved the inset even when online, which left a band
  // of blank white above the map in standalone mode where the map should reach the
  // top of the screen.
  insetTop?: boolean
}>()

// Only shown once we know who the user is. On the login screen the offline state is
// explained in terms of what they were about to do (see the onboarding login step) rather than as a
// standalone warning.
// Deliberately ONE LINE. During the event, offline is the normal state, not an
// incident — a participant can be without signal for hours — so this has to cost as
// little of the map as possible. An earlier three-line version was honest and
// unusable.
const show = computed(() => !app.online && session.isAuthenticated)

const insetStyle = computed(() =>
  props.insetTop ? { paddingTop: 'var(--sat)' } : undefined,
)
</script>

<template>
  <div v-if="show" :style="insetStyle">
    <Alert class="mx-4 mt-3 w-auto border-amber-300 bg-amber-50 text-amber-900">
      <WifiOff aria-hidden="true" />
      <AlertTitle>Ingen forbindelse — kort og regler virker stadig</AlertTitle>
    </Alert>
  </div>
</template>
