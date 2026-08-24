import type { Component } from 'vue'
import { Map, Users, BookOpen, Megaphone, CalendarDays, Siren, HelpCircle } from '@lucide/vue'
import type { Role } from '@/stores/session.store'

// A single declarative destination drives both routing and the bottom nav.
// Icons are Lucide components (repo convention). `roles` gates visibility:
// undefined means "all signed-in roles"; otherwise only the listed roles.
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
  // Role-gated example: only service roles see the SOS/samarit page. This pushes
  // those roles past 5 destinations and exercises the "More" overflow.
  { name: 'sos', path: '/sos', label: 'SOS', icon: Siren, roles: ['samarit', 'guide', 'postmandskab'] },
  { name: 'faq', path: '/faq', label: 'FAQ', icon: HelpCircle },
]

// visibleDestinations returns the destinations the given role may see, in order.
export function visibleDestinations(role: Role | null): NavDestination[] {
  return destinations.filter((d) => !d.roles || (role !== null && d.roles.includes(role)))
}
