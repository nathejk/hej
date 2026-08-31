// Member lifecycle status, as the patrol lookup shows it (PRD 007, task 168).
//
// The vocabulary is `shared-go/types/member.go`'s `MemberStatus`, which the BFF returns verbatim
// from the person projection. This module is only the *display* half: Danish labels, and whether
// a status means the member has left the race.
//
// # Why the values are duplicated here
//
// Same reason `config/roles.ts` duplicates `users.Role`: the wire carries strings, and the client
// has to recognise them at runtime. Keep this list identical to shared-go's constants — they are
// one vocabulary expressed twice, and shared-go owns it.
//
// # Unknown statuses are shown, not swallowed
//
// A status this build does not know still means something to the crew member reading it, and
// hiding it would silently under-report a member's situation during an incident. Unknown values
// render as-is; only the *marking* logic below is conservative about them.

export const MEMBER_STATUSES = [
  'registered',
  'seated',
  'racing',
  'finished',
  'waiting',
  'transit',
  'sheltered',
  'reunited',
  'released',
] as const

export type MemberStatus = (typeof MEMBER_STATUSES)[number]

// Danish labels, phrased as an operator would say them over the radio rather than as the event
// stream names them.
//
// `Record<MemberStatus, string>` is load-bearing: adding a status to the list above without a
// label is a type error rather than a blank cell.
const STATUS_LABELS: Record<MemberStatus, string> = {
  registered: 'Tilmeldt',
  seated: 'Har plads',
  racing: 'På ruten',
  finished: 'Gennemført',
  waiting: 'Venter på afhentning',
  transit: 'I bil',
  sheltered: 'I hus',
  reunited: 'Tilbage hos patruljen',
  released: 'Hentet af værge',
}

/** The statuses that mean the member's Nathejk is over, one way or another. */
const ENDED: MemberStatus[] = ['finished', 'reunited', 'released']

/**
 * The statuses where Nathejk is responsible for the member's physical whereabouts.
 *
 * Mirrors `MemberStatus.InOurCare()` in shared-go. Worth surfacing in the lookup: a samarit
 * sent to a member who is already `transit` or `sheltered` needs to know that before they set
 * off, because somebody else has already got them.
 */
const IN_OUR_CARE: MemberStatus[] = ['waiting', 'transit', 'sheltered']

function known(status: string): status is MemberStatus {
  return (MEMBER_STATUSES as readonly string[]).includes(status)
}

/** A Danish label for a status, falling back to the raw value for anything unrecognised. */
export function memberStatusLabel(status: string): string {
  if (!status) return ''
  return known(status) ? STATUS_LABELS[status] : status
}

/**
 * Whether the member has left the race — as distinct from having finished it.
 *
 * `finished` is deliberately **not** included: finishing means walking the route to the end, and
 * marking a finisher as a withdrawal would quietly turn an achievement into a dropout.
 * shared-go's own documentation is careful about that distinction, and so is the BFF's
 * `stillInRace`.
 *
 * An unknown status returns false: assuming somebody has left the race on the strength of a value
 * we do not recognise is the more damaging guess.
 */
export function hasLeftRace(status: string): boolean {
  return known(status) && (status === 'reunited' || status === 'released')
}

/** Whether the member's Nathejk is over, including having finished it. */
export function hasEnded(status: string): boolean {
  return known(status) && ENDED.includes(status)
}

/** Whether Nathejk currently has charge of the member. */
export function isInOurCare(status: string): boolean {
  return known(status) && IN_OUR_CARE.includes(status)
}
