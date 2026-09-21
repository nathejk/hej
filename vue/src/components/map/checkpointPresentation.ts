import type { Checkpoint } from '@/stores/checkpoints.store'
import { clock } from '@/helpers/eventTime'

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
 * Which of the three things a checkpoint currently is, to the patrol looking at it.
 *
 * - `visited` — the patrol scanned *this* post.
 * - `cleared` — the patrol scanned another post in the same postlinje, so this one no longer needs
 *   visiting. A line holds an A and a B and reaching either clears it (PRD 016 §11.12).
 * - `ahead` — still to do.
 */
export type CheckpointState = 'visited' | 'cleared' | 'ahead'

/**
 * Marker appearance for a checkpoint.
 *
 * Three states, which is a **deliberate change** from the two this file shipped with. The original comment
 * argued against a third state on the grounds that "a patrol cannot act on that distinction" — and that was
 * right about the state it was refusing (revealed-but-not-yet-reachable). `cleared` is the opposite case:
 * it is precisely actionable, because it means *you do not have to walk there*. Leaving it looking identical
 * to a post still ahead sends patrols to a post they have already finished with, at night, on foot.
 *
 * `cleared` is deliberately quieter than `visited`: an outline rather than a fill. The patrol did not go
 * there, and a marker that shouted the same as a real visit would overstate what happened.
 */
export interface CheckpointMarkerStyle {
  background: string
  /** Glyph and border colour. Separate from the fill so `cleared` can be an outline. */
  foreground: string
  border: string
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
  foreground: '#ffffff',
  border: '#ffffff',
  glyph: '&#9873;', // flag
  label: 'Post',
}

const VISITED: CheckpointMarkerStyle = {
  background: '#047857',
  foreground: '#ffffff',
  border: '#ffffff',
  glyph: '&#10003;', // tick
  label: 'Besøgt post',
}

// Inverted rather than a different hue: white fill, green edge, green tick. It shares the tick with
// `visited` on purpose — both mean "done with" — while the inversion carries "but you were not there".
// A third colour would have implied a third kind of thing.
const CLEARED: CheckpointMarkerStyle = {
  background: '#ffffff',
  foreground: '#047857',
  border: '#047857',
  glyph: '&#10003;', // tick
  label: 'Klaret post',
}

export function checkpointMarkerStyle(state: CheckpointState): CheckpointMarkerStyle {
  switch (state) {
    case 'visited':
      return VISITED
    case 'cleared':
      return CLEARED
    default:
      return AHEAD
  }
}

/**
 * Work out which state a checkpoint is in.
 *
 * `visited` wins over `cleared`: a post the patrol actually scanned is a visit, even though scanning it is
 * also what cleared its line. Checking that first is what stops every visited post rendering as merely
 * cleared.
 */
export function checkpointState(
  cp: Pick<Checkpoint, 'id' | 'checkgroup'>,
  scannedIds: ReadonlySet<string>,
  clearedCheckgroups: ReadonlySet<string>,
): CheckpointState {
  if (scannedIds.has(cp.id)) return 'visited'
  if (cp.checkgroup !== '' && clearedCheckgroups.has(cp.checkgroup)) return 'cleared'
  return 'ahead'
}

/**
 * The checkgroups a patrol has cleared: those containing at least one scanned post.
 *
 * Derived from the scans rather than stored, so it cannot disagree with the markers. Takes a lookup rather
 * than the checkpoint list, because the caller already holds one (`checkpoints.byId`) and building a second
 * would be a second thing to keep in step.
 */
export function clearedCheckgroupsFor(
  scannedIds: readonly string[],
  byId: ReadonlyMap<string, Pick<Checkpoint, 'checkgroup'>>,
): Set<string> {
  const out = new Set<string>()
  for (const id of scannedIds) {
    const cp = byId.get(id)
    // A scan whose post is not in the revealed set tells us nothing we can place. Skipped rather than
    // guessed: it is the same honest-absence rule the rest of this feature follows.
    if (cp && cp.checkgroup !== '') out.add(cp.checkgroup)
  }
  return out
}

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
  const from = clock(new Date(cp.openFrom * 1000))
  const until = clock(new Date(cp.openUntil * 1000))
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
 * window, if there is one; then what the post is to them now.
 *
 * The `cleared` line says why it needs no visit rather than just labelling it, because "Klaret" alone next
 * to a post they have never been to invites exactly the wrong guess — that the app has muddled them up.
 */
export function checkpointPopupHtml(cp: Checkpoint, state: CheckpointState): string {
  const parts = [`<strong>${escapeHtml(cp.name)}</strong>`]

  const window = checkpointWindowText(cp)
  if (window) {
    parts.push(`Åben ${window}`)
  }
  if (state === 'visited') {
    parts.push('Besøgt')
  } else if (state === 'cleared') {
    parts.push('Klaret – postlinjen er taget')
  }
  return parts.join('<br>')
}
