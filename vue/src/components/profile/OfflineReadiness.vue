<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { Check, CloudDownload, HardDrive, RefreshCw, Square, Trash2, TriangleAlert } from '@lucide/vue'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { OFFLINE_DATASETS, offlineDataset, type OfflineDatasetId } from '@/config/offline'
import { registerOfflineDatasets } from '@/helpers/offline/reporters'
import { useOfflineStore, type OfflineDatasetState } from '@/stores/offline.store'

// "Klar til offline" — the one place that answers "have I got what I need before I walk into the
// woods?" (PRD 009 §7, task 187).
//
// # Why one surface instead of per-feature indicators
//
// Readiness is a single question. Four independent progress bars cannot answer it, and the user
// asking it is usually standing in a car park with a few minutes of signal left.
//
// # It reports, it does not store
//
// Everything here comes from `offline.store`, which holds status only. Sync and clear are handler
// callbacks registered by whichever feature owns the storage, so a dataset with nothing to offer
// simply shows no button rather than a button that does nothing.
//
// # `TrackStatusView` is linked, not duplicated
//
// That page already does this in depth for the position track — gaps, upload backlog, diagnostics.
// Reproducing any of it here would give us two places to fix when the track's storage changes.

const offline = useOfflineStore()

// Re-measure when the page is opened, not only at startup. The service worker writes the tile and
// portrait caches without telling the page, so a figure from twenty minutes and one map session ago
// would be wrong in the one direction that matters: it would understate what the user has.
onMounted(() => {
  const api = typeof caches === 'undefined' ? undefined : caches
  void registerOfflineDatasets(api)
  void offline.refreshStorage(navigator.storage)
})

const nf = new Intl.NumberFormat('da-DK')

function bytes(value: number | null): string {
  if (value === null) return 'ukendt'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${nf.format(Math.round(value / 1024))} kB`
  return `${nf.format(Number((value / (1024 * 1024)).toFixed(1)))} MB`
}

// Relative rather than a timestamp: "for 2 timer siden" is what a participant actually wants to
// know at 03:00, and an absolute clock time invites the question "was that today?".
function since(at: number | null): string {
  if (!at) return 'aldrig'
  const seconds = Math.max(0, Math.round((Date.now() - at) / 1000))
  if (seconds < 60) return 'lige nu'
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `for ${minutes} min. siden`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `for ${hours} ${hours === 1 ? 'time' : 'timer'} siden`
  const days = Math.round(hours / 24)
  return `for ${days} ${days === 1 ? 'dag' : 'dage'} siden`
}

// Coarse on purpose: "om 12 dage" is what the sentence needs. An exact timestamp would invite the
// question of whether it is precise, which it is not — the deadline moves forward on every sync.
function until(at: number): string {
  const days = Math.round((at - Date.now()) / 86_400_000)
  if (days <= 0) return 'snart'
  if (days === 1) return 'i morgen'
  return `om ${days} dage`
}

// Every state gets words, and no state gets colour alone: this is read in bright sun and at 04:00,
// and a coloured dot is unreadable in both.
const STATE_COPY: Record<
  OfflineDatasetState,
  { label: string; variant: 'default' | 'secondary' | 'outline' | 'warning' | 'destructive' }
> = {
  unknown: { label: 'Ikke tjekket', variant: 'outline' },
  empty: { label: 'Mangler', variant: 'destructive' },
  syncing: { label: 'Hentes…', variant: 'secondary' },
  synced: { label: 'Klar', variant: 'default' },
  stale: { label: 'Kan være gammel', variant: 'warning' },
  // Deliberately not "Mangler". The user *had* this and the phone removed it, which is a different
  // thing to be told — and on iOS it is the normal way to lose a cache.
  evicted: { label: 'Slettet af telefonen', variant: 'warning' },
}

interface Row {
  id: OfflineDatasetId
  label: string
  state: OfflineDatasetState
  badge: { label: string; variant: 'default' | 'secondary' | 'outline' | 'warning' | 'destructive' }
  detail: string
  canSync: boolean
  canClear: boolean
  canCancel: boolean
  problem: 'quota' | 'offline' | null
}

const rows = computed<Row[]>(() =>
  OFFLINE_DATASETS.map((dataset) => {
    const status = offline.statuses[dataset.id]
    const parts: string[] = []

    if (status.bytes !== null) parts.push(bytes(status.bytes))
    if (status.itemCount !== null) parts.push(`${nf.format(status.itemCount)} stk.`)
    if (status.state !== 'unknown' && status.state !== 'empty') parts.push(since(status.syncedAt))
    // Stored, current and incomplete is its own thing: a map that covers part of the area is
    // useful, and calling it "Klar" would be the dishonesty this whole surface exists to avoid.
    if (status.complete === false && status.state === 'synced') parts.push('ikke komplet')
    // Named rather than hidden: a participant is owed the fact that this data removes itself, both
    // because it explains a pane that empties on its own and because "we do not keep this" is worth
    // saying out loud about other people's phone numbers and photographs.
    if (status.expiresAt) parts.push(`slettes ${until(status.expiresAt)}`)

    return {
      id: dataset.id,
      label: dataset.label,
      state: status.state,
      badge: STATE_COPY[status.state],
      detail: parts.join(' · ') || 'Ikke hentet endnu',
      canSync: Boolean(offline.handlers[dataset.id]?.sync),
      canCancel: Boolean(offline.handlers[dataset.id]?.cancel) && status.state === 'syncing',
      problem: status.problem,
      // Never offered for unrecoverable data. `offline.store.clear` refuses it too — belt and
      // braces, because this is the one button in the app whose bug is unrecoverable data loss.
      canClear: Boolean(offline.handlers[dataset.id]?.clear) && !dataset.unrecoverable,
    }
  }),
)

const totalBytes = computed(() => bytes(offline.reportedBytes || offline.usageBytes))
const usedShare = computed(() => {
  if (offline.usageBytes === null || !offline.quotaBytes) return null
  return Math.min(100, Math.round((offline.usageBytes / offline.quotaBytes) * 100))
})

const trackUnrecoverable = computed(() => offlineDataset('track').unrecoverable)
</script>

<template>
  <section>
    <h2 class="font-nathejk text-lg text-slate-900">Klar til offline</h2>

    <div class="mt-2 rounded-xl border border-slate-200 bg-white shadow-xs">
      <div class="flex items-start gap-3 border-b border-slate-100 px-4 py-3">
        <HardDrive class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
        <div class="min-w-0 flex-1">
          <div class="flex items-baseline justify-between gap-3">
            <p class="text-sm font-medium text-slate-800">
              {{ offline.ready ? 'Alt er hentet' : 'Noget mangler' }}
            </p>
            <Badge :variant="offline.ready ? 'default' : 'outline'">
              <Check v-if="offline.ready" aria-hidden="true" />
              {{ offline.ready ? 'Klar' : `${offline.missing.length} mangler` }}
            </Badge>
          </div>

          <p class="mt-1 text-xs text-slate-500">
            {{ totalBytes }} gemt på denne telefon<span v-if="usedShare !== null">
              · {{ usedShare }} % af telefonens plads til appen</span
            >
          </p>

          <Progress
            v-if="offline.syncing"
            class="mt-2"
            :model-value="offline.syncPercent ?? 0"
            aria-label="Henter data til offline brug"
          />
          <p v-if="offline.syncPercent !== null" class="mt-1 text-xs text-slate-500">
            {{ offline.syncPercent }} % hentet. Du kan lukke appen — det, der er hentet, bliver gemt.
          </p>

          <Button
            v-if="offline.syncable.length"
            class="mt-2"
            size="sm"
            :disabled="offline.syncing"
            @click="offline.prepareAll()"
          >
            <CloudDownload aria-hidden="true" />
            {{ offline.syncing ? 'Henter…' : 'Forbered til offline' }}
          </Button>

          <!-- The size goes *next to the button*, before anything is fetched. On iOS the app
               cannot tell WiFi from cellular, so this number is the whole consent mechanism for a
               download that can reach several hundred megabytes. -->
          <p v-if="offline.syncable.length && offline.pendingBytes > 0" class="mt-1 text-xs text-slate-500">
            Henter ca. {{ bytes(offline.pendingBytes) }}. Brug helst wi-fi.
          </p>
        </div>
      </div>

      <div class="divide-y divide-slate-100 px-4">
        <div v-for="row in rows" :key="row.id" class="flex items-start gap-3 py-3">
          <div class="min-w-0 flex-1">
            <div class="flex items-baseline justify-between gap-3">
              <p class="text-sm font-medium text-slate-800">{{ row.label }}</p>
              <Badge :variant="row.badge.variant">{{ row.badge.label }}</Badge>
            </div>
            <p class="mt-1 text-xs text-slate-500">{{ row.detail }}</p>

            <!-- Two causes, two different things to do about them. Silence would leave a download that
                 stopped at 60% looking like one that finished. -->
            <p v-if="row.problem === 'quota'" class="mt-1 text-xs text-amber-800">
              Der er ikke mere plads på telefonen. Det, der er hentet, virker stadig — slet noget andet,
              hvis du vil have resten af kortet med.
            </p>
            <p v-else-if="row.problem === 'offline'" class="mt-1 text-xs text-amber-800">
              Forbindelsen holdt ikke hele vejen. Prøv igen, når du har wi-fi — det hentede beholdes.
            </p>

            <div v-if="row.canSync || row.canClear || row.canCancel" class="mt-2 flex gap-2">
              <Button
                v-if="row.canSync"
                size="sm"
                variant="outline"
                :disabled="row.state === 'syncing'"
                @click="offline.sync(row.id)"
              >
                <RefreshCw aria-hidden="true" />
                Hent nu
              </Button>
              <!-- A progress bar with no way out of it is a trap: the tile download can run for minutes
                   on rural mobile data. Stopping keeps every tile already fetched. -->
              <Button v-if="row.canCancel" size="sm" variant="outline" @click="offline.cancel(row.id)">
                <Square aria-hidden="true" />
                Stop
              </Button>
              <Button
                v-if="row.canClear"
                size="sm"
                variant="ghost"
                @click="offline.clear(row.id)"
              >
                <Trash2 aria-hidden="true" />
                Slet
              </Button>
            </div>
          </div>
        </div>
      </div>

      <!-- The only persistence state worth a sentence. 'unsupported' is deliberately silent:
           nothing the user can act on, and a warning about a decision no browser made is noise
           that teaches people to ignore this surface. -->
      <div v-if="offline.evictable" class="px-4 pb-3">
        <Alert variant="destructive">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>Telefonen kan slette det hentede</AlertTitle>
          <AlertDescription>
            Din browser vil ikke garantere pladsen. Åbn appen fra hjemmeskærmen og se ind i den
            en gang i ugen op til løbet — så er der størst chance for, at kortet stadig er der.
          </AlertDescription>
        </Alert>
      </div>

      <!-- Linked, not duplicated: that page owns gaps, backlog and diagnostics for the track. -->
      <div v-if="trackUnrecoverable" class="border-t border-slate-100 px-4 py-3">
        <RouterLink to="/sporing" class="text-xs text-slate-500 underline">
          Se detaljer om din gemte rute
        </RouterLink>
      </div>
    </div>
  </section>
</template>
