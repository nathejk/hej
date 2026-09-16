<script setup lang="ts">
import { ref } from 'vue'
import { RefreshCw } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { refreshNow, type SyncStatus } from '@/composables/useSyncLoop'
import { useAppStore } from '@/stores/app.store'

// The manual refresh control (PRD 017 §7, task 282).
//
// # Why it exists at all, given the app checks on every foreground
//
// The interval decides how fresh the app is when nobody is asking; this decides how fresh it is when
// somebody is. That is what makes the interval's exact value low-stakes: a patrol who suspects the
// drawer is behind can settle the question in one tap instead of waiting out a tick they cannot see.
//
// # It must say something even when nothing changed
//
// Nothing-changed is the *expected* answer — the whole endpoint is built around it being the common
// case. A control that goes quiet then reads as broken and gets tapped again, which is the traffic the
// debounce exists to absorb. So every outcome gets a sentence, including the reassuring one.
//
// # Forced, but not privileged
//
// It goes through the app's one sync loop (`refreshNow`), so there is a single path to `/api/sync`
// rather than a second one that drifts. Forcing skips the debounce only: a tap while a check is
// already in flight is reported as such rather than doubling the request on a link that is evidently
// already slow.

const app = useAppStore()

const busy = ref(false)
const message = ref('')

// Danish, and deliberately about **the action, not the connection**.
//
// "You are offline" belongs to `OfflineNotice` and nowhere else (PRD 009 §6, task 188, and there is a
// structural test that enforces it). During this event a participant may be without signal for hours,
// so a second component saying the same thing in different words is not a cosmetic problem — and the
// two can disagree, one claiming offline while the other has already recovered. This control therefore
// says only what happened to *the refresh*; the banner above it says why.
const messages: Record<SyncStatus, string> = {
  refreshed: 'Opdateret.',
  unchanged: 'Alt er opdateret.',
  offline: 'Kunne ikke opdatere.',
  error: 'Kunne ikke opdatere. Prøv igen.',
  unauthenticated: 'Du er blevet logget ud.',
  skipped: 'Opdaterer allerede …',
}

let clearTimer: number | null = null

function show(status: SyncStatus) {
  message.value = messages[status]
  if (clearTimer !== null) window.clearTimeout(clearTimer)
  // Long enough to read, short enough not to become furniture. Cleared rather than left, so the pane
  // does not carry a stale claim about a check from ten minutes ago.
  clearTimer = window.setTimeout(() => {
    message.value = ''
    clearTimer = null
  }, 4000)
}

async function onClick() {
  if (busy.value) return

  // Answered without a request: the browser's own offline signal is a hint rather than the truth, but
  // when it says we are offline it is right often enough that a doomed request is worse than the
  // sentence. The loop's reconnect trigger picks this up the moment signal returns — and `OfflineNotice`
  // is already on screen saying why, which is why the message here does not repeat it.
  if (!app.online) {
    show('offline')
    return
  }

  busy.value = true
  message.value = ''
  try {
    show((await refreshNow()).status)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex items-center gap-2">
    <Button
      variant="ghost"
      size="sm"
      :disabled="busy"
      :aria-label="busy ? 'Opdaterer' : 'Opdatér'"
      @click="onClick"
    >
      <!-- The icon spins while a check is in flight; the live region below carries the outcome for
           anyone who cannot see it spin. -->
      <RefreshCw class="size-4" :class="busy && 'animate-spin'" />
      <span class="sr-only">Opdatér</span>
    </Button>
    <!-- Polite rather than assertive: this never interrupts what the user is doing, and the common
         outcome is deliberately unremarkable. -->
    <p v-if="message" aria-live="polite" class="text-muted-foreground text-xs">
      {{ message }}
    </p>
  </div>
</template>
