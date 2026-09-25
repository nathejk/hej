package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The shared photo viewer's assets and the route that serves them (task 402, PRD 023 §7.3).
//
// # What can and cannot be tested here
//
// The route is behavioural: an ordinary Go HTTP test, and everything about caching, content types and refusal
// is asserted from the outside.
//
// The viewer's *behaviour* is not. There is no way to execute JavaScript in this suite, so the guards below
// read the file and look for things that must or must not be in it. That is weak, and PRD 023 §8 says so
// plainly — task 411's manual QA is the real test of the overlay. Two rules make these guards worth having
// rather than merely present:
//
//  1. **Comments are stripped before searching.** A guard that matches the comment explaining it has cost this
//     repo four false positives. `viewer.js`'s own header discusses admin surfaces at length, which is exactly
//     the text `TestTheViewerKnowsNothingAboutItsSurfaces` must not find.
//  2. **Each guard asserts a decision, not an implementation detail.** "No surface check in this file" and "the
//     filmstrip is hidden by media query" are both things a later edit could undo while every other test stayed
//     green.

// viewerAsset reads one of the viewer's files out of the embedded filesystem.
//
// From `viewerFS` rather than from disk, so the guards run against the bytes the binary actually serves. A file
// renamed on disk but left out of the embed directive would fail to compile; one edited on disk after a stale
// build would pass a disk-reading test and ship broken.
func viewerAsset(t *testing.T, name string) string {
	t.Helper()
	body, err := viewerFS.ReadFile("viewer/" + name)
	if err != nil {
		t.Fatalf("reading the embedded %s: %v", name, err)
	}
	return string(body)
}

// withoutComments removes `//` line comments and `/* … */` blocks.
//
// Crude on purpose: it does not understand string literals, so a `//` inside one would be mistaken for a
// comment. Neither of the viewer's files contains such a literal — there is no URL written into either of them,
// which is itself a property worth having — and a real parser here would be more code than the thing it guards.
// If that changes, this comment is the warning.
func withoutComments(src string) string {
	var out strings.Builder
	for len(src) > 0 {
		block := strings.Index(src, "/*")
		line := strings.Index(src, "//")

		if block < 0 && line < 0 {
			out.WriteString(src)
			break
		}
		if block >= 0 && (line < 0 || block < line) {
			out.WriteString(src[:block])
			rest := src[block+2:]
			end := strings.Index(rest, "*/")
			if end < 0 {
				break
			}
			src = rest[end+2:]
			continue
		}
		out.WriteString(src[:line])
		rest := src[line:]
		end := strings.Index(rest, "\n")
		if end < 0 {
			break
		}
		src = rest[end:]
	}
	return out.String()
}

func TestTheViewerAssetsAreServedAndCacheable(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for name, wantType := range map[string]string{
		"viewer.js":  "javascript",
		"viewer.css": "text/css",
	} {
		resp, err := http.Get(srv.URL + viewerAssetPath(name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: want 200, got %d", name, resp.StatusCode)
			continue
		}
		if got := resp.Header.Get("Content-Type"); !strings.Contains(got, wantType) {
			t.Errorf("%s: want a %s content type, got %q", name, wantType, got)
		}
		// Cacheable for a year, which the content hash in the path is what makes honest. Without the hash this
		// header would strand a fix in every cache that had the old file.
		if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") ||
			!strings.Contains(got, "max-age=31536000") {
			t.Errorf("%s: want a long immutable cache, got %q", name, got)
		}
		if len(body) == 0 {
			t.Errorf("%s: served no bytes", name)
		}
	}
}

// The path is outside /admin, and that is the point of it (task 371, PRD 023 §6).
//
// Every `/admin/*` response carries `no-store`. An asset shared with a public page cannot live under that rule
// and still be cached, so it lives somewhere else entirely — no exception, and nothing for the next thing that
// wants one to cite.
func TestTheViewerAssetsAreNotUnderTheAdminPrefix(t *testing.T) {
	for _, name := range []string{"viewer.js", "viewer.css"} {
		path := viewerAssetPath(name)
		if strings.HasPrefix(path, "/admin") {
			t.Errorf("%s is served from %q, which would need an exception to the no-store rule", name, path)
		}
		if !strings.Contains(path, viewerVersion) {
			t.Errorf("%s is served from %q, which carries no version — a year-long cache would strand a fix "+
				"there", name, path)
		}
	}
}

// Only the two assets, matched against a fixed map (PRD 023 §8).
//
// The parameter comes from the URL, and joining it to a directory is one encoded `../` from serving this
// service's templates out of the embedded filesystem. Asserted because "we look it up in a map" is exactly the
// kind of detail a later refactor tidies into a `filepath.Join` — the same reason
// `TestTheAdminVendorRouteServesOnlyThePinnedAssets` exists.
func TestTheViewerRouteServesOnlyItsTwoAssets(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, name := range []string{
		"page.js",
		"pico.min.css",
		"viewer.js.map",
		"viewer.JS",
		"..%2f..%2fetc%2fpasswd",
		"..%2fadminui%2fpage.html",
		"%2e%2e%2fviewer.js",
	} {
		resp, err := http.Get(srv.URL + "/viewer/" + viewerVersion + "/" + name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			t.Errorf("/viewer/…/%s answered 200; only the two assets may be served", name)
		}
		if strings.Contains(string(body), "package main") || strings.Contains(string(body), "hejViewer") {
			t.Errorf("/viewer/…/%s leaked embedded source", name)
		}
	}
}

// An old version in the path is answered rather than refused.
//
// Deliberate, and documented at `viewerAssetCacheControl`: the version is a cache key, not a lookup. A page
// served just before a deploy — inside the public site's 60-second window — would otherwise lose its viewer
// entirely, which is a worse outcome than one minute of a possibly mismatched pair on a page whose plain form
// works regardless. Storing every past version to avoid that would be a build system.
func TestAnOlderViewerVersionIsStillServed(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/viewer/0000000000/viewer.js")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("a stale page asking for an old version must still get the asset, got %d", resp.StatusCode)
	}
}

// The assets are static files, not templates (the rule that already governs `adminui/*.js`).
//
// An action inside a script is escaped as JavaScript by `html/template`, which mangles values in ways nobody
// notices until a browser does something strange — and it stops the file being something a formatter or a
// linter can read, which is the whole reason these are real files.
func TestTheViewerAssetsCarryNoTemplateActions(t *testing.T) {
	for _, name := range []string{"viewer.js", "viewer.css"} {
		if src := viewerAsset(t, name); strings.Contains(src, "{{") {
			t.Errorf("%s contains a template action; page-specific values arrive through data- attributes", name)
		}
	}
}

// The viewer does not know which surface it is on (PRD 023 §7.7).
//
// This is the guard that makes "the public viewer has no caption editor" a fact about the code rather than a
// promise about a branch. Which controls exist comes from the host page's `data-viewer-actions`; an `isAdmin`
// in here would be the same behaviour one inverted boolean from a public edit button, and nothing in this suite
// could execute the branch to prove otherwise.
//
// Comments are stripped first, because the file's own header explains the admin tool at length.
func TestTheViewerKnowsNothingAboutItsSurfaces(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	for _, forbidden := range []struct{ needle, why string }{
		{"isAdmin", "a surface check is the thing this design exists to avoid"},
		{"/admin", "the viewer must not know an admin URL; endpoints arrive as data- attributes"},
		{"/api/", "no endpoint may be written into the shared file"},
		{"/2026", "no year, and no address of any page it is loaded by"},
		{"requireAdmin", "authorisation is the server's, and is not a concept here"},
	} {
		if strings.Contains(code, forbidden.needle) {
			t.Errorf("viewer.js contains %q: %s", forbidden.needle, forbidden.why)
		}
	}

	// And the positive half: the action row really is built from what the page declared. Without this, a file
	// that hard-coded every button would pass the list above.
	if !strings.Contains(code, "data-viewer-actions") {
		t.Error("viewer.js must read its action row from the host page's data-viewer-actions")
	}
}

// The filmstrip is hidden by a media query, on both dimensions (PRD 023 §6, answering §11.3).
//
// Both, because a landscape phone is a *short* viewport rather than a narrow one, and that is exactly where 15%
// of the height for five thumbnails hurts most. And by CSS rather than JavaScript, because a media query
// re-evaluates on rotation for free while a branch taken once at open is wrong the moment a phone turns.
func TestTheFilmstripHidesOnSmallScreensByMediaQueryAlone(t *testing.T) {
	css := withoutComments(viewerAsset(t, "viewer.css"))

	i := strings.Index(css, "@media (max-width: 40rem), (max-height: 34rem)")
	if i < 0 {
		t.Fatalf("no media query covering narrow *and* short viewports\n%s", css)
	}
	rule := css[i:]
	if j := strings.Index(rule, "\n}"); j >= 0 {
		rule = rule[:j]
	}
	if !strings.Contains(rule, ".hv-strip") || !strings.Contains(rule, "display: none") {
		t.Errorf("the small-screen media query must be what hides the filmstrip:\n%s", rule)
	}

	code := withoutComments(viewerAsset(t, "viewer.js"))
	for _, forbidden := range []string{"matchMedia", "innerWidth", "innerHeight"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("viewer.js measures the window with %s; the filmstrip's visibility is CSS's job so that "+
				"it survives a rotation", forbidden)
		}
	}
}

// Fullscreen is gated on the API existing, and speaks both dialects (task 404, PRD 023 §7.5).
//
// The three facts these needles stand for are platform facts, not preferences, and each of them produces a bug
// that is invisible on the machine most of this was written on:
//
//   - Safari wants `webkitRequestFullscreen`, so a Mac gets a button that silently does nothing without it.
//   - An **iPhone has no element fullscreen at all** — iOS Safari implements it only for video, while iPadOS
//     supports it for elements — so the control must be *absent* rather than inert. A button that does nothing
//     is worse than no button, and it costs an iPhone nothing real: the overlay already fills the viewport.
//   - `Esc` is wanted by two features at once. Left to the browser, which one wins varies, and "Esc closed the
//     whole viewer when I only wanted the window back" is a complaint nobody can reproduce on demand.
func TestFullscreenIsFeatureGatedAndSpeaksBothDialects(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	for _, want := range []struct{ needle, why string }{
		{"requestFullscreen", "the standard call"},
		{"webkitRequestFullscreen", "Safari's, without which the button silently does nothing on a Mac"},
		{"exitFullscreen", "leaving again"},
		{"webkitExitFullscreen", "and leaving again on Safari"},
		{"fullscreenEnabled", "the feature gate: an iPhone has no element fullscreen at all"},
		{"webkitFullscreenEnabled", "the same gate in Safari's dialect"},
		{"fullscreenchange", "the state comes from the event, because Esc and system gestures bypass our button"},
		{"webkitfullscreenchange", "and Safari's spelling of it"},
	} {
		if !strings.Contains(code, want.needle) {
			t.Errorf("viewer.js is missing %s — %s", want.needle, want.why)
		}
	}

	// The control is registered **inside** the gate, which is what makes it absent rather than inert where the
	// API is missing.
	gate := strings.Index(code, "if (fullscreenSupported())")
	reg := strings.Index(code, "register('fullscreen'")
	if gate < 0 || reg < 0 || reg < gate {
		t.Error("the fullscreen control must be registered inside the feature gate, so a device without the API " +
			"gets no button rather than a button that does nothing")
	}

	// And Esc in fullscreen leaves fullscreen rather than closing the viewer.
	if !strings.Contains(code, "case 'Escape':") || !strings.Contains(code, "exitFullscreen();") {
		t.Error("Esc must be decided here: in fullscreen it is the fullscreen key, otherwise the dialog's")
	}
}

// Every class the viewer's CSS defines is prefixed, and its JS uses the same prefix.
//
// The viewer is loaded beside two other stylesheets it does not control — the public site's inline CSS and the
// admin tool's Pico plus `page.css`. An unprefixed `.stage` or `.info` would collide with one of them
// eventually, and the failure would present as a viewer bug on one surface only, which is the most expensive
// kind of bug this arrangement can produce.
func TestTheViewerStylesArePrefixed(t *testing.T) {
	css := withoutComments(viewerAsset(t, "viewer.css"))

	for _, line := range strings.Split(css, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ".") {
			continue
		}
		for _, selector := range strings.Split(line, ",") {
			selector = strings.TrimSpace(selector)
			if !strings.HasPrefix(selector, ".") {
				continue
			}
			if !strings.HasPrefix(selector, ".hv") {
				t.Errorf("unprefixed selector %q: the viewer shares a page with two stylesheets it does not "+
					"own", selector)
			}
		}
	}
}
