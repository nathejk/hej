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
| One `imaging.Prepare` of a 12MP phone JPEG | **174 MB** of live heap |
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

### 1. Convert once — 174 MB → 83 MB

`Prepare` now converts to RGBA once and hands the same image to every rendition. `toRGBA` already returned its
argument unchanged for an origin-based `*image.RGBA`, so this is a two-line change for a **2.1× reduction**.

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

## Acceptance Criteria

- [x] The cause identified with measurements rather than inference
- [x] Per-image peak reduced, with the output unchanged where it matters
- [x] Concurrent decoding bounded server-side, at every call site
- [x] The container limit sized by the measurement, with `GOMEMLIMIT` below it
- [x] A test that a new ungated `Prepare` cannot be added quietly
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-28 — Diagnosed from the compose files and a memory probe rather than from production logs, which I cannot
  reach. The chain — 502 means the backend vanished, the backend is memory-limited, the pipeline allocates per
  rendition — is checkable end to end from the repo, and every link was.
- 2026-09-28 — Worth noting for next time: the dev stack having **no** memory limit is what made this a
  production-only failure. A dev limit of the same order would have caught it on the first card.
