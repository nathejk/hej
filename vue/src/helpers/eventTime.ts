/**
 * The event's timezone, and every clock the app prints (task 358).
 *
 * # Why this module exists
 *
 * The maintainer's instruction, 2026-09-21: *"all timestamps shall be printed in CEST tz"*. The BFF had a real
 * bug — the public pages rendered UTC, two hours early — and the app had a subtler one: every formatter here
 * asked `Intl` for `da-DK` and **no timezone**, which means the *device's*. That is right for a phone in Denmark
 * and wrong for every other case, and the cases are not exotic:
 *
 *   - A phone whose timezone is wrong, or still set to somewhere a family drove back from.
 *   - A leader abroad reading a patrol's scan list, who would see a different night from the patrol's.
 *   - A screenshot in a bug report, which is now unambiguous about what clock produced it.
 *
 * The event happens on one clock. Two people looking at the same scan list should read the same time, and it
 * should be the time on the wall at the checkpoint.
 *
 * # CEST versus Europe/Copenhagen
 *
 * The zone, not the offset — matching `internal/eventtime` on the server, for the same reason: `Europe/Copenhagen`
 * is CEST in September and CET in winter, and pinning +02:00 would go wrong in late October while the pages are
 * still being read.
 */

/** The IANA zone. One constant so a grep finds one answer, and so the spec can assert on it. */
export const EVENT_TIME_ZONE = 'Europe/Copenhagen'

/** What every formatter here starts from. */
const base: Intl.DateTimeFormatOptions = { timeZone: EVENT_TIME_ZONE }

/**
 * Builds a formatter pinned to the event's timezone.
 *
 * Exported so a caller with an unusual shape does not have to reach for `Intl` directly and forget the zone —
 * which is the mistake this module exists to make hard. `eventTime.spec.ts` fails the build if any source file
 * constructs a date formatter without it.
 */
export function eventFormat(options: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
  return new Intl.DateTimeFormat('da-DK', { ...base, ...options })
}

const clockFormat = eventFormat({ hour: '2-digit', minute: '2-digit' })
const clockSecondsFormat = eventFormat({ hour: '2-digit', minute: '2-digit', second: '2-digit' })
const weekdayClockFormat = eventFormat({ weekday: 'short', hour: '2-digit', minute: '2-digit' })
const dayMonthFormat = eventFormat({ day: 'numeric', month: 'short' })

/** A time of day: `21:58`. */
export function clock(at: Date | number): string {
  return clockFormat.format(at)
}

/** A time of day with seconds: `21:58:44`. For diagnostics, where the seconds are the point. */
export function clockWithSeconds(at: Date | number): string {
  return clockSecondsFormat.format(at)
}

/** A weekday and a time: `fre. 21.58`. For a night that crosses midnight, where the day matters. */
export function weekdayClock(at: Date | number): string {
  return weekdayClockFormat.format(at)
}

/** A day and a month: `18. sep.`. For something older than a week, where the clock no longer helps. */
export function dayMonth(at: Date | number): string {
  return dayMonthFormat.format(at)
}
