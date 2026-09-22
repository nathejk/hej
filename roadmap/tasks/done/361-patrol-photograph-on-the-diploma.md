# 361 — The patrol's photograph on the diploma

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-22
**Completed:** 2026-09-22

## Description

Maintainer instruction, 2026-09-21/22:

> the diploma should carry start photo, like it did in the diplom-repo. The photo has 2 projections in hq (copy
> here if needed) photo and photocover, if more than 1 photo exists for patrulje, the cover can be manually
> selected.
>
> include cover photo in own blob store, these photos will also be part of the photo gallery shortly.

Three pieces: a projection (`nathejk/table/patrolphoto`), a byte fetcher (`internal/photobytes`), and the
rendering (`internal/diploma`).

### The privacy decision, recorded rather than assumed

`internal/diploma` refused a photograph at length, and a structural test failed the build if the field came back.
It fired on the first compile, which is exactly what it was for. The refusal's reasoning stands on the record:
the diploma is an **unauthenticated URL addressed by a patrol number**; PRD 011 §0b.2 allows photographs only
where a curator obtained consent; and a picture of eight children's faces is a stronger identifier than the names
task 337 protects.

What makes it defensible now is *which* photograph: the patrol's **cover**, selected by an organizer in hq — the
curation step §0b.2 asks for, in a tool where a human can see the picture. Where nobody has chosen, the fallback
picks the newest `start` photograph. The guard still refuses a person: no name, no phone, no email, no
photographer.

**A flagged photograph is never used and never downloaded**, on the maintainer's follow-up instruction the next
day: *"if attention flag is raised, then skip photo, do not download"*. `attention` is therefore a **filter, not a
preference** — flagged rows are excluded by the WHERE clause, so their refs never leave the projection and
`internal/photobytes` is never asked for their bytes. This replaced the shape the projection landed with, where an
explicit cover selection won even over the flag on the grounds that a human had chosen it; the maintainer's rule
is both safer and simpler, and a flagged cover now falls through to the next unflagged candidate.

### Where the bytes come from, and why they are copied here

The `photographed` event carries refs, never image data, and hq deliberately never proxies the bytes. A diploma
is rendered server-side, so they must be in this process. Per the instruction they are fetched **once** and kept
in this app's own blob store.

Both stores key objects by the sha256 of their contents, so the fetch is **verified, not trusted**: bytes that do
not hash to the ref the event named are refused and logged loudly, because that is corruption or substitution
rather than an outage. Bounded at 12 MB, single-flighted (50 concurrent callers → one fetch, tested under
`-race`), and every failure degrades to "no photograph" rather than "no diploma".

## Findings

- **`blob.Ref.Valid` accepts uppercase hex.** An uppercase ref reached the network in the first test run. The
  stores emit lowercase, so an uppercase ref is not ours — and honouring it stores identical bytes under two
  names and misses the local store forever. `photobytes` and the projection both apply the stricter rule.
  Tightening `blob.Ref` itself is worth doing and was not done as a side effect here.
- **`pdf.SetError(nil)` is a no-op.** fpdf's `SetError` only ever *sets*, and only when nothing is latched. The
  "recover from an unreadable photograph" branch therefore did nothing, and `Output` returned `invalid JPEG
  format: missing SOI marker` — a 500 on a public route instead of a certificate. `ClearError` is the real one.
- **The text printed across the patrol's faces.** The text had been moved up into the empty middle while the page
  carried no photograph; adding the photograph at `diplom`'s coordinates put the box underneath it. One geometry
  again, as `diplom` had it.
- **hq's `PHOTO_BASEURL` is a browser URL, this one is not.** Copied verbatim, it resolves to the container's own
  loopback (`dial tcp 127.0.0.1:443: connection refused`) and every diploma silently renders without a picture.
  Dev now maps the host gateway; the variable's doc records the distinction.
- **Measured against the real stream:** 362 photographs for 2026 — 185 `start`, 177 **`maal`** (the finish type is
  spelled that way), one `attention` flag in each — and **0 covers selected**, so every diploma currently takes
  the fallback path.

## Acceptance Criteria

- [x] A photograph flagged `attention` is never chosen and its bytes are never fetched — filtered in SQL, not
      sorted; verified by breaking the guard, and against real data (2026 has two flagged photographs, patrols 46
      and 58, and the query picks each patrol's other picture rather than nothing)
- [x] Otherwise: the cover wins, then the newest `start`, then any
- [x] Bytes are fetched once, verified against the ref, and served from the local store afterwards
- [x] No column, struct field or SQL statement can carry foto's `original` or `sourceUrl`
- [x] A purge clears a cover that pointed at the purged ref
- [x] Every photograph failure still produces a diploma — five cases tested, plus one proving a good photograph
      really is embedded
- [x] Verified on a rendered sample, twice, after the first one printed text over the photograph
- [ ] A successful fetch verified end to end — **not done**: the foto stack was not running locally
