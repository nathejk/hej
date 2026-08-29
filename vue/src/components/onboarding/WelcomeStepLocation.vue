<script setup lang="ts">
import { computed, ref } from 'vue'
import { MapPin } from '@lucide/vue'

import PermissionPrompt from '@/components/PermissionPrompt.vue'
import { blockedGuidance } from '@/config/permissions'
import { useLocationStore } from '@/stores/location.store'

// The location step of onboarding (PRD 005 §5 step 4).
//
// The explanation comes **before** the native dialog, always: PRD 005 §6 forbids requesting
// geolocation before an explanation screen has been shown, and a declined system prompt is
// close to unrecoverable on iOS — the browser will not ask again, so the only way back is
// through Settings.
//
// The copy is task 085's, deliberately not a second version of it. That wording was written
// once for the map's prompt because the honest description of what is shared ("din rute
// gemmes og sendes til arrangørerne") is bigger than a permission dialog implies, and it
// points at the privacy page for the rest. Two texts for the same request would drift, and
// the one on the less-visited screen would be the stale one.
//
// Granted or denied, the flow continues. Only login is mandatory (PRD 005 §6).

const location = useLocationStore()
const emit = defineEmits<{ done: []; skip: [] }>()

const busy = ref(false)

// Already denied at OS level: no dialog will ever appear again, so calling into the API
// would fail silently and an "allow" button would be a dead end. Show the platform's
// settings path (task 101) instead and let the user move on.
const blocked = computed(() => location.permission === 'denied')

async function accept() {
  if (busy.value) return
  busy.value = true
  try {
    await location.request()
  } finally {
    busy.value = false
    // Emitted whether the fix succeeded or the user said no: the step's job is to have
    // *asked*, and the store's resolved permission is what the flow reads next.
    emit('done')
  }
}
</script>

<template>
  <PermissionPrompt
    variant="page"
    title="Vis din placering"
    message="Appen viser dig på kortet og gemmer din rute, som sendes til arrangørerne. Du kan altid slå det fra igen."
    cta="Slå placering til"
    :icon="MapPin"
    :more-to="{ name: 'privacy' }"
    more-label="Hvad gemmer I?"
    dismiss-label="Spring over"
    :blocked="blocked"
    :blocked-guidance="blockedGuidance('location')"
    @accept="accept"
    @dismiss="emit('skip')"
  />
</template>
