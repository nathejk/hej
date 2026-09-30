# 471 — Production uploads failed with 502; dev was fine

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

Reported by the maintainer with a screenshot: three photographs (`20260918_205915.jpg` and two others) uploaded to
100% — *"Færdig: 3 af 3"* — and then every row showed **Fejl 502**. The same three files had uploaded cleanly in dev.

## Diagnosis

A 502 is Traefik's, not the app's: the proxy either got no response or the backend connection died. That plus
"works in dev" pointed at the environment rather than the files, and the numbers settled it. All of these were
**measured**, not reasoned about:

| | |
|---|---|
| One `imaging.Prepare` of a 12MP phone JPEG | **122–170 MB** of live heap |
| The uploader's concurrency (`upload.js`) | **3** files at a time |
| The production container (`docker-compose.prod.yml`) | **256 MB** |
| The dev container | **no limit** |

Three concurrent decodes is ~520 MB against a 256 MB cgroup limit. The kernel killed the process mid-request, every
connection in flight died at once, and Traefik — which cannot know why a backend vanished — answered 502 for all
three. Dev has no limit, so the same photographs sailed through on the host's RAM.

That also explains the shape of the report exactly: the bytes were all sent (the progress bar completed), and the
failure came afterwards, during processing.

### Where 174 MB was going

`Prepare` calls `render` once per rendition — display, 800px, 320px — and each `render` calls `Fit`, which calls
`toRGBA`. So a 4000×3000 photograph was being converted to RGBA **three times at full size**, 48 MB a copy, for no
reason: the conversions are identical.

## What shipped

Three changes, because the limit alone would have moved the failure rather than fixed it.

### 1. Convert once — 170 MB → 79 MB, for an unrotated photograph only

`Prepare` now converts to RGBA once and hands the same image to every rendition. `toRGBA` already returned its
argument unchanged for an origin-based `*image.RGBA`, so this is a two-line change.

**Corrected after the maintainer pushed back on the numbers.** I first measured a synthetic 4000×3000 fixture and
reported a uniform 2.1× win. Re-measured on the three photographs that actually failed:

| file | on disk | EXIF orientation | peak before | after |
|---|---|---|---|---|
| `20260918_205915.jpg` | 5.3 MB | 1 | 170 MB | **79 MB** |
| `20260918_210005.jpg` | 2.7 MB | 6 | 122 MB | 122 MB |
| `20260918_211252.jpg` | 4.5 MB | 6 | 126 MB | 126 MB |

So it helps the **unrotated** photograph and does nothing for the two rotated ones — because `applyOrientation`
already returns an `*image.RGBA` when it rotates, which `toRGBA` passes through, so those two never had duplicate
conversions to remove. Orientation 6 is a phone held upright, which is most of them.

My fixture had no EXIF rotation, so it measured the best case and I generalised from it. The lesson is the ordinary
one: **measure the input that failed**, not one shaped like it.

Output bytes are identical wherever a rendition is actually scaled, which is every photograph from a camera. The one
case that changes is an image already *smaller* than a target edge, where `Fit` returns its input untouched: that used
to hand the decoder's own YCbCr planes to the encoder and now hands it RGBA, so the chroma makes one extra round
trip. Imperceptible at quality 82, and the only way to have both would be keeping a second full-size image alive —
which is the thing being fixed.

### 2. A decode gate, so the peak is the server's decision

`cmd/api/decodegate.go`: at most `min(GOMAXPROCS, 2)` images are decoded at once, process-wide. **A queue, not a
refusal** — a waiting request holds until a slot frees, which costs nothing against a ten-minute upload deadline, and
a 429 would turn a resource decision of ours into an error a photographer has to interpret.

This is the part that matters more than the limit. The uploader sends three at a time *today*; a curator with two tabs
sends six, and a future version that sends eight would rediscover this on the one afternoon of the year it matters. A
server that decodes at most N at once has a ceiling it controls.

**All five callers are gated** — the curator's upload, a glimt, a portrait, the rendition backfill and the on-demand
repair — because a ceiling with one door open is not a ceiling. The backfill needed it most: it is a loop, and it runs
while people are uploading. `TestEveryDecodeGoesThroughTheGate` reads the source and fails if any call to
`imaging.Prepare` is not inside a slot; mutation-checked by ungating the glimt path.

### 3. The deployment: 512 MiB, and a `GOMEMLIMIT`

The limit is now 512 MiB with the arithmetic written beside it (2 × 83 MB + ~60 MB baseline), and
`GOMEMLIMIT=400MiB` is set.

`GOMEMLIMIT` is the part that would have been easy to miss: **the garbage collector cannot see a cgroup limit.** Left
alone it targets roughly twice the live heap, so a process with 166 MB live will happily reserve ~330 MB and keep
going until the kernel kills it — no warning, no error, just a container that disappears mid-request. With the limit
declared, the runtime collects harder instead, which is the difference between a slow upload and a failed one.

`TestTheProductionMemoryLimitsPayForTheGate` reads the compose file and asserts the three numbers still agree, since
they are one decision recorded in two files.

## Why 2–5 MB on disk costs 125 MB in memory

The maintainer's question, and it deserves an answer rather than a restatement: *"your measurements are in the
+100MB scale, actual filesizes on disk (and over the wire) is 2-5MB"*.

Two multiplications sit between the two numbers, and neither is avoidable:

1. **JPEG is compressed**, about 9–17× for these files. 5.3 MB of JPEG is 12 million pixels.
2. **A pixel in memory is 4 bytes**, not the ~0.44 a byte-per-pixel average implies. 4000 × 3000 × 4 = **46 MB** for
   one copy of the image, and the pipeline needs the decoded frame, a rotated copy, and a destination for each
   rendition.

Which is why the cost tracks **pixels, not bytes on the wire** — the 2.7 MB file peaked *higher* than the 5.3 MB one.
All three are 12 MP. A file half the size is not half the work; it is the same work on a better-compressed photograph.

The corollary matters for the limit: `maxAdminUpload` is 32 MiB, and a 32 MiB JPEG is likely 50 MP or more — around
200 MB as RGBA. The gate bounds how many are in flight, not how big one is, so if uploads start failing again the
first thing to measure is one `Prepare` of the file that failed.

## A flake I introduced, and caught

`TestNotYetPageCarriesNoPatrolData` searches the closed patrol page for the bare string `"42"`. Since PRD 026 the page
carries absolute URLs, so it contains the test server's `127.0.0.1:PORT` — and roughly one ephemeral port in twenty
contains "42". It passed alone and failed in the suite, which is the shape of a flake worth chasing rather than
re-running: found by running it forty times.

Fixed with `withoutOrigin`, which strips the **exact** origin string and leaves the URL *paths* intact, so a real leak
into an `og:url` path is still caught. In production the origin is `https://hej.nathejk.dk` and has no digits at all.

## What this does not do

- **It does not make a 32 MiB photograph free.** A 50MP camera file will cost proportionally more; the gate bounds how
  many are in flight, not how big one is. If uploads start failing again, the first thing to measure is one `Prepare`
  of the file that failed.
- **It does not add a streaming decode.** Go's `image/jpeg` has no scaled-decode entry point, so the full frame must
  exist in memory at least once. Halving the copies is the whole win available without a new dependency.

## Dev now gets the same ceiling — with one deliberate asymmetry

Asked for after the fix landed. `GOMEMLIMIT` is applied **to the api binary**, on the exec in `docker/init/api-dev`,
from `API_GOMEMLIMIT` in `docker-compose.yml`.

Not a container memory limit, and not `GOMEMLIMIT` in the compose environment, because **this container is not only
the app**: it runs `go test`, `staticcheck` and `go build` on every save, and those need several times the app's whole
budget. A container limit would starve the toolchain; a bare `GOMEMLIMIT` in the environment would be inherited by the
compiler and make it collect against a ceiling meant for a web server. A distinct variable name applied to one exec is
what separates them, and `TestDevGivesTheAppTheSameMemoryCeiling` asserts both halves — including that the bare name
does *not* appear in the dev environment, since moving it there looks like a tidy-up.

It is a **soft** limit, so dev gets slow where production got killed. That is the honest maximum without giving up the
dev loop, and slow-under-pressure is still a signal where there was none.

**Worth knowing:** `docker/init/api-dev` is `COPY`'d into the image, not bind-mounted — the dev loop hot-reloads Go
source only. Changing it needs `docker compose build api`. I lost a rebuild cycle to that.

Verified in the dev container: the api process has `GOMEMLIMIT=400MiB`, the toolchain does not, and three concurrent
6 MB 12MP uploads answer 200 in 1.2 s wall with timings (0.79, 0.79, 1.16 s) that show the gate admitting two and
queueing the third.

## Acceptance Criteria

- [x] The cause identified with measurements rather than inference
- [x] Per-image peak reduced, with the output unchanged where it matters
- [x] Concurrent decoding bounded server-side, at every call site
- [x] The container limit sized by the measurement, with `GOMEMLIMIT` below it
- [x] A test that a new ungated `Prepare` cannot be added quietly
- [x] Dev applies the same ceiling to the app without starving the toolchain
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Diagnosed from the compose files and a memory probe rather than from production logs, which I cannot
  reach. The chain — 502 means the backend vanished, the backend is memory-limited, the pipeline allocates per
  rendition — is checkable end to end from the repo, and every link was.
- 2026-09-28 — Worth noting for next time: the dev stack having **no** memory limit is what made this a
  production-only failure. A dev limit of the same order would have caught it on the first card.
