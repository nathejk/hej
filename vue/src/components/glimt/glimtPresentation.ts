import type { Glimt, GlimtHold } from '@/stores/glimt.store'
import { dayMonth } from '@/helpers/eventTime'

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

/**
 * How the caller's own attribution reads: **their unit, in the second person.**
 *
 * # Keyed on the glimt's frozen group, not the viewer's current role
 *
 * This was the constant `'Dit hold'` until 2026-09-17. Two things were wrong with it. "Hold" is our
 * internal word for a unit — a spejder is in a *patrulje* and says so (the same objection that
 * produced `holdShortcutLabel`) — and `glimtresponse.go` and its test had been documenting for weeks
 * that the own card reads "Din patrulje", so the Go side and the client disagreed in the record.
 *
 * The argument comes from the glimt, not the session: PRD 019 §6 freezes `authorGroup` at creation
 * precisely so a glimt keeps saying what it said. A member who was out as a bandit and is now crew
 * should still see "Din klan" on the glimt they posted then — reading their *current* role would
 * relabel history, which is the thing the freeze exists to prevent.
 *
 * Falls back to "Dit glimt" when the group is unknown, which is a real state rather than a fault: a
 * **queued** glimt has no frozen attribution yet, because nothing has reached the server to freeze it.
 * It reads correctly next to the *Venter* badge, and it never guesses a unit it might then publish
 * differently.
 */
export function ownAttribution(group: string): string {
  switch (group) {
    case 'spejder':
      return 'Din patrulje'
    case 'bandit':
      return 'Din klan'
    case 'crew':
      // Crew have a section rather than a numbered hold, and `attributionLine` shows other people
      // the bare section name for the same reason.
      return 'Din sektion'
    default:
      return 'Dit glimt'
  }
}

/**
 * The label on the feed's shortcut to the caller's own hold collection.
 *
 * # Why it varies by role
 *
 * "Se dit holds glimt" was one label for everyone, and it is the kind of phrasing that reads fine to
 * whoever wrote it and like machinery to whoever reads it. **"Hold" is our word, not theirs**: a
 * spejder is in a *patrulje* and says so. Maintainer direction, 2026-09-17.
 *
 * So a spejder gets "Se din patruljes glimt", which is a sentence they can check against reality — the
 * same reasoning `groupLabelFor` records for the audience selector's first option.
 *
 * Everyone else gets **"Se dine glimt"**, in the second person rather than named after their unit.
 * That is the maintainer's call and it is worth knowing it is deliberate rather than a gap: see the
 * note below about bandits.
 */
export function holdShortcutLabel(role: string | null | undefined): string {
  if (role === 'spejder') return 'Se din patruljes glimt'
  // **Note for whoever reads this next.** For a bandit this collection is their *klan's* glimt, not
  // only their own, so "dine" is looser than `holdWord` would allow — and PRD 019 §0b is emphatic
  // that patrulje and klan are distinct rather than two words for a hold. It is written this way on
  // purpose. If a bandit ever reports it as confusing, "Se din klans glimt" is the change, and this
  // function is the only place it lives.
  return 'Se dine glimt'
}

/**
 * The audience chip: who this glimt reached.
 *
 * Deliberately plain and slightly blunt, especially for `public`. This chip is the only place after
 * posting where a member can see how far a photograph went, so it has to be readable at a glance in
 * the dark by a twelve-year-old — not a term of art.
 *
 * # The group chip names the population, and takes the glimt's group to do it
 *
 * It read **"Min gruppe"** until 2026-09-17, which was wrong twice over. It understated the reach the
 * same way the composer's "Min patrulje" did — `users.MaySeeGlimt` matches this scope on group, so it
 * is every spejder at the event — and "min" is only true for a reader who happens to be in that group.
 * In the **moderation queue** it was plainly false: a Team member reviewing a bandit's post saw "Min
 * gruppe" about a group they are not in.
 *
 * So it takes the glimt's **frozen** `authorGroup` and names that population. One label, correct for
 * whoever is reading — the author, another member of the group, or a moderator from outside it.
 *
 * `group` is optional so the two wide audiences can still be labelled without one; only the narrow
 * scope needs it. An absent or unknown group falls back to "Kun én gruppe", which is vague but true —
 * unlike "Min gruppe", which was specific and sometimes false.
 */
export function audienceLabel(audience: Glimt['audience'], group?: string): string {
  switch (audience) {
    case 'nathejk':
      return 'Alle på Nathejk'
    case 'public':
      return 'Offentligt'
    default:
      return groupAudienceLabel(group)
  }
}

function groupAudienceLabel(group: string | undefined): string {
  switch (group) {
    case 'spejder':
      return 'Alle spejdere'
    case 'bandit':
      return 'Alle banditter'
    case 'crew':
      // No "Alle": the crew bucket is every crew-ish role together (users.GlimtGroupFor), and "Alle
      // crew" reads as a quantity where the others read as a population.
      return 'Crew'
    default:
      return 'Kun én gruppe'
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
  key: 'delete' | 'report' | 'hide' | 'unhide' | 'discard' | 'retry'
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
  // A queued glimt has never reached the server, so none of the server actions apply: there is
  // nothing to delete, nothing to report, and its id is a draft id that would 404 (task 325).
  //
  // *Fjern* rather than *Slet*, because the distinction is real and a member can act on it — this
  // discards something that was never shared, which is a smaller thing than deleting a post other
  // people may have seen. *Send nu* is offered alongside because the honest answer to "why is it
  // still waiting?" is sometimes "try again now", and PRD 019 §5 forbids implying background upload.
  if (glimt.pending) {
    return [
      { key: 'retry', label: 'Send nu', destructive: false },
      { key: 'discard', label: 'Fjern', destructive: true },
    ]
  }
  if (glimt.own) {
    return [{ key: 'delete', label: 'Slet', destructive: true }]
  }
  return [{ key: 'report', label: 'Anmeld', destructive: false }]
}

/**
 * The actions a Team-section moderator gets on a queue card (PRD 019 §7, task 309).
 *
 * # Exactly one action, and it is the opposite of the current state
 *
 * *Skjul* on a visible glimt, *Vis igen* on a hidden one — never both. A menu offering "hide" next to
 * "unhide" makes the moderator work out which one is a no-op, at 03:00, on a photograph somebody has
 * complained about. The card already shows a **Skjult** badge, so the available action doubles as a
 * second reading of the state.
 *
 * # Why *Skjul* is not marked destructive
 *
 * It looks like the destructive one and it is not. Hiding sets a column and publishes an event; the
 * row and the media survive, and *Vis igen* puts it back — only the author's own delete destroys
 * bytes. Painting it red would push a moderator towards hesitating, and PRD 019 §6 wants the
 * opposite: hiding is deliberately cheap so it can be the immediate answer to an ambiguous report,
 * because the public scope cannot afford waiting for a human to be sure.
 *
 * # No *Anmeld*, and no *Slet*
 *
 * Reporting is a way to ask a moderator to look; a moderator is already looking, and the endpoint
 * would only inflate the count they are triaging by. And a moderator has no delete: destroying
 * another member's photograph is not a power this feature grants anyone — the author deletes, the
 * Team hides, and retention eventually does the rest. `own` is still honoured, so a moderator looking
 * at their *own* glimt in the queue gets the author's *Slet* as well.
 */
export function moderationActions(glimt: Glimt): GlimtAction[] {
  const moderate: GlimtAction = glimt.hidden
    ? { key: 'unhide', label: 'Vis igen', destructive: false }
    : { key: 'hide', label: 'Skjul', destructive: false }
  if (glimt.own) {
    return [moderate, { key: 'delete', label: 'Slet', destructive: true }]
  }
  return [moderate]
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

  return dayMonth(new Date(createdAt))
}

// The narrowest and widest shapes a media strip may take, as width/height.
//
// # Where these numbers come from
//
// A strip has one shape and every slide fills it with `object-cover`, centred — so the clamps decide
// two things at once: how tall a card can get, and how much a photograph that does not match gets
// cropped.
//
//   - **4:3 portrait (0.75) is the floor**, because it is the most ordinary shape a phone camera
//     produces in portrait, and cropping the most ordinary shape is the wrong default. At 0.75 a 3:4
//     photograph is shown whole. Maintainer direction, 2026-09-18: crop "a little (maybe 10–15%)",
//     not a lot.
//   - **16:9 (1.78) is the ceiling**, so a panorama cannot flatten a card into a letterbox.
//
// What that costs: a 9:16 phone portrait (0.5625) still crops ~25% of its height. That is accepted
// deliberately — a 9:16 card is nearly a whole screen tall, which means one glimt per scroll and an
// unreadable feed. A very tall photograph is the one case where the card wins.
//
// Anything between the two is used exactly, so most glimt are not cropped at all.
export const MIN_STRIP_RATIO = 3 / 4
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
  if (ratio < MIN_STRIP_RATIO) return '3 / 4'
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
  const who = glimt.own ? ownAttribution(glimt.hold.group) : attributionLine(glimt.hold)
  const total = glimt.media.length
  if (total <= 1) return `Glimt fra ${who}`
  return `Glimt fra ${who}, billede ${ordinal + 1} af ${total}`
}
