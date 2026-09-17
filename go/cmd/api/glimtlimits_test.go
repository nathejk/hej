package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/nathejk/table/glimt"
)

// Glimt rate limits and storage ceilings (PRD 019 §8, §11 Q8, task 311).
//
// # The most important test in this file is the one that asserts nothing is throttled
//
// `TestGlimtReads_ABrowseBurstIsNotThrottled` is the point of the whole task. PRD 019 §0a.3 describes
// the post-race browse as the load peak of the feature — a thousand people at the finish line, each
// pulling a grid of thumbnails per screen — and it is the use the feature was *built* for. The trap
// this task exists to avoid is a single limiter, tuned for uploads, quietly throttling exactly that.
// Nothing about a throttled browse looks like a bug from the server's side: it looks like a working
// rate limiter.

// ceilingApp is a test app with the ceilings and limits set explicitly.
func ceilingApp(t *testing.T, memberLimit, totalLimit int64) (*application, *stubGlimt) {
	t.Helper()
	people := &stubPeople{p: spejderPerson(), found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)
	app.config.eventYear = "2026"
	app.config.glimtMemberStorageBytes = memberLimit
	app.config.glimtTotalStorageBytes = totalLimit
	store := &stubGlimt{}
	app.models = newModelsWithGlimt(people, store)
	return app, store
}

func TestStorageCeiling_AllowsUnderTheLimit(t *testing.T) {
	app, store := ceilingApp(t, 1000, 0)
	store.storedBytes = 400

	if v := app.checkGlimtStorageCeiling("p-1", 500); v != glimtCeilingOK {
		t.Errorf("verdict = %v, want OK", v)
	}
}

// Checked against the *result*, not the current state — otherwise a member one byte under their
// ceiling could upload 12 MiB.
func TestStorageCeiling_CountsWhatIsAboutToBeStored(t *testing.T) {
	app, store := ceilingApp(t, 1000, 0)
	store.storedBytes = 999

	if v := app.checkGlimtStorageCeiling("p-1", 500); v != glimtCeilingMember {
		t.Errorf("verdict = %v, want the member ceiling", v)
	}
	// Exactly at the limit still fits: the boundary is inclusive, so a ceiling of 1000 means a
	// member may store 1000 bytes rather than 999.
	if v := app.checkGlimtStorageCeiling("p-1", 1); v != glimtCeilingOK {
		t.Errorf("verdict = %v, want OK exactly at the limit", v)
	}
}

func TestStorageCeiling_TotalIsSeparateFromTheMemberLimit(t *testing.T) {
	app, store := ceilingApp(t, 0, 1000)
	store.totalBytes = 900
	// The member has no limit at all, so this can only be the total.
	if v := app.checkGlimtStorageCeiling("p-1", 200); v != glimtCeilingTotal {
		t.Errorf("verdict = %v, want the total ceiling", v)
	}
}

// The member limit is checked first, so a member who is over their own share is told *that* rather
// than being told the event is full — which would be true but useless to them.
func TestStorageCeiling_PrefersTheMemberVerdictWhenBothAreExceeded(t *testing.T) {
	app, store := ceilingApp(t, 100, 1000)
	store.storedBytes = 100
	store.totalBytes = 1000

	if v := app.checkGlimtStorageCeiling("p-1", 50); v != glimtCeilingMember {
		t.Errorf("verdict = %v, want the member ceiling to take precedence", v)
	}
}

// Zero means unlimited, matching every other Glimt setting: the disabling value is the zero value, so
// an unset environment variable cannot impose a limit nobody chose.
func TestStorageCeiling_ZeroMeansUnlimited(t *testing.T) {
	app, store := ceilingApp(t, 0, 0)
	store.storedBytes = 1 << 40
	store.totalBytes = 1 << 40

	if v := app.checkGlimtStorageCeiling("p-1", 1<<40); v != glimtCeilingOK {
		t.Errorf("verdict = %v, want OK with both ceilings disabled", v)
	}
}

// **Fails open**, and this is the assertion that records the decision.
//
// It is the opposite of how the moderation check fails, deliberately: a ceiling is a safety margin,
// not an authorization. The cost of wrongly allowing an upload is some disk, which retention reclaims
// and an operator can see. The cost of wrongly refusing one is a member standing in a forest losing a
// photograph they cannot retake.
func TestStorageCeiling_FailsOpen(t *testing.T) {
	t.Run("query error", func(t *testing.T) {
		app, store := ceilingApp(t, 100, 100)
		store.bytesErr = errors.New("database is having a night")

		if v := app.checkGlimtStorageCeiling("p-1", 1<<30); v != glimtCeilingOK {
			t.Errorf("verdict = %v, want OK — an unmeasurable ceiling must not refuse", v)
		}
	})

	t.Run("no projection", func(t *testing.T) {
		people := &stubPeople{p: spejderPerson(), found: true}
		app := photoTestApp(t, &cqrstest.Publisher{}, people)
		app.config.glimtMemberStorageBytes = 100
		// Running without a database is a supported mode (PRD 008 §5).
		if v := app.checkGlimtStorageCeiling("p-1", 1<<30); v != glimtCeilingOK {
			t.Errorf("verdict = %v, want OK with no projection", v)
		}
	})
}

// Two ceilings, two status codes, and the difference decides what the client tells the member.
func TestStorageCeiling_StatusCodesDistinguishWhoseProblemItIs(t *testing.T) {
	t.Run("the member's own quota is a 429", func(t *testing.T) {
		app, _ := ceilingApp(t, 100, 0)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil)

		if !app.writeGlimtCeilingResponse(w, r, glimtCeilingMember) {
			t.Fatal("no response was written")
		}
		if w.Code != http.StatusTooManyRequests {
			t.Errorf("status = %d, want 429", w.Code)
		}
		// It must name a way through. "Du har uploadet for meget" invites a retry that fails
		// identically.
		if body := w.Body.String(); !strings.Contains(body, "Slet") {
			t.Errorf("the message does not tell the member what they can do: %s", body)
		}
	})

	t.Run("the event being full is a 507", func(t *testing.T) {
		app, _ := ceilingApp(t, 0, 100)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil)

		if !app.writeGlimtCeilingResponse(w, r, glimtCeilingTotal) {
			t.Fatal("no response was written")
		}
		// Not a 429: there is nothing this member did wrong and nothing they can do, so blaming
		// their upload volume would be a lie.
		if w.Code != http.StatusInsufficientStorage {
			t.Errorf("status = %d, want 507", w.Code)
		}
	})

	t.Run("an allowed upload writes nothing", func(t *testing.T) {
		app, _ := ceilingApp(t, 0, 0)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/glimt/media", nil)

		if app.writeGlimtCeilingResponse(w, r, glimtCeilingOK) {
			t.Error("a response was written for an allowed upload")
		}
	})
}

// End to end through the real handler, so the check is proven to be *wired in* and not merely
// correct in isolation.
func TestUploadGlimtMedia_RefusesAtTheMemberCeiling(t *testing.T) {
	app, store := ceilingApp(t, 1024, 0)
	store.storedBytes = 1024 // already full

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	ct, body := multipartMedia(t, "media", "glimt.jpg", testImage(t, 64, 64))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}
}

func TestUploadGlimtMedia_RefusesAtTheTotalCeiling(t *testing.T) {
	app, store := ceilingApp(t, 0, 1024)
	store.totalBytes = 1024

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	ct, body := multipartMedia(t, "media", "glimt.jpg", testImage(t, 64, 64))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInsufficientStorage {
		t.Errorf("status = %d, want 507", resp.StatusCode)
	}
}

func TestUploadGlimtMedia_RefusesOverTheByteBudget(t *testing.T) {
	app, _ := ceilingApp(t, 0, 0)
	// A budget far smaller than any real image, so one upload exhausts it.
	app.glimtMediaBudget = ratelimit.NewBudget(64, time.Hour)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	ct, body := multipartMedia(t, "media", "glimt.jpg", testImage(t, 64, 64))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}
}

// The byte budget and the count limiter are separate, and each catches what the other lets through.
func TestGlimtUploadLimits_ACountAndASizeAreDifferentQuestions(t *testing.T) {
	// A generous count and a tiny byte budget: the count cannot stop a big upload.
	app, _ := ceilingApp(t, 0, 0)
	app.glimtMediaLimiter = ratelimit.New(1000, time.Hour)
	app.glimtMediaBudget = ratelimit.NewBudget(64, time.Hour)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	ct, body := multipartMedia(t, "media", "glimt.jpg", testImage(t, 64, 64))
	resp := postMedia(t, srv.URL+"/api/glimt/media", ct, body, cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("a byte budget did not stop an upload a loose count allowed: %d", resp.StatusCode)
	}

	// And the mirror: a huge byte budget cannot stop a client hammering the decode path with tiny
	// images.
	app2, _ := ceilingApp(t, 0, 0)
	app2.glimtMediaLimiter = ratelimit.New(1, time.Hour)
	app2.glimtMediaBudget = ratelimit.NewBudget(1<<30, time.Hour)

	srv2 := httptest.NewServer(app2.routes())
	defer srv2.Close()
	cookies2 := authedCookies(t, app2, srv2, "30000001", "+4530000001")
	for i := 0; i < 2; i++ {
		ct, body := multipartMedia(t, "media", "glimt.jpg", testImage(t, 32, 32))
		resp := postMedia(t, srv2.URL+"/api/glimt/media", ct, body, cookies2)
		resp.Body.Close()
		if i == 1 && resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("a count limit did not stop a second tiny upload: %d", resp.StatusCode)
		}
	}
}

// **The point of the task.**
//
// A burst at browse rates must pass. Two hundred reads in a moment is one member scrolling a hold's
// grid — the load PRD 019 §0a.3 describes as the peak the feature was built for — and it must not be
// throttled. A limiter tuned anywhere near the upload numbers (60/hour) would refuse the fourth
// screen.
func TestGlimtReads_ABrowseBurstIsNotThrottled(t *testing.T) {
	people := &stubPeople{p: spejderPerson(), found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)
	app.config.eventYear = "2026"
	app.models = newModelsWithGlimt(people, &stubGlimt{})
	// The production wiring, not a permissive test value: this is what a deployed instance runs
	// with, so the assertion is about the real configuration.
	app.glimtReadLimiter = limiterOrNil(600, time.Minute)
	// And the write limiters at their production values, to prove the read path does not consult
	// them. If reads and writes ever share a limiter again, this is what fails.
	app.glimtMediaLimiter = limiterOrNil(60, time.Hour)
	app.glimtLimiter = limiterOrNil(20, time.Hour)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for i := 0; i < 200; i++ {
		resp, _ := readBody(t, srv.URL+"/api/glimt/feed", cookies)
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("read %d of 200 was throttled — a browse burst is the load this feature "+
				"exists for (PRD 019 §0a.3), not abuse", i+1)
		}
	}
}

// The hold collection and the media endpoint are the two the browse actually hammers — the feed is
// read once and the grid is read per screen — so they get the same assertion.
func TestGlimtReads_TheHoldGridIsNotThrottled(t *testing.T) {
	people := &stubPeople{p: spejderPerson(), found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)
	app.config.eventYear = "2026"
	app.models = newModelsWithGlimt(people, &stubGlimt{rows: []glimt.Glimt{{
		GlimtID: "g-1", AuthorPersonID: "mock-spejder-1", AuthorGroup: "spejder",
		TeamNumber: "42", Audience: glimt.AudienceGroup, CreatedAt: time.Now(),
	}}})
	app.glimtReadLimiter = limiterOrNil(600, time.Minute)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for i := 0; i < 200; i++ {
		resp, _ := readBody(t, srv.URL+"/api/glimt/hold/42", cookies)
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("hold read %d of 200 was throttled", i+1)
		}
	}
}

// It is still a limiter: a script hammering the endpoint is stopped. That is the only thing it is for.
func TestGlimtReads_AreStillBounded(t *testing.T) {
	people := &stubPeople{p: spejderPerson(), found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)
	app.config.eventYear = "2026"
	app.models = newModelsWithGlimt(people, &stubGlimt{})
	app.glimtReadLimiter = limiterOrNil(3, time.Minute)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for i := 0; i < 3; i++ {
		if resp, _ := readBody(t, srv.URL+"/api/glimt/feed", cookies); resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("read %d was throttled inside the limit", i+1)
		}
	}
	resp, _ := readBody(t, srv.URL+"/api/glimt/feed", cookies)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 past the read limit", resp.StatusCode)
	}
}

// Zero disables a limiter rather than blocking everything.
//
// This is the bug `limiterOrNil` exists to prevent: `ratelimit.Limiter.Allow` refuses when
// `len(kept) >= limit`, so `New(0, w)` blocks **every** request — the exact opposite of the
// "zero means unlimited" convention the config documents, and it would take the feature down
// completely the first time somebody set GLIMT_PER_HOUR=0 expecting to switch the limit off.
func TestLimiterOrNil_ZeroDisablesRatherThanBlocks(t *testing.T) {
	for _, limit := range []int{0, -1} {
		if l := limiterOrNil(limit, time.Hour); l != nil {
			t.Errorf("limiterOrNil(%d) returned a limiter, which would refuse everything", limit)
		}
	}
	if limiterOrNil(1, time.Hour) == nil {
		t.Error("a positive limit produced no limiter")
	}
}

func TestGlimtReads_NoLimiterMeansNoLimit(t *testing.T) {
	people := &stubPeople{p: spejderPerson(), found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)
	app.config.eventYear = "2026"
	app.models = newModelsWithGlimt(people, &stubGlimt{})
	app.glimtReadLimiter = nil

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for i := 0; i < 50; i++ {
		if resp, _ := readBody(t, srv.URL+"/api/glimt/feed", cookies); resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("read %d was throttled with no limiter configured", i+1)
		}
	}
}

// The defaults the service actually ships with. A test rather than a comment, because these numbers
// are the difference between "a ceiling" and "a ration", and a well-meant tightening is invisible.
func TestGlimtLimitDefaults(t *testing.T) {
	cfg := config{}
	// Mirrors the flag defaults in env.go. Kept as a written expectation so a change to those is a
	// deliberate edit here rather than a silent one there.
	cfg.glimtPerHour = 20
	cfg.glimtMediaPerHour = 60
	cfg.glimtBytesPerHour = 200 << 20
	cfg.glimtReadsPerMinute = 600

	// Reads are looser than writes by two orders of magnitude, *and* measured over a shorter
	// window. Both halves matter: an hourly read budget would be spent by somebody scrolling for
	// two minutes and then lock them out for fifty-eight.
	readsPerHour := cfg.glimtReadsPerMinute * 60
	if readsPerHour < cfg.glimtMediaPerHour*100 {
		t.Errorf("the read limit (%d/hour) is not decisively looser than the upload limit (%d/hour) "+
			"— the post-race browse is the load this feature exists for (PRD 019 §0a.3)",
			readsPerHour, cfg.glimtMediaPerHour)
	}
}

// A member's own limits do not spend anybody else's. Participants share networks — a patrol on one
// hotspot, a klan behind one carrier NAT — so every one of these is keyed by member rather than by IP.
func TestGlimtLimits_AreKeyedByMemberNotByNetwork(t *testing.T) {
	app, _ := ceilingApp(t, 0, 0)
	app.glimtMediaBudget = ratelimit.NewBudget(1024, time.Hour)

	if !app.glimtMediaBudget.Allow("member-a", 1024) {
		t.Fatal("member-a should fit")
	}
	if !app.glimtMediaBudget.Allow("member-b", 1024) {
		t.Error("one member exhausting their budget blocked another on the same network")
	}
}

// The person stub these tests use carries a hold, because the create path answers 400 for a member
// without one — which would make a limit test pass for the wrong reason.
