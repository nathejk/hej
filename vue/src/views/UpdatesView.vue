<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Megaphone, Bell } from '@lucide/vue'
import PagePlaceholder from '@/components/PagePlaceholder.vue'
import PermissionPrompt from '@/components/PermissionPrompt.vue'
import { useNotificationsStore } from '@/stores/notifications.store'

const notifications = useNotificationsStore()
const DISMISS_KEY = 'hej.prompt.notifications.dismissed'
const dismissed = ref(localStorage.getItem(DISMISS_KEY) === '1')

onMounted(async () => {
  notifications.syncPermission()
  // Same order as the profile page, and for the same reason: the server key has to be
  // known before a subscription can be judged stale (see notifications.store).
  //
  // The prompt below hides once `subscribed` is true, so without this a subscription
  // bound to a rotated key would keep it hidden while push quietly did nothing.
  await notifications.syncConfigured()
  await notifications.syncSubscription()
})

const showPrompt = computed(
  () =>
    notifications.available &&
    !dismissed.value &&
    !notifications.subscribed &&
    (notifications.permission === 'unknown' || notifications.permission === 'default'),
)

async function accept() {
  await notifications.enable()
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
      title="Få besked om nyheder"
      message="Slå notifikationer til, så du får de vigtige opdateringer under løbet direkte på telefonen."
      cta="Slå notifikationer til"
      :icon="Bell"
      @accept="accept"
      @dismiss="dismiss"
    />
    <PagePlaceholder class="flex-1" title="Nyt" :icon="Megaphone" />
  </div>
</template>
