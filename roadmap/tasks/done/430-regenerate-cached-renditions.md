# 430 — Rebuild cached renditions from originals

**Status:** done
**Priority:** medium
**Created:** 2026-09-26
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

Task 429 split the blob store into `original/` (must be backed up) and `cache/` (need not be), on the
stated grounds that everything in `cache/` can be produced again. **Nothing currently produces it
again.** Renditions are made once, at upload, and never afterwards.

Two things want this, and they are the same machinery:

1. **Restore and reclaim.** Emptying `cache/`, or restoring a backup that omitted it, should be
   recoverable on purpose rather than tolerated.
2. **Adding a rendition to photographs that already exist.** `photo.go` notes that changing the
   rendition list does **not** rewrite existing portraits — a new size appears only for portraits taken
   after the change — and tasks 409/410 want a medium 800px rendition for the viewer.

**Where the source is.** A portrait has a real original when `PORTRAIT_KEEP_ORIGINAL` is on (task 111).
Glimt and library photographs do **not** keep a separate original — their full rendition *is* the
original (see task 429's classification table), so a thumbnail is rebuilt by re-resizing that. Lossier
than resizing from a camera file, and a deliberate choice of those features.

And do not let this become a purge: originals are never deleted by anything here (PRD 022 §11 Q2).

## Resolved: a derived ref is a name, so rebuild under the ref that missed

**Decided 2026-09-26 (maintainer).** On a cache miss, rebuild the rendition and store it under the
**existing** ref — the one the projection already holds — even though the re-encoded bytes hash to
something else.

This was initially written off (see the superseded analysis below) on the grounds that it breaks content
addressing. Examined properly, it does not break anything that was load-bearing, and it is materially
less invasive than the alternative:

- **Nothing outside `internal/blob` changes.** No projection column, no new key format, no new URL
  shape, no fan-out change, no migration. The refs in `photo`, `album_item` and `person` stay exactly as
  they are.
- **No read path writes to the event stream.** This was the fatal objection to rebuilding under a *new*
  ref, and it simply evaporates: if the ref does not change, no row has to converge, so PRD 008 §8 is
  not even in play.

The move that makes it coherent: for **derived** objects the ref is a **stable name**, not a checksum.
Nothing outside the package was relying on a thumbnail's ref being verifiable — only on it being the
address of that thumbnail.

### The boundary that makes it safe

All of the safety is in one place, and it is confined to the cache class:

| | ref means | verifiable | rebuildable |
|---|---|---|---|
| `original/` | its content hash, always | yes | n/a — nothing to rebuild it from |
| `cache/` | a stable name | no | yes, via `PutAs` |

The symmetry is the argument: **the class that gives up verifiability is exactly the class that is
expendable.** An object that cannot be checked is fine when it can be discarded and made again. An
original keeps its checksum property precisely because it cannot.

### What was checked rather than assumed

- `internal/photobytes.fromStore` does **not** re-verify the hash on a local read — the verification is
  on the foto *fetch* path only, and photobytes only ever handles originals (its bytes become an album
  item's canonical ref). So the "hash-verification will reject rebuilt objects" objection was wrong.
- The ETag consequence is real and is **accepted**: the media routes serve `ETag: "<ref>"` with
  `immutable`, so a client holding pre-rebuild bytes will not revalidate. A rebuild runs the same source
  through the same recipe, so the stale copy is a perceptually identical thumbnail differing in JPEG
  entropy coding. Changing what an image *is* still needs a new ref, because that is a new upload.
- Minor and tolerated: a later `PutCache` of rebuilt bytes computes a different ref and stores a second
  cache copy. Wasted cache space, which is by definition reclaimable.

### Built (2026-09-26)

`blob.Store.PutAs(ctx, ref, data)` — writes bytes under a caller-chosen ref, **cache class only**.

- **Refuses if the ref names an original.** This is the whole guardrail. A repair path holds a source
  ref and a target ref, and passing them the wrong way round is an ordinary mistake — one that would
  otherwise overwrite the only copy of a photograph with a thumbnail of it, inside the subtree the backup
  covers, with the checksum that would have caught it deliberately given up.
  `TestPutAsRefusesToOverwriteAnOriginal` is the most important test in the package; mutation-checked by
  removing the guard.
- Mirrored in `MemoryStore`, because that is the store every `cmd/api` test runs against and a test store
  more permissive than production would hide precisely this bug.
- Shares `writeObject` with `put`, so a rebuild lands through the same atomic rename and 0600. A rebuild
  happens while requests are being served: a reader must see the old bytes or the new ones, never a
  half-written JPEG.
- Returns no `Ref`. The caller supplied it, and handing it back would imply it was derived from the
  bytes when the entire point is that it was not.

## Built: lazy repair on the serving paths

The rebuild happens on a **cache miss during a serve**, not in a batch pass.

`renditionRepair` (`cmd/api/blobrepair.go`) carries the recipe — target ref, source ref, edge, quality —
from the place that knows it to the place that needs it. The store deliberately does not learn about
images: it is handed opaque bytes and a ref, and has no idea one object is a 320px resize of another. But
**every serving call site already knows**, because it holds both refs and the requested variant. So the
plan travels from the call site and the store stays a store.

Wired at every call site that serves a derived rendition — found by changing the signatures and letting
the compiler enumerate them, rather than by grepping and hoping:

| Route | Source of the rebuild |
|---|---|
| glimt media (authenticated) | the item's full rendition — glimt keep no original |
| glimt media (public) | same, via `streamPublicGlimtMedia` |
| album media (public site) | the item's full rendition (PRD 022 §8.5 keeps no original) |
| admin library media | same — and the biggest burst, one thumbnail per photograph on a contact sheet |
| own portrait, contacts portrait, patrol portrait | the **display image**, not task 111's original |

Portraits rebuild from the display image on purpose. The kept original has its metadata stripped, so its
EXIF orientation tag is gone and only `PortraitOriginal.Orientation` knows which way up it goes;
re-rendering from it would mean applying that rotation by hand, and getting it wrong means sideways
faces. The display image already has the rotation baked into its pixels. Slightly lossier, and
unambiguous.

### Off the request path, and why the fallback is safe

A miss serves the **source** immediately and rebuilds in the background. A cold cache under the post-race
spike (PRD 019 §0a.3) is precisely when nobody wants a resize between request and response, and the
fallback costs nothing to build because every one of these surfaces already degraded to the full image
when a rendition was absent — that pre-existing fallback is what made excluding `cache/` from the backup
safe in the first place.

The one thing the fallback must not do is let a client cache the full-size image *as* the thumbnail. So
on the degraded path the **bytes, the ETag and the cache window move together**: the answer carries the
source's ref as its ETag and `degradedRenditionCacheControl` (`private, max-age=30`, explicitly **not**
`immutable`). Serving source bytes under the thumbnail's ETag with a year's `immutable` would tell every
cache the two are the same bytes — permanently, on a page that shows sixty of them.

`no-store` is left alone. A patrol member's portrait must not be cached at all (PRD 007 §8), and a
missing thumbnail is a poor reason to weaken that.

### Coalescing: in-process, not a lock file

`repairRendition` is single-flighted on the target ref (`singleflight.Group` on `application`). An album
page asks for up to sixty thumbnails, so a cold cache answers sixty misses at once; without this, sixty
goroutines decode and resize the same JPEG on the one process that is also serving the pages.

A filesystem marker was considered (maintainer's proposal) and **rejected for this deployment**, because
the service is single-process by design — `docker-compose.prod.yml:306` forbids replicas for two reasons
much older than this feature (the in-memory PIN store; projections as ephemeral consumers with no queue
group — task 064, PRD 008 §11 Q1). A lock file would coordinate between processes that cannot exist,
while adding a TTL to guess wrong and residue to clean up after a crash.

The specific variant proposed — a zero-byte file at the thumbnail's own path — would also have been
actively harmful, and the reasoning is recorded because it is not obvious: such a file is not inert, it
is a **valid cache entry**. `Get` would succeed and return zero bytes, and the media route would then
serve an empty JPEG under `ETag: "<ref>"` with `immutable` — so any client landing in the window caches a
broken image for a year. It would also make `Exists` true, which the purge paths use to decide whether
bytes are still referenced. If a cross-process guard is ever needed, the marker goes **beside** the
object (`<ref>.building`) and is created with `O_CREAT|O_EXCL`, since stat-then-create is itself racy.

Two further protections, both cheap: the context is **detached** from the request's
(`context.WithoutCancel` + a 30s timeout), because the request is answered from the source within
milliseconds and a rebuild on `r.Context()` would be cancelled the instant the response finished — one
abandoned resize per request, forever. And there is an `Exists` re-check *inside* the flight, so a burst
of misses that does not overlap perfectly still costs one rebuild.

## Acceptance Criteria

- [x] Decided and documented how a rebuild addresses its output, without serving mismatched bytes under
      an original's content-hash ETag
- [x] A store primitive for writing a rebuilt rendition under its existing ref, refusing originals
- [x] A serve that misses a derived ref rebuilds it from the original and succeeds
- [x] Concurrent misses of the same rendition do the work once
- [x] No read path publishes events or rewrites a projection row in order to serve an image
- [x] Verified by emptying `cache/` and having the pages come back correct — against the real
      `FileStore`, not the in-memory one
- [x] Task 429's claim that `cache/` is expendable is true by demonstration, not by argument

## Tests, and what each is for

`cmd/api/blobrepair_test.go`. Every failure mode this feature has is quiet, which is what the tests are
shaped around: a rebuild that never happens looks like a slow page; a degraded answer cached under the
wrong ETag looks fine for a year; a rebuild that writes to the wrong ref destroys a photograph and
returns 200.

- **`TestEmptyingTheWholeCacheDirectoryIsRecoverable`** — the one that settles task 429. Runs against a
  real `blob.FileStore`, deletes the entire `cache/` directory, requests a thumbnail, and asserts the
  page still serves, the cache refills itself, the rebuilt object lands in `cache/` and **not** in
  `original/`, and the result is a decodable 320px JPEG. Separate from the rest because every other test
  here runs against `MemoryStore`, which could agree with production about everything except what
  matters — the same reason `TestTheFileStoreReportsFreeSpace` exists.
- **`TestTheDegradedAnswerServesTheSourceWithoutPoisoningCaches`** — the ETag and cache-window guard.
  Mutation-checked by setting `degradedRenditionCacheControl` to the immutable year: fails.
- **`TestConcurrentMissesRebuildOnlyOnce`** — 50 concurrent callers, blocked mid-write to force the
  overlap rather than hope for it. Mutation-checked by removing the single-flight: 50 rebuilds.
- **`TestAnImpossiblePlanIsNeverActedOn`** — including `Target == Source`, which is what a swapped pair
  of arguments looks like and would mean writing a thumbnail over a photograph.
- **`TestAMissingFullRenditionStill404s`** — an original is not repairable and must not pretend to be.
- **`TestTheDegradedPathNeverWeakensNoStore`** — PRD 007 §8.
- `go test -race` clean over `cmd/api` and `internal/blob`.

## Follow-up, deliberately not done here

- **A batch pass** for adding a rendition to photographs that already exist — the 800px rendition tasks
  409/410 want. The same `repairRendition` over a walk of the originals; it is a backfill, not a repair,
  and it wants its own task.
- **Negative caching.** A rendition whose source is also gone is unrebuildable, and each request will
  attempt it once. The attempt fails on the source read, before any decode, so it is cheap — but a
  pathological case (source present, undecodable) would decode per request. Worth a short-lived failure
  memo if it is ever observed.

## Superseded analysis: why this looked impossible at first

Kept because the reasoning is what produced the guardrail, and because the ETag consequence it
identified is real and merely accepted rather than absent.

The original objection was that a rendition's ref is `sha256(its bytes)`, a rebuild does not reproduce
those bytes, and therefore a rebuild must either store under the old ref (putting an object in a
content-addressed store that does not hash to its address) or under a new one (requiring a GET handler
to publish events so the projection converges — on unauthenticated public routes, under load).

The proposed escape was to key derived objects by recipe, `<source ref>/thumb320-v1`, making the key
stable under re-encoding. That works, and it is strictly more machinery: a new key format, a projection
change, a new URL shape, a different fan-out. **Treating the existing ref as the stable name achieves
the same thing and changes nothing outside the package**, which is why it won.

What the original analysis got wrong: it asserted `internal/photobytes` would reject rebuilt objects. It
verifies the *fetch*, not local reads, and never touches derived objects.

## Progress Log

- 2026-09-26 — Task created as the explicit follow-up to task 429. Split out rather than bundled,
  because the layout change is what makes the backup smaller and is worth having on its own, while this
  is the larger question of rendition lifecycle and touches the ref-stability rule in PRD 008 §8.
- 2026-09-26 — Asked whether the rebuild could simply happen on a cache miss. Wrote it up as *not as
  stated*: the ETag on the glimt and album media routes **is** the content hash, served `immutable` for a
  year. Proposed recipe-addressed derived objects instead.
- 2026-09-26 — Maintainer's counter-proposal: keep the old ref for the rebuilt rendition. Re-examined and
  **adopted**. It reaches the same place as recipe-addressing while changing nothing outside
  `internal/blob` — no projection change, no new URL shape, no events from a read path. Also found that my
  `photobytes` objection was wrong: it verifies the foto fetch, not local reads.
- 2026-09-26 — ✅ Built `PutAs` on both stores, cache-class-only, refusing any ref that names an
  original, sharing the atomic write with `put`. Tests in `internal/blob/rebuild_test.go`; the
  overwrite-an-original refusal mutation-checked. `go vet` and the full suite clean.
- 2026-09-26 — Maintainer raised the thundering herd: a deleted cache under load means many clients
  requesting the same thumbnail at once. Proposed a zero-byte marker at the thumbnail's path with a 60s
  TTL, backing off to the original. **Requirement accepted, mechanism changed.** A zero-byte file at the
  object's own path is a valid cache entry, so `Get` would serve an empty JPEG under an `immutable`
  year-long ETag and `Exists` would lie to the purge paths. Established from
  `docker-compose.prod.yml:306` that the service is single-process by design, which makes an in-process
  single-flight both sufficient and strictly better — no TTL to guess, no crash residue, perfect
  coalescing. The back-off-to-original half of the proposal was kept as the default.
- 2026-09-26 — ✅ Wired the repair into all seven serving call sites, found by changing the signatures
  and letting the compiler enumerate them. Changed `glimtVariantRef`, `albumItemRef` and a new
  `portraitRefAndRepair` to return a plan alongside the ref.
- 2026-09-26 — Decision while building: on the degraded path the ETag and cache window switch to the
  source **together with** the bytes. Serving source bytes under the thumbnail's ETag with the routes'
  usual `immutable` year would let one unlucky client pin a multi-megabyte image under a thumbnail URL
  permanently. Mutation-checked.
- 2026-09-26 — ✅ Added `TestEmptyingTheWholeCacheDirectoryIsRecoverable` against a real `FileStore`,
  which is what closes the last criterion honestly: every other test here runs against `MemoryStore`,
  and a wrapper that agrees with production about everything except the subtree layout would prove
  nothing about a restore.
- 2026-09-26 — All criteria met. `gofmt`, `go vet`, full suite and `go test -race` over `cmd/api` and
  `internal/blob` clean. Moving to done. Left a batch backfill and negative caching as noted follow-ups
  rather than folding them in.
