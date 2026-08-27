<script setup lang="ts">
// Diagnostic status page for the position track (task 082).
//
// WHY IT EXISTS. Two of task 082's criteria can only be answered on a real phone: what
// happens to recording when the app is backgrounded or the screen locks, and what it
// costs in battery. Neither is observable from a laptop, and neither is something a
// maintainer should have to read out of a debugger — so this page renders the answer as
// text that can be copied and pasted straight back into a task log.
//
// The important measurement is the GAP ANALYSIS. A web app cannot record while
// backgrounded on any platform; what nobody knows without a device is how much that
// actually costs. Every gap longer than one sampling interval is a period the app was not
// running, and the hidden/visible events either side of it say why.
import { computed, onMounted, ref } from 'vue'
import { Copy, Check, RefreshCw } from '@lucide/vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { listEvents, listPoints, type TrackEvent, type TrackPoint } from '@/helpers/trackDb'
import { TRACK_SAMPLE_SECONDS } from '@/config/track'
import { useTrackStore } from '@/stores/track.store'
import { useLocationStore } from '@/stores/location.store'
import { useSessionStore } from '@/stores/session.store'
import { useAppStore } from '@/stores/app.store'

const track = useTrackStore()
const location = useLocationStore()
const session = useSessionStore()
const app = useAppStore()

const points = ref<TrackPoint[]>([])
const events = ref<TrackEvent[]>([])
const persisted = ref<boolean | null>(null)
const estimate = ref<StorageEstimate | null>(null)
const loading = ref(true)
const copied = ref(false)

const clock = (ms: number) =>
  new Date(ms).toLocaleTimeString('da-DK', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
const dur = (ms: number) => {
  const s = Math.round(ms / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ${s % 60}s`
  return `${Math.floor(m / 60)}t ${m % 60}m`
}
const mb = (bytes?: number) => (bytes === undefined ? '?' : `${(bytes / 1e6).toFixed(1)} MB`)

async function load() {
  loading.value = true
  try {
    const [p, e] = await Promise.all([listPoints(), listEvents()])
    points.value = p.sort((a, b) => a.ts - b.ts)
    events.value = e.sort((a, b) => a.at - b.at)
    if (navigator.storage?.persisted) persisted.value = await navigator.storage.persisted()
    if (navigator.storage?.estimate) estimate.value = await navigator.storage.estimate()
  } finally {
    loading.value = false
  }
  void track.refreshCount()
}
onMounted(load)

// A gap is anything longer than twice the sampling interval — one missed sample can be a
// slow fix, but two means the app was not running.
const gapThresholdMs = TRACK_SAMPLE_SECONDS * 1000 * 2

const analysis = computed(() => {
  const ts = points.value.map((p) => p.ts)
  if (ts.length < 2) {
    return { span: 0, gaps: [] as { from: number; to: number; ms: number }[], covered: 0, expected: 0 }
  }
  const gaps: { from: number; to: number; ms: number }[] = []
  let covered = 0
  for (let i = 1; i < ts.length; i++) {
    const ms = ts[i] - ts[i - 1]
    if (ms > gapThresholdMs) gaps.push({ from: ts[i - 1], to: ts[i], ms })
    else covered += ms
  }
  const span = ts[ts.length - 1] - ts[0]
  return {
    span,
    gaps: gaps.sort((a, b) => b.ms - a.ms),
    covered,
    // How many points a perfect recorder would have taken over the same span.
    expected: Math.round(span / (TRACK_SAMPLE_SECONDS * 1000)) + 1,
  }
})

const accuracy = computed(() => {
  if (!points.value.length) return null
  const a = points.value.map((p) => p.accuracy).sort((x, y) => x - y)
  // Rounded: the browser reports full float precision (4.986577009122841 m), which is
  // noise in a report meant to be read.
  const r = (n: number) => Math.round(n * 10) / 10
  return { min: r(a[0]), median: r(a[Math.floor(a.length / 2)]), max: r(a[a.length - 1]) }
})

// How long the app spent suspended, paired from the event log. A `hidden` with no
// following `visible` means iOS killed the app rather than resuming it — worth counting
// separately, because that is the case that also loses the in-memory recorder.
const suspends = computed(() => {
  const spans: { from: number; to: number; ms: number }[] = []
  let killed = 0
  let openedAt: number | null = null
  for (const e of events.value) {
    if (e.kind === 'hidden') openedAt = e.at
    else if (e.kind === 'visible' && openedAt !== null) {
      spans.push({ from: openedAt, to: e.at, ms: e.at - openedAt })
      openedAt = null
    } else if (e.kind === 'load' && openedAt !== null) {
      killed += 1
      openedAt = null
    }
  }
  return { spans: spans.sort((a, b) => b.ms - a.ms), killed, total: spans.reduce((s, x) => s + x.ms, 0) }
})

// The report. Plain text on purpose: it is going into a task log or a chat message, and
// it has to survive being pasted with no formatting.
const report = computed(() => {
  const a = analysis.value
  const p = points.value
  const lines: string[] = []
  const L = (k: string, v: string | number | boolean) => lines.push(`${k}: ${v}`)

  lines.push('=== HEJ NATHEJK — SPORINGSSTATUS (task 082) ===')
  L('rapport', new Date().toISOString())
  L('app', __APP_VERSION__)
  lines.push('')
  lines.push('-- miljø --')
  L('standalone', window.matchMedia('(display-mode: standalone)').matches)
  L('platform', navigator.platform || '?')
  L('userAgent', navigator.userAgent)
  L('online', app.online)
  L('rolle', session.user?.role ?? '(ingen)')
  lines.push('')
  lines.push('-- optagelse --')
  L('optager nu', track.recording)
  L('placeringstilladelse', location.permission)
  L('problem', track.problem || '(ingen)')
  L('interval', `${TRACK_SAMPLE_SECONDS}s`)
  lines.push('')
  lines.push('-- lager --')
  L('punkter', p.length)
  L('storage.persisted()', persisted.value === null ? 'ikke understøttet' : persisted.value)
  L('forbrug', mb(estimate.value?.usage))
  L('kvote', mb(estimate.value?.quota))
  lines.push('')
  lines.push('-- dækning --')
  if (p.length >= 2) {
    L('første punkt', new Date(p[0].ts).toISOString())
    L('sidste punkt', new Date(p[p.length - 1].ts).toISOString())
    L('samlet periode', dur(a.span))
    L('punkter faktisk / forventet', `${p.length} / ${a.expected}`)
    L('dækning', `${Math.round((p.length / Math.max(a.expected, 1)) * 100)}%`)
    L(`huller (> ${(gapThresholdMs / 1000) | 0}s)`, a.gaps.length)
    L('tid i huller', dur(a.span - a.covered))
    a.gaps.slice(0, 15).forEach((g, i) => {
      lines.push(`  hul ${i + 1}: ${dur(g.ms)} — ${clock(g.from)} → ${clock(g.to)}`)
    })
  } else {
    lines.push('(for få punkter til at beregne dækning)')
  }
  if (accuracy.value) {
    L('nøjagtighed min/median/max', `${accuracy.value.min}/${accuracy.value.median}/${accuracy.value.max} m`)
  }
  lines.push('')
  lines.push('-- baggrund --')
  L('gange i baggrunden', suspends.value.spans.length + suspends.value.killed)
  L('heraf dræbt af iOS (ingen genoptagelse)', suspends.value.killed)
  L('samlet tid i baggrunden (målt)', dur(suspends.value.total))
  suspends.value.spans.slice(0, 10).forEach((s, i) => {
    lines.push(`  baggrund ${i + 1}: ${dur(s.ms)} — ${clock(s.from)} → ${clock(s.to)}`)
  })
  lines.push('')
  lines.push('-- hændelser: optalt over hele perioden --')
  {
    const counts = new Map<string, number>()
    events.value.forEach((e) => counts.set(e.kind, (counts.get(e.kind) ?? 0) + 1))
    ;[...counts.entries()].sort((a, b) => b[1] - a[1]).forEach(([k, n]) => L(k, n))
    // How often a point cost us a fix of our own versus reusing the map's. This is the
    // battery argument, measured.
    const pts = events.value.filter((e) => e.kind === 'point')
    const reused = pts.filter((e) => e.detail?.includes('reused=1')).length
    if (pts.length) L('punkter genbrugt kortets fix', `${reused}/${pts.length}`)
  }
  lines.push('')
  lines.push(`-- hændelser (seneste ${Math.min(events.value.length, 200)} af ${events.value.length}) --`)
  events.value.slice(-200).forEach((e) => {
    lines.push(`${new Date(e.at).toISOString()} ${e.kind}${e.detail ? ' — ' + e.detail : ''}`)
  })
  lines.push('')
  lines.push('-- batteri (udfyldes i hånden) --')
  lines.push('batteri før: ____%   batteri efter: ____%')
  lines.push('(iOS har ikke noget batteri-API, så det kan appen ikke selv læse)')
  lines.push('=== SLUT ===')
  return lines.join('\n')
})

async function copy() {
  try {
    await navigator.clipboard.writeText(report.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 2500)
  } catch {
    // Clipboard can be refused; the textarea below is the fallback and is always there,
    // which is also why the report is rendered as selectable text rather than only
    // living behind the button.
    copied.value = false
  }
}
</script>

<template>
  <div class="space-y-4 pb-4">
    <header>
      <h1 class="font-nathejk text-2xl text-slate-900">Sporingsstatus</h1>
      <p class="mt-1 text-sm text-slate-500">
        Teknisk side til at måle, hvor meget af ruten appen faktisk får optaget på en
        telefon. Kopiér rapporten og send den til udviklerne.
      </p>
    </header>

    <Alert v-if="!track.recording" class="border-amber-300 bg-amber-50 text-amber-900">
      <AlertTitle>Optager ikke lige nu</AlertTitle>
      <AlertDescription class="text-amber-800">
        Tilladelse: {{ location.permission }}. Åbn kortet og giv lov til placering, hvis du
        vil starte optagelsen.
      </AlertDescription>
    </Alert>

    <section class="grid grid-cols-2 gap-2 text-sm">
      <div class="rounded-lg border border-slate-200 bg-white p-3">
        <div class="text-xs text-slate-500">Punkter</div>
        <div class="text-lg font-medium">{{ points.length }}</div>
      </div>
      <div class="rounded-lg border border-slate-200 bg-white p-3">
        <div class="text-xs text-slate-500">Huller</div>
        <div class="text-lg font-medium">{{ analysis.gaps.length }}</div>
      </div>
      <div class="rounded-lg border border-slate-200 bg-white p-3">
        <div class="text-xs text-slate-500">Periode</div>
        <div class="text-lg font-medium">{{ analysis.span ? dur(analysis.span) : '—' }}</div>
      </div>
      <div class="rounded-lg border border-slate-200 bg-white p-3">
        <div class="text-xs text-slate-500">Dækning</div>
        <div class="text-lg font-medium">
          {{ analysis.expected ? Math.round((points.length / analysis.expected) * 100) + '%' : '—' }}
        </div>
      </div>
    </section>

    <div class="flex gap-2">
      <button
        type="button"
        class="flex flex-1 items-center justify-center gap-2 rounded-lg bg-slate-900 px-4 py-3 font-medium text-white"
        @click="copy"
      >
        <component :is="copied ? Check : Copy" class="h-4 w-4" aria-hidden="true" />
        {{ copied ? 'Kopieret' : 'Kopiér rapport' }}
      </button>
      <button
        type="button"
        class="flex items-center justify-center gap-2 rounded-lg border border-slate-300 px-4 py-3 text-sm"
        @click="load"
      >
        <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': loading }" aria-hidden="true" />
        Opdatér
      </button>
    </div>

    <!-- Always rendered, not just copied: clipboard access can be refused, and a report
         you can select by hand is the difference between a measurement and a lost night. -->
    <textarea
      readonly
      class="h-80 w-full rounded-lg border border-slate-300 bg-slate-50 p-2 font-mono text-[10px] leading-tight text-slate-700"
      :value="report"
    ></textarea>
  </div>
</template>
