# 440 — Read the EXIF capture time

**Status:** open
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 024 §6 R3. An album sorted "by time" means *when the photograph was taken*, and nothing in the
library knows that: `photo` has `uploadedAt` and nothing else. `uploadedAt` is not a usable substitute —
the uploader runs three requests at a time, so arrival order is not even file order.

`internal/imaging` already reads EXIF with the repo's own code and **no dependency**: `ReadOrientation`,
`ReadGPS`, `findExifSegment`, `ifdEntry`, `ifdLongTag`. `DateTimeOriginal` is tag `0x9003` in the Exif
sub-IFD at `0x8769` — the same sub-IFD-pointer shape `ReadGPS` already walks — so this is a `ReadShotAt`
beside it rather than a new library.

The value is ASCII `YYYY:MM:DD HH:MM:SS` with **no timezone**. Parsed as the event's local time, because
a Nathejk photograph was taken at Nathejk; see `internal/eventtime` for what the rest of the service
means by local.

Read from the **original bytes before re-encoding**, like the GPS fix, since re-encoding strips all EXIF.

## Acceptance Criteria

- [ ] `imaging.ReadShotAt(raw []byte) (time.Time, bool)`
- [ ] Absent, malformed, zeroed (`0000:00:00 00:00:00`) and out-of-range values return `ok == false`
      rather than a wrong time
- [ ] Both byte orders, as `ReadGPS`'s tests cover for GPS
- [ ] Added to `fuzz_test.go` — this parses bytes from a file somebody handed us
- [ ] No new module dependency

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R3, §8.
