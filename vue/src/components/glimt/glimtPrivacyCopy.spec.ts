import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import {
  GLIMT_NO_NAMES,
  GLIMT_PRIVACY_HEADING,
  GLIMT_REMOVAL,
  GLIMT_STORED,
  GLIMT_TEAM_REACH,
  GLIMT_WHO_CAN_SEE,
  glimtRetentionCopy,
} from '@/components/glimt/glimtPrivacyCopy'
import { TEAM_DISCLOSURE } from '@/components/glimt/audienceChoice'

// What the privacy page promises about Glimt (PRD 019 §6, task 321).
//
// These look like copy tests and two of them are not. The Team-section reach and the retention window
// are *disclosures* — the PRD requires the first to be stated rather than discoverable, and the second
// to match the deployment rather than a number somebody typed. Neither can be checked by looking at
// the page, because the reach is invisible by definition and the window is only wrong on a deployment
// configured differently from the developer's.

const ALL_TEXT = [
  GLIMT_PRIVACY_HEADING,
  ...GLIMT_STORED,
  ...GLIMT_WHO_CAN_SEE,
  GLIMT_TEAM_REACH,
  GLIMT_NO_NAMES,
  ...GLIMT_REMOVAL,
].join(' ')

describe('the Glimt privacy copy', () => {
  it('covers all five points PRD 019 §6 asks for', () => {
    // Kept as one assertion per point rather than a word list, so a failure names what went missing.

    // 1. What is stored.
    expect(ALL_TEXT).toMatch(/tekst/)
    expect(ALL_TEXT).toMatch(/hvilket hold/)

    // 1b. And what is deliberately not: the location is stripped on upload.
    expect(ALL_TEXT).toMatch(/fjerner stedet/)

    // 2. Who can see it — all three audiences.
    expect(ALL_TEXT).toMatch(/patrulje eller klan/)
    expect(ALL_TEXT).toMatch(/alle der er med til Nathejk/i)
    expect(ALL_TEXT).toMatch(/offentligt/)

    // 3. The Team-section reach.
    expect(ALL_TEXT).toMatch(/Team/)

    // 4. No names.
    expect(ALL_TEXT).toMatch(/ikke noget navn|ikke med dit navn|ikke med dig/)

    // 5. How to get something removed.
    expect(ALL_TEXT).toMatch(/slette/)
    expect(ALL_TEXT).toMatch(/anmelde/)
  })

  // The disclosure the PRD singles out. It must say *what* Team can see and that it includes the
  // narrowest audience — "Team kan se glimt" would be true and useless.
  it('states the Team-section reach explicitly, including group-scoped glimt', () => {
    expect(GLIMT_TEAM_REACH).toMatch(/alle glimt/)
    expect(GLIMT_TEAM_REACH).toMatch(/kun har delt med din patrulje/)
    // And it says why, because a reach with no reason reads as surveillance rather than as the
    // ability to take a bad photograph down quickly.
    expect(GLIMT_TEAM_REACH).toMatch(/fjerne/)
  })

  // The composer says the same thing in five words. If one of them is ever softened, the other should
  // not be able to stay strong on its own.
  it('agrees with the composer’s short form', () => {
    expect(TEAM_DISCLOSURE).toMatch(/Team/)
    expect(TEAM_DISCLOSURE).toMatch(/alle glimt/)
  })

  it('says the public choice cannot be taken back, and that nobody reviews it first', () => {
    const text = GLIMT_WHO_CAN_SEE.join(' ')
    expect(text).toMatch(/kan ikke ændre det bagefter/i)
    // With no approval queue in front of the public scope (PRD 019 §0), this is the fact a parent
    // most needs and would least expect.
    expect(text).toMatch(/ingen, der ser det igennem først/)
  })

  it('says reporting hides a glimt immediately', () => {
    // The useful thing to know is that it works without waiting for an adult to wake up.
    expect(GLIMT_REMOVAL.join(' ')).toMatch(/skjult med det samme/)
  })

  it('does not confuse hiding with deleting', () => {
    // The author's delete removes the files; a report hides. Saying both are "sletning" would
    // surprise somebody later.
    expect(GLIMT_REMOVAL[0]).toMatch(/slettet/)
    expect(GLIMT_REMOVAL[1]).not.toMatch(/slettet/)
  })

  // Plain Danish, for a twelve-year-old and their parent. No terms of art.
  it('uses no legalese', () => {
    for (const word of [
      'behandling',
      'databehandler',
      'samtykke',
      'jf.',
      'jf ',
      'synlighed',
      'hjemmel',
      'persondata',
      'GDPR',
      'i henhold til',
    ]) {
      expect(ALL_TEXT.toLowerCase()).not.toContain(word.toLowerCase())
    }
  })

  it('is written in the second person', () => {
    // Their photographs, so their pronoun. A page in the third person is a policy, not an
    // explanation.
    expect(ALL_TEXT).toMatch(/\bdu\b/i)
    expect(ALL_TEXT).toMatch(/\bdin\b/i)
  })
})

describe('glimtRetentionCopy', () => {
  it('states the configured number, not a hard-coded one', () => {
    // The whole point of task 321's third criterion: a page reading "90 dage" on a deployment set to
    // 30 is a promise about a child's photographs that the service will not keep.
    expect(glimtRetentionCopy(30, 0)).toContain('30 dage')
    expect(glimtRetentionCopy(90, 0)).toContain('90 dage')
    expect(glimtRetentionCopy(90, 0)).not.toContain('30')
  })

  it('states both windows when both are configured, public first', () => {
    const text = glimtRetentionCopy(90, 30)
    expect(text).toContain('30 dage')
    expect(text).toContain('90 dage')
    // The order answers the two questions a parent asks in the order they ask them: how long is it on
    // the open web, then how long do you keep it at all.
    expect(text.indexOf('30 dage')).toBeLessThan(text.indexOf('90 dage'))
  })

  it('says nothing at all when retention is switched off', () => {
    // A dev or test deployment. "0 dage" and "for altid" are both worse than silence — and the view
    // drops the paragraph entirely rather than rendering an empty one.
    expect(glimtRetentionCopy(0, 0)).toBe('')
  })

  it('still speaks when only one window is set', () => {
    expect(glimtRetentionCopy(90, 0)).not.toBe('')
    expect(glimtRetentionCopy(0, 30)).not.toBe('')
    // With no overall window there is nothing honest to say about internal glimt, so it must not
    // claim one.
    expect(glimtRetentionCopy(0, 30)).not.toContain('Alle glimt bliver slettet')
  })

  it('gets the singular right', () => {
    expect(glimtRetentionCopy(1, 0)).toContain('1 dag')
    expect(glimtRetentionCopy(1, 0)).not.toContain('1 dage')
  })
})

// The half that catches the page being rearranged rather than the copy being reworded. The module is
// only a guarantee if the view actually renders it.
describe('PrivacyView renders the Glimt copy', () => {
  const VIEW = fileURLToPath(new URL('../../views/PrivacyView.vue', import.meta.url))

  it('imports every constant and the retention function', () => {
    const source = readFileSync(VIEW, 'utf8')
    for (const symbol of [
      'GLIMT_PRIVACY_HEADING',
      'GLIMT_STORED',
      'GLIMT_WHO_CAN_SEE',
      'GLIMT_TEAM_REACH',
      'GLIMT_NO_NAMES',
      'GLIMT_REMOVAL',
      'glimtRetentionCopy',
    ]) {
      expect(source, `${symbol} is no longer rendered by the privacy page`).toContain(symbol)
    }
  })

  it('does not hard-code a retention figure of its own', () => {
    // The failure mode this whole module exists to prevent, in the one file most likely to
    // reintroduce it.
    const source = readFileSync(VIEW, 'utf8').replace(/<!--[\s\S]*?-->/g, '')
    expect(source).not.toMatch(/\b(30|60|90)\s*dage/)
  })
})
