<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Megaphone, Bell } from '@lucide/vue'
import PagePlaceholder from '@/components/PagePlaceholder.vue'
import PermissionPrompt from '@/components/PermissionPrompt.vue'
import { useNotificationsStore } from '@/stores/notifications.store'

const notifications = useNotificationsStore()
const DISMISS_KEY = 'hej.prompt.notifications.dismissed'
const dismissed = ref(localStorage.getItem(DISMISS_KEY) === '1')

onMounted(() => {
  notifications.syncPermission()
  // The prompt below is hidden once the user is subscribed, so it has to know
  // whether a subscription survives from an earlier visit (task 100) — otherwise a
  // subscribed user is asked to enable push again after every reload.
  void notifications.syncSubscription()
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
