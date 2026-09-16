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

export type BrowserEvent = (typeof PROBED_EVENTS)[number]

/**
 * `'mount'` is not a browser event — it is the probe recording that the page itself started.
 *
 * It has to be distinguishable from a real `pageshow`, and the first version of this file got that
 * wrong: it recorded the mount *as* a `pageshow`, so a genuine `pageshow` on resume produced a second
 * identical-looking row with no way to tell which was which. That corrupts the one measurement this
 * page exists to take.
 */
export type ProbedEvent = BrowserEvent | 'mount'

/**
 * The events the app's freshness loop actually reacts to today.
 *
 * Kept as data, and deliberately duplicated from `browserFreshnessTarget` rather than imported from
 * it: this is the *claim under test*. If someone widens the loop's listeners, this list should be
 * updated as a decision — a probe that silently tracked the implementation could never disagree with
 * it, and disagreeing is its entire job.
 *
 * `'mount'` is in it because `useFreshnessLoop` checks on construction when the document is visible —
 * "mounting counts as foregrounding" (PRD 017 §6). Leaving it out made the cold-start control report
 * itself as a failure, which is the most misleading thing this page could have done: the control is
 * what tells you whether to trust the other four rows.
 *
 * **Measured on iOS 18.7 (Safari 26.6.1), installed home-screen PWA, 2026-09-16 (task 280): this list
 * is sufficient.** `visibilitychange` → `visible` fires on lock/unlock return, on app-switcher return
 * and on a bfcache restore. `focus` and `pageshow` add no coverage and would multiply checks per resume
 * — `focus` fired *twice* before `visibilitychange` on one unlock. `freeze`/`resume` never fired at all.
 * So do not add to this list without measuring again; the reason it is short is evidence, not oversight.
 */
export const LOOP_EVENTS: ProbedEvent[] = ['visibilitychange', 'online', 'mount']

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
  /**
   * Which document load recorded this, as a short random id minted once per module load.
   *
   * Added after the first real run showed two `mount` rows a second apart, which the log could not
   * explain: two mounts in *one* document would mean the view (and possibly the app shell, and with it
   * the sync loop) is being created twice — the "exactly one app-level loop" rule quietly broken. Two
   * mounts in *two* documents is just a reload. Same id means the first; different ids mean the second.
   */
  load?: string
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
 * The verdict for one labelled run.
 *
 * `'events-but-no-check'` is the finding that would make PRD 017 a silent failure on that resume path,
 * and therefore the one worth naming rather than leaving a reader to scan a table for it.
 *
 * `'not-a-resume'` exists because of a false alarm this page produced on its first real use: opening it
 * records a `mount` and nothing else, and the verdict then read "hændelser, men INTET tjek" — which looks
 * exactly like the damning finding while meaning only "you have opened the page". A diagnostic that
 * cries wolf on first sight is worse than no diagnostic.
 */
export type Verdict = 'checked' | 'events-but-no-check' | 'none' | 'not-a-resume' | 'left-not-returned'

export function verdictFor(entries: ProbeEntry[]): Verdict {
  if (entries.length === 0) return 'none'
  // Only the page starting. Nothing has been left and returned to, so there is nothing to judge — even
  // though a mount *does* trigger a check and would otherwise report a cheerful, meaningless green.
  if (entries.every((e) => e.event === 'mount')) return 'not-a-resume'
  if (entries.some(wouldCheck)) return 'checked'
  // Nothing was recorded while the document was visible, so there is no evidence the app has come back
  // yet — this is the departure half of a path still in progress. Reporting it as a failed resume is what
  // the cold-start control did on its first run: swiping the app away records only `blur`/`hidden`, and
  // the return lands in a *new document* under a fresh label.
  //
  // The ambiguity is real and cannot be resolved from the log: "left and not back yet" and "came back and
  // truly nothing fired" look identical from here. So this is labelled neutrally *and* the label says
  // that if the tester did come back, the silence is itself the finding.
  if (!entries.some((e) => e.visibility === 'visible')) return 'left-not-returned'
  return 'events-but-no-check'
}

/** Groups the log by the tester's marks, so each resume path gets its own verdict. */
export function groupByMark(entries: ProbeEntry[]): { mark: string; entries: ProbeEntry[] }[] {
  const groups: { mark: string; entries: ProbeEntry[] }[] = []
  for (const entry of entries) {
    const mark = entry.mark ?? UNMARKED
    const last = groups[groups.length - 1]
    if (last && last.mark === mark) last.entries.push(entry)
    else groups.push({ mark, entries: [entry] })
  }
  return groups
}

/**
 * The label for entries recorded before the tester picked a path.
 *
 * The page now starts here rather than on the first path, so opening it cannot file a mount under
 * "lås / lås op" and invent a failed lock/unlock test that nobody ran.
 */
export const UNMARKED = 'ingen vej valgt'

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
    lines.push('| tid | gap | event | visibility | persisted | loop? | load |')
    lines.push('|---|---|---|---|---|---|---|')
    let previous: number | null = null
    for (const entry of group.entries) {
      const gap = previous === null ? '' : `${Math.round((entry.at - previous) / 1000)}s`
      previous = entry.at
      lines.push(
        `| ${clock(entry.at)} | ${gap} | \`${entry.event}\` | ${entry.visibility} | ` +
          `${entry.persisted === undefined ? '' : entry.persisted} | ${wouldCheck(entry) ? '**ja**' : 'nej'} | ` +
          `${entry.load ?? ''} |`,
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
    case 'left-not-returned':
      return 'forlod appen — intet registreret ved tilbagevenden. Hvis du ER kommet tilbage, er det selve fundet.'
    case 'not-a-resume':
      return 'siden blev åbnet — ingen genoptagelse målt endnu'
    case 'none':
      return 'ingen hændelser'
  }
}
