import { glimtPublicRetention, glimtRetention } from '@/config/runtime'

// The audience choice (PRD 019 §7, task 317).
//
// # This is the gate
//
// The public scope publishes with no approval queue in front of it (PRD 019 §0). There is no
// moderator between a twelve-year-old tapping "Offentligt" and a photograph being on the open web.
// So **the consequence line under that option is the only thing standing there**, and it is written
// out here, tested, rather than left as a string in a template.
//
// Three rules follow, and each is asserted in the spec:
//
//  1. **The narrowest option is first and is the default.** Widening is an act, never an accident.
//  2. **Every option states its consequence in plain Danish**, readable in the dark by a child. No
//     term of art, no "synlighed", nothing that needs a glossary.
//  3. **`public` says that people outside Nathejk can see it.** Not "offentligt" alone, which a
//     twelve-year-old may reasonably read as "everyone at the event".
//
// # Retention is stated, not implied
//
// The window is per-deployment (task 310) and served on `/api/config`, so the copy reads the real
// number. Saying "90 dage" while the deployment keeps things for 30 would be a promise about
// somebody's photographs that the service does not keep — and saying nothing is better than saying
// a wrong number, which is why an unknown window omits the sentence entirely.

export type Audience = 'group' | 'nathejk' | 'public'

export interface AudienceOption {
  value: Audience
  label: string
  /** One line, always present: what choosing this actually does. */
  consequence: string
  /** True for the option that reaches beyond Nathejk. Drives the warning styling. */
  reachesOutside: boolean
}

/**
 * The three options, **narrowest first**.
 *
 * The order is load-bearing: the composer selects the first by default, so this array is what makes
 * the safe choice the free one.
 */
export function audienceOptions(groupLabel: string): AudienceOption[] {
  return [
    {
      value: 'group',
      label: groupLabel,
      consequence: 'Kun dem der er med i din gruppe kan se det.',
      reachesOutside: false,
    },
    {
      value: 'nathejk',
      label: 'Alle på Nathejk',
      consequence: 'Alle der er med til Nathejk kan se det — også de andre grupper.',
      reachesOutside: false,
    },
    {
      value: 'public',
      // Named for who it reaches rather than for what it is called. "Offentligt" alone is a word a
      // child can read as "everyone here".
      label: 'Offentligt',
      consequence: 'Alle på internettet kan se det — også folk uden for Nathejk.',
      reachesOutside: true,
    },
  ]
}

/** The default. The narrowest option, and the first one. */
export const DEFAULT_AUDIENCE: Audience = 'group'

/**
 * What a member's own group is called, for the first option's label.
 *
 * Their actual group, not the word "gruppe": "Kun min patrulje" is a sentence a spejder can check
 * against reality, while "Min gruppe" needs them to know what the app means by it.
 */
export function groupLabelFor(role: string | null | undefined): string {
  switch (role) {
    case 'spejder':
      return 'Min patrulje'
    case 'bandit':
      return 'Min klan'
    default:
      // Every crew-ish role, including gøgler and the unclassified fallback — they share one
      // audience bucket (see users.GlimtGroupFor).
      return 'Crew'
  }
}

/**
 * The retention sentence for an audience, or '' when there is nothing honest to say.
 *
 * `public` gets the public window when one is configured, because how long the *open web* keeps it is
 * the part a member is most entitled to know before tapping that option. Everything else gets the
 * ordinary window.
 */
export function retentionNote(
  audience: Audience,
  retentionDays: number = glimtRetention.value,
  publicRetentionDays: number = glimtPublicRetention.value,
): string {
  if (audience === 'public' && publicRetentionDays > 0) {
    return `Det ligger offentligt i ${dayPhrase(publicRetentionDays)}.`
  }
  if (retentionDays > 0) {
    return `Det bliver slettet efter ${dayPhrase(retentionDays)}.`
  }
  // Retention off, which is a dev or test deployment. Saying nothing beats inventing a number.
  return ''
}

function dayPhrase(days: number): string {
  return days === 1 ? '1 dag' : `${days} dage`
}

/**
 * The consent line, shown above the share button on every post whatever the audience.
 *
 * PRD 019 §6 asks for it and the reason is worth restating: a photograph contains people who did not
 * choose to be in it. This is not a checkbox and does not gate anything — a forced tick would train
 * people to tick it. It is a sentence at the moment of deciding, which is the only moment it can do
 * any work.
 */
export const CONSENT_NOTE = 'Andre er også med på billedet — spørg dem først.'

/**
 * The attribution reassurance.
 *
 * Worth a line because it is the single most reassuring true thing about this feature, and because a
 * member who does not know it will assume the opposite — every other app they use puts their name on
 * what they post.
 */
export function attributionNote(role: string | null | undefined): string {
  const what = groupLabelFor(role).replace(/^Min /, '').toLowerCase()
  if (what === 'crew') return 'Glimtet bliver vist med din sektion — ikke med dit navn.'
  return `Glimtet bliver vist med din ${what} — ikke med dit navn.`
}

/**
 * The Team-section disclosure.
 *
 * PRD 019 §6 requires this to be **disclosed, not discovered**: the Team section can read every
 * glimt at every scope, including a `group`-scoped one, so "Min gruppe" is not the same as "only my
 * gruppe". A participant choosing the narrowest option is entitled to know that before they choose
 * it, not afterwards from a privacy page they never open.
 */
export const TEAM_DISCLOSURE = 'Team kan altid se alle glimt.'
