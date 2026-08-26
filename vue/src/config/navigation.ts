import type { Component } from 'vue'
import { Map, Users, BookOpen, Megaphone, CalendarDays, Siren, HelpCircle, ShieldCheck } from '@lucide/vue'
import type { Role } from '@/stores/session.store'

// A single declarative destination drives both routing and the bottom nav.
// Icons are Lucide components (repo convention). `roles` gates visibility:
// undefined means "all signed-in roles"; otherwise only the listed roles.
//
// Note what "all signed-in roles" now includes: `gøgler` and the least-privileged
// `crew` fallback (PRD 006, task 067). Every destination without a `roles` list is
// therefore visible to an account whose function could not be determined — which is
// fine for the shared content pages below, and is why anything sensitive must gate
// explicitly rather than relying on being unlisted.
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
  { name: 'contacts', path: '/contacts', label: 'Kontakter', icon: Users },
  { name: 'rulebook', path: '/rulebook', label: 'Regler', icon: BookOpen },
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
