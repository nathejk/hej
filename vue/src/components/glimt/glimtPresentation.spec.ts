import { describe, expect, it } from 'vitest'

import {
  MAX_STRIP_RATIO,
  MIN_STRIP_RATIO,
  OWN_ATTRIBUTION,
  attributionLine,
  audienceLabel,
  audienceVariant,
  glimtActions,
  holdWord,
  mediaAltText,
  relativeTime,
  stripAspectRatio,
} from '@/components/glimt/glimtPresentation'
import type { Glimt } from '@/stores/glimt.store'

// Presentation rules for a glimt card (task 316).
//
// These read like formatting tests and are mostly privacy tests: the attribution line is what the
// card shows *instead of* a person's name, and the action list is what decides whether the report
// control is one tap away.

function glimt(over: Partial<Glimt> = {}): Glimt {
  return {
    id: 'g-1',
    hold: { number: '42', name: 'Ørnene', group: 'spejder' },
    own: false,
    audience: 'group',
    caption: '',
    createdAt: Date.parse('2026-09-17T21:00:00Z'),
    media: [{ ordinal: 0, kind: 'image', width: 1600, height: 1200, durationMs: 0, hasThumb: true }],
    hidden: false,
    ...over,
  }
}

describe('attributionLine', () => {
  it('names the hold, with the group word first', () => {
    // The group word distinguishes a Patrulje 42 from a Klan 42 — two different holds that can
    // share a number.
    expect(attributionLine({ number: '42', name: 'Ørnene', group: 'spejder' })).toBe(
      'Patrulje 42 · Ørnene',
    )
    expect(attributionLine({ number: '7', name: 'Nord', group: 'bandit' })).toBe('Klan 7 · Nord')
  })

  it('uses the bare section name for crew, who have no numbered hold', () => {
    // "Crew Postmandskab" would be the result of prefixing a word that is already a hold name.
    expect(attributionLine({ number: '', name: 'Postmandskab', group: 'crew' })).toBe(
      'Postmandskab',
    )
  })

  it('degrades to the group word rather than to nothing', () => {
    // An unattributed card would read as though the app were hiding who posted, when the truth is
    // that we never knew.
    expect(attributionLine({ number: '', name: '', group: 'spejder' })).toBe('Patrulje')
    expect(attributionLine({ number: '', name: '', group: '' })).toBe('Ukendt hold')
  })

  it('does not repeat a name that is just the number', () => {
    expect(attributionLine({ number: '42', name: '42', group: 'spejder' })).toBe('Patrulje 42')
  })

  it('never contains a person', () => {
    // The payload carries no author at all (task 302), so this is really an assertion about the
    // type — but it is the property the whole feature rests on, so it is stated out loud.
    const line = attributionLine({ number: '42', name: 'Ørnene', group: 'spejder' })
    expect(line).not.toMatch(/@|\+45|\d{8}/)
  })
})

describe('holdWord', () => {
  it('maps the three groups', () => {
    expect(holdWord('spejder')).toBe('Patrulje')
    expect(holdWord('bandit')).toBe('Klan')
    expect(holdWord('crew')).toBe('')
  })

  it('says nothing for a group it does not know', () => {
    // A group string from a newer BFF must not produce "undefined 42".
    expect(holdWord('gøgler')).toBe('')
    expect(holdWord('')).toBe('')
  })
})

describe('audience chip', () => {
  it('labels the three audiences in plain Danish', () => {
    expect(audienceLabel('group')).toBe('Min gruppe')
    expect(audienceLabel('nathejk')).toBe('Alle på Nathejk')
    expect(audienceLabel('public')).toBe('Offentligt')
  })

  // The chip is the only place after posting where a member can see how far a photograph went, so
  // `public` is the one that gets a colour.
  it('gives only public a loud variant', () => {
    expect(audienceVariant('public')).toBe('default')
    expect(audienceVariant('group')).toBe('secondary')
    expect(audienceVariant('nathejk')).toBe('secondary')
  })
})

describe('glimtActions', () => {
  // The property this whole file exists to protect. With no approval queue in front of the public
  // scope, reporting is the safety mechanism — and a safety mechanism that is hard to find is a
  // decoration.
  it('offers Anmeld on every glimt the caller did not write', () => {
    const actions = glimtActions(glimt({ own: false }))
    expect(actions.map((a) => a.key)).toEqual(['report'])
    expect(actions[0].label).toBe('Anmeld')
  })

  it('offers Anmeld regardless of audience', () => {
    for (const audience of ['group', 'nathejk', 'public'] as const) {
      expect(glimtActions(glimt({ own: false, audience })).map((a) => a.key)).toEqual(['report'])
    }
  })

  it('offers Slet, and only Slet, on your own', () => {
    // Reporting your own is allowed by the API — deliberately, as the fastest way to pull something
    // off the public feed — but it is not something a menu should suggest to an author who has a
    // delete button.
    const actions = glimtActions(glimt({ own: true }))
    expect(actions.map((a) => a.key)).toEqual(['delete'])
    expect(actions[0].destructive).toBe(true)
  })

  it('offers exactly one action, so the menu is never empty', () => {
    // An empty overflow menu is a button that opens nothing.
    expect(glimtActions(glimt({ own: true }))).toHaveLength(1)
    expect(glimtActions(glimt({ own: false }))).toHaveLength(1)
  })
})

describe('relativeTime', () => {
  const now = Date.parse('2026-09-17T22:00:00Z')

  it('reads relatively during the event', () => {
    expect(relativeTime(Date.parse('2026-09-17T21:58:00Z'), now)).toBe('for 2 min.')
    expect(relativeTime(Date.parse('2026-09-17T21:00:00Z'), now)).toBe('for 1 time')
    expect(relativeTime(Date.parse('2026-09-17T19:00:00Z'), now)).toBe('for 3 timer')
    expect(relativeTime(Date.parse('2026-09-16T22:00:00Z'), now)).toBe('for 1 dag')
    expect(relativeTime(Date.parse('2026-09-14T22:00:00Z'), now)).toBe('for 3 dage')
  })

  it('switches to a date once it is not "recent" any more', () => {
    // After the race the date is what a reader wants; "for 43 dage" is not information.
    const long = relativeTime(Date.parse('2026-08-01T22:00:00Z'), now)
    expect(long).not.toContain('for ')
    expect(long).toMatch(/aug/)
  })

  // A phone's clock is the likeliest thing to be wrong at 03:00, and a glimt posted a moment ago
  // can round ahead of `now`. "for -3 min." is not an acceptable rendering of either.
  it('says "Lige nu" rather than a negative interval', () => {
    expect(relativeTime(Date.parse('2026-09-17T22:00:30Z'), now)).toBe('Lige nu')
    expect(relativeTime(Date.parse('2026-09-18T04:00:00Z'), now)).toBe('Lige nu')
  })

  it('renders nothing for a missing timestamp', () => {
    // Better an absent line than "Lige nu" on a glimt whose date failed to parse.
    expect(relativeTime(0, now)).toBe('')
  })

  it('uses the singular for one', () => {
    expect(relativeTime(Date.parse('2026-09-17T21:00:00Z'), now)).toBe('for 1 time')
    expect(relativeTime(Date.parse('2026-09-16T22:00:00Z'), now)).toBe('for 1 dag')
  })
})

// The strip's height, and the bug it exists to prevent (reported from a device, 2026-09-18).
//
// A carousel is a flex row, so a row is as tall as its tallest child. With a per-item aspect ratio, a
// glimt containing a 1600×900 landscape *and* a 900×1600 portrait rendered as a portrait-tall box with
// the landscape floating at the top and a screen of empty card underneath.
describe('stripAspectRatio', () => {
  it('takes its shape from the first item, which the author put first', () => {
    expect(stripAspectRatio([{ width: 1600, height: 1200 }])).toBe('1600 / 1200')
  })

  // The actual regression. One ratio for the whole strip means the *second* item's shape cannot
  // stretch the container — and it also means the card does not change height as you swipe, which
  // would move everything below it in the feed.
  it('ignores later items, however tall they are', () => {
    const mixed = [
      { width: 1600, height: 900 },
      { width: 900, height: 1600 },
    ]
    expect(stripAspectRatio(mixed)).toBe('1600 / 900')

    // And the reverse order gives the portrait's clamped shape, not the landscape's.
    expect(stripAspectRatio([...mixed].reverse())).toBe('3 / 4')
  })

  // A 9:16 phone portrait card is nearly a whole screen tall — one glimt per scroll, which makes a
  // feed unreadable. It is the one case where the card wins over the photograph, at ~25% of its
  // height cropped.
  it('clamps a very tall portrait to 3:4', () => {
    expect(stripAspectRatio([{ width: 900, height: 1600 }])).toBe('3 / 4')
    expect(stripAspectRatio([{ width: 1080, height: 1920 }])).toBe('3 / 4')
  })

  it('clamps a panorama to 16:9', () => {
    expect(stripAspectRatio([{ width: 4000, height: 1000 }])).toBe('16 / 9')
  })

  it('leaves ordinary shapes alone', () => {
    // The shapes a phone camera actually produces are all inside the clamps and are respected
    // exactly — which is the point of loosening the floor to 3:4 (maintainer, 2026-09-18: crop "a
    // little", not a lot). Most glimt are therefore not cropped at all.
    expect(stripAspectRatio([{ width: 1600, height: 1200 }])).toBe('1600 / 1200')
    expect(stripAspectRatio([{ width: 1400, height: 1400 }])).toBe('1400 / 1400')
    expect(stripAspectRatio([{ width: 1200, height: 1600 }])).toBe('1200 / 1600')
  })

  // Regression guard for the floor. It was 4:5, which cropped the most ordinary portrait shape a
  // phone produces by ~6% for no good reason. If somebody tightens it again, this says why not.
  it('does not crop a 3:4 phone portrait', () => {
    expect(stripAspectRatio([{ width: 1200, height: 1600 }])).toBe('1200 / 1600')
    expect(MIN_STRIP_RATIO).toBeLessThanOrEqual(3 / 4)
  })

  it('reserves a neutral shape when dimensions are missing', () => {
    // A real state — an old row, or an upload whose decode did not report them — and collapsing the
    // card to nothing would be worse than reserving a plausible box.
    expect(stripAspectRatio([])).toBe('4 / 3')
    expect(stripAspectRatio([{ width: 0, height: 0 }])).toBe('4 / 3')
    expect(stripAspectRatio([{ width: 1600, height: 0 }])).toBe('4 / 3')
  })

  it('never returns a ratio outside the clamps', () => {
    // Property-ish: whatever comes in, the card's height stays within a readable band.
    const shapes = [
      { width: 1, height: 10000 },
      { width: 10000, height: 1 },
      { width: 3, height: 4 },
      { width: 16, height: 10 },
    ]
    for (const shape of shapes) {
      const [w, h] = stripAspectRatio([shape]).split(' / ').map(Number)
      const ratio = w / h
      expect(ratio).toBeGreaterThanOrEqual(MIN_STRIP_RATIO - 0.001)
      expect(ratio).toBeLessThanOrEqual(MAX_STRIP_RATIO + 0.001)
    }
  })
})

describe('mediaAltText', () => {
  it('describes the hold and the position, and invents nothing', () => {
    // We do not know what is in the photograph, and a made-up description would be worse than a
    // plain one. The caption is real text next to the image, so a screen reader reaches it anyway.
    const g = glimt({
      media: [
        { ordinal: 0, kind: 'image', width: 1, height: 1, durationMs: 0, hasThumb: true },
        { ordinal: 1, kind: 'image', width: 1, height: 1, durationMs: 0, hasThumb: true },
      ],
    })
    expect(mediaAltText(g, 0)).toBe('Glimt fra Patrulje 42 · Ørnene, billede 1 af 2')
    expect(mediaAltText(g, 1)).toBe('Glimt fra Patrulje 42 · Ørnene, billede 2 af 2')
  })

  it('omits the count for a single item', () => {
    expect(mediaAltText(glimt(), 0)).toBe('Glimt fra Patrulje 42 · Ørnene')
  })

  it('says "Dit hold" for the caller’s own', () => {
    expect(mediaAltText(glimt({ own: true }), 0)).toBe(`Glimt fra ${OWN_ATTRIBUTION}`)
  })
})
