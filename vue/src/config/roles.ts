// The app roles (mirrors the BFF's users.Role). Drives which pages the nav shows.
//
// These are *app* roles, not signup categories: the upstream data speaks in team
// types (patrulje/klan/crew/gøgler), and PRD 006's person projection owns the
// translation. Keep this list identical to `AllRoles` in
// `go/internal/users/directory.go` — they are one enum expressed twice.
//
// `crew` is the least-privileged fallback for a crew member whose function could
// not be determined from their section slug. It is not "crew with crew powers": an
// account lands there because classification failed.
//
// A runtime array with the type derived from it, rather than a bare type union: the
// remembered-identity helper (@/helpers/identity, task 090) has to validate a role
// that came out of localStorage, and a type alone cannot do that at runtime. One
// list, two uses — not two lists to keep in sync.
//
// This lives in config/ rather than in session.store so that helpers/identity can
// validate a role without importing the store that imports it. That cycle worked by
// accident (the array is only read inside a function, so evaluation order happened
// to save it) and is not worth relying on.
export const ALL_ROLES = [
  'spejder',
  'bandit',
  'postmandskab',
  'guide',
  'samarit',
  'gøgler',
  'crew',
] as const

export type Role = (typeof ALL_ROLES)[number]

// allRolesExcept returns every known role except the given ones.
//
// Exists so a role gate can be written as "everyone but X" rather than by listing the
// permitted roles. The difference matters when a role is added: an explicit list silently
// excludes the newcomer from a page it probably should have, while this includes it — and
// the places that genuinely want an allow-list (the SOS page) keep spelling one out.
//
// Used by the contacts destination, which is "everyone except spejder" (PRD 007).
export function allRolesExcept(...excluded: Role[]): Role[] {
  return ALL_ROLES.filter((r) => !excluded.includes(r))
}

/**
 * Whether this role has the contacts pane at all.
 *
 * Mirrors `users.MayUseContacts` in the BFF, which is the actual authority — this is for deciding
 * whether to *ask*. Two uses, and the second is why it exists as a function rather than being
 * inlined in the navigation config:
 *
 *  - drawing the nav entry (via `allRolesExcept('spejder')`);
 *  - the quiet prefetch (task 194), which runs on every launch. Without a role gate, every spejder
 *    device would ask for a directory the server will always refuse, on every foreground — a few
 *    hundred phones generating 403s all race for a pane they cannot open.
 *
 * A `null` role — nobody signed in, or a role this build does not know — answers false: prefetching
 * on a guess is worse than not prefetching.
 */
export function hasContactsPane(role: string | null | undefined): boolean {
  if (!role) return false
  return role !== 'spejder' && (ALL_ROLES as readonly string[]).includes(role)
}

// The crew roles, including the least-privileged fallback.
//
// Mirrors `Role.IsCrew()` in `go/internal/users/directory.go`, and like it says nothing about
// privilege on its own — it is a population, not a permission.
//
// PRD 007's patrol lookup is crew-only and **includes `crew`**: that was a deliberate decision
// (§11.3), not an accident of the fallback, because task 078 found every real crew member
// currently lands on it. The BFF enforces this; the client uses it only to decide whether to draw
// the entry point, since offering a control that answers 404 is worse than not offering it.
const CREW_ROLES: Role[] = ['postmandskab', 'guide', 'samarit', 'crew']

export function isCrewRole(role: Role | null): boolean {
  return role !== null && CREW_ROLES.includes(role)
}

// Danish display labels for the roles.
//
// Kept next to the enum rather than in each component that needs one, so "gøgler"
// is capitalised and spelled the same way on the profile page, in the user menu
// and in the login chooser. `Record<Role, string>` is load-bearing: adding a role
// to ALL_ROLES without a label is a type error rather than a blank badge.
//
// `crew` reads "Crew" deliberately, with no function named — an account lands on
// that role because its section slug could not be classified, so inventing a
// specific job title here would state something we do not know.
export const ROLE_LABELS: Record<Role, string> = {
  spejder: 'Spejder',
  bandit: 'Bandit',
  postmandskab: 'Postmandskab',
  guide: 'Guide',
  samarit: 'Samarit',
  'gøgler': 'Gøgler',
  crew: 'Crew',
}

// Who is signed in. The id is opaque to the frontend; the role decides what the nav
// offers. Never treated as proof of anything — the BFF authorizes each request.
export interface Identity {
  userId: string
  role: Role
}
