import type { Scan } from '@/stores/scans.store'

// What a registration row looks like in the drawer (task: three-way row icon).
//
// Pure, and separate from `scanVerdict`, because it answers a different question: not "were they on time"
// but "what kind of thing is this at all". Tested in node; ScanList.vue maps the answer to an icon.

/**
 * The three kinds of registration a patrol accumulates, as the drawer distinguishes them.
 *
 * - `bandit` — a bandit catch. Keeps its red skull.
 * - `checkpoint` — a scan the rota placed at a post. Emphasised: this is the row that carries a verdict.
 * - `plain` — a registration belonging to neither. It happened, and we cannot say where.
 */
export type ScanVariant = 'bandit' | 'checkpoint' | 'plain'

/**
 * Classify a row.
 *
 * A checkpoint scan requires an actual attributed post, not merely `kind === 'checkpoint'`. That
 * distinction is the point of this function: the BFF reports every non-bandit scan as
 * `kind: 'checkpoint'` because that is the honest default when the scanner cannot be classified (see
 * `internal/scans`), so `kind` alone would render an unplaceable registration as a post visit — a flag and
 * emerald emphasis for something we cannot locate. `checkpointId` is the fact that separates them.
 *
 * `plain` is therefore common rather than exceptional: an unrecorded shift is an upstream gap this app
 * cannot fix, and there are already real patrols carrying a dozen such scans. They get a neutral,
 * unemphasised row — true, and visibly not a post visit.
 */
export function scanVariant(scan: Pick<Scan, 'kind' | 'checkpointId'>): ScanVariant {
  if (scan.kind === 'bandit') return 'bandit'
  return scan.checkpointId === '' ? 'plain' : 'checkpoint'
}
