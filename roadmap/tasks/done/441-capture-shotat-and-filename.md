# 441 — Capture the capture time on upload (and not the filename)

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 024 §6 R3/R5. Needs task 440 first.

Two new fields on a photograph, both of which the upload already has and throws away:

- `shotAt` — from `imaging.ReadShotAt`, read before re-encoding. `DATETIME NULL`; null means the file did
  not say, and the sort falls back to `uploadedAt` for that photograph.
- `fileName` — from the multipart part's filename. `r.FormFile("photo")` reads the header today and
  discards it (`file, _, err :=`). `VARCHAR(255) NOT NULL DEFAULT ""`, stored as given and never parsed
  for meaning. Empty on the raw-body path, which the drop zone does not use.

**A filename is person-shaped and the privacy guard will not catch it.** Usually `IMG_0123.JPG`, but it is
free text from somebody's computer and can name a person (`mor-og-far.jpg`). The public site names no human
being with one written-down exception (PRD 011 §0b.1, task 393), and `isPersonShaped` in
`publicprivacy_test.go` does **not** flag a field called `fileName` today. So:

- add `filename` to that guard's needles, and
- except it by name for the admin surface with the reasoning, exactly as `credit` was excepted in task 393
  — same treatment, opposite conclusion.

Event fields are additive and optional, so replaying an older log folds to the defaults above.

## Acceptance Criteria

- [x] `photo.Uploaded.ShotAt`; column and fold to match
- [ ] ~~`.FileName`~~ — refused by the privacy guards; task 448 holds the decision
- [x] Read before re-encoding, alongside the GPS fix
- [ ] ~~The multipart filename is captured~~ — see task 448
- [x] `filename` is in `isPersonShaped`'s needles — as a **trap**, not an exception (task 448)
- [x] No public response carries either field
- [x] Replaying a log without the field folds to null

## Progress Log

- 2026-09-28 — Task created from PRD 024 §6 R3/R5 and its privacy note.

## What shipped, and what did not

**`shotAt` shipped. `fileName` did not, and that is the interesting half.**

### shotAt

`photo.Uploaded.ShotAt *time.Time`, `photo.shotAt DATETIME NULL`, folded, and on `LibraryPhoto` as a string
in the projection's own `YYYY-MM-DD HH:MM:SS` — the same shape as `UploadedAt`, which is useful rather than
merely consistent: the format is fixed-width, so comparing the two as **strings** compares them
chronologically and the sort in task 443 needs no parsing and has no parse error to handle.

Read in `storeAlbumImage`, next to `imaging.ReadGPS`, where the comment already says "Read first. After
Prepare there is nothing left to read — which is the whole design." In the **event's** timezone
(`eventtime.Location()`), not the server's: EXIF carries no offset, and a UTC reading of 23:41 on a
September night lands it on the previous day, which is precisely when Nathejk photographs are taken.

**The fold fills a gap and never opens one** — `shotAt=COALESCE(VALUES(shotAt), shotAt)` rather than a plain
`VALUES()`. Re-dragging a card is the documented recovery procedure (task 372) and the id is the hash of the
*stored rendition*, so the same `photoId` can legitimately arrive from a **different file**: the same pixels
with the EXIF stripped, or re-saved by an editor. A plain upsert would let that second file blank a capture
time the first one supplied, and an album sorted by time would reorder underneath the curator for a
photograph nobody meant to touch. The other direction is worth having — a photograph first uploaded without
EXIF and later re-uploaded from the original does gain its time — so the rule is exactly "a later file may
add what the earlier one lacked, and may not take away". `caption` and `credit` are absent from the clause
entirely rather than treated this way, because those are a curator's words.

### fileName — stopped by two privacy guards, correctly

The column, the event field, the curator field, the sanitiser and the plumbing were all written. Then:

- `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` failed on `Uploaded.FileName`;
- `TestTheLibraryTablesDeclareNoPersonShapedColumn` failed on the column.

Both rest on PRD 022 §6 — *no personal data in the library projection, with one written-down exception* —
and the exception is `credit`, which is admitted because it names **a consenting adult volunteer** in a
string typed **in order to be published**. A filename meets neither test: it is typed on somebody's laptop
for a private reason, about a person who may well be a child, and nobody asked. `IMG_0123.JPG` is harmless;
`mor-og-far.jpg` is the same field, and no code can tell them apart at upload time.

This PRD's own privacy note had only said "never on a public read", which was an understatement: the posture
here is about the **column**, because a column is in every backup taken since it was added.

So the filename half was **removed rather than excepted**, and the decision became **task 448**. Excepting a
guard like that to make a build go green would defeat the one mechanism that makes this codebase's privacy
claims worth anything.

What was left behind instead:

- `readAdminUpload` does not read `header.Filename`, with the reason at the call site;
- `filename` is now a needle in `isPersonShaped` — **a trap for a field that does not exist**, so that
  anybody who adds it has to come and write down why, which is exactly how `credit` came to be excepted;
- `TestTheUploadDoesNotReadTheFilename` covers the *read*, because the filename is available for free from
  `r.FormFile`: this rule will be broken by convenience, not by decision.

PRD 024's `time-*` and `manual` modes are unaffected. R5 and R6 wait on task 448.

## Progress Log

- 2026-09-28 — Picked up. Both fields written end to end: events, columns, fold, curator read, handler.
- 2026-09-28 — Chose `COALESCE` over leaving `shotAt` out of the upsert clause entirely (the `caption`
  treatment). These are file-derived rather than curator-authored, so a *better* file should be able to fill
  a gap; what must never happen is a worse file opening one.
- 2026-09-28 — ✅ `shotAt` end to end, with `TestTheUploadReadsTheCaptureTimeBeforeReEncoding` pinning the
  one ordering the whole feature depends on. That ordering is invisible in a diff and is what a tidy-up of
  `storeAlbumImage` would reverse.
- 2026-09-28 — ❌ `fileName` refused by `libraryprivacy_test.go` and `publicprivacy_test.go`. Stopped rather
  than excepted; opened task 448 for the decision and reverted that half. Updated PRD 024 §6 R5/R6, its
  privacy note and §11 Q5 to say so.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` all clean.
