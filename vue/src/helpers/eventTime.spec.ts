import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import { EVENT_TIME_ZONE, clock, clockWithSeconds, dayMonth, weekdayClock } from '@/helpers/eventTime'

// Every clock the app prints is the event's clock (task 358).
//
// # Why a source-walking test and not only unit tests
//
// The maintainer's rule is *"all timestamps shall be printed in CEST tz"* — a property of the whole codebase,
// not of one function. Unit tests below pin that these helpers convert; this file's second half pins that
// nothing goes around them.
//
// That guard is the valuable half. The bug it prevents is not a wrong output, it is an **omission**: a new view
// calls `toLocaleTimeString('da-DK', …)`, which looks correct, reads correct in review, and renders correct on
// every machine in Denmark. It is only wrong on a device whose timezone is not ours — which is exactly the
// device nobody tests on.

const SRC = resolve(__dirname, '..')

function sourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry)
    if (statSync(path).isDirectory()) {
      out.push(...sourceFiles(path))
      continue
    }
    if (!/\.(ts|vue)$/.test(entry)) continue
    if (/\.spec\.ts$/.test(entry)) continue
    out.push(path)
  }
  return out
}

describe('the event clock', () => {
  // The instant the server-side bug was measured from: patrol 71's first scan of 2026, 21:58:44 CEST.
  const firstScan = new Date(1789761524 * 1000)

  it('renders in the event’s timezone, whatever the machine is set to', () => {
    expect(clock(firstScan)).toBe('21.58')
    expect(clockWithSeconds(firstScan)).toBe('21.58.44')
    expect(weekdayClock(firstScan)).toContain('21.58')
  })

  // A night race crosses midnight, and a wrong conversion moves the date as well as the clock: 22:30 UTC is
  // half past midnight *the next day* in Copenhagen.
  it('keeps the right day across midnight', () => {
    const afterMidnight = new Date(Date.UTC(2026, 8, 18, 22, 30))
    expect(dayMonth(afterMidnight)).toContain('19')
  })

  // Summer is +2 and winter is +1, which is the whole reason this is a zone and not the offset the instruction
  // named. A page read in November must still show the right time.
  it('follows the zone rather than pinning +02:00', () => {
    expect(EVENT_TIME_ZONE).toBe('Europe/Copenhagen')
    expect(clock(new Date(Date.UTC(2026, 0, 1, 12, 0)))).toBe('13.00')
    expect(clock(new Date(Date.UTC(2026, 6, 1, 12, 0)))).toBe('14.00')
  })
})

describe('nothing formats a time behind the helper’s back', () => {
  const files = sourceFiles(SRC)

  // Sanity: a walk that found nothing would pass every assertion below.
  it('walks the source tree', () => {
    expect(files.length).toBeGreaterThan(50)
  })

  it('has no date formatting without the event timezone', () => {
    const offenders: string[] = []

    for (const path of files) {
      if (path.endsWith(join('helpers', 'eventTime.ts'))) continue
      const source = readFileSync(path, 'utf8')

      // `toLocaleTimeString` / `toLocaleDateString` / `toLocaleString` take the device's zone unless told
      // otherwise, and the options object is where the telling would have to happen. Rather than parse it, the
      // rule is simpler and stricter: don't call them at all — use the helper.
      for (const call of source.match(/toLocale(Time|Date)?String\s*\(/g) ?? []) {
        offenders.push(`${path.slice(SRC.length + 1)}: ${call.trim()}`)
      }

      // A formatter built by hand must carry the zone. `eventFormat` is the way to get one that does.
      for (const match of source.match(/new Intl\.DateTimeFormat\([\s\S]{0,200}?\)/g) ?? []) {
        if (!match.includes('timeZone')) {
          offenders.push(`${path.slice(SRC.length + 1)}: new Intl.DateTimeFormat without timeZone`)
        }
      }
    }

    expect(offenders, `use @/helpers/eventTime instead:\n${offenders.join('\n')}`).toEqual([])
  })
})
