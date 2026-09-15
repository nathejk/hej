import { describe, expect, it } from 'vitest'

import {
  checkpointMarkerStyle,
  checkpointPopupHtml,
  checkpointWindowText,
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
    const ahead = checkpointMarkerStyle(false)
    const visited = checkpointMarkerStyle(true)

    expect(ahead.background).not.toBe(visited.background)
    expect(ahead.glyph).not.toBe(visited.glyph)
  })

  // The map also carries the patrol's own position (blue), its scan registrations (slate) and bandit catches
  // (red). A checkpoint colour that collided with any of those would make two different things read the
  // same.
  it('does not reuse the colours already on the map', () => {
    const used = ['#2563eb', '#3b82f6', '#0f172a', '#b91c1c']

    for (const visited of [true, false]) {
      expect(used).not.toContain(checkpointMarkerStyle(visited).background)
    }
  })

  it('labels both states in Danish', () => {
    expect(checkpointMarkerStyle(false).label).toBe('Post')
    expect(checkpointMarkerStyle(true).label).toBe('Besøgt post')
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
    const html = checkpointPopupHtml(checkpoint({ name: 'Post 4 – Gjern Bakker' }), false)

    expect(html.indexOf('Post 4')).toBeLessThan(html.length)
    expect(html.startsWith('<strong>')).toBe(true)
  })

  it('includes the window when there is one', () => {
    const html = checkpointPopupHtml(
      checkpoint({ openFrom: 1749844800, openUntil: 1749857400 }),
      false,
    )

    expect(html).toContain('Åben')
  })

  it('omits the window line entirely when none is recorded', () => {
    const html = checkpointPopupHtml(checkpoint(), false)

    expect(html).not.toContain('Åben')
  })

  it('marks a visited post', () => {
    expect(checkpointPopupHtml(checkpoint(), true)).toContain('Besøgt')
    expect(checkpointPopupHtml(checkpoint(), false)).not.toContain('Besøgt')
  })

  // Checkpoint names are written by organizers in another system and reach us over the event stream, so they
  // are not this app's input to trust. A stray `<` would break the marker; a tag would do worse.
  it('escapes a name from upstream', () => {
    const html = checkpointPopupHtml(
      checkpoint({ name: '<img src=x onerror=alert(1)>' }),
      false,
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
