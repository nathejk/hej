import { ROLE_LABELS, type Role } from '@/config/roles'
import type { ChoiceCandidate } from '@/stores/session.store'

// Turning a candidate list into something a person can actually choose from (PRD 012).
//
// A pure function rather than logic inside `ProfileChooser.vue`, for the reason `vitest.config.ts`
// gives for running in `node`: there is no jsdom and no @vue/test-utils, so anything worth testing
// has to be reachable without mounting a component. Same split as `config/nudge.ts` and
// `helpers/contactSearch.ts`.
//
// # The problem this exists for
//
// Reported 2026-09-01 with a screenshot: five rows all reading "Klaus". The number carried one
// `postmandskab` profile named "Klaus Jørgensen" and four `gøgler` profiles named "Klaus" — and the
// payload stripped the surname and did not carry the role, so every discriminator the data had was
// discarded before it reached the screen.
//
// That is the common shape rather than an oddity. Most shared numbers are duplicate registrations
// of one person (PRD 006 §11 Q1): 70 of 85 shared numbers in the 2026 data carry rows with the same
// name, the largest being nine rows for one member.

export interface CandidateRow {
  userId: string
  name: string
  /** Everything under the name: affiliation, role, and a number when rows still collide. */
  subtitle: string
}

/**
 * The secondary line, in order of how much it tells one person apart from another.
 *
 * Patrulje or klan first — it is what separates two siblings. Then the crew section. Then the role,
 * which matters because duplicate registrations frequently carry no affiliation at all, and then
 * "Postmandskab" beside "Gøgler" is the only thing distinguishing two rows.
 */
export function candidateSubtitle(c: ChoiceCandidate): string {
  if (c.team) return c.team
  if (c.section) return c.section
  if (!c.role) return ''
  return ROLE_LABELS[c.role as Role] ?? c.role
}

/**
 * Rows ready to render, with identical ones numbered.
 *
 * Numbering is the honest end of the line. A number can carry several registrations of one person
 * with the same name, the same role and no affiliation on any of them; nothing in the record
 * distinguishes them, so nothing here can either. What numbering buys is that the reader can see
 * these are distinct records rather than a rendering bug, and can pick deliberately instead of
 * guessing which of five identical lines they already tried.
 *
 * Only ambiguous rows are numbered, so a number never implies an ordering over the whole list.
 *
 * The real fix is upstream de-duplication (PRD 006 §11 Q9). This keeps the dialog usable until then.
 */
export function candidateRows(candidates: ChoiceCandidate[]): CandidateRow[] {
  const totals = new Map<string, number>()
  for (const c of candidates) {
    const key = `${c.name}|${candidateSubtitle(c)}`
    totals.set(key, (totals.get(key) ?? 0) + 1)
  }

  const seen = new Map<string, number>()
  return candidates.map((c) => {
    const subtitle = candidateSubtitle(c)
    const key = `${c.name}|${subtitle}`
    const nth = (seen.get(key) ?? 0) + 1
    seen.set(key, nth)

    const ambiguous = (totals.get(key) ?? 0) > 1
    return {
      userId: c.user_id,
      name: c.name,
      subtitle: ambiguous ? [subtitle, `profil ${nth}`].filter(Boolean).join(' · ') : subtitle,
    }
  })
}
