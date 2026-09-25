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

// Share sends a link and never the bytes (task 405, PRD 023 §4, §7.8).
//
// # Why `files` is the needle that matters
//
// `navigator.share` accepts them, and one line would turn this into a way to hand out copies of a child's
// photograph that outlive a takedown. A shared **link** stops working when a photograph comes down; a shared
// **JPEG** does not. That is the whole distinction between "look at this" and republishing, and it is one word
// away in either direction — so it is asserted rather than trusted.
func TestShareSendsALinkAndNeverTheBytes(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	if !strings.Contains(code, "navigator.share") {
		t.Fatal("no share at all")
	}
	// Asserted on the whole call rather than on the word "files", because "files" appears in no end of innocent
	// contexts and a needle that matches the code removing the thing it checks for has fooled this repo before.
	if !strings.Contains(code, "navigator.share({ title: ctx.config.shareTitle, url: url })") {
		t.Errorf("the share call must pass a title and a url and nothing else — never files, which would hand out " +
			"copies that outlive a takedown")
	}

	// The title comes from the container's declaration, which is the album's. A caption is free text a curator
	// typed and the one place a person's name could plausibly end up, and a share sheet's title is what gets
	// quoted into a group chat.
	if strings.Contains(code, "title: ctx.item.caption") {
		t.Error("the share title must be the album's, never a caption")
	}

	// A clipboard fallback exists, and the control is absent where neither API does — the same rule as the
	// fullscreen button. A share button that does nothing on a laptop is worse than one that copies.
	if !strings.Contains(code, "navigator.clipboard.writeText") {
		t.Error("desktop Firefox has no navigator.share, so copying is the fallback")
	}
	if !strings.Contains(code, "Linket er kopieret.") {
		t.Error("copying must say so, in Danish, or it looks like nothing happened")
	}
	gate := strings.Index(code, "if (canShare || canCopy)")
	reg := strings.Index(code, "register('share'")
	if gate < 0 || reg < 0 || reg < gate {
		t.Error("the share control must be registered inside its feature gate, so a browser with neither API gets " +
			"no button rather than an inert one")
	}

	// The shared address carries both halves (task 401): the query for the server, the fragment for the browser.
	if !strings.Contains(code, "url.searchParams.set(param, ctx.item.ordinal)") ||
		!strings.Contains(code, "url.hash = param") {
		t.Error("a shared link needs the query and the fragment: one picks the page, the other scrolls to the tile")
	}
}

// A shared link must not carry the window it happened to be cut to.
//
// `?side=2` in a shared address is a page number that stops meaning anything the moment a curator adds
// photographs, and the server derives the right window from the ordinal anyway (task 401).
func TestASharedLinkDropsThePageNumber(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))
	if !strings.Contains(code, "url.searchParams.delete('side')") {
		t.Error("the share URL must drop ?side=: it is how this page is cut up today, not part of what is shared")
	}
}

// A closed dialog must be hidden by our own stylesheet (task 415).
//
// # The bug this is the guard for, because it is genuinely counter-intuitive
//
// The browser's stylesheet has `dialog:not([open]) { display: none }`, and `.hv { display: grid }` looks like it
// should lose to it — (0,1,1) against (0,1,0) on specificity. It does not: **the cascade compares origin before
// specificity**, and any author declaration beats a user-agent one however specific. So setting `display` on a
// dialog silently un-hides it when closed.
//
// Reported from Brave on macOS: clicking the close control made the photograph vanish (the JavaScript ran) while
// the dark overlay stayed (the CSS won), and `Esc` then did nothing because the dialog was already closed. Pico
// carries the same rule for the same reason, which is the hint that this is a property of `dialog` rather than a
// mistake peculiar to this file.
func TestAClosedViewerIsHidden(t *testing.T) {
	css := withoutComments(viewerAsset(t, "viewer.css"))

	i := strings.Index(css, ".hv:not([open])")
	if i < 0 {
		t.Fatal("viewer.css must hide a closed dialog itself: it sets display on .hv, which beats the browser's " +
			"own dialog:not([open]) rule by cascade origin, so .close() would leave the overlay on screen")
	}
	rule := css[i:]
	if j := strings.Index(rule, "}"); j >= 0 {
		rule = rule[:j]
	}
	if !strings.Contains(rule, "display: none") {
		t.Errorf("the closed-dialog rule must hide it:\n%s", rule)
	}
}

// The controls are registered before anything can open the viewer (task 415).
//
// # Why the order is a correctness property and not tidiness
//
// `start()` does two things: it binds the containers, and it **opens the viewer immediately** when the address
// carries `?foto=`. It used to run halfway up the file, before the controls at the bottom were registered, so a
// cold load of a shared link built its action row from an empty registry — a viewer with a close button and
// nothing else.
//
// That is not a corner case. The viewer puts `?foto=` on the address as you move, so reloading the page
// reproduces it every time, which is exactly how it was found: "there is no fullscreen icon, and no share icon"
// from Brave on macOS.
//
// Asserted by position, because the failure is invisible in review — both orderings look equally sensible — and
// silent in every Go test, since none of them can execute this file.
func TestTheViewerRegistersItsControlsBeforeItCanOpen(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	start := strings.LastIndex(code, "start();")
	if start < 0 {
		t.Fatal("viewer.js never starts")
	}
	for _, name := range []string{"register('fullscreen'", "register('share'"} {
		at := strings.Index(code, name)
		if at < 0 {
			t.Errorf("no %s", name)
			continue
		}
		if at > start {
			t.Errorf("%s is registered after the viewer can open, so a ?foto= load builds its action row from an "+
				"empty registry — a viewer with nothing but a close button", name)
		}
	}
}

// The caption must not move the arrows (task 416).
//
// Reported by the maintainer: the info panel "takes up space and changes the position of the left/right arrows".
// It was a row of the dialog's grid, so a captioned photograph made the stage shorter than an uncaptioned one and
// the arrows sat somewhere else — furniture moving under the cursor between one press and the next, which is the
// kind of thing that feels broken without being nameable.
//
// The fix is that the panel is positioned over the photograph rather than laid out beside it, and that is what
// this asserts: a later "tidy-up" putting it back in the flow would bring the bug back exactly.
func TestTheCaptionOverlaysThePhotographRatherThanMovingIt(t *testing.T) {
	css := withoutComments(viewerAsset(t, "viewer.css"))

	rule := ruleFor(t, css, ".hv-info {")
	if !strings.Contains(rule, "position: absolute") {
		t.Errorf("the info panel must be positioned over the stage, or a caption changes where the arrows are:\n%s",
			rule)
	}
	if !strings.Contains(rule, "pointer-events: none") {
		t.Error("nothing in the info panel is interactive and it sits over the middle of the picture, where a " +
			"swipe starts — a caption that swallowed a swipe would be a caption that broke the album")
	}
	// Two rows, and they are the frame's rather than the dialog's (task 419): the frame is what goes fullscreen, so
	// the layout has to live on it to be the same in both containers.
	if frame := ruleFor(t, css, ".hv-frame {"); !strings.Contains(frame, "grid-template-rows: 1fr auto") {
		t.Errorf("want the stage and the filmstrip as the frame's only rows:\n%s", frame)
	}
}

// The arrows are small to look at and large to hit (task 416).
//
// "It's fine that right/left arrows are relatively small, but make sure that I can click a larger area." The two
// are different properties and there is no reason for them to be the same number: the button is a tall column
// down the side of the photograph, and the circle you see is painted on the icon inside it.
func TestTheViewerArrowsHaveALargerHitAreaThanIcon(t *testing.T) {
	css := withoutComments(viewerAsset(t, "viewer.css"))

	rule := ruleFor(t, css, ".hv-nav {")
	for _, want := range []struct{ needle, why string }{
		{"top: 0", "the hit area runs the height of the stage"},
		{"bottom: 0", "both edges, or it is a strip rather than a column"},
		{"width: 22%", "and it is a proportion of the width, so it scales with the viewport"},
		{"background: transparent", "the button itself is invisible; only the icon is seen"},
	} {
		if !strings.Contains(rule, want.needle) {
			t.Errorf("the arrow's hit area is missing %q — %s:\n%s", want.needle, want.why, rule)
		}
	}
	// The visible circle is on the icon, which is what keeps the two sizes independent.
	if icon := ruleFor(t, css, ".hv-nav svg {"); !strings.Contains(icon, "border-radius: 50%") {
		t.Errorf("the icon carries the visible circle:\n%s", icon)
	}
}

// ruleFor returns the declarations of one CSS rule, for guards that are about a specific decision in it.
func ruleFor(t *testing.T, css, selector string) string {
	t.Helper()

	i := strings.Index(css, selector)
	if i < 0 {
		t.Fatalf("no %s rule in viewer.css", selector)
	}
	rule := css[i:]
	if j := strings.Index(rule, "}"); j >= 0 {
		rule = rule[:j]
	}
	return rule
}

// Fullscreen asks the frame, and reports a refusal (tasks 416–419).
//
// # Four targets were tried on a real browser and three are wrong
//
// This took four rounds, so the conclusions are asserted rather than described — and **each wrong answer is named**,
// because a guard that only asserts the current target is satisfied by whatever happens to be there. One that only
// did that was green through two of these rounds while describing a bug.
//
//   - **The dialog is refused.** It is in the top layer because `showModal` put it there, and Chromium will not
//     fullscreen an element already in it — the promise rejects (Brave, macOS).
//   - **The document element is accepted and renders wrong.** It joins the top layer *after* the dialog, so the
//     album page paints over the photograph and the info panel falls behind it.
//   - **The stage loses the filmstrip**, which is its sibling rather than its child.
//   - **The frame works**: an ordinary div, not in the top layer itself, containing the whole viewer.
//
// # The reporting is what ended the guessing
//
// The original complaint was "fullscreen is not working", and the first version caught the rejected promise and
// said nothing, so a refusal and a rendering fault were indistinguishable from outside. Two of the four rounds went
// on that. A refusal now says so on screen.
func TestFullscreenAsksTheFrameAndReportsRefusal(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	if !strings.Contains(code, "enterFullscreen(ctx.frame)") {
		t.Error("fullscreen must be requested on the frame: it is not in the top layer, so the request is accepted, " +
			"and it contains the whole viewer including the filmstrip")
	}
	for _, wrong := range []struct{ needle, why string }{
		{"enterFullscreen(ctx.dialog)",
			"Chromium refuses a fullscreen request for an element already in the top layer, which a modal dialog " +
				"is (task 417)"},
		{"enterFullscreen(document.documentElement)",
			"the document element joins the top layer above the dialog, so the album page paints over the " +
				"photograph and the caption falls behind it (task 416)"},
		{"enterFullscreen(ctx.stage)",
			"the stage does not contain the filmstrip, so a fullscreen view had no way to see where you were in " +
				"the album (task 418)"},
	} {
		if strings.Contains(code, wrong.needle) {
			t.Errorf("%s was tried and is wrong: %s", wrong.needle, wrong.why)
		}
	}
	// A refusal says so. Asserted on the message, because an empty catch block is the shape this guards against
	// and an empty block is hard to match reliably.
	if !strings.Contains(code, "Fuld skærm er ikke tilgængelig her.") {
		t.Error("a refused fullscreen request must say so: swallowing it is what made this take four rounds")
	}

	// The frame has to bring its own background when it becomes the fullscreen element, since the dialog that
	// supplies the colour in windowed mode is no longer behind it.
	css := withoutComments(viewerAsset(t, "viewer.css"))
	if !strings.Contains(css, ".hv-frame:fullscreen") || !strings.Contains(css, ".hv-frame::backdrop") {
		t.Error("the fullscreen frame needs its own background and backdrop, or it is a photograph on the " +
			"browser's black rather than on the viewer's")
	}
}

// The filmstrip is inside the fullscreen element (task 419).
//
// Reported plainly: "now the row of thumbnails has gone from the fullscreen view". Task 418 had made the stage the
// fullscreen element and called losing the strip a reasonable trade. It was not — the strip is how you see where
// you are in an album, and fullscreen is exactly when an album is being looked through rather than glanced at.
//
// Asserted structurally, because "is it visible" is not a question this suite can ask: the strip must be a child of
// the frame, so that whatever the frame fills, the strip is in it.
func TestTheFilmstripIsInsideTheFullscreenElement(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	for _, want := range []string{"frame.appendChild(stage)", "frame.appendChild(strip)", "dialog.appendChild(frame)"} {
		if !strings.Contains(code, want) {
			t.Errorf("want %s: the frame must hold the whole viewer, or fullscreening it leaves part behind", want)
		}
	}
	for _, forbidden := range []string{"dialog.appendChild(stage)", "dialog.appendChild(strip)"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("%s puts a piece of the viewer outside the frame, so fullscreen would lose it", forbidden)
		}
	}
}

// Nothing in the viewer is selectable, except the field you type in (task 420).
//
// Two fast presses on the next arrow are a double-click as far as the browser is concerned, and a double-click
// selects — so moving quickly through an album left a blue highlight over the photograph. Reported from Brave on
// macOS. The clicks themselves were always fine; it is the selection that is unwanted.
//
// The exception matters as much as the rule: the caption editor's field is text being written, and a curator who
// could not select a word of their own caption to replace it would have a worse problem than the one this fixes.
func TestTheViewerDoesNotSelectItselfOnADoubleClick(t *testing.T) {
	css := withoutComments(viewerAsset(t, "viewer.css"))

	root := ruleFor(t, css, ".hv {")
	for _, want := range []string{"user-select: none", "-webkit-user-select: none"} {
		if !strings.Contains(root, want) {
			t.Errorf("the overlay is missing %q, so two fast presses on an arrow highlight the photograph:\n%s",
				want, root)
		}
	}

	field := ruleFor(t, css, ".hv-edit-field {")
	for _, want := range []string{"user-select: text", "-webkit-user-select: text"} {
		if !strings.Contains(field, want) {
			t.Errorf("the editor's field must opt back in to selection (%q), or a curator cannot select a word of "+
				"their own caption to replace it:\n%s", want, field)
		}
	}
}

// Closing the viewer takes the photograph out of the address (task 421).
//
// # The report, and the two facts that were one flag
//
// "When I close the album the URL stays `?foto=9#foto-9`, and then if I reload the overview the previously
// selected photo opens."
//
// Closing wound the history back — and only that. Which works when the viewer pushed the entry it is sitting on,
// and fails in two ways when it did not:
//
//   - opened from a shared or reloaded `?foto=` link in a **fresh tab**, there is nothing behind it, so back does
//     nothing at all: the address keeps the parameter and a reload reopens the photograph, exactly as reported;
//   - and where there *is* something behind it, back leaves the album altogether.
//
// The cause was one flag standing for two different facts. `reflected` means the address already names the current
// photograph, so moving should replace rather than push. `pushed` means *we* added an entry, so closing may wind
// it back. A viewer opened from a link is the first and not the second.
func TestClosingTheViewerClearsThePhotographFromTheAddress(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	// The two facts are separate flags, and the link case sets them differently. This is the line the bug was on.
	open := strings.Index(code, "openFromURL")
	if open < 0 {
		t.Fatal("no openFromURL")
	}
	tail := code[open:]
	if !strings.Contains(tail, "ui.reflected = true") || !strings.Contains(tail, "ui.pushed = false") {
		t.Error("a viewer opened from the address is reflected but not pushed: the entry it sits on belongs to " +
			"whoever sent the link, so closing must not press back")
	}

	// And closing has both paths.
	if !strings.Contains(code, "unreflectURL()") {
		t.Error("closing a viewer we did not push must take the parameter out of the address in place")
	}
	for _, want := range []struct{ needle, why string }{
		{"url.searchParams.delete(param)", "the parameter goes, or a reload reopens the photograph"},
		{"url.hash = ''", "and the fragment with it — half a cleaned address gets reported a second time"},
		{"window.history.replaceState({}, '', url.toString())", "in place, without navigating anywhere"},
	} {
		if !strings.Contains(code, want.needle) {
			t.Errorf("unreflectURL is missing %s — %s", want.needle, want.why)
		}
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
