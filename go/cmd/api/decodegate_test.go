package main

import (
	"context"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The decode gate (task 471), which is the reason production uploads stopped failing with 502.
//
// # What these protect
//
// Not a feature — a **ceiling**. Decoding one 12MP photograph costs ~83 MB of live heap, the container is limited to
// 512 MiB, and the uploader sends three files at once. The gate is what makes the peak a property of this process
// rather than of how many requests happen to arrive, and every one of these tests exists because a plausible edit
// would quietly remove that property: gating one call site and not another, sizing the gate off a big host's core
// count, or holding a slot across a whole backfill loop.

// **No more than the gate's size are ever inside at once.**
//
// The assertion is about the peak, not the total: a semaphore that let everything through would still finish the
// work, and the test would pass if it only counted completions.
func TestTheDecodeGateBoundsConcurrency(t *testing.T) {
	var inside, peak atomic.Int64

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = withDecodeSlot(context.Background(), func() error {
				n := inside.Add(1)
				for {
					old := peak.Load()
					if n <= old || peak.CompareAndSwap(old, n) {
						break
					}
				}
				// Long enough that twenty goroutines genuinely overlap; short enough not to slow the suite.
				time.Sleep(2 * time.Millisecond)
				inside.Add(-1)
				return nil
			})
		}()
	}
	wg.Wait()

	if got := peak.Load(); got > int64(cap(decodeGate)) {
		t.Errorf("%d decodes ran at once, but the gate holds %d: the memory ceiling is whatever the peak is",
			got, cap(decodeGate))
	}
	if got := inside.Load(); got != 0 {
		t.Errorf("%d slots were never released — a leaked slot deadlocks every later upload", got)
	}
}

// The gate is at most `maxConcurrentDecodes`, whatever the host.
//
// A 32-core build machine must not set a peak the memory limit cannot pay for, which is the failure this cap exists
// for: the number is bounded by RAM, not by cores.
func TestTheDecodeGateIsCappedRegardlessOfCores(t *testing.T) {
	if cap(decodeGate) > maxConcurrentDecodes {
		t.Errorf("the gate holds %d, above the cap of %d", cap(decodeGate), maxConcurrentDecodes)
	}
	if cap(decodeGate) < 1 {
		t.Fatal("a gate of zero would deadlock every upload")
	}
	if n := gateSize(); n != cap(decodeGate) {
		t.Errorf("gateSize() = %d but the gate holds %d", n, cap(decodeGate))
	}
	t.Logf("GOMAXPROCS=%d, gate=%d, cap=%d", runtime.GOMAXPROCS(0), cap(decodeGate), maxConcurrentDecodes)
}

// A waiting request gives up when its own context does.
//
// Without this a photographer who closes the tab leaves work queued for a response nobody will read — and during the
// burst this bounds, that backlog is exactly what turns a slow minute into a failed upload for everyone behind it.
func TestAWaitingDecodeRespectsItsContext(t *testing.T) {
	// Fill the gate and hold it.
	release := make(chan struct{})
	held := make(chan struct{})
	for i := 0; i < cap(decodeGate); i++ {
		go func() {
			_ = withDecodeSlot(context.Background(), func() error {
				held <- struct{}{}
				<-release
				return nil
			})
		}()
	}
	for i := 0; i < cap(decodeGate); i++ {
		<-held
	}
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	ran := false
	err := withDecodeSlot(ctx, func() error {
		ran = true
		return nil
	})
	if err == nil {
		t.Error("a request whose context expired while waiting must not proceed")
	}
	if ran {
		t.Error("the work ran after its context was done")
	}
}

// **Every call to imaging.Prepare goes through the gate.**
//
// The one that matters most, and a source read because there is no other way to assert it: a ceiling with one door
// left open is not a ceiling. There are five callers — the curator's upload, a glimt, a portrait, the rendition
// backfill and the on-demand repair — and the backfill is the one that would hurt most if it were missed, because it
// is a loop that runs while people are uploading.
func TestEveryDecodeGoesThroughTheGate(t *testing.T) {
	prepare := regexp.MustCompile(`imaging\.Prepare\(`)
	inSlot := regexp.MustCompile(`withDecodeSlot\(`)

	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	found := 0
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		// Comments mention `imaging.Prepare` in several places — including this rule's own reasoning — so they are
		// stripped first. The same trap task 439 records: a guard that greps for a symbol finds the prose about it.
		code := stripGoComments(string(src))

		calls := len(prepare.FindAllString(code, -1))
		if calls == 0 {
			continue
		}
		found += calls
		if gates := len(inSlot.FindAllString(code, -1)); gates < calls {
			t.Errorf("%s calls imaging.Prepare %d times but withDecodeSlot only %d: an ungated decode is 83 MB "+
				"nothing accounted for, and the ceiling is a fiction", name, calls, gates)
		}
	}

	// A count, so deleting every call site does not make this pass by vacuity.
	if found < 5 {
		t.Errorf("found %d calls to imaging.Prepare, expected at least 5 — if a path was removed, update this "+
			"number deliberately", found)
	}
}

// The deployment's numbers agree with the gate's.
//
// The gate bounds concurrency; the container's limit and GOMEMLIMIT have to be able to pay for it. These three
// numbers are one decision recorded in two files, and the failure mode of them drifting is the 502 this task was
// about — so the arithmetic is asserted rather than trusted to a comment.
func TestTheProductionMemoryLimitsPayForTheGate(t *testing.T) {
	compose, err := os.ReadFile("../../../docker-compose.prod.yml")
	if err != nil {
		t.Skipf("compose file not readable from here: %v", err)
	}
	text := string(compose)

	limit := firstInt(t, text, `memory: (\d+)M\b`)
	memLimit := firstInt(t, text, `GOMEMLIMIT: \$\{GOMEMLIMIT:-(\d+)MiB\}`)

	// Measured: 83 MB per decode. Held as a constant here so the relationship is checked rather than the number
	// being repeated as a hope.
	const perDecodeMB = 83
	need := perDecodeMB*cap(decodeGate) + 60 // + the app's own baseline

	if limit < need {
		t.Errorf("the container limit is %d MB but %d concurrent decodes need about %d MB plus the app's baseline: "+
			"this is the configuration that produced 502s", limit, cap(decodeGate), need)
	}
	if memLimit >= limit {
		t.Errorf("GOMEMLIMIT is %d MiB and the container limit %d MB — GOMEMLIMIT must be below it, or the runtime "+
			"is still allowed to grow into the kill", memLimit, limit)
	}
	if memLimit < perDecodeMB*cap(decodeGate) {
		t.Errorf("GOMEMLIMIT is %d MiB, below the %d MB the gate can have live at once: the collector would thrash "+
			"against work that legitimately needs the memory", memLimit, perDecodeMB*cap(decodeGate))
	}
}

func firstInt(t *testing.T, text, pattern string) int {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("no match for %s in the production compose file", pattern)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}
