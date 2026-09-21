# 358 — All printed timestamps in the event's timezone

**Status:** done
**Priority:** high
**Created:** 2026-09-21
**Picked up by:** agent session (Zed)
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

Maintainer instruction, 2026-09-21:

> all timestamps shall be printed in CEST tz

It was a bug report, not a preference. **Every timestamp on the public site was rendered in UTC** — two hours
early, all through the night the pages are about.

Measured, not inferred: patrol 71's first scan of 2026 is epoch `1789761524`, which is 21:58:44 CEST. The patrol
page printed *kl. 19:58*.

### Why, and why nobody noticed

Nothing was *converting* anything. Scans are stored as Unix seconds; `time.Unix` returns an instant in the
process's local zone; the api container has no `/etc/timezone`, so local is UTC; and the formatter printed
whatever zone its input happened to carry. A rendering that inherits its timezone from its input has no
timezone at all.

Three things hid it:

- **The diploma was right.** It has converted to Copenhagen since task 345, so one surface showed the correct
  clock — and it is the surface somebody would check.
- **A test pinned the wrong answer.** `TestPatrolPageShowsTheFinishTimeAndTheDiplomaSlot` asserted
  `"20. september 2026 kl. 07:00"`, and its own comment said it existed *"so a timezone or date-rollover bug
  shows up here"*. Meanwhile `TestTheDiplomaUsesTheGatesFinishTimeInLocalTime` asserted hour 9 **from the same
  fixture**. Two tests, two files, one instant, two answers.
- **19:58 is plausible.** A night race starting in the evening makes a two-hour error look like a schedule.

### A second bug found on the way

`glimtpublic.go` formatted dates with the layout string `"2. januar 2006 kl. 15:04"`. **`januar` is not a Go
layout token** — Go copied it through as a literal, so *every* public glimt was dated in January. A September
photograph read *"18. januar 2026"*.

It had a comment claiming it matched the other surface's format, which is what stopped anybody comparing them.
Verified before fixing: `time.Unix(1789761524, 0).UTC().Format("2. januar 2006 kl. 15:04")` →
`18. januar 2026 kl. 19:58`. Both bugs in nine characters.

### The fix

**Server:** a new `internal/eventtime` — one zone constant, a cached `Location()`, `In()`, and the two Danish
formatters. Every rendering boundary goes through it: the public site's template func, the glimt page's, the
patrol page's finish label, and the diploma (whose own `eventLocation` is now a wrapper over it).

**App:** a new `helpers/eventTime.ts`. All eight `Intl`/`toLocale*` call sites converted. They were not wrong
in Denmark — they used the *device's* zone — but that is wrong on a phone set to somewhere else, for a leader
reading a patrol's scan list from abroad, and for any screenshot in a bug report.

**The zone, not the offset.** The instruction says CEST; this loads `Europe/Copenhagen`, which is CEST in
September and CET in winter. Pinning +02:00 would be wrong for a page still being read in November — and the
page is the archive, so it will be.

### What is deliberately not touched

- **Wire formats.** JSON carries instants, not rendered strings; a timezone belongs to how a human is shown a
  moment. Only rendering converts.
- **`internal/scans/projection.go`'s `.UTC().Format("20060102150405")`** — that is an *id*, not a printed time.
  Changing it would change every scan's identity.

## Acceptance Criteria

- [x] The public patrol page prints scan times in the event's zone (verified against real 2026 data: 21:58,
      22:12, 23:08, 00:06)
- [x] The public glimt list prints the right month (verified: *19. september 2026*, was *januar*)
- [x] The finish time agrees between the page and the diploma
- [x] Midnight rollover keeps the right date — tested both sides
- [x] Winter renders CET and summer CEST, so the zone is doing the work
- [x] All twelve Danish month names are tested, since the bug this replaces was right for exactly one of them
- [x] A guard fails the build if any app source formats a date without the zone — verified by planting a
      violation and watching it name the file
