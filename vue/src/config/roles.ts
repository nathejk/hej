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

// Who is signed in. The id is opaque to the frontend; the role decides what the nav
// offers. Never treated as proof of anything — the BFF authorizes each request.
export interface Identity {
  userId: string
  role: Role
}
