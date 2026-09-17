import type { Component } from 'vue'
import { Map, Users, BookOpen, Megaphone, CalendarDays, Siren, HelpCircle, ShieldCheck, Camera } from '@lucide/vue'
import type { Role } from '@/stores/session.store'
import { allRolesExcept } from '@/config/roles'

// A single declarative destination drives both routing and the bottom nav.
// Icons are Lucide components (repo convention). `roles` gates visibility:
// undefined means "all signed-in roles"; otherwise only the listed roles.
//
// Note what "all signed-in roles" now includes: `gøgler` and the least-privileged
// `crew` fallback (PRD 006, task 067). Every destination without a `roles` list is
// therefore visible to an account whose function could not be determined — which is
// fine for the shared content pages below, and is why anything sensitive must gate
// explicitly rather than relying on being unlisted.
// **The order of this array is the bottom-bar ordering, and it is a decision (task 320).**
// `BottomNav` draws the first four visible entries plus "Mere"; everything after that is in the
// overflow sheet. There is deliberately **no per-role ordering** — one ordered list that gets
// *filtered* is what makes the outcome checkable for all seven roles at once, and role-gating
// already produces the right bar for each of them, because a destination a role cannot see costs
// it no slot. Inserting an entry above `glimt` pushes something out of somebody's bar; the tests
// in `navSlots.spec.ts` say whose.
export interface NavDestination {
  name: string
  path: string
  label: string
  icon: Component
  roles?: Role[]
  /**
   * Render the page edge-to-edge: the shell hides its top bar and drops the
   * scroll container, giving the view everything above the bottom nav. Used by
   * the map (PRD 002).
   */
  fullBleed?: boolean
}

export const destinations: NavDestination[] = [
  { name: 'maps', path: '/maps', label: 'Kort', icon: Map, fullBleed: true },
  // The contacts directory (PRD 007): crew, banditter and gøglere, never spejdere.
  //
  // Spejdere get no contacts pane at all. That is the decision that keeps a browsable index
  // of minors' faces out of the app, and it is enforced three times over: this list hides the
  // nav entry, the router guard refuses the route, and the BFF answers 403. Only the last one
  // is security — a hidden menu item is not access control, and the note at the top of this
  // file spells out why anything sensitive must gate explicitly.
  //
  // Written as "everyone except spejder" rather than by listing the six permitted roles, so a
  // role added later gets the pane by default instead of being silently left out of it.
  {
    name: 'contacts',
    path: '/contacts',
    label: 'Kontakter',
    icon: Users,
    roles: allRolesExcept('spejder'),
  },
  { name: 'rulebook', path: '/rulebook', label: 'Regler', icon: BookOpen },
  // Glimt (PRD 019): sharing a moment. **No `roles`, deliberately** — every role can post and can
  // see what was shared with them, and spejdere are the primary audience rather than an exception,
  // which makes this the one destination they get that `contacts` denies them.
  //
  // **Placed here, above `updates`, as task 320's ordering decision.** PRD 019 §7 requires Glimt in
  // the bar for spejdere, and this position puts it there for every role at once without demoting
  // anything that was previously in a bar:
  //
  //   spejder            Kort · Regler · Glimt · Nyt        + Mere
  //   bandit / gøgler    Kort · Kontakter · Regler · Glimt  + Mere
  //   crew / service     Kort · Kontakter · Regler · Glimt  + Mere
  //
  // A spejder gets it in third position rather than fourth purely because they have no `contacts`
  // entry — the same list, filtered. Nothing became unreachable: `schedule`, `faq` and `privacy`
  // were already in "Mere" for every role before Glimt existed, and so was `sos`.
  //
  // **`sos` sitting in the overflow for samarit/guide/postmandskab is not a consequence of this
  // line** and was deliberately left alone. It has been behind "Mere" since task 011, and whether
  // an emergency page should be one tap for a medic is an operational question for someone who
  // knows how the response chain actually works — not something to change as a side effect of
  // adding a photo feature. Promoting it is cheap when someone wants to (move it up; roles that
  // cannot see it are unaffected), but it costs those roles the Glimt slot.
  //
  // The Team-section moderation view (task 309) must **not** be added to this array: it is
  // organizer tooling for a handful of accounts and would spend a participant slot. It belongs in
  // the router as a non-destination route, reached from the feed, like `/glimt/hold/:number`.
  { name: 'glimt', path: '/glimt', label: 'Glimt', icon: Camera },
  { name: 'updates', path: '/updates', label: 'Nyt', icon: Megaphone },
  { name: 'schedule', path: '/schedule', label: 'Program', icon: CalendarDays },
  // Role-gated: only the identified service functions see the SOS/samarit page.
  //
  // `gøgler` and `crew` are deliberately absent. Gøglere staff posts but are not
  // part of the medical/guide response chain, and `crew` is the fallback for a crew
  // member whose function could not be determined (PRD 006) — granting it the SOS
  // page would mean an unrecognised section slug silently widens access, which is
  // exactly what the least-privileged fallback exists to prevent.
  { name: 'sos', path: '/sos', label: 'SOS', icon: Siren, roles: ['samarit', 'guide', 'postmandskab'] },
  { name: 'faq', path: '/faq', label: 'FAQ', icon: HelpCircle },
  // Data and privacy (PRD 002 §11.1, task 085). Listed in navigation, not only linked
  // from the location pre-prompt: a page reachable *only* from a prompt becomes
  // unreachable the moment someone dismisses it, and this is the page a participant or a
  // parent goes looking for afterwards. It belongs on the profile page too once PRD 003
  // lands — that is where the maintainer asked for it — but the profile does not exist
  // yet, and the copy should not wait for it.
  { name: 'privacy', path: '/privatliv', label: 'Data og privatliv', icon: ShieldCheck },
]

// visibleDestinations returns the destinations the given role may see, in order.
export function visibleDestinations(role: Role | null): NavDestination[] {
  return destinations.filter((d) => !d.roles || (role !== null && d.roles.includes(role)))
}
