<script setup lang="ts">
// The Team-section moderation queue (PRD 019 §6, §7, task 309).
//
// # Not a separate tool
//
// Team-section members are participants too: in the app, on their phones, at the event, at 03:00. A
// separate admin console would mean the person best placed to act on a report is the person least
// likely to be at a laptop. So this is a route in the same PWA, reusing `GlimtCard` — which is why
// that component takes an `actions` prop and two slots rather than a `moderating` flag. A forked
// moderation card would have drifted from the feed's within a week.
//
// # This is the only screen in the app that shows who posted a glimt
//
// Everywhere else a glimt is attributed to its hold and the author is projected out of the response
// entirely (PRD 019 §0b), so it cannot leak because it is not there. Here it is deliberate: a report
// cannot be answered against an anonymous author. The consequences are handled in the store — the
// queue lives in `moderationQueue`, which `persist()` never writes, and is dropped on unmount — and
// asserted by `glimtModerationNotCached.spec.ts`.
//
// # The client gate is convenience, not control
//
// The route checks `session.moderatesGlimt` and this view checks it again, but neither is the
// boundary: all three moderation endpoints re-read the caller's current Team-section assignment per
// request (task 300/308), so revoking it revokes the power on the next request no matter what this
// bundle believes. `moderationForbidden` exists precisely for that case.
//
// # Nothing is re-sorted, and nothing is removed
//
// The BFF returns triage order: reported-and-not-yet-hidden first, then by report count, then newest.
// A client-side sort would be a second opinion for the first to disagree with, and it would make a
// card jump out from under a moderator's thumb the moment they hid it. Hidden glimt stay listed —
// a reversal must be possible, and a malicious report is only visible if its target still is.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Loader2, ShieldCheck } from '@lucide/vue'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import GlimtCard from '@/components/glimt/GlimtCard.vue'
import GlimtViewer from '@/components/glimt/GlimtViewer.vue'
import { moderationActions } from '@/components/glimt/glimtPresentation'
import type { GlimtAction } from '@/components/glimt/glimtPresentation'
import { useOpenGlimt } from '@/composables/useOpenGlimt'
import { useSessionStore } from '@/stores/session.store'
import { useGlimtStore, type ModerationGlimt } from '@/stores/glimt.store'

const router = useRouter()
const session = useSessionStore()
const glimt = useGlimtStore()

const viewer = useOpenGlimt()

// Hiding gets a confirmation; restoring does not.
//
// Asymmetric on purpose. *Skjul* takes a participant's photograph away from everyone who could see
// it, and a dropdown item is one tap — while *Vis igen* only undoes that, so a misclick on it is
// repaired by the action that is already right there. The point of hiding being cheap (PRD 019 §6) is
// that it is *reversible*, not that it is unconsidered.
const pending = ref<ModerationGlimt | null>(null)
const working = ref(false)

const queue = computed(() => glimt.moderationQueue)
// Reported and still visible: what the queue exists to get through. Counted rather than filtered,
// because the list order already puts them first and a separate tab would hide the rest.
const outstanding = computed(
  () => queue.value.filter((g) => g.reportCount > 0 && !g.hidden).length,
)

onMounted(() => {
  glimt.fetchModeration()
})

// Author names must not outlive the screen. A Pinia store outlives the component, so without this
// they would sit in memory behind whatever page the moderator opened next.
onUnmounted(() => {
  glimt.clearModeration()
})

function act(entry: ModerationGlimt, key: GlimtAction['key']) {
  if (key === 'hide') {
    pending.value = entry
    return
  }
  if (key === 'unhide') {
    glimt.setHidden(entry.id, false)
    return
  }
  // `delete` reaches here only for a moderator's own glimt, where `moderationActions` offers the
  // author's action alongside. It is the author's power, not the moderator's.
  if (key === 'delete') glimt.remove(entry.id)
}

async function confirmHide() {
  const current = pending.value
  if (!current || working.value) return
  working.value = true
  try {
    await glimt.setHidden(current.id, true)
  } finally {
    working.value = false
    pending.value = null
  }
}

/** How a report count reads. Zero is not drawn at all — no badge is the absence of a problem. */
function reportLabel(count: number): string {
  return count === 1 ? '1 anmeldelse' : `${count} anmeldelser`
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-xl flex-col gap-3 p-3">
    <header class="flex items-center gap-2">
      <Button
        variant="ghost"
        size="icon"
        class="size-11 shrink-0"
        aria-label="Tilbage"
        @click="router.back()"
      >
        <ArrowLeft class="size-5" aria-hidden="true" />
      </Button>
      <div class="min-w-0">
        <h1 class="truncate font-nathejk text-2xl tracking-wide">Moderering</h1>
        <p v-if="queue.length" class="text-xs text-muted-foreground">
          {{ queue.length }} glimt<template v-if="outstanding">
            · {{ outstanding }} afventer</template
          >
        </p>
      </div>
    </header>

    <!-- The reach, stated on the screen that has it. PRD 019 §6 requires the Team section's ability
         to see every scope to be disclosed rather than discovered; the composer and the privacy page
         say so to members, and this says so to the moderator. -->
    <p class="flex items-start gap-2 rounded-md bg-muted px-3 py-2 text-xs text-muted-foreground">
      <ShieldCheck class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
      <span>
        Du ser alle glimt — også dem der kun er delt i en gruppe. Når du skjuler et glimt, bliver
        billederne ikke slettet, og du kan vise det igen.
      </span>
    </p>

    <!-- Not an error and not retryable: the assignment is gone, or was never there. The endpoint is
         the authority, so this is what an out-of-date bundle looks like when it finds out. -->
    <p
      v-if="glimt.moderationForbidden || !session.moderatesGlimt"
      class="rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground"
    >
      Du har ikke adgang til at moderere glimt.
    </p>

    <template v-else>
      <p
        v-if="glimt.error"
        class="rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground"
      >
        {{ glimt.error }}
      </p>

      <div v-if="glimt.loadingModeration && !queue.length" class="flex justify-center py-10">
        <Loader2 class="size-6 animate-spin text-muted-foreground" aria-hidden="true" />
      </div>

      <div
        v-else-if="!queue.length"
        class="flex flex-col items-center gap-2 rounded-lg border border-dashed px-6 py-10 text-center"
      >
        <ShieldCheck class="size-8 text-muted-foreground" aria-hidden="true" />
        <p class="text-sm text-muted-foreground">Ingen glimt at se på.</p>
      </div>

      <GlimtCard
        v-for="entry in queue"
        :key="entry.id"
        :glimt="entry"
        hide-hold-link
        :actions="moderationActions(entry)"
        @open="(ordinal) => viewer.open(entry, ordinal)"
        @action="(key) => act(entry, key)"
      >
        <!-- The author. The reason this response carries one at all: a report is about a person's
             conduct, and a uuid is not something a human can act on. Falls back to the id when the
             person row is missing, which is itself a case worth being able to see. -->
        <template #meta>
          <p class="truncate text-xs text-muted-foreground">
            {{ entry.authorName || entry.authorPersonId || 'Ukendt forfatter' }}
          </p>
        </template>

        <template #badges>
          <Badge v-if="entry.reportCount > 0" variant="destructive" class="shrink-0">
            {{ reportLabel(entry.reportCount) }}
          </Badge>
        </template>
      </GlimtCard>
    </template>

    <GlimtViewer
      v-model:open="viewer.isOpen.value"
      :glimt="viewer.glimt.value"
      :ordinal="viewer.ordinal.value"
    />

    <Dialog :open="pending !== null" @update:open="(o) => !o && (pending = null)">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Skjul glimtet?</DialogTitle>
          <DialogDescription>
            Glimtet forsvinder for alle. Billederne bliver ikke slettet, og du kan vise det igen.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" :disabled="working" @click="pending = null">Annuller</Button>
          <Button :disabled="working" @click="confirmHide">Skjul</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
