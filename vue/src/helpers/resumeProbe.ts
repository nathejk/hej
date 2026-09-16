// The resume probe's rules (task 280, PRD 017 §11 *Decided*).
//
// # What this measures and why it cannot be measured here
//
// `useFreshnessLoop` listens to `visibilitychange` and `online`. That is enough for a browser tab, but
// this app ships as an **installed home-screen PWA**, and returning to one from the iOS app switcher —
// or a bfcache restore after following an external link — does not reliably present as a
// `visibilitychange`. If it does not, PRD 017 *appears* to work: it works on a cold start and on a tab
// switch, and silently fails on the most common way the app is resumed during an event.
//
// No amount of unit testing answers that. It is a property of an engine on a device, so the probe
// records what actually fires and this module turns the recording into the answer.
//
// # The rules live here rather than in the view
//
// So they can be tested without a DOM, in the same spirit as `config/nudge.ts` and
// `components/map/*Presentation.ts`. The view is a table and two buttons.

/** Everything worth listening for on a resume, in the order it tends to fire. */
export const PROBED_EVENTS = [
  'pagehide',
  'pageshow',
  'freeze',
  'resume',
  'visibilitychange',
  'focus',
  'blur',
  'online',
  'offline',
] as const

export type ProbedEvent = (typeof PROBED_EVENTS)[number]

/**
 * The events the app's freshness loop actually reacts to today.
 *
 * Kept as data, and deliberately duplicated from `browserFreshnessTarget` rather than imported from
 * it: this is the *claim under test*. If someone widens the loop's listeners, this list should be
 * updated as a decision — a probe that silently tracked the implementation could never disagree with
 * it, and disagreeing is its entire job.
 */
export const LOOP_EVENTS: ProbedEvent[] = ['visibilitychange', 'online']

export interface ProbeEntry {
  /** Epoch ms. Persisted, so entries survive the app being killed while hidden. */
  at: number
  event: ProbedEvent
  /** `document.visibilityState` when it fired. */
  visibility: string
  /** For `pageshow`/`pagehide`: whether the page came from / went into the bfcache. */
  persisted?: boolean
  /** A tester's label for the path being exercised ("app switcher", "cold start", …). */
  mark?: string
}

/**
 * Whether this entry is one the freshness loop would have heard *and acted on*.
 *
 * `visibilitychange` only counts when the document became **visible**: the loop stops on hidden, so a
 * hidden event is not a check. This distinction is the difference between "an event fired" and "the app
 * refreshed", and conflating them is how this measurement would produce a confident wrong answer.
 */
export function wouldCheck(entry: ProbeEntry): boolean {
  if (!LOOP_EVENTS.includes(entry.event)) return false
  if (entry.event === 'visibilitychange') return entry.visibility === 'visible'
  return true
}

/**
 * The verdict for one labelled run: did anything the loop hears fire?
 *
 * `'none'` is the finding that would make PRD 017 a silent failure on this resume path, and therefore
 * the one worth naming rather than leaving a reader to scan a table for it.
 */
export type Verdict = 'checked' | 'events-but-no-check' | 'none'

export function verdictFor(entries: ProbeEntry[]): Verdict {
  if (entries.some(wouldCheck)) return 'checked'
  return entries.length > 0 ? 'events-but-no-check' : 'none'
}

/** Groups the log by the tester's marks, so each resume path gets its own verdict. */
export function groupByMark(entries: ProbeEntry[]): { mark: string; entries: ProbeEntry[] }[] {
  const groups: { mark: string; entries: ProbeEntry[] }[] = []
  for (const entry of entries) {
    const mark = entry.mark ?? 'ikke markeret'
    const last = groups[groups.length - 1]
    if (last && last.mark === mark) last.entries.push(entry)
    else groups.push({ mark, entries: [entry] })
  }
  return groups
}

const clock = (ms: number) =>
  new Date(ms).toLocaleTimeString('da-DK', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })

/**
 * The log as a Markdown table, for pasting into task 280's Progress Log.
 *
 * The output is the deliverable: the task asks for a table of resume path × event, and a page that
 * rendered the data but made a maintainer retype it would not get one. Includes the verdict per group,
 * and the gap since the previous event, which is what makes a suspend visible at all.
 */
export function toMarkdown(entries: ProbeEntry[], platform: string): string {
  const lines: string[] = []
  lines.push(`Resume probe — ${platform}`)
  lines.push('')

  for (const group of groupByMark(entries)) {
    lines.push(`**${group.mark}** — ${verdictLabel(verdictFor(group.entries))}`)
    lines.push('')
    lines.push('| tid | gap | event | visibility | persisted | loop? |')
    lines.push('|---|---|---|---|---|---|')
    let previous: number | null = null
    for (const entry of group.entries) {
      const gap = previous === null ? '' : `${Math.round((entry.at - previous) / 1000)}s`
      previous = entry.at
      lines.push(
        `| ${clock(entry.at)} | ${gap} | \`${entry.event}\` | ${entry.visibility} | ` +
          `${entry.persisted === undefined ? '' : entry.persisted} | ${wouldCheck(entry) ? '**ja**' : 'nej'} |`,
      )
    }
    lines.push('')
  }

  if (entries.length === 0) lines.push('_Ingen hændelser optaget._')
  return lines.join('\n')
}

export function verdictLabel(verdict: Verdict): string {
  switch (verdict) {
    case 'checked':
      return 'loopet ville have tjekket'
    case 'events-but-no-check':
      return 'hændelser, men INTET tjek — loopet hører dem ikke'
    case 'none':
      return 'ingen hændelser'
  }
}
