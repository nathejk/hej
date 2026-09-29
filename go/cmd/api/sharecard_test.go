package main

import (
	"bytes"
	"image"
	_ "image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The share card route (PRD 026).
//
// # What is worth asserting at this level
//
// `internal/sharecard` tests what the picture is; these test what the *route* promises, and the promises belong to
// whoever fetches it — which is not a browser. `og:image` is loaded by Facebook's scraper, from a server we know
// nothing about, with no cookie and no session. So: it answers unauthenticated, it says it is a PNG, it is
// cacheable, and it is rendered once rather than per request.

func TestTheShareCardIsServedAsACacheablePNG(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/2026/share-card.png", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	// An hour, like the diploma thumbnail: the bytes are generated from embedded artwork rather than
	// content-addressed, so `immutable` would be a lie the day the poster is replaced.
	if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "max-age=3600") {
		t.Errorf("Cache-Control = %q, want an hour", got)
	}
	// Every image on this surface says it, and a scraper fetching og:image ignores it — so this costs nothing and
	// keeps the posture uniform (task 427).
	if got := resp.Header.Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Errorf("X-Robots-Tag = %q, want noindex", got)
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("decoding the card: %v", err)
	}
	if format != "png" || cfg.Width != shareCardWidth || cfg.Height != shareCardHeight {
		t.Errorf("card is %s %dx%d, want png %dx%d",
			format, cfg.Width, cfg.Height, shareCardWidth, shareCardHeight)
	}
}

// It ignores the session, like the rest of the public surface.
//
// The card is fetched by a stranger's server. A route that varied on a cookie would be a route that could serve one
// visitor's view into somebody's chat thread — and this one has nothing to vary on, which is the point.
func TestTheShareCardIgnoresTheSession(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	plain, first := getPublic(t, srv.URL+"/2026/share-card.png", nil)
	withCookie, second := getPublic(t, srv.URL+"/2026/share-card.png",
		[]*http.Cookie{{Name: "session", Value: "whatever"}})

	if plain.StatusCode != http.StatusOK || withCookie.StatusCode != http.StatusOK {
		t.Fatalf("both must answer 200, got %d and %d", plain.StatusCode, withCookie.StatusCode)
	}
	if !bytes.Equal(first, second) {
		t.Error("the card must not depend on who asked")
	}
}

// Rendered once for the life of the process.
//
// The same property `diplomaThumbnail` has, for the same reason: this decodes a 1.4 MB JPEG and walks a page of
// pixels, and the frontpage being shared is exactly when several scrapers ask at once.
func TestTheShareCardIsRenderedOnce(t *testing.T) {
	app, _, _ := publicApp(t)

	first, err := app.shareCard()
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	second, err := app.shareCard()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	// The same slice, not merely equal bytes: a second render would allocate a new one.
	if len(first) == 0 || &first[0] != &second[0] {
		t.Error("the card must be rendered once and remembered")
	}
}
