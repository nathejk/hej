package main

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/glimt"
)

// Glimt dev fixture tests (task 327).
//
// The fixture's job is to be *lookable at*, which no test can check. What is testable is that it does
// not exist outside development, that it produces images a browser will accept, and that the set
// actually covers the cases the views have to render — because a fixture that only ever produced one
// landscape image with a short caption would look like it worked.

func TestDevGlimtFixture_DoesNotExistOutsideDevelopment(t *testing.T) {
	// Registered rather than guarded, per dev.go: outside development there is no handler to
	// reach, so a misconfiguration cannot expose it. Asserted through the router.
	app, _, _ := glimtApp(t, nil, spejderPerson())
	app.config.env = "production"

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, srv.URL+"/api/dev/glimt-fixture", nil, cookies)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 — the route must not exist in production", resp.StatusCode)
	}
}

func TestDevGlimtFixture_RequiresAuth(t *testing.T) {
	app, _, _ := glimtApp(t, nil, spejderPerson())
	app.config.env = envDevelopment

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := postGlimtJSON(t, srv.URL+"/api/dev/glimt-fixture", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDevGlimtFixture_PublishesACoveringSet(t *testing.T) {
	app, _, pub := glimtApp(t, nil, spejderPerson())
	app.config.env = envDevelopment

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := postGlimtJSON(t, srv.URL+"/api/dev/glimt-fixture", nil, cookies)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", resp.StatusCode, payload)
	}

	var out devGlimtFixtureResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Created) != len(devGlimtFixtures()) {
		t.Fatalf("created %d glimt, want %d", len(out.Created), len(devGlimtFixtures()))
	}

	// Decode every published event and check the set covers what the views must render. A fixture
	// that quietly narrowed to one shape would still pass a count assertion.
	audiences := map[string]bool{}
	holds := map[string]bool{}
	var maxItems, minItems int
	minItems = 99
	sawOwn, sawOther, sawPortrait, sawLandscape, sawSquare, sawLongCaption, sawNoCaption := false, false, false, false, false, false, false
	reports := 0

	for _, msg := range pub.Messages {
		subject := msg.Subject().Subject()
		if strings.HasSuffix(subject, ".reported") {
			reports++
			continue
		}
		var body glimt.Created
		if err := msg.Body(&body); err != nil {
			t.Fatalf("decode created: %v", err)
		}

		audiences[body.Audience] = true
		holds[body.TeamNumber] = true
		if n := len(body.Media); n > maxItems {
			maxItems = n
		}
		if n := len(body.Media); n < minItems {
			minItems = n
		}
		if body.AuthorPersonID == "mock-spejder-1" {
			sawOwn = true
		} else {
			sawOther = true
		}
		if body.Caption == "" {
			sawNoCaption = true
		}
		if len(body.Caption) > 120 {
			sawLongCaption = true
		}
		for _, m := range body.Media {
			switch {
			case m.Height > m.Width:
				sawPortrait = true
			case m.Width > m.Height:
				sawLandscape = true
			default:
				sawSquare = true
			}
			if m.Ref == "" {
				t.Error("a fixture media item has no ref")
			}
			if m.ThumbRef == "" {
				t.Error("a fixture media item has no thumbnail — the grid would fetch full media")
			}
		}
	}

	for _, want := range []string{glimt.AudienceGroup, glimt.AudienceNathejk, glimt.AudiencePublic} {
		if !audiences[want] {
			t.Errorf("the fixture never uses the %q audience", want)
		}
	}
	if len(holds) < 3 {
		t.Errorf("the fixture spans %d holds; the hold index needs several to be worth looking at", len(holds))
	}
	if minItems != 1 {
		t.Error("no single-item glimt — the strip's non-carousel path would go unseen")
	}
	if maxItems < 3 {
		t.Errorf("largest glimt has %d items; the carousel and its dots need more", maxItems)
	}
	if !sawOwn || !sawOther {
		t.Error("the fixture must include both the caller's own glimt and someone else's, or Slet/Anmeld go unseen")
	}
	if !sawPortrait || !sawLandscape || !sawSquare {
		t.Error("the fixture must mix orientations, or a bad aspect-ratio or object-fit stays invisible")
	}
	if !sawLongCaption || !sawNoCaption {
		t.Error("the fixture must include both a long caption and none, to see wrapping and an absent line")
	}
	if reports != 1 {
		t.Errorf("published %d reports, want exactly 1 so the Skjult badge and the moderation queue have a subject", reports)
	}
}

// TestDevGlimtFixture_SpreadsTimestamps — without this every card reads "Lige nu" and the relative-time
// formatting goes unseen by eye, which is one of the things a fixture exists to expose.
func TestDevGlimtFixture_SpreadsTimestamps(t *testing.T) {
	app, _, pub := glimtApp(t, nil, spejderPerson())
	app.config.env = envDevelopment

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	postGlimtJSON(t, srv.URL+"/api/dev/glimt-fixture", nil, cookies)

	seen := map[int64]bool{}
	for _, msg := range pub.Messages {
		if !strings.HasSuffix(msg.Subject().Subject(), ".created") {
			continue
		}
		var body glimt.Created
		if err := msg.Body(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		seen[body.CreatedAt.Unix()] = true
	}
	if len(seen) < 3 {
		t.Errorf("%d distinct timestamps; the feed order and relative times need a spread", len(seen))
	}
}

func TestDevFixtureImage_IsADecodableJpegOfTheRequestedSize(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1600, 1200}, {1200, 1600}, {1400, 1400}, {64, 64}} {
		data := devFixtureImage(size.w, size.h, 0, 0, "TEST")
		if len(data) == 0 {
			t.Fatalf("%dx%d produced no bytes", size.w, size.h)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%dx%d is not a decodable JPEG: %v", size.w, size.h, err)
		}
		if cfg.Width != size.w || cfg.Height != size.h {
			t.Errorf("decoded %dx%d, want %dx%d", cfg.Width, cfg.Height, size.w, size.h)
		}
	}
}

// TestDevFixtureImage_DiffersPerGlimt is what lets a card in a feed be matched to a tile in a grid by
// eye — the fixture's whole purpose.
// TestDevFixtureImage_MarkersAreActuallyLight is the test that should have existed first.
//
// The first version drew the corner markers with `color.RGBA{255, 255, 255, 220}`, which is
// **alpha-premultiplied** and therefore out of gamut (R > A) — Go rendered them almost black, on a
// dark purple background, making them invisible. The bug was found by looking at the output, which is
// exactly the kind of thing no assertion in this file was checking.
//
// Asserts the corners are much lighter than the middle-ish background, which is the property the
// markers exist for: they have to be visible enough that a bad `object-fit` crop is obvious.
func TestDevFixtureImage_MarkersAreActuallyLight(t *testing.T) {
	for palette := 0; palette < 6; palette++ {
		data := devFixtureImage(400, 300, palette, 0, "TEST")
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("palette %d: decode: %v", palette, err)
		}

		// A few pixels into the top-left corner marker.
		cr, cg, cb, _ := img.At(4, 4).RGBA()
		cornerLuma := (cr + cg + cb) / 3

		// Background, sampled away from every marker and block.
		br, bg, bb, _ := img.At(370, 40).RGBA()
		bgLuma := (br + bg + bb) / 3

		if cornerLuma <= bgLuma {
			t.Errorf("palette %d: corner marker (luma %d) is not lighter than the background (luma %d) — a wrong crop would be invisible",
				palette, cornerLuma, bgLuma)
		}
		// And genuinely light, not merely lighter. JPEG softens the edges, so this is loose.
		if cornerLuma < 40000 {
			t.Errorf("palette %d: corner marker luma %d is too dark to read as a marker", palette, cornerLuma)
		}
	}
}

func TestDevFixtureImage_DiffersPerGlimt(t *testing.T) {
	first := devFixtureImage(400, 300, 0, 0, "A")
	second := devFixtureImage(400, 300, 1, 0, "A")
	if bytes.Equal(first, second) {
		t.Error("two glimt produced identical images; they must be distinguishable at a glance")
	}

	// And per item within a glimt, so "the third photo" is a thing you can point at.
	third := devFixtureImage(400, 300, 0, 2, "A")
	if bytes.Equal(first, third) {
		t.Error("two items of one glimt produced identical images")
	}
}
