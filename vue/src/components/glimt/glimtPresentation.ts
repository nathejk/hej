import type { Glimt, GlimtHold } from '@/stores/glimt.store'

// How a glimt is presented (PRD 019 §7, task 316).
//
// # Why this is a module and not template logic
//
// The unit suite runs in `node` with no DOM and there is no `@vue/test-utils`, so components are
// never mounted. The skill's rule follows from that: when a component holds a decision worth
// checking, extract the decision and leave the component as markup — `arrowGeometry.ts` and
// `checkpointPresentation.ts` set the precedent. Otherwise the alternative to extracting is not
// testing.
//
// Everything here is a decision worth checking, because most of it is a privacy rule wearing the
// clothes of a formatting function:
//
//   - the attribution line is what the card shows *instead of* a person's name
//   - the audience chip is what tells a member how far their photo went
//   - the action list is what decides whether "Anmeld" is one tap away
//
// Getting any of those subtly wrong is invisible in a screenshot.

/**
 * The attribution line: the hold, never the person (PRD 019 §0b).
 *
 * "Patrulje 42 · Ørnene" for a spejder, "Klan 7 · Nord" for a bandit, and the bare section name for
 * crew, who have no numbered hold. The group word comes first because it is what distinguishes a
 * Patrulje 42 from a Klan 42 — two different holds that can share a number.
 *
 * Falls back to the group word alone when a hold has neither number nor name, which is a data state
 * rather than an error. It must never fall back to *nothing*: an unattributed card would read as
 * though the app were hiding who posted, when the truth is that we never knew.
 */
export function attributionLine(hold: GlimtHold): string {
  const kind = holdWord(hold.group)
  const parts: string[] = []
  if (hold.number) parts.push(`${kind} ${hold.number}`.trim())
  else if (kind) parts.push(kind)
  if (hold.name && hold.name !== hold.number) parts.push(hold.name)
  return parts.join(' · ') || 'Ukendt hold'
}

/**
 * The Danish word for a hold of this group.
 *
 * Crew return '' on purpose: their attribution is a section name that already reads as one
 * ("Postmandskab"), and prefixing it would produce "Crew Postmandskab".
 */
export function holdWord(group: string): string {
  switch (group) {
    case 'spejder':
      return 'Patrulje'
    case 'bandit':
      return 'Klan'
    default:
      return ''
  }
}

/** How the caller's own attribution reads. Short, and in the second person. */
export const OWN_ATTRIBUTION = 'Dit hold'

/**
 * The audience chip: who this glimt reached.
 *
 * Deliberately plain and slightly blunt, especially for `public`. This chip is the only place after
 * posting where a member can see how far a photograph went, so it has to be readable at a glance in
 * the dark by a twelve-year-old — not a term of art.
 */
export function audienceLabel(audience: Glimt['audience']): string {
  switch (audience) {
    case 'nathejk':
      return 'Alle på Nathejk'
    case 'public':
      return 'Offentligt'
    default:
      return 'Min gruppe'
  }
}

/**
 * The shadcn `badge` variant for an audience chip.
 *
 * `public` is the only one that gets a colour, and it gets the loud one. The other two are
 * unremarkable states; "this is on the open web" is not, and a member scrolling their own posts
 * should be able to spot it without reading.
 */
export function audienceVariant(audience: Glimt['audience']): 'secondary' | 'default' {
  return audience === 'public' ? 'default' : 'secondary'
}

/** What the overflow menu offers for this glimt. */
export interface GlimtAction {
  key: 'delete' | 'report'
  label: string
  /** Destructive actions get the red styling shadcn's dropdown provides. */
  destructive: boolean
}

/**
 * The actions available on a card.
 *
 * **`Anmeld` is on every card the caller did not write, always, in the same one-tap overflow.** Not
 * behind a long-press, not only on public glimt. With no approval queue in front of the public scope
 * (PRD 019 §0), reporting *is* the safety mechanism, and a safety mechanism that is hard to find is a
 * decoration. Task 307's endpoint hides a reported glimt with no human step, which only helps if
 * somebody can reach the button.
 *
 * Your own glimt offers `Slet` and not `Anmeld`, because reporting your own is a thing the API allows
 * — deliberately, as the fastest way to pull something off the public feed — but not a thing a menu
 * should suggest. Deleting is the right control to offer an author; the API keeps the other door open
 * for anyone who needs it.
 */
export function glimtActions(glimt: Glimt): GlimtAction[] {
  if (glimt.own) {
    return [{ key: 'delete', label: 'Slet', destructive: true }]
  }
  return [{ key: 'report', label: 'Anmeld', destructive: false }]
}

/**
 * A short relative time, in Danish.
 *
 * Relative rather than a clock time because during the event "for 20 min." is what a reader wants,
 * and afterwards the date is — so it switches. Nothing here goes through
 * `Intl.RelativeTimeFormat`: the vocabulary is four cases, and the formatter produces "for 1 dage
 * siden" without more configuration than the strings cost.
 */
export function relativeTime(createdAt: number, now: number = Date.now()): string {
  if (!createdAt) return ''
  const seconds = Math.floor((now - createdAt) / 1000)

  // A clock skewed forward on the phone, or a glimt posted a moment ago whose timestamp rounds
  // ahead. "Lige nu" is the honest answer to both, and better than "for -3 min.".
  if (seconds < 60) return 'Lige nu'

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `for ${minutes} min.`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `for ${hours} ${hours === 1 ? 'time' : 'timer'}`

  const days = Math.floor(hours / 24)
  if (days < 7) return `for ${days} ${days === 1 ? 'dag' : 'dage'}`

  return new Date(createdAt).toLocaleDateString('da-DK', { day: 'numeric', month: 'short' })
}

// The narrowest and widest shapes a media strip may take, as width/height.
//
// A phone photograph is often 3:4 or 9:16, and a 9:16 card is nearly a whole screen tall — one glimt
// per scroll, which makes a feed unreadable. Clamped to 4:5 portrait and 16:9 landscape, which is
// roughly where Instagram landed and for the same reason.
export const MIN_STRIP_RATIO = 4 / 5
export const MAX_STRIP_RATIO = 16 / 9

/**
 * The single aspect ratio a whole media strip is drawn at, as a CSS `aspect-ratio` value.
 *
 * # Why one ratio for the strip rather than one per item
 *
 * Because a carousel is a flex row, and a row is as tall as its tallest child. With a per-item ratio,
 * a glimt containing a 1600×900 landscape *and* a 900×1600 portrait renders a portrait-tall container
 * with the landscape floating at the top of it and a screen of empty card underneath — which is exactly
 * what the first version did, and it is unmistakable the moment you look at it.
 *
 * Fixing it by making the container fit each slide would be worse: the card would then change height
 * as you swipe, moving everything below it.
 *
 * So the strip gets one shape, taken from the **first** item — the author put it first — and every slide
 * fills it with `object-cover`. Later items in a different orientation are cropped, which is the
 * honest trade: a consistent card that crops beats a jumping card that does not.
 *
 * Clamped at both ends, so neither a panorama nor a 9:16 phone portrait can set the height of a feed.
 */
export function stripAspectRatio(media: Array<{ width: number; height: number }>): string {
  const first = media[0]
  // No dimensions is a real state — an old row, or an upload whose decode did not report them — and
  // 4:3 is a reasonable neutral shape to reserve rather than collapsing the card to nothing.
  if (!first || first.width <= 0 || first.height <= 0) return '4 / 3'

  const ratio = first.width / first.height
  if (ratio < MIN_STRIP_RATIO) return '4 / 5'
  if (ratio > MAX_STRIP_RATIO) return '16 / 9'
  return `${first.width} / ${first.height}`
}

/**
 * The alt text for a media item.
 *
 * Describes what the reader learns from the card anyway — whose hold, and which of how many —
 * because there is nothing else honest to say: we do not know what is in the photograph, and
 * inventing a description would be worse than a plain one. The caption is rendered as real text
 * beside the image, so a screen reader reaches it regardless.
 */
export function mediaAltText(glimt: Glimt, ordinal: number): string {
  const who = glimt.own ? OWN_ATTRIBUTION : attributionLine(glimt.hold)
  const total = glimt.media.length
  if (total <= 1) return `Glimt fra ${who}`
  return `Glimt fra ${who}, billede ${ordinal + 1} af ${total}`
}
