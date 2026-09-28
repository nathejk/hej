# 440 — Read the EXIF capture time

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

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

- [x] `imaging.ReadShotAt(raw []byte) (time.Time, bool)`
- [x] Absent, malformed, zeroed (`0000:00:00 00:00:00`) and out-of-range values return `ok == false`
      rather than a wrong time
- [x] Both byte orders, as `ReadGPS`'s tests cover for GPS
- [x] Added to `fuzz_test.go` — this parses bytes from a file somebody handed us
- [x] No new module dependency

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R3, §8.

## What changed

`imaging.ReadShotAt(raw, loc) (time.Time, bool)`, beside `ReadGPS` and under the same rules: read before
re-encoding, every malformed case is "the file did not say", nothing reaches the stored bytes.

`DateTimeOriginal` (0x9003) in the Exif sub-IFD (0x8769) — the same sub-IFD-pointer shape `ReadGPS`
already walks, one tag along. **Deliberately no fallback to `DateTime` (0x0132)** in IFD0: that is the
*file's* timestamp, which a card reader or an editing tool rewrites, so falling back to it would silently
sort an album by "when this file was last touched". Absent is a better answer than a plausible wrong one,
because the caller has `uploadedAt` to fall back to and knows that it is falling back.

`ifdASCIIAt` is the new helper — the mirror image of `gpsRef`, which refuses a value held at an offset
because a hemisphere ref never legitimately has one. Here the opposite holds: 20 bytes can never be inline,
so a count of four or fewer is refused rather than read from the wrong place.

**Two semantic refusals, not just structural ones.** A camera whose clock was never set writes
`0000:00:00 00:00:00` or a field of spaces; one whose battery died reports 1980. Both are structurally
valid EXIF. `time.ParseInLocation` rejects the first two for free (month 0 is an error, not a wrap-around),
and a `[2000, 2100]` year bound rejects the third. At an event where half the cameras are borrowed this is
not a corner case, and sorting on 1980 would park those photographs at one end of the album — which reads
as a broken sort rather than as a camera nobody set.

The timestamp carries **no timezone**, so the location is a parameter (nil means UTC) rather than this
package importing `internal/eventtime`. `imaging` is a pure function of bytes and should stay that way;
the caller in task 441 passes `eventtime.Location()`.

## Beyond the task

`ReadGPS` had **no fuzz coverage**, and `fuzz_test.go`'s own comment claimed to cover "these two
functions" while naming the two that were fuzzed. `ReadGPS` walks strictly more structure than
`ReadOrientation` — a sub-IFD pointer, then RATIONALs at a second offset — and had table-driven tests for
every break somebody thought of and none for the ones nobody did. Writing `ReadShotAt`, which walks the
same shape again, is what made the gap visible. Added `FuzzReadShotAt` and `FuzzReadGPS`, both asserting a
sane *value* and not only the absence of a panic, and seeded the corpus with a file carrying each sub-IFD
so the pointer arithmetic actually runs instead of bailing at the TIFF header.

## Progress Log

- 2026-09-28 — Picked up. Read `ReadGPS` and `gpsRef` first: the sub-IFD walk and the bounds-checking rules
  are already established here, so this is a tag and a string parser rather than new machinery.
- 2026-09-28 — Split `parseExifTime` out so the string rules can be tested without building a JPEG around
  them. That is where the interesting cases are, since they come from cameras rather than attackers.
- 2026-09-28 — Added the year bound after thinking about borrowed cameras. It is the one piece of judgement
  in here that is not in the EXIF spec, so it is documented at the constant with the reason.
- 2026-09-28 — ✅ `shotat_test.go`: a byte-by-byte fixture builder (`jpegWithShotAt`) with an option per
  break, both byte orders, the unset-clock forms, the structural breaks, rubbish input, non-interference
  with `ReadOrientation`, and that `Prepare` destroys the value — the property that makes reading it before
  storage the only place this can be done.
- 2026-09-28 — ✅ Fuzzing. 3.0M executions on `FuzzReadShotAt`, 3.8M on `FuzzReadGPS`, no crashes.
- 2026-09-28 — Mutation-checked: removing the year bound fails the unset-clock cases; swapping
  `ParseInLocation` for `Parse` fails the location cases.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` all clean.
