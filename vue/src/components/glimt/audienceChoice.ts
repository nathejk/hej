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
 *
 * # The `group` scope is not a patrulje, and saying so was a real bug
 *
 * Until 2026-09-17 the first option read **"Min patrulje"** with "Kun dem der er med i din gruppe kan
 * se det." That was wrong in the one direction that matters. `users.MaySeeGlimt` matches this scope on
 * `GlimtGroupFor(viewer.Role) == g.AuthorGroup` — **the patrulje number is never consulted** — so for a
 * spejder it reaches *every spejder at the event*, some 750 people, not the six in their patrulje.
 *
 * We told a twelve-year-old their photograph was going to their patrulje and sent it to the whole
 * division. Caught by the maintainer reading the screen, which is the only way it could have been
 * caught: the code was correct, the label was a lie, and no test compares a label to a predicate.
 *
 * So each option now describes the population it actually reaches, named for the population rather
 * than for a unit. Getting this wrong again means understating reach on the default option, which is
 * the worst place in the feature to be wrong.
 */
export function audienceOptions(role: string | null | undefined): AudienceOption[] {
  return [
    {
      value: 'group',
      label: groupScopeLabel(role),
      consequence: groupScopeConsequence(role),
      reachesOutside: false,
    },
    {
      value: 'nathejk',
      label: 'Alle på Nathejk',
      // Names who else that includes rather than saying "de andre grupper", which is our word for a
      // population a participant thinks of by name. Maintainer direction, 2026-09-17.
      consequence: `Alle der er med til Nathejk kan se det — også ${otherGroupsFor(role)}.`,
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

/**
 * What the narrowest scope is called, for this role.
 *
 * Plural and population-shaped on purpose — "Alle spejderpatruljer", not "Min patrulje". The label has
 * to make the size of the audience obvious at a glance, because it is the default and most members
 * will never change it.
 */
function groupScopeLabel(role: string | null | undefined): string {
  switch (role) {
    case 'spejder':
      return 'Alle spejderpatruljer'
    case 'bandit':
      return 'Alle klaner'
    default:
      // Every crew-ish role, including gøgler and the unclassified fallback — they share one
      // audience bucket (see users.GlimtGroupFor).
      return 'Alt crew'
  }
}

/** Who, exactly, the narrowest scope reaches. */
function groupScopeConsequence(role: string | null | undefined): string {
  switch (role) {
    case 'spejder':
      return 'Kun dem som deltager som spejdere kan se det.'
    case 'bandit':
      return 'Kun dem som deltager som banditter kan se det.'
    default:
      return 'Kun dem som deltager som crew kan se det.'
  }
}

/**
 * The other populations inside Nathejk, from this role's point of view.
 *
 * Role-dependent because "også banditterne" is only the right sentence for a spejder — telling a
 * bandit that banditter can see it says nothing. The maintainer's wording for the spejder case, and
 * the same shape for the others.
 */
function otherGroupsFor(role: string | null | undefined): string {
  switch (role) {
    case 'spejder':
      return 'banditterne'
    case 'bandit':
      return 'spejderne'
    default:
      return 'spejderne og banditterne'
  }
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
 *
 * **The only note left in the composer**, and the rule that decides that: the composer carries what a
 * member must be aware of **each time they post**; standing facts about the system belong on the
 * privacy page (maintainer direction, 2026-09-17).
 *
 * This line qualifies because it is a different question every time — a different photograph, with
 * different people in it, who did not choose to be. The two notes that sat with it did not: that the
 * Team section can see everything, and that media is deleted after 90 days, are true before the member
 * opens the composer and unchanged by anything they do in it. Both are on `/privatliv` (task 321),
 * which has room to explain them. Three grey lines also read as boilerplate, which is how the one that
 * matters gets skipped along with the ones that do not.
 */
export const CONSENT_NOTE = 'Hvis andre er også med på billedet — spørg dem først.'

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
 * glimt at every scope, including a `group`-scoped one, so the narrowest option is not the same as
 * "only my group". A participant choosing it is entitled to know that before they choose it, not
 * afterwards from a privacy page they never open.
 *
 * **No longer shown in the composer** as of 2026-09-17 (maintainer direction). The rule: the composer
 * carries what a member must be aware of **each time they post**, and standing facts about the system
 * belong on the privacy page. This is a standing fact — true before they opened the composer and
 * unchanged by anything they choose in it — so it lives on `/privatliv`, in the longer form that has
 * room to say who Team are and why they can see everything (`GLIMT_TEAM_REACH`).
 *
 * That is a **narrowing of what §6 asked for** — it names the composer *and* the privacy page — so it
 * is recorded in the PRD's §11 as a decision rather than a tidy-up. §6's actual requirement, that the
 * reach be *disclosed rather than discovered*, is still met.
 *
 * Kept exported and tested so the wording does not rot while it is out of the composer, and so
 * restoring it is one line.
 */
export const TEAM_DISCLOSURE = 'Team kan altid se alle glimt.'
