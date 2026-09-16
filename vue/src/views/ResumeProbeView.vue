<script setup lang="ts">
// The resume probe (task 280, PRD 017 §11 *Decided*).
//
// WHY IT EXISTS. `useFreshnessLoop` listens to `visibilitychange` and `online`. On an installed
// home-screen PWA — which is how this app is actually used — returning from the iOS app switcher, or
// from a bfcache restore, may not present as either. If it does not, PRD 017 *appears* to work: fine on
// a cold start, fine on a tab switch, and silently stale on the most common resume path during an
// event. That question cannot be answered from a laptop, so this page answers it on the phone and
// renders the answer as Markdown to paste back into the task.
//
// Unlisted, like `/sporing` (task 082): a measurement tool, not a feature, reached from the tracking
// diagnostic rather than from the nav.
//
// THE LOG IS WRITTEN SYNCHRONOUSLY AND PERSISTED ON EVERY EVENT. Not an optimisation to skip: iOS may
// kill an app while it is hidden, and the paths worth measuring are exactly the ones where that
// happens. An in-memory log would lose the evidence for the case it was built to catch.
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Copy, Check, Trash2 } from '@lucide/vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  LOOP_EVENTS,
  PROBED_EVENTS,
  groupByMark,
  toMarkdown,
  verdictFor,
  verdictLabel,
  wouldCheck,
  type ProbeEntry,
  type ProbedEvent,
} from '@/helpers/resumeProbe'

const STORAGE_KEY = 'hej.resumeProbe.v1'

// The paths from task 280, in the order a tester should walk them. The last one is the control: a cold
// start must show up as such, or the log cannot be trusted for the others.
const PATHS = [
  'lås / lås op',
  'app-skifter',
  'ekstern link + tilbage (bfcache)',
  'telefonopkald',
  'koldstart (kontrol)',
]

const entries = ref<ProbeEntry[]>([])
const mark = ref<string>(PATHS[0])
const copied = ref(false)

function read(): ProbeEntry[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as ProbeEntry[]) : []
  } catch {
    return []
  }
}

function write(next: ProbeEntry[]) {
  try {
    // Capped, so a page left open for an hour cannot fill the quota the offline map needs.
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next.slice(-400)))
  } catch {
    // Quota or a privacy mode. The in-memory list still works for a session; nothing here is worth
    // failing the page over.
  }
}

function record(event: ProbedEvent, persisted?: boolean) {
  const entry: ProbeEntry = {
    at: Date.now(),
    event,
    visibility: typeof document === 'undefined' ? 'unknown' : document.visibilityState,
    mark: mark.value,
  }
  if (persisted !== undefined) entry.persisted = persisted
  const next = [...entries.value, entry]
  entries.value = next
  // Written inside the handler rather than in a watcher: `pagehide` may be the last code that runs
  // before the app is discarded, and a deferred write would not survive it.
  write(next)
}

const listeners: (() => void)[] = []

function listen(targetName: 'window' | 'document', event: ProbedEvent) {
  const target: EventTarget = targetName === 'window' ? window : document
  const handler = (e: Event) => {
    // `persisted` is the whole point of pageshow/pagehide here: it distinguishes a bfcache restore
    // from an ordinary load, and a bfcache restore is one of the paths under test.
    const persisted =
      event === 'pageshow' || event === 'pagehide'
        ? (e as PageTransitionEvent).persisted
        : undefined
    record(event, persisted)
  }
  target.addEventListener(event, handler)
  listeners.push(() => target.removeEventListener(event, handler))
}

onMounted(() => {
  entries.value = read()

  // `visibilitychange` is on document; the rest are on window. `freeze`/`resume` are Page Lifecycle
  // events that Chrome fires and Safari does not — their absence is itself a finding, so they are
  // registered rather than feature-detected away.
  listen('document', 'visibilitychange')
  for (const event of PROBED_EVENTS) {
    if (event === 'visibilitychange') continue
    listen('window', event)
  }

  // A mount is a resume too — and on the cold-start control it is the *only* signal, which is exactly
  // what makes it the control.
  record('pageshow', false)
})

onBeforeUnmount(() => listeners.forEach((off) => off()))

const groups = computed(() => groupByMark(entries.value))

const platform = computed(() =>
  [
    navigator.userAgent,
    `standalone=${window.matchMedia('(display-mode: standalone)').matches}`,
    // iOS Safari's own flag, which is how an installed iOS PWA actually identifies itself.
    `iosStandalone=${'standalone' in navigator ? String((navigator as { standalone?: boolean }).standalone) : 'n/a'}`,
  ].join(' · '),
)

async function copy() {
  try {
    await navigator.clipboard.writeText(toMarkdown(entries.value, platform.value))
    copied.value = true
    window.setTimeout(() => (copied.value = false), 2000)
  } catch {
    copied.value = false
  }
}

function clear() {
  entries.value = []
  write([])
}

const clock = (ms: number) =>
  new Date(ms).toLocaleTimeString('da-DK', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
</script>

<template>
  <div class="space-y-6 px-4 pt-4 pb-8">
    <header>
      <h1 class="font-nathejk text-2xl text-slate-900">Genoptagelse</h1>
      <p class="mt-1 text-sm text-slate-500">
        Diagnostik til opgave 280: hvilke hændelser fyrer, når appen kommer frem igen?
      </p>
    </header>

    <Alert>
      <AlertTitle>Sådan bruges siden</AlertTitle>
      <AlertDescription>
        <ol class="list-decimal space-y-1 pl-4 text-sm">
          <li>Vælg hvilken vej du tester nedenfor.</li>
          <li>Forlad appen den vej — og kom tilbage.</li>
          <li>Læs om loopet ville have tjekket. Gentag for hver vej.</li>
          <li>Tryk «Kopiér» og indsæt i opgavens log.</li>
        </ol>
      </AlertDescription>
    </Alert>

    <section>
      <h2 class="text-xs font-semibold uppercase tracking-wide text-slate-500">Vej der testes</h2>
      <div class="mt-2 flex flex-wrap gap-2">
        <Button
          v-for="p in PATHS"
          :key="p"
          :variant="mark === p ? 'default' : 'outline'"
          size="sm"
          @click="mark = p"
        >
          {{ p }}
        </Button>
      </div>
      <!-- The external link is here rather than in the instructions because the bfcache path needs a
           real cross-document navigation to come back from, and typing a URL into an installed PWA is
           not possible. -->
      <p class="mt-3 text-sm text-slate-500">
        Til bfcache-testen:
        <a href="https://nathejk.dk" class="underline">åbn nathejk.dk</a> og gå tilbage.
      </p>
    </section>

    <section>
      <div class="flex items-center justify-between gap-2">
        <h2 class="text-xs font-semibold uppercase tracking-wide text-slate-500">
          Optagelse ({{ entries.length }})
        </h2>
        <div class="flex gap-1">
          <Button variant="ghost" size="sm" @click="copy">
            <Check v-if="copied" class="size-4" />
            <Copy v-else class="size-4" />
            {{ copied ? 'Kopieret' : 'Kopiér' }}
          </Button>
          <Button variant="ghost" size="sm" @click="clear">
            <Trash2 class="size-4" />
            Ryd
          </Button>
        </div>
      </div>

      <p v-if="!entries.length" class="mt-2 text-sm text-slate-500">Ingen hændelser endnu.</p>

      <div v-for="(group, i) in groups" :key="i" class="mt-4 rounded-xl border border-slate-200 bg-white p-3">
        <p class="text-sm font-semibold text-slate-900">{{ group.mark }}</p>
        <!-- The verdict, not just the rows: "events fired but the loop hears none of them" is the
             finding that would make PRD 017 silently stale, and it should not have to be spotted by
             reading a table. -->
        <p
          class="mt-0.5 text-xs"
          :class="verdictFor(group.entries) === 'checked' ? 'text-emerald-700' : 'text-amber-700'"
        >
          {{ verdictLabel(verdictFor(group.entries)) }}
        </p>
        <table class="mt-2 w-full text-left text-xs tabular-nums">
          <thead class="text-slate-500">
            <tr>
              <th class="py-1 pr-2 font-medium">tid</th>
              <th class="py-1 pr-2 font-medium">event</th>
              <th class="py-1 pr-2 font-medium">vis.</th>
              <th class="py-1 pr-2 font-medium">bfcache</th>
              <th class="py-1 font-medium">loop</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(entry, j) in group.entries" :key="j" class="border-t border-slate-100">
              <td class="py-1 pr-2 text-slate-500">{{ clock(entry.at) }}</td>
              <td class="py-1 pr-2 font-mono">{{ entry.event }}</td>
              <td class="py-1 pr-2 text-slate-500">{{ entry.visibility }}</td>
              <td class="py-1 pr-2 text-slate-500">
                {{ entry.persisted === undefined ? '—' : entry.persisted }}
              </td>
              <td class="py-1" :class="wouldCheck(entry) ? 'font-semibold text-emerald-700' : 'text-slate-400'">
                {{ wouldCheck(entry) ? 'ja' : 'nej' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="rounded-xl border border-slate-200 bg-slate-50 p-3">
      <h2 class="text-xs font-semibold uppercase tracking-wide text-slate-500">Loopet lytter på</h2>
      <p class="mt-1 font-mono text-xs text-slate-700">{{ LOOP_EVENTS.join(', ') }}</p>
      <p class="mt-2 text-xs text-slate-500">
        Kun disse tæller som et tjek — og <code>visibilitychange</code> kun når dokumentet bliver
        synligt.
      </p>
      <p class="mt-2 break-all font-mono text-[10px] text-slate-400">{{ platform }}</p>
    </section>
  </div>
</template>
