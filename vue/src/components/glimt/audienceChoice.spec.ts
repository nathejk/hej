import { describe, expect, it } from 'vitest'

import {
  CONSENT_NOTE,
  DEFAULT_AUDIENCE,
  TEAM_DISCLOSURE,
  attributionNote,
  audienceOptions,
  groupLabelFor,
  retentionNote,
} from '@/components/glimt/audienceChoice'
import { ALL_ROLES } from '@/config/roles'

// The audience choice (task 317).
//
// With no approval queue in front of the public scope (PRD 019 §0), there is no moderator between a
// twelve-year-old tapping "Offentligt" and a photograph being on the open web. The consequence line
// under that option is the only thing standing there — so it is tested, not left as a string in a
// template.

describe('audienceOptions', () => {
  const options = audienceOptions('spejder')

  it('offers exactly the three audiences the BFF accepts', () => {
    expect(options.map((o) => o.value)).toEqual(['group', 'nathejk', 'public'])
  })

  // The order is load-bearing: the composer selects the first by default, so this is what makes the
  // safe choice the free one and widening an act rather than an accident.
  it('puts the narrowest first, and defaults to it', () => {
    expect(options[0].value).toBe('group')
    expect(DEFAULT_AUDIENCE).toBe('group')
    expect(options[0].value).toBe(DEFAULT_AUDIENCE)
  })

  it('gives every option a consequence line', () => {
    for (const option of options) {
      expect(option.consequence.length).toBeGreaterThan(10)
      // A sentence, not a label — it has to read as a consequence rather than as a category.
      expect(option.consequence).toMatch(/\.$/)
    }
  })

  // The line that is doing the actual work. "Offentligt" alone is a word a child may reasonably read
  // as "everyone at the event", so the consequence has to name the internet and the outside.
  it('says plainly that the public option reaches beyond Nathejk', () => {
    const pub = options.find((o) => o.value === 'public')!
    expect(pub.reachesOutside).toBe(true)
    expect(pub.consequence).toContain('internettet')
    expect(pub.consequence).toContain('uden for Nathejk')
  })

  it('marks only the public option as reaching outside', () => {
    // Drives the warning styling. If the internal scopes were marked too, the warning would stop
    // meaning anything.
    expect(options.filter((o) => o.reachesOutside).map((o) => o.value)).toEqual(['public'])
  })

  // **The bug this replaced, pinned so it cannot come back.**
  //
  // Until 2026-09-17 the narrowest option read "Min patrulje" with "Kun dem der er med i din gruppe
  // kan se det." `users.MaySeeGlimt` matches this scope on group, and **never on the patrulje
  // number** — so it reaches every spejder at the event, some 750 people, not the six in a patrulje.
  //
  // We told a twelve-year-old their photograph was going to their patrulje and sent it to the whole
  // division. The code was correct and the label was a lie, which is why no test caught it: nothing
  // compares a label to a predicate. These assertions are the closest thing to that comparison.
  describe('the narrowest option describes a group, never a patrulje', () => {
    it('does not call the group scope a patrulje or a klan', () => {
      for (const role of ALL_ROLES) {
        const group = audienceOptions(role)[0]
        const text = `${group.label} ${group.consequence}`
        expect(text, `role ${role}: "${text}"`).not.toMatch(/min patrulje|min klan|din patrulje|din klan/i)
      }
    })

    it('names the whole population for a spejder', () => {
      const group = audienceOptions('spejder')[0]
      expect(group.label).toBe('Alle spejderpatruljer')
      expect(group.consequence).toBe('Kun dem som deltager som spejdere kan se det.')
    })

    it('names the whole population for a bandit', () => {
      const group = audienceOptions('bandit')[0]
      expect(group.label).toBe('Alle klaner')
      expect(group.consequence).toContain('banditter')
    })

    it('groups every crew-ish role together, as the BFF does', () => {
      // users.GlimtGroupFor puts postmandskab, guide, samarit, gøgler and crew in one bucket, so the
      // label must not imply a narrower one.
      for (const role of ['postmandskab', 'guide', 'samarit', 'gøgler', 'crew']) {
        expect(audienceOptions(role)[0].label).toBe('Alt crew')
      }
    })

    // An unknown role must still get a label that does not understate reach.
    it('falls back to the widest of the group labels', () => {
      for (const role of [null, undefined, '', 'role-this-build-does-not-know']) {
        expect(audienceOptions(role)[0].label).toBe('Alt crew')
      }
    })
  })

  // The middle option names who else is inside Nathejk, rather than saying "de andre grupper" — our
  // word for a population a participant thinks of by name.
  describe('the nathejk option names the other populations', () => {
    it('tells a spejder about the banditter', () => {
      expect(audienceOptions('spejder')[1].consequence).toContain('banditterne')
    })

    // Role-dependent because telling a bandit that banditter can see it says nothing.
    it('tells a bandit about the spejdere', () => {
      const text = audienceOptions('bandit')[1].consequence
      expect(text).toContain('spejderne')
      expect(text).not.toContain('banditterne')
    })

    it('tells crew about both', () => {
      const text = audienceOptions('crew')[1].consequence
      expect(text).toContain('spejderne')
      expect(text).toContain('banditterne')
    })
  })

  it('uses no jargon a twelve-year-old would need explained', () => {
    const text = options.map((o) => `${o.label} ${o.consequence}`).join(' ').toLowerCase()
    for (const jargon of ['synlighed', 'audience', 'scope', 'privat', 'metadata']) {
      expect(text).not.toContain(jargon)
    }
  })
})

describe('groupLabelFor', () => {
  it('names the member’s actual group, not the word "gruppe"', () => {
    // "Kun min patrulje" is a sentence a spejder can check against reality; "Min gruppe" needs them
    // to know what the app means by it.
    expect(groupLabelFor('spejder')).toBe('Min patrulje')
    expect(groupLabelFor('bandit')).toBe('Min klan')
  })

  it('puts every crew-ish role in one bucket', () => {
    // They share one audience bucket server-side (users.GlimtGroupFor), including gøgler and the
    // unclassified fallback, so the label must not promise a narrower reach than they get.
    for (const role of ['crew', 'samarit', 'guide', 'postmandskab', 'gøgler']) {
      expect(groupLabelFor(role)).toBe('Crew')
    }
  })

  it('falls back rather than rendering undefined', () => {
    expect(groupLabelFor(null)).toBe('Crew')
    expect(groupLabelFor(undefined)).toBe('Crew')
  })
})

describe('retentionNote', () => {
  it('states the deployment’s real number', () => {
    expect(retentionNote('group', 90, 30)).toBe('Det bliver slettet efter 90 dage.')
  })

  // How long the *open web* keeps it is the part a member is most entitled to know before choosing
  // that option.
  it('uses the public window for the public option', () => {
    expect(retentionNote('public', 90, 30)).toBe('Det ligger offentligt i 30 dage.')
  })

  it('falls back to the ordinary window when no public one is set', () => {
    expect(retentionNote('public', 90, 0)).toBe('Det bliver slettet efter 90 dage.')
  })

  // Saying nothing beats inventing a number. A dev deployment runs with retention off, and "0 dage"
  // would be both wrong and alarming.
  it('says nothing when retention is disabled', () => {
    expect(retentionNote('group', 0, 0)).toBe('')
    expect(retentionNote('public', 0, 0)).toBe('')
  })

  it('gets the singular right', () => {
    expect(retentionNote('group', 1, 0)).toContain('1 dag.')
    expect(retentionNote('group', 2, 0)).toContain('2 dage.')
  })
})

describe('the quiet lines PRD 019 §6 requires', () => {
  // A photograph contains people who did not choose to be in it. Not a checkbox: a forced tick
  // trains people to tick it, and this has to work at the moment of deciding.
  it('asks about the other people in the picture', () => {
    expect(CONSENT_NOTE).toContain('spørg dem først')
  })

  // Disclosed, not discovered. The Team section can read every glimt at every scope, so "Min
  // gruppe" is not the same as "only my gruppe" — and a participant is entitled to know that before
  // they choose it, not afterwards from a privacy page they never open.
  it('discloses that Team sees everything', () => {
    expect(TEAM_DISCLOSURE).toContain('Team')
    expect(TEAM_DISCLOSURE).toContain('alle glimt')
  })

  // The single most reassuring true thing about this feature, and one a member will assume the
  // opposite of — every other app they use puts their name on what they post.
  it('says the glimt is signed with the hold, not the name', () => {
    expect(attributionNote('spejder')).toBe(
      'Glimtet bliver vist med din patrulje — ikke med dit navn.',
    )
    expect(attributionNote('bandit')).toBe('Glimtet bliver vist med din klan — ikke med dit navn.')
    expect(attributionNote('crew')).toContain('sektion')
    for (const role of ['spejder', 'bandit', 'crew', null]) {
      expect(attributionNote(role)).toContain('ikke med dit navn')
    }
  })
})
