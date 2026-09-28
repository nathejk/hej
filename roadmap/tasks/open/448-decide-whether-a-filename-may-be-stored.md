# 448 — Decide whether the library may store a filename

**Status:** open
**Priority:** high
**Created:** 2026-09-28
**Picked up by:**
**Started:**
**Completed:**

## Description

**This is a decision, not an implementation. It blocks PRD 024's two `filename-*` sort modes and nothing
else.**

PRD 024 §6 R5 says an album can be sorted by the name the photographer's file had, which needs a
`photo.fileName` column. Task 441 implemented it. **Two existing privacy guards refused it**, which is the
guards doing exactly what they were built for:

- `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` — walks every struct in the `photo` package and the
  admin handlers;
- `TestTheLibraryTablesDeclareNoPersonShapedColumn` — walks the schema, because "a column outlives the
  commit that added it — the values are in every backup taken since".

Both rest on PRD 022 §6: *"No personal data in the new projection — with one written-down exception. The
library row has no uploader, no curator, no person id, no phone number, and emphatically no
`phoneParent`."*

### Why a filename is not covered by the `credit` exception

The credit line is the one field in this service meant to name a human being, and PRD 022 §6 admits it on
two grounds. A filename meets neither:

| | credit | filename |
|---|---|---|
| Who is named | a consenting adult volunteer, in a professional capacity | whoever the photographer was thinking of — possibly a child |
| Why it exists | typed **in order to be published** | typed on a laptop, for a private reason, years of habit |
| Consent | asked to be credited | never asked, never told |

`IMG_0123.JPG` is harmless and is what most files are called. `mor-og-far.jpg` and
`malthe-ved-baalet.jpg` are the same field. The guard cannot tell them apart, and neither can we at upload
time.

### What was done in the meantime

Task 441 shipped **only** the capture time (`shotAt`), which is not person-shaped. The filename half was
removed rather than excepted:

- no `fileName` column, no event field, no curator field;
- `readAdminUpload` explicitly does **not** read `header.Filename`, with a comment saying why;
- `filename` was added to `isPersonShaped`'s needles as a **trap for a field that does not exist**, so that
  anybody adding it has to come and write down why it is allowed — the mechanism by which `credit` came to
  be excepted;
- `TestTheUploadDoesNotReadTheFilename` covers the read, because the filename is *available for free* from
  `r.FormFile`, so this rule gets broken by convenience rather than by decision.

So `time-asc` / `time-desc` / `manual` are unaffected. Only the two `filename-*` modes wait on this.

## The options

**A — Except it, and narrow PRD 022 §6.** Add the column, except `filename` in both guards with the
reasoning written down, and state the narrowing in PRD 022 §6 as task 393 did for `credit`. Honest, and it
does put every photographer's filenames in every backup for the sake of one sort mode.

**B — Drop the `filename-*` modes.** PRD 024 §11 Q5 already asked whether they were worth having, given
nothing already uploaded can have one. Costs the least, removes a want that may be real.

**C — Store a derived sequence, not the name.** What a curator almost certainly means by "filename order"
is *the camera's counter*: `IMG_0123.JPG` → 123. An integer names nobody, survives a backup, and sorts a
card correctly. It is lossy and surprising when filenames are not counter-shaped (`mor-og-far.jpg` → no
counter), and mixed cameras in one album would interleave by counter rather than by camera. Needs a stated
fallback, probably `shotAt`.

**D — Keep it out of the projection and off the log entirely**, using the filename only within the request
that carries it. This does not work for PRD 024: the sort runs when photographs are *added to an album*,
long after the upload, so the value has to persist to be usable. Recorded so it is not re-proposed.

## Acceptance Criteria

- [ ] A decision recorded here, with reasoning
- [ ] If A: PRD 022 §6 narrowed in writing; both guards excepted **by name**, never by loosening a needle;
      the `isPersonShaped` comment updated from "a field that does not exist" to the exception
- [ ] If B: PRD 024 §6 R1/R6 updated to three modes; the Danish labels in §7 follow
- [ ] If C: the derived key specified, including what happens to a filename with no counter
- [ ] Either way, PRD 024 §11 Q5 answered rather than left open

## Progress Log

- 2026-09-28 — Created while implementing task 441. The column was written, both privacy guards failed, and
  the right response to a guard like that is to stop rather than to except it — the whole point of
  `isPersonShaped` is that an exception has to be a sentence somebody wrote on purpose.
