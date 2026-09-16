import type { Scan } from '@/stores/scans.store'

// How a scan's on-time verdict reads in the drawer (PRD 016 §11.5, task 266).
//
// # Why this is a pure module
//
// The same reasoning as checkpointPresentation.ts: the *decision* — when a badge shows, what it says, what
// colour it carries — is what will be argued about and changed, and it can be tested in node without a
// component. ScanList.vue keeps the markup; this file keeps the judgement.

/**
 * A verdict badge to render, or null when there is nothing to show.
 *
 * Null is the common case, and it is deliberate: a bandit catch, an unattributed scan, a post with no
 * window, and a relative window with no anchor all arrive with no verdict, and a grey "ukendt" chip on
 * half the rows would be noise a patrol reads past. Absence says the true thing.
 */
export interface VerdictBadge {
  text: string
  /** Maps to a shadcn Badge variant: on time is `success` (green), late is `warning` (amber). */
  tone: 'success' | 'warning'
}

/**
 * Render an out-of-window delta as Danish copy, in whole minutes.
 *
 * A patrol thinks in minutes, not seconds — "12 min for sent" is actionable where "just over 11 minutes"
 * is not. The **sign carries meaning** and must be honoured: a positive delta is late ("for sent"), a
 * negative one is early ("for tidligt"). Labelling an early scan "for sent" would be a plain lie about
 * what happened. Rounding (not truncating) so 90 s reads as "2 min"; a scan inside the same minute as the
 * boundary reads without a number rather than a misleading "0 min".
 */
export function deltaText(deltaSeconds: number): string {
  const minutes = Math.round(Math.abs(deltaSeconds) / 60)
  const direction = deltaSeconds < 0 ? 'for tidligt' : 'for sent'
  return minutes > 0 ? `${minutes} min ${direction}` : direction
}

/**
 * The badge for a scan, or null when none should show.
 *
 * Only checkpoint scans can carry a verdict; a bandit catch keeps its own styling and never gets one. A
 * checkpoint scan shows "på tid" when on time and the late text otherwise. A null onTime (no window / no
 * anchor / unattributed) yields no badge.
 */
export function verdictBadge(scan: Pick<Scan, 'kind' | 'onTime' | 'deltaSeconds'>): VerdictBadge | null {
  if (scan.kind !== 'checkpoint' || scan.onTime === null) return null
  if (scan.onTime) return { text: 'på tid', tone: 'success' }
  // Outside the window: late or early, distinguished by the sign of the server's delta.
  return { text: deltaText(scan.deltaSeconds ?? 0), tone: 'warning' }
}
