<script setup lang="ts">
// Glimt — the moments shared with you (PRD 019, task 316).
//
// # Not fullBleed
//
// The feed scrolls, and `fullBleed` drops App.vue's `overflow-y-auto` wrapper. The map wants that;
// a list does not.
//
// # Freshness is the app-level loop's job
//
// This view hydrates the cached copy so it draws immediately — cold, offline, on a slow link — and
// then leaves refreshing alone. `/api/sync` carries a `glimt` key and `useSyncLoop` dispatches to
// the store (PRD 017). A pane-level loop here would be the mistake `useFreshnessLoop.ts` documents:
// two loops polling the same dataset on the same triggers.
//
// # The empty state is the front door
//
// For a spejder opening Glimt before anything exists there is nothing to show and everything to
// invite. Spejdere have almost nothing in this app that is theirs (PRD 019 §2), so an empty feed
// that only said "ingen glimt" would waste the one moment this feature has to explain itself.
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Camera, CloudUpload, WifiOff } from '@lucide/vue'

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
import GlimtComposer from '@/components/glimt/GlimtComposer.vue'
import type { GlimtAction } from '@/components/glimt/glimtPresentation'
import { useGlimtStore, type Glimt } from '@/stores/glimt.store'

const router = useRouter()
const glimt = useGlimtStore()

// The hold collection is a separate task (326) and registers its own route. Checked rather than
// assumed, so this view works in a build where it does not exist yet and gains the link
// automatically when it lands — instead of throwing, which is what `router.push({ name })` does on an
// unregistered name.
const canBrowseHolds = computed(() => router.hasRoute('glimt-hold'))

// The composer is a drawer rather than a route (PRD 019 §7): the feed stays behind it, so a member
// who changes their mind is back where they were rather than navigated somewhere and then back.
const composerOpen = ref(false)

// Both destructive-ish actions get a confirmation, for different reasons: a delete cannot be undone
// by its author, and a report takes somebody else's photograph off the public feed immediately. A
// dropdown item is one tap and neither of those should be.
const pending = ref<{ entry: Glimt; key: GlimtAction['key'] } | null>(null)
const working = ref(false)

const confirmCopy = computed(() => {
  if (pending.value?.key === 'delete') {
    return {
      title: 'Slet glimtet?',
      body: 'Billederne bliver slettet. Det kan ikke fortrydes.',
      confirm: 'Slet',
      destructive: true,
    }
  }
  return {
    title: 'Anmeld glimtet?',
    body: 'Glimtet bliver skjult med det samme, og Team ser det og tager stilling.',
    confirm: 'Anmeld',
    destructive: false,
  }
})

onMounted(() => {
  glimt.hydrate()
  // How many posts are waiting, so the notice below is honest on a cold start too. The *sending* is
  // the sync loop's job (task 314) — this only reads the count.
  glimt.refreshPending()
})

function openHold(number: string) {
  if (canBrowseHolds.value) router.push({ name: 'glimt-hold', params: { number } })
}

async function confirmAction() {
  const current = pending.value
  if (!current || working.value) return
  working.value = true
  try {
    if (current.key === 'delete') await glimt.remove(current.entry.id)
    else await glimt.report(current.entry.id)
  } finally {
    working.value = false
    pending.value = null
  }
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-xl flex-col gap-3 p-3">
    <header class="flex items-center justify-between gap-2">
      <h1 class="font-nathejk text-2xl tracking-wide">Glimt</h1>
      <Button size="lg" class="gap-2" @click="composerOpen = true">
        <Camera class="size-5" aria-hidden="true" />
        Del et glimt
      </Button>
    </header>

    <!-- Queued posts. Deliberately worded as waiting for the network rather than "sender i
         baggrunden": there is no background upload on iOS, so a post moves when the app is open and
         claiming otherwise would misplace somebody's photographs (PRD 019 §5, task 314). -->
    <p
      v-if="glimt.pending > 0"
      class="flex items-center gap-2 rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground"
    >
      <CloudUpload class="size-4 shrink-0" aria-hidden="true" />
      {{
        glimt.pending === 1
          ? 'Et glimt venter på nettet.'
          : `${glimt.pending} glimt venter på nettet.`
      }}
    </p>

    <!-- Offline or a failed refresh: the cached copy is still shown, so this is a notice rather than
         an error screen. -->
    <p
      v-if="glimt.error"
      class="flex items-center gap-2 rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground"
    >
      <WifiOff class="size-4 shrink-0" aria-hidden="true" />
      {{ glimt.error }}
    </p>

    <!-- The BFF answers 403 for a caller with no feed. Nothing to retry and nothing to apologise
         for, so nothing is drawn. -->
    <template v-if="!glimt.forbidden">
      <div
        v-if="!glimt.hasCopy && glimt.hydrated"
        class="flex flex-col items-center gap-3 rounded-lg border border-dashed px-6 py-10 text-center"
      >
        <Camera class="size-8 text-muted-foreground" aria-hidden="true" />
        <h2 class="font-nathejk text-xl tracking-wide">Ingen glimt endnu</h2>
        <p class="max-w-sm text-sm text-muted-foreground">
          Del et øjeblik fra løbet med dit hold, med hele Nathejk — eller offentligt, så forældre
          kan følge med.
        </p>
        <Button size="lg" class="gap-2" @click="composerOpen = true">
          <Camera class="size-5" aria-hidden="true" />
          Del det første glimt
        </Button>
      </div>

      <GlimtCard
        v-for="entry in glimt.newestFirst"
        :key="entry.id"
        :glimt="entry"
        @open-hold="openHold"
        @action="(key) => (pending = { entry, key })"
      />
    </template>

    <GlimtComposer v-model:open="composerOpen" />

    <Dialog :open="pending !== null" @update:open="(o) => !o && (pending = null)">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ confirmCopy.title }}</DialogTitle>
          <DialogDescription>{{ confirmCopy.body }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" :disabled="working" @click="pending = null">Annuller</Button>
          <Button
            :variant="confirmCopy.destructive ? 'destructive' : 'default'"
            :disabled="working"
            @click="confirmAction"
          >
            {{ confirmCopy.confirm }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
