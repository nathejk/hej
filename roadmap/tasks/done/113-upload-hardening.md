# 113 — Upload hardening: server timeouts, oversized-multipart status, parser fuzzing

**Status:** done
**Priority:** high
**Created:** 2026-08-29
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-29
**Completed:** 2026-08-29

## Description

A verification pass over the portrait upload path once the Docker daemon was available
again, prompted by two things: the upload cap had just been raised to 8 MiB (task 112),
and the riskiest code written for PRD 003 — hand-rolled EXIF/JPEG/PNG parsing — had
never been run through the security linters or a fuzzer.

Four findings, in rough order of how much they would have hurt.

## 1. `ReadTimeout: 5s` would have failed uploads in the field

`http.Server.ReadTimeout` covers reading the **entire request, including the body**. At
5 seconds, an 8 MiB portrait needs ~1.6 MB/s to get through — so a member on a weak
mobile link would have had the read aborted partway and seen an upload that "just fails
sometimes". Intermittent, in a forest, at night: close to the worst place to diagnose
anything.

Pre-existing, but raising the cap to 8 MiB made it near-certain rather than unlikely.

Fixed by keeping the general limits modest and letting the one endpoint that needs
minutes extend its own deadline:

- `ReadHeaderTimeout: 10s` — **new**, so that relaxing `ReadTimeout` does not relax
  slowloris protection with it.
- `ReadTimeout: 30s` — generous for JSON, no longer the upload's constraint.
- `WriteTimeout: 60s` (was 10s) — in production this binary also serves the SPA bundle,
  and 10 seconds assumed roughly 60 KB/s of a phone in the field.
- `updatePhotoHandler` sets its own 3-minute read deadline via
  `http.NewResponseController`, sized as 8 MiB at ~50 KB/s (a realistic single-bar
  link) plus room.

## 2. An oversized multipart upload answered 400, not 413

Caught **live**, not by a test: an 11.6 MB upload returned
`400 {"error":"forventede et billede i feltet \"photo\""}`. Doubly wrong — the field was
there, and the client was told to fix its request when the truth was "your photo is too
big".

The cause is a path split: for multipart, the limit error surfaces inside `r.FormFile`
rather than at the body read, so it never reached the code that maps it to 413. The
existing test passed throughout **because it sent a raw body**, which takes the other
branch — the app itself only ever sends multipart. A test that exercised the shape the
client does not use.

Now `413 {"error":"billedet er større end 8 MB"}`, with a test for the multipart path
that asserts both the status and that the message is about size.

Also distinguished a connection that dies mid-upload from an over-limit body: the
former is common on a weak link and deserves "retry", not "shrink your picture".

## 3. Four HIGH-severity gosec findings, all in the new resampler

G115, integer overflow, on `uint8(r / n)` over a `uint32` accumulator — and not a false
positive. Each channel sums bytes so `sum/n ≤ 255` always; the hazard is `sum` itself,
which overflows once a single destination pixel averages more than ~16.8 M source
pixels.

Today's call sites cannot reach it (`Fit` always makes the long edge 1024 or 256), but
that is an argument from the callers, not from the function, and it stops being true the
first time someone calls `Fit(img, 8)` for an icon. Accumulators are now `uint64` with a
clamped `mean()` helper, which removes the class rather than reasoning about it.

gosec is back to **0 findings** in `internal/imaging` and `cmd/api`. The four remaining
repo-wide are pre-existing and understood (`blob/file.go`'s path is guarded by
`Ref.Valid()`; the session cookie's `Secure` flag is config-driven so dev can run
without TLS).

## 4. The parsers are now fuzzed

`ReadOrientation` and `StripMetadata` are the only places in the service that walk
attacker-supplied binary structure by hand — index arithmetic over untrusted lengths.
Everything else goes through Go's own decoders.

Two fuzz targets, asserting more than "did not panic":

- orientation is always within 1–8, because the value indexes a transform and an
  out-of-range answer would rotate a portrait into nonsense;
- anything `StripMetadata` accepts still decodes **and keeps its dimensions** — i.e. the
  scrubber did not quietly corrupt a member's photo while removing metadata, which is
  the failure that would surface months later with the upload long gone.

**6.3 M and 2.6 M executions, no failures.** Seeds include a bare SOI, a segment
claiming a 0xFFFF length, and a PNG signature with no chunks.

Worth recording for context: there is no `recover()` anywhere in this codebase, and it
is not needed for this — `net/http` recovers a handler panic per connection, so a parser
panic would have cost one request and a stack trace, not the API. Still worth ruling out
rather than relying on.

## Acceptance Criteria

- [x] Upload of a body between the old and new caps succeeds; over the new cap gives
      **413** with the size message, for **both** multipart and raw bodies.
- [x] Read timeouts cannot abort a slow-but-legitimate upload; header timeout still
      bounds slowloris.
- [x] gosec clean in the touched packages; govulncheck reports nothing affecting us.
- [x] Fuzz targets for both hand-written parsers, passing.
- [x] `gofmt`, `go test ./...`, `vet`, `staticcheck`, `npm run type-check`,
      `npm run build`, and `docker build --target prod` all green.

## Live verification (dev stack, 2026-08-29)

| Upload | Before | After |
|---|---|---|
| 4.1 MB multipart | 413 (old 4 MiB cap) | **200** |
| 11.6 MB multipart | 400, misleading message | **413**, "billedet er større end 8 MB" |

And the 4.1 MB upload's event shows the task 112 guard doing its job:

```
display : 768 x 1024   365657 B
thumb   : 192 x 256      8019 B
original: 2100 x 2800  4129442 B  orientation=1
```

Non-square display (the square-frame bug is gone), and an original kept because it
genuinely holds more pixels. Note the 365 KB display is worst-case: the fixture is
random noise, which JPEG cannot compress. A real photograph is far smaller.
