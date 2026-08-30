<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Bell } from '@lucide/vue'

import PermissionPrompt from '@/components/PermissionPrompt.vue'
import { blockedGuidance } from '@/config/permissions'
import { useNotificationsStore } from '@/stores/notifications.store'

// The notifications step of onboarding (PRD 005 §5 step 5).
//
// Explanation first, then `notifications.enable()`, which does permission + push
// subscription + POST to the BFF in one action. Nothing is requested before the explanation
// (PRD 005 §6).
//
// Three states this step must not treat as failures:
//
// - **Unavailable.** On iOS below 16.4 there is no Web Push for home-screen apps at all.
//   That is outside the browser baseline in `.rules`, so the step reports `unavailable` and
//   is skipped rather than shown as something the user got wrong (PRD 005 §5).
// - **Blocked.** Permission already denied at OS level: no dialog will reappear, so show
//   task 101's settings path instead of a button that cannot work.
// - **Server not configured.** Found on a real device (task 108/115): permission granted, no
//   VAPID keys on the server, and a subscribe button that silently did nothing. If the server
//   is not ready, say so plainly — it is not the member's phone that is wrong.
//
// Declining is fine. The profile page (PRD 003) is where a member can turn this on later.

const notifications = useNotificationsStore()
const emit = defineEmits<{ done: []; skip: [] }>()

const busy = ref(false)

onMounted(() => {
  // Cheap, non-throwing, and it decides which of the states below we are in — including
  // whether the server has push set up at all.
  notifications.syncPermission()
  void notifications.syncConfigured()
  // Also the live subscription, because a granted permission is not the question this step
  // exists to answer — "is there a subscription registered with the BFF?" is (task 144). A
  // member who already has one has nothing to do here; one who does not needs this step even
  // though the permission prompt will never appear for them.
  void notifications.syncSubscription()
})

const unavailable = computed(
  () => !notifications.available || notifications.permission === 'unavailable',
)
const blocked = computed(() => notifications.permission === 'denied')
const serverNotReady = computed(() => notifications.configured === false)

// What replaces the accept button when pressing it provably cannot work. Two different
// reasons, two different sentences — an empty one would render as a blank gap where the
// button used to be.
const guidance = computed(() => {
  if (blocked.value) return blockedGuidance('notifications')
  if (serverNotReady.value) return 'Du kan slå beskeder til senere under Min profil.'
  return ''
})

const message = computed(() => {
  if (serverNotReady.value) {
    return 'Nathejk har ikke sat beskeder op endnu. Det er ikke din telefon, der er noget i vejen med — prøv igen senere under Min profil.'
  }
  return 'Nathejk sender beskeder til dig under løbet — ændringer, aflysninger og vigtige ting du skal vide. Det er den hurtigste måde at nå dig i skoven.'
})

async function accept() {
  if (busy.value) return
  busy.value = true
  try {
    await notifications.enable()
  } finally {
    busy.value = false
    emit('done')
  }
}
</script>

<template>
  <!--
    Unavailable: nothing to ask for. Reported as a fact about the device rather than as a
    step the user failed, and the flow moves on.
  -->
  <div v-if="unavailable" class="flex h-full flex-col justify-center gap-6 text-center">
    <div class="flex flex-col items-center gap-4">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <Bell class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">Beskeder fra Nathejk</h1>
      <p class="text-sm leading-relaxed text-slate-600">
        Denne telefon kan ikke modtage beskeder fra appen. Du får dem i stedet fra din leder
        eller i startområdet.
      </p>
    </div>
    <button type="button" class="px-2 py-2 text-sm text-slate-500" @click="emit('done')">
      Videre
    </button>
  </div>

  <PermissionPrompt
    v-else
    variant="page"
    title="Beskeder fra Nathejk"
    :message="message"
    cta="Slå beskeder til"
    :icon="Bell"
    dismiss-label="Spring over"
    :blocked="blocked || serverNotReady"
    :blocked-guidance="guidance"
    @accept="accept"
    @dismiss="emit('skip')"
  />
</template>
