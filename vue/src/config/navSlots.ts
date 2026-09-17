import type { NavDestination } from '@/config/navigation'

/**
 * How many slots the bottom bar has.
 *
 * Five is a device constraint, not a preference: below about 64px a slot's icon-plus-label stops
 * being a comfortable touch target on the narrowest phone in the baseline (task 011).
 */
export const MAX_SLOTS = 5

export interface NavSlots {
  /** Destinations drawn directly in the bar. */
  primary: NavDestination[]
  /** Destinations behind "Mere". Empty when everything fits. */
  overflow: NavDestination[]
  hasOverflow: boolean
}

/**
 * Split a role's visible destinations into the bar and the "Mere" sheet.
 *
 * When there are more destinations than slots, the last slot is spent on the burger itself — so the
 * bar shows the first `MAX_SLOTS - 1`, not `MAX_SLOTS`. Getting that off by one is how a destination
 * silently disappears from both lists, which is why this is a function with a test rather than two
 * `slice` calls in a component.
 *
 * Extracted from `BottomNav.vue` for task 320: the ordering decision this encodes is now load-bearing
 * (PRD 019 §7 requires Glimt in the bar for spejdere), and Vitest here runs in node with no DOM, so a
 * rule left inside a component cannot be asserted at all.
 */
export function splitNavSlots(visible: NavDestination[]): NavSlots {
  const hasOverflow = visible.length > MAX_SLOTS
  if (!hasOverflow) return { primary: visible, overflow: [], hasOverflow }
  return {
    primary: visible.slice(0, MAX_SLOTS - 1),
    overflow: visible.slice(MAX_SLOTS - 1),
    hasOverflow,
  }
}

/** Whether a named destination lands in the bar itself, rather than behind "Mere". */
export function inBar(visible: NavDestination[], name: string): boolean {
  return splitNavSlots(visible).primary.some((d) => d.name === name)
}
