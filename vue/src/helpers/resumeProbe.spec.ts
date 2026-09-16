import { describe, expect, it } from 'vitest'

import {
  LOOP_EVENTS,
  UNMARKED,
  groupByMark,
  toMarkdown,
  verdictFor,
  wouldCheck,
  type ProbeEntry,
} from '@/helpers/resumeProbe'

// The probe's value is entirely in its verdict, so the verdict is what is tested here. A page that
// rendered a plausible-looking table and got the "did the loop hear this?" column wrong would be worse
// than no page: it would produce a confident, wrong answer to the one question task 280 exists to
// settle, and the answer would then be quoted in a task log for the rest of the project.

function entry(over: Partial<ProbeEntry> = {}): ProbeEntry {
  return { at: 1_000, event: 'visibilitychange', visibility: 'visible', ...over }
}

describe('wouldCheck', () => {
  it('counts the events the loop listens to', () => {
    expect(wouldCheck(entry({ event: 'visibilitychange', visibility: 'visible' }))).toBe(true)
    expect(wouldCheck(entry({ event: 'online' }))).toBe(true)
  })

  // The distinction that makes this measurement mean anything: the loop stops when the document is
  // hidden, so a `visibilitychange` to hidden is an event, not a check.
  it('does not count a visibilitychange to hidden', () => {
    expect(wouldCheck(entry({ event: 'visibilitychange', visibility: 'hidden' }))).toBe(false)
  })

  // The whole hypothesis under test: these may be the *only* events on some resume paths, and the loop
  // hears none of them.
  it('does not count pageshow, focus or resume', () => {
    expect(wouldCheck(entry({ event: 'pageshow', persisted: true }))).toBe(false)
    expect(wouldCheck(entry({ event: 'focus' }))).toBe(false)
    expect(wouldCheck(entry({ event: 'resume' }))).toBe(false)
  })

  // Deliberately duplicated from `browserFreshnessTarget` rather than imported: the probe has to be
  // able to *disagree* with the implementation, so this pins the claim rather than tracking it.
  it('states the loop’s listeners as data', () => {
    expect(LOOP_EVENTS).toEqual(['visibilitychange', 'online', 'mount'])
  })

  // `useFreshnessLoop` checks on construction when the document is visible — "mounting counts as
  // foregrounding" (PRD 017 §6). Leaving `mount` out of LOOP_EVENTS made the cold-start control report
  // itself as a failure, which is the most misleading thing this page could do: the control is what tells
  // you whether to trust the other four rows.
  it('counts a mount, because the loop checks on construction', () => {
    expect(wouldCheck(entry({ event: 'mount' }))).toBe(true)
  })
})

describe('verdictFor', () => {
  it('says the loop would have checked', () => {
    expect(verdictFor([entry({ event: 'pageshow' }), entry({ event: 'visibilitychange' })])).toBe(
      'checked',
    )
  })

  // The finding that would make PRD 017 silently stale on a resume path, and the reason the page shows
  // a verdict instead of only rows.
  it('distinguishes "events fired but none of them was a check"', () => {
    expect(
      verdictFor([
        entry({ event: 'pageshow', persisted: true }),
        entry({ event: 'focus' }),
        entry({ event: 'visibilitychange', visibility: 'hidden' }),
      ]),
    ).toBe('events-but-no-check')
  })

  it('reports an empty run as none', () => {
    expect(verdictFor([])).toBe('none')
  })

  // The false alarm this page produced on its first real use: opening it records a mount and nothing
  // else, and the verdict read "hændelser, men INTET tjek" — which looks exactly like the damning finding
  // while meaning only "you have opened the page".
  it('does not report a bare mount as a resume measurement', () => {
    expect(verdictFor([entry({ event: 'mount' })])).toBe('not-a-resume')
  })

  // But a mount alongside real events is a cold start, which is a resume path in its own right — the
  // control case — and there the loop genuinely does check.
  it('reports a cold start as checked', () => {
    expect(
      verdictFor([entry({ event: 'pagehide', visibility: 'hidden' }), entry({ event: 'mount' })]),
    ).toBe('checked')
  })
})

describe('groupByMark', () => {
  it('keeps consecutive entries of one path together', () => {
    const groups = groupByMark([
      entry({ mark: 'lås', event: 'pagehide' }),
      entry({ mark: 'lås', event: 'pageshow' }),
      entry({ mark: 'app-skifter', event: 'focus' }),
    ])
    expect(groups.map((g) => g.mark)).toEqual(['lås', 'app-skifter'])
    expect(groups[0].entries).toHaveLength(2)
  })

  // Returning to an earlier path starts a new group rather than merging with the first: the two runs
  // happened at different times and a merged verdict could hide a failure inside a success.
  it('starts a new group when a path is revisited', () => {
    const groups = groupByMark([
      entry({ mark: 'lås' }),
      entry({ mark: 'app-skifter' }),
      entry({ mark: 'lås' }),
    ])
    expect(groups).toHaveLength(3)
  })

  it('labels unmarked entries rather than dropping them', () => {
    expect(groupByMark([entry()])[0].mark).toBe(UNMARKED)
  })
})

describe('toMarkdown', () => {
  it('produces a table per path, with the verdict and the gaps', () => {
    const md = toMarkdown(
      [
        entry({ mark: 'app-skifter', event: 'pagehide', visibility: 'hidden', at: 1_000_000 }),
        entry({ mark: 'app-skifter', event: 'pageshow', persisted: true, at: 1_012_000 }),
      ],
      'iPhone · standalone=true',
    )

    expect(md).toContain('iPhone · standalone=true')
    expect(md).toContain('**app-skifter**')
    expect(md).toContain('INTET tjek')
    expect(md).toContain('`pageshow`')
    // The gap is what makes a suspend visible at all.
    expect(md).toContain('12s')
  })

  it('says so when nothing was recorded', () => {
    expect(toMarkdown([], 'x')).toContain('Ingen hændelser optaget')
  })
})
