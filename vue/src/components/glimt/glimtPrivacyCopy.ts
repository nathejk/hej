import { glimtPublicRetention, glimtRetention } from '@/config/runtime'

// What the privacy page says about Glimt (PRD 019 §6, task 321).
//
// # Why this is a module rather than paragraphs in the template
//
// Two of these sentences are the disclosures PRD 019 insists on rather than copy, and one of them
// carries a number that must match the deployment:
//
//   - **The Team-section reach.** A member choosing "Min patrulje" is entitled to know that is not
//     the same as "only my patrulje". §6 requires it stated, not discoverable — which means it must
//     be impossible to quietly drop, and a sentence in a template can be dropped by anyone
//     rearranging a page.
//   - **The retention window**, read from `/api/config`. A page that says "90 dage" while the
//     deployment keeps things for 30 is worse than saying nothing: it is a promise about a child's
//     photographs that the service will not keep.
//
// The unit suite runs in node with no DOM and never mounts a component, so copy left in
// `PrivacyView.vue` cannot be asserted at all. `audienceChoice.ts` set this precedent for the
// composer; this is the same argument for the page a parent actually reads.
//
// # Audience
//
// A twelve-year-old and their parent, both. No legalese, no "behandling", no "synlighed" — and
// second person throughout, because it is their photographs being described.

/** The section heading. */
export const GLIMT_PRIVACY_HEADING = 'Glimt — billeder fra løbet'

/**
 * What is actually stored.
 *
 * The location sentence is here because it is the most reassuring true thing on the page and the
 * thing a reader would never assume: every other app they use keeps the GPS position in a photo.
 * Stated as what we do rather than as what we do not have, so it reads as a decision.
 */
export const GLIMT_STORED = [
  'Når du deler et glimt, gemmer vi billedet eller videoen, din tekst, hvilket hold der har delt det, og hvornår.',
  'Vi fjerner stedet fra billedet, før vi gemmer det. Kameraet skriver normalt ind i billedfilen, hvor det er taget — det klipper vi væk, så et glimt ikke fortæller, hvor I var.',
] as const

/**
 * Who can see it.
 *
 * The three audiences in the same words the composer uses, so a member recognises them, and then the
 * Team sentence. **The Team sentence is last on purpose**: it applies to all three above it, and
 * putting it anywhere else would read as a footnote to one of them.
 */
export const GLIMT_WHO_CAN_SEE = [
  'Du vælger selv, hvem der kan se et glimt: kun din patrulje eller klan, alle der er med til Nathejk, eller offentligt — hvor alle på internettet kan se det, også folk uden for Nathejk.',
  'Du kan ikke ændre det bagefter. Vælger du offentligt, ligger det offentligt fra det øjeblik, du deler det — der er ingen, der ser det igennem først.',
] as const

/**
 * The Team-section disclosure, in the page's longer form.
 *
 * The composer says it in five words (`TEAM_DISCLOSURE`); here there is room to say who they are and
 * why, which is what makes it reassuring rather than alarming. Both must be true at once: it has to
 * be *unmissable* and it has to be *fair*, because the reach exists so that somebody can take a bad
 * photograph down quickly.
 */
export const GLIMT_TEAM_REACH =
  'Team — de voksne, der står for løbet — kan se alle glimt, også dem du kun har delt med din patrulje. Det er, så vi kan fjerne et billede hurtigt, hvis nogen synes, det ikke skal ligge der. Vælger du "min patrulje", er det altså din patrulje og Team, der kan se det.'

/** No name is attached. */
export const GLIMT_NO_NAMES =
  'Der står ikke noget navn på et glimt. Det bliver vist med patruljen eller klanen — ikke med dig.'

/**
 * How to get something removed.
 *
 * Says what happens *immediately* when you report, because the useful thing to know is that it works
 * without waiting for an adult to wake up. And it says hiding is not the same as deleting, so nobody
 * is surprised later.
 */
export const GLIMT_REMOVAL = [
  'Har du delt et glimt, kan du selv slette det. Så bliver billederne slettet.',
  'Er der et glimt, du synes skal væk, kan du anmelde det. Det bliver skjult med det samme — også hvis det lå offentligt — og så ser Team på det.',
] as const

/**
 * The retention sentence, or '' when the deployment has retention switched off.
 *
 * Returns '' rather than "0 dage" or "for altid": retention off is a dev or test deployment, and the
 * page should say nothing about a window it does not have rather than invent one. The view drops the
 * paragraph when this is empty.
 *
 * Two windows are stated separately when both are configured, because they answer two different
 * questions a parent asks in this order: how long is it on the open web, and how long do you keep it
 * at all.
 */
export function glimtRetentionCopy(
  retentionDays: number = glimtRetention.value,
  publicRetentionDays: number = glimtPublicRetention.value,
): string {
  if (retentionDays <= 0 && publicRetentionDays <= 0) return ''

  const parts: string[] = []
  if (publicRetentionDays > 0) {
    parts.push(`Et offentligt glimt bliver taget ned fra den offentlige side efter ${dayPhrase(publicRetentionDays)}.`)
  }
  if (retentionDays > 0) {
    parts.push(`Alle glimt bliver slettet efter ${dayPhrase(retentionDays)} — også dem I har delt internt. Det sker automatisk, og I skal ikke gøre noget.`)
  }
  return parts.join(' ')
}

function dayPhrase(days: number): string {
  return days === 1 ? '1 dag' : `${days} dage`
}
