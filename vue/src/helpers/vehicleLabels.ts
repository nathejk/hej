// Label helpers for the vehicle surfaces (PRD 010).
//
// Extracted from MyVehicles.vue rather than left inline so they can be tested as functions:
// this project has no jsdom and no @vue/test-utils, so a rule that stays in a template can only
// be asserted against the template's source. Wording that has an operational consequence
// belongs in the half that can be tested properly.

import type { Vehicle } from '@/stores/vehicles.store'

/** Brand, model and colour as one line, skipping whatever is missing. */
export function vehicleSummary(vehicle: Pick<Vehicle, 'brand' | 'model' | 'color'>): string {
  return [vehicle.brand, vehicle.model, vehicle.color].filter((part) => part.length > 0).join(' · ')
}

/**
 * The seat count, spelled out.
 *
 * Every phrasing says **"ud over dig selv"** — excluding the driver — because that is what the
 * number means to shared-go and to a coordinator: `seatCount` is how many members the car can
 * carry, not how many people fit in it. A label that dropped the qualifier would be read as
 * total capacity, and the cost of that off-by-one is a car sent to collect four people with room
 * for three, at night, on a route.
 *
 * Zero is a real answer rather than a missing one: a car brought only for its owner's own
 * transport, which is not offered for pickups.
 */
export function seatsLabel(seatCount: number): string {
  if (seatCount <= 0) return 'Ingen ekstra pladser'
  if (seatCount === 1) return '1 plads ud over dig selv'
  return `${seatCount} pladser ud over dig selv`
}
