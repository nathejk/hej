import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { seatsLabel, vehicleSummary } from '@/helpers/vehicleLabels'

// The "Mine køretøjer" section (PRD 010, task 242).
//
// Split the way `contactCheck.spec.ts` and `layout.spec.ts` split: the rules that are functions
// are tested as functions, and the two rules that live only in the template are asserted against
// the template's source. There is no jsdom and no @vue/test-utils here, so the alternative is
// pretending — a structural assertion is the honest form.

const SOURCE = readFileSync(
  fileURLToPath(new URL('./MyVehicles.vue', import.meta.url)),
  'utf8',
)

function sourceWithoutComments(): string {
  return SOURCE.replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/^\s*\/\/.*$/gm, '')
}

describe('vehicleSummary', () => {
  it('joins what is there', () => {
    expect(vehicleSummary({ brand: 'VW', model: 'Transporter', color: 'rød' })).toBe(
      'VW · Transporter · rød',
    )
  })

  // Every field but the plate is optional, so a half-filled form must not produce a line of
  // stray separators.
  it('skips the missing parts rather than leaving separators', () => {
    expect(vehicleSummary({ brand: 'VW', model: '', color: '' })).toBe('VW')
    expect(vehicleSummary({ brand: '', model: '', color: 'rød' })).toBe('rød')
    expect(vehicleSummary({ brand: '', model: '', color: '' })).toBe('')
  })
})

describe('seatsLabel', () => {
  // The qualifier is the point of this function. `seatCount` excludes the driver, so a label
  // that dropped "ud over dig selv" would be read as total capacity — and a car sent to collect
  // four people with room for three is the failure PRD 010 §7 calls out.
  it('always says the seats are besides the driver', () => {
    expect(seatsLabel(1)).toContain('ud over dig selv')
    expect(seatsLabel(4)).toContain('ud over dig selv')
  })

  it('gets the singular right', () => {
    expect(seatsLabel(1)).toBe('1 plads ud over dig selv')
    expect(seatsLabel(2)).toBe('2 pladser ud over dig selv')
  })

  // Zero is a real answer — a car brought only for its owner's own transport, not offered for
  // pickups — so it reads as a statement rather than as missing data.
  it('treats zero as an answer, not a blank', () => {
    expect(seatsLabel(0)).toBe('Ingen ekstra pladser')
    expect(seatsLabel(-1)).toBe('Ingen ekstra pladser')
  })
})

describe('MyVehicles.vue', () => {
  // The section must be *absent* for a spejder, not empty: an empty one invites them to look for
  // an action the BFF will refuse. Expressed as an exclusion so a role added later is included.
  it('gates the whole section on the role, as an exclusion', () => {
    const source = sourceWithoutComments()
    expect(source).toMatch(/session\.role !== 'spejder'/)
    // The gate has to be on the <section> itself. A gate on the inner list would still render a
    // heading and an "add" button for a spejder.
    expect(source).toMatch(/<section v-if="mayRegister">/)
  })

  // PRD 010 §5: somebody who skipped registration must be able to add a vehicle later "without a
  // nag that implies they did something wrong". Not bringing a car is ordinary, and the app
  // cannot tell "did not bother" from "came by train".
  it('does not nag a member with no vehicle', () => {
    const source = sourceWithoutComments()
    for (const nag of ['mangler', 'ufuldstændig', 'husk at', 'Du skal']) {
      expect(source.toLowerCase()).not.toContain(nag.toLowerCase())
    }
  })

  // A plate is quick to delete and slow to retype, and the consequence of an accident is a car
  // the coordinator can no longer send out.
  it('confirms before removing, in a dialog rather than confirm()', () => {
    const source = sourceWithoutComments()
    expect(source).toContain('Fjern køretøj?')
    expect(source).not.toMatch(/\bconfirm\(/)
  })

  // A failed read must not render as an empty list: "we could not reach the server" is not "you
  // have nothing registered", and the second invites a duplicate registration.
  it('distinguishes a failed read from an empty list', () => {
    const source = sourceWithoutComments()
    expect(source).toMatch(/v-else-if="vehicles\.error"/)
    expect(source).toMatch(/v-else-if="vehicles\.hasAny"/)
  })
})
