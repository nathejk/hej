import type { Checkpoint } from '@/stores/checkpoints.store'

// What a checkpoint marker looks like and says (PRD 016).
//
// # Why this is not inside EventMap.vue
//
// Leaflet owns its DOM imperatively, so anything that touches it can only be verified by looking at a
// screen. The *decisions* a marker embodies — which colour means what, whether a window is worth showing,
// how a visited post reads — are pure, and they are the parts that will be argued about and changed. Split
// out, they can be tested in node and reviewed without a map.
//
// EventMap keeps the Leaflet call; this file keeps the judgement.

/**
 * Marker appearance for a checkpoint.
 *
 * Two states only: a post still ahead, and one already reached. There is deliberately no third state for
 * "revealed but not yet reachable" — a patrol cannot act on that distinction, and the map is read at night
 * while walking.
 */
export interface CheckpointMarkerStyle {
  background: string
  /** An HTML entity, so the marker needs no icon font and no image asset through Vite. */
  glyph: string
  label: string
}

// Colours are chosen to survive **both** base layers. The Danish topo map is pale and the aerial is dark
// green and brown, so a mid-tone would disappear on one of them; a saturated fill inside a white border
// reads on both (PRD 002 ships both layers, and people switch between them constantly).
//
// They are also chosen to differ from every other thing on the map: the patrol's own position is blue, its
// scan registrations are slate, a bandit catch is red. Orange for "go here" and green for "been here" leaves
// no pair that reads the same at a glance.
const AHEAD: CheckpointMarkerStyle = {
  background: '#ea580c',
  glyph: '&#9873;', // flag
  label: 'Post',
}

const VISITED: CheckpointMarkerStyle = {
  background: '#047857',
  glyph: '&#10003;', // tick
  label: 'Besøgt post',
}

export function checkpointMarkerStyle(visited: boolean): CheckpointMarkerStyle {
  return visited ? VISITED : AHEAD
}

const clockFormat = new Intl.DateTimeFormat('da-DK', { hour: '2-digit', minute: '2-digit' })

/**
 * The window a post is open, as a patrol reads it, or '' when none is recorded.
 *
 * Date-less on purpose: somebody reading this at 02:00 knows what night it is, and "22:40–23:40" is quicker
 * to take in than a full timestamp.
 *
 * **Zero means "not recorded", not midnight 1970.** A real window on this event is never near the epoch, so
 * a zero is always an absent value — and rendering it would put "01:00–01:00" on a marker, which looks like
 * data rather than a gap.
 */
export function checkpointWindowText(cp: Pick<Checkpoint, 'openFrom' | 'openUntil'>): string {
  if (cp.openFrom <= 0 || cp.openUntil <= 0) return ''
  const from = clockFormat.format(new Date(cp.openFrom * 1000))
  const until = clockFormat.format(new Date(cp.openUntil * 1000))
  return `${from}–${until}`
}

/**
 * Escape text that goes into marker HTML.
 *
 * Checkpoint names are written by organizers in another system and travel here through the event stream, so
 * they are not this app's input to trust. A name containing a stray `<` would otherwise break the marker,
 * and one containing a tag would do worse.
 */
export function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

/**
 * The marker's popup, in Danish.
 *
 * Name first, because that is what a patrol is trying to match against the paper in their hand; then the
 * window, if there is one; then "Besøgt" when they have already been.
 */
export function checkpointPopupHtml(cp: Checkpoint, visited: boolean): string {
  const parts = [`<strong>${escapeHtml(cp.name)}</strong>`]

  const window = checkpointWindowText(cp)
  if (window) {
    parts.push(`Åben ${window}`)
  }
  if (visited) {
    parts.push('Besøgt')
  }
  return parts.join('<br>')
}
