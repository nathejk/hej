package main

import (
	"context"
	"runtime"
)

// Bounding how many photographs this process decodes at once (task 471).
//
// # The failure this exists to prevent
//
// Production uploads failed with **502** while the identical files uploaded cleanly in dev. The cause was memory, and
// the numbers are worth keeping because every one of them was measured rather than reasoned about:
//
//	one Prepare of a 12MP phone JPEG   174 MB of live heap  (before task 471's fix)
//	the same, after converting once     83 MB
//	the uploader's concurrency           3 files at a time
//	the production container            256 MB
//
// Three concurrent uploads is 249 MB of image data on a 256 MB limit, with nothing left for the projections, the
// connection pools or the caches. The cgroup killed the process mid-request, every connection in flight died, and
// Traefik — which cannot know why a backend vanished — answered 502. Dev has no memory limit, so the same three
// photographs sailed through.
//
// # Why a semaphore rather than only a bigger limit
//
// Because the limit is the deployment's business and **the peak must not be the client's decision**. The uploader
// sends three at a time today; a curator with two browser tabs sends six, and a future version that sends eight
// would rediscover this failure on the one afternoon of the year it matters. A server that decodes at most N images
// at once has a memory ceiling it controls, whatever anybody asks of it.
//
// It is a **queue, not a refusal**: a request that arrives while the gate is full waits. The upload endpoint's
// deadline is ten minutes (`adminUploadTimeout`) precisely because a 32 MiB file over a venue connection is slow, so
// waiting a second or two for a slot costs nothing a photographer would notice — and the alternative, a 429, would
// turn a resource decision of ours into an error message they have to understand.
//
// # Where the number comes from
//
// `GOMAXPROCS`, capped at 2. Resizing is CPU-bound, so there is nothing to gain from more concurrency than there are
// cores; and the cap is what keeps a big host from setting a peak the *memory* limit cannot pay for. Two is the
// measured fit: 2 × 83 MB leaves room inside a 256 MB container, and inside the 512 MB the deploy now asks for it
// leaves plenty.
//
// This is deliberately not configurable. A knob would need a number nobody can pick without the measurement above,
// and the measurement says "two" on every host this runs on.
const maxConcurrentDecodes = 2

// decodeGate is the semaphore. Buffered channel rather than `golang.org/x/sync/semaphore`, which is already a
// dependency — a buffered channel is four lines and needs no context plumbing beyond the one below.
var decodeGate = make(chan struct{}, gateSize())

func gateSize() int {
	n := runtime.GOMAXPROCS(0)
	if n > maxConcurrentDecodes {
		n = maxConcurrentDecodes
	}
	if n < 1 {
		n = 1
	}
	return n
}

// withDecodeSlot runs fn with one of the decode slots held.
//
// Honours the request's context while waiting, so a photographer who closes the tab or a deadline that expires does
// not leave work queued for a response nobody will read — which is exactly what would build up a backlog during the
// burst this bounds.
func withDecodeSlot(ctx context.Context, fn func() error) error {
	select {
	case decodeGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-decodeGate }()

	return fn()
}
