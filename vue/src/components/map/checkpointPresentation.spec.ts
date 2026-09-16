import { describe, expect, it } from 'vitest'

import {
  checkpointMarkerStyle,
  checkpointPopupHtml,
  checkpointState,
  checkpointWindowText,
  clearedCheckgroupsFor,
  escapeHtml,
} from '@/components/map/checkpointPresentation'
import type { Checkpoint } from '@/stores/checkpoints.store'

function checkpoint(over: Partial<Checkpoint> = {}): Checkpoint {
  return {
    id: 'cp-1',
    name: 'Post 1',
    checkgroup: 'cg-1',
    sortOrder: 0,
    lat: 56.1382,
    lng: 9.5521,
    openFrom: 0,
    openUntil: 0,
    openDurationMinutes: 0,
    ...over,
  }
}

describe('checkpointMarkerStyle', () => {
  // A post still ahead and one already reached have to be distinguishable at a glance, at night, by someone
  // walking. Colour alone is not enough — some users will not separate orange from red — so the glyph
  // differs too.
  it('distinguishes a visited post from one still ahead by colour *and* glyph', () => {
    const ahead = checkpointMarkerStyle('ahead')
    const visited = checkpointMarkerStyle('visited')

    expect(ahead.background).not.toBe(visited.background)
    expect(ahead.glyph).not.toBe(visited.glyph)
  })

  // The map also carries the patrol's own position (blue), its scan registrations (slate) and bandit catches
  // (red). A checkpoint colour that collided with any of those would make two different things read the
  // same.
  it('does not reuse the colours already on the map', () => {
    const used = ['#2563eb', '#3b82f6', '#0f172a', '#b91c1c']

    for (const state of ['ahead', 'visited', 'cleared'] as const) {
      expect(used).not.toContain(checkpointMarkerStyle(state).background)
    }
  })

  it('labels every state in Danish', () => {
    expect(checkpointMarkerStyle('ahead').label).toBe('Post')
    expect(checkpointMarkerStyle('visited').label).toBe('Besøgt post')
    expect(checkpointMarkerStyle('cleared').label).toBe('Klaret post')
  })

  // `cleared` must be quieter than `visited`: the patrol did not go there, and a marker shouting as loudly
  // as a real visit would overstate what happened. Encoded as an inversion — white fill, green edge — so
  // the assertion is about the actual visual weight rather than a name.
  it('renders cleared as an outline and visited as a fill', () => {
    const visited = checkpointMarkerStyle('visited')
    const cleared = checkpointMarkerStyle('cleared')

    expect(cleared.background).toBe('#ffffff')
    expect(cleared.foreground).toBe(visited.background)
    expect(cleared.border).toBe(visited.background)
    // Filled: the glyph contrasts against a saturated ground.
    expect(visited.foreground).toBe('#ffffff')
  })

  // Both mean "done with", so they share the tick; the inversion carries "but you were not there". A third
  // glyph would have implied a third kind of thing.
  it('shares the tick between cleared and visited, and keeps the flag for ahead', () => {
    expect(checkpointMarkerStyle('cleared').glyph).toBe(checkpointMarkerStyle('visited').glyph)
    expect(checkpointMarkerStyle('ahead').glyph).not.toBe(checkpointMarkerStyle('visited').glyph)
  })
})

describe('checkpointState', () => {
  const scanned = new Set(['cp-1'])
  const cleared = new Set(['cg-1'])

  it('reports a scanned post as visited', () => {
    expect(checkpointState({ id: 'cp-1', checkgroup: 'cg-1' }, scanned, cleared)).toBe('visited')
  })

  // The sibling post in a line the patrol has taken. This is the state the change exists for: it no longer
  // needs walking to, and must not look like somewhere they still have to go.
  it('reports the sibling post in a cleared line as cleared', () => {
    expect(checkpointState({ id: 'cp-1b', checkgroup: 'cg-1' }, scanned, cleared)).toBe('cleared')
  })

  it('reports a post in an untouched line as ahead', () => {
    expect(checkpointState({ id: 'cp-2', checkgroup: 'cg-2' }, scanned, cleared)).toBe('ahead')
  })

  // Visited beats cleared: scanning a post is what clears its line, so without this ordering every visited
  // post would render as merely cleared.
  it('prefers visited over cleared for the post actually scanned', () => {
    expect(checkpointState({ id: 'cp-1', checkgroup: 'cg-1' }, scanned, cleared)).toBe('visited')
  })

  it('never treats an empty checkgroup as cleared', () => {
    expect(checkpointState({ id: 'cp-x', checkgroup: '' }, scanned, new Set([''])))
      .toBe('ahead')
  })
})

describe('clearedCheckgroupsFor', () => {
  const byId = new Map([
    ['cp-1', { checkgroup: 'cg-1' }],
    ['cp-2', { checkgroup: 'cg-2' }],
    ['cp-nogroup', { checkgroup: '' }],
  ])

  it('collects the checkgroups of the scanned posts', () => {
    expect([...clearedCheckgroupsFor(['cp-1'], byId)]).toEqual(['cg-1'])
  })

  it('deduplicates two scans in one line', () => {
    const both = new Map([...byId, ['cp-1b', { checkgroup: 'cg-1' }]])
    expect([...clearedCheckgroupsFor(['cp-1', 'cp-1b'], both)]).toEqual(['cg-1'])
  })

  // A scan whose post is not in the revealed set tells us nothing we can place — the same honest-absence
  // rule the rest of the feature follows. Notably this is what stops an unattributed scan clearing a line.
  it('ignores a scan whose post is not revealed', () => {
    expect([...clearedCheckgroupsFor(['cp-unknown'], byId)]).toEqual([])
  })

  it('ignores a post with no checkgroup', () => {
    expect([...clearedCheckgroupsFor(['cp-nogroup'], byId)]).toEqual([])
  })
})

describe('checkpointWindowText', () => {
  it('renders a window as clock times', () => {
    // 2025-06-13 20:00–23:30 UTC. Formatting is da-DK, so the exact rendering depends on the runner's zone;
    // what matters is that both ends appear and are separated.
    const text = checkpointWindowText({ openFrom: 1749844800, openUntil: 1749857400 })

    expect(text).toMatch(/^\d{2}[.:]\d{2}–\d{2}[.:]\d{2}$/)
  })

  // Zero is "not recorded", which is a normal state for a post whose hours nobody has set. Rendering it
  // would put "01:00–01:00" on the marker, which looks like data rather than a gap.
  it('treats zero as no window rather than as 1970', () => {
    expect(checkpointWindowText({ openFrom: 0, openUntil: 0 })).toBe('')
    expect(checkpointWindowText({ openFrom: 1749844800, openUntil: 0 })).toBe('')
    expect(checkpointWindowText({ openFrom: 0, openUntil: 1749857400 })).toBe('')
  })

  it('treats a negative value as no window', () => {
    expect(checkpointWindowText({ openFrom: -1, openUntil: 1749857400 })).toBe('')
  })
})

describe('checkpointPopupHtml', () => {
  it('leads with the name, which is what a patrol matches against the paper', () => {
    const html = checkpointPopupHtml(checkpoint({ name: 'Post 4 – Gjern Bakker' }), 'ahead')

    expect(html.indexOf('Post 4')).toBeLessThan(html.length)
    expect(html.startsWith('<strong>')).toBe(true)
  })

  it('includes the window when there is one', () => {
    const html = checkpointPopupHtml(
      checkpoint({ openFrom: 1749844800, openUntil: 1749857400 }),
      'ahead',
    )

    expect(html).toContain('Åben')
  })

  it('omits the window line entirely when none is recorded', () => {
    const html = checkpointPopupHtml(checkpoint(), 'ahead')

    expect(html).not.toContain('Åben')
  })

  it('marks a visited post', () => {
    expect(checkpointPopupHtml(checkpoint(), 'visited')).toContain('Besøgt')
    expect(checkpointPopupHtml(checkpoint(), 'ahead')).not.toContain('Besøgt')
  })

  // "Klaret" alone, on a post they have never been to, invites exactly the wrong guess — that the app has
  // muddled them up. The popup says why it needs no visit, and must not claim they were there.
  it('explains a cleared post without claiming it was visited', () => {
    const html = checkpointPopupHtml(checkpoint(), 'cleared')

    expect(html).toContain('Klaret')
    expect(html).toContain('postlinjen')
    expect(html).not.toContain('Besøgt')
  })

  // Checkpoint names are written by organizers in another system and reach us over the event stream, so they
  // are not this app's input to trust. A stray `<` would break the marker; a tag would do worse.
  it('escapes a name from upstream', () => {
    const html = checkpointPopupHtml(
      checkpoint({ name: '<img src=x onerror=alert(1)>' }),
      'ahead',
    )

    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;img')
  })
})

describe('escapeHtml', () => {
  it('escapes the characters that matter in an attribute or a tag', () => {
    expect(escapeHtml('a & b')).toBe('a &amp; b')
    expect(escapeHtml('<b>')).toBe('&lt;b&gt;')
    expect(escapeHtml('say "hi"')).toBe('say &quot;hi&quot;')
  })

  // Ampersand first, or the escapes escape each other's output.
  it('does not double-escape its own output', () => {
    expect(escapeHtml('<')).toBe('&lt;')
    expect(escapeHtml('&lt;')).toBe('&amp;lt;')
  })
})
