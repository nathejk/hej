package main

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

// The drop zone (PRD 022 §7, task 373).
//
// # What can and cannot be tested here
//
// The uploader is hand-written JavaScript in a Go template, and this repository has no JavaScript test runner
// that reaches it — `vue/`'s Vitest covers the PWA, which this page deliberately is not part of. So there is no
// honest way to assert from Go that three requests run concurrently.
//
// What *is* testable, and is what these tests do: that the page renders at all (a template action that fails at
// runtime is a blank screen for the curator), that the properties the acceptance criteria name are present in
// the markup, and that the numbers the design depends on have not quietly drifted. The gap is recorded rather
// than papered over — a browser-level test of the batching belongs with the field test in task 329's manner, if
// it is ever judged worth a runner.

// renderAdminPage fetches the tool with the credential and returns its HTML.
//
// A library stub is attached, so the page renders its **normal** state. Without one `PhotoCurator` is nil, the
// page takes its "kan ikke læses lige nu" branch, and the whole counts block is absent — which quietly made an
// early version of `TestTheAdminHeaderReadsEveryCount` fail for a reason that had nothing to do with the header.
func renderAdminPage(t *testing.T) string {
	t.Helper()

	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}

	resp := getAdmin(t, srv, "/admin", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	return adminBody(t, resp)
}

// The degraded branch, asserted on purpose: a nil library must say so rather than show zeroes that read as "you
// have uploaded nothing". The distinction matters most to the person who just uploaded three hundred files.
func TestAdminPageSaysSoWhenTheLibraryIsUnavailable(t *testing.T) {
	_, srv := adminApp(t) // no PhotoCurator

	resp := getAdmin(t, srv, "/admin", testAdminUser, testAdminPass)
	body := adminBody(t, resp)

	if !strings.Contains(body, "kan ikke læses lige nu") {
		t.Error("an unavailable library must be stated, not shown as an empty one")
	}
	// And it still renders the chrome, so the curator can tell they are logged in and the fault is downstream.
	if !strings.Contains(body, adminPageMarker) {
		t.Error("the tool's own chrome should still render, or this is indistinguishable from a wrong password")
	}
}

// The template executes. Worth its own test because `template.Must` only catches *parse* errors at init — an
// action referencing a field that does not exist fails at execute time, which for this handler means a
// half-written body and a log line the curator never sees.
func TestAdminPageRendersWholly(t *testing.T) {
	body := renderAdminPage(t)

	// The closing tag is the cheap proof that execution reached the end rather than dying mid-template.
	if !strings.Contains(body, "</html>") {
		t.Errorf("the page did not render to completion:\n%s", body)
	}
	if strings.Contains(body, "{{") || strings.Contains(body, "[no value]") {
		t.Errorf("the page contains an unexecuted or failed template action:\n%s", body)
	}
}

// The upload control is operable and labelled without a pointer. Two or three people use this for hours
// (PRD 022 §6), and a file input reached only by clicking a styled div is the usual way that breaks.
func TestAdminUploadControlIsKeyboardOperable(t *testing.T) {
	body := renderAdminPage(t)

	if !strings.Contains(body, `<label class="filelabel" for="files">`) {
		t.Error("the file input needs a real <label for>, so it is reachable and announced without JavaScript")
	}
	if !strings.Contains(body, `<input type="file" id="files" multiple`) {
		t.Error("the file input must exist and accept multiple files")
	}

	// Visually hidden, not display:none. `display: none` removes an input from the tab order, which is exactly
	// the bug a hidden-file-input pattern usually ships with.
	css := body[strings.Index(body, "<style>"):strings.Index(body, "</style>")]
	inputRule := css[strings.Index(css, "#files {"):]
	inputRule = inputRule[:strings.Index(inputRule, "}")]
	if strings.Contains(inputRule, "display: none") || strings.Contains(inputRule, "display:none") {
		t.Errorf("#files must not be display:none — it would leave the tab order\n%s", inputRule)
	}
	if !strings.Contains(inputRule, "opacity: 0") {
		t.Errorf("#files should be visually hidden with opacity rather than removed\n%s", inputRule)
	}
	// And the focus ring has to land somewhere visible, since the input itself is invisible.
	if !strings.Contains(css, ".filelabel:focus-within") {
		t.Error("the label needs a :focus-within style, or keyboard focus on the hidden input is invisible")
	}
}

// The year is the one thing on this page that cannot be undone by editing (PRD 022 §5), so it is stated for a
// human and carried in a data attribute for the script rather than interpolated into JavaScript.
func TestAdminPageStatesTheYearUnmistakably(t *testing.T) {
	body := renderAdminPage(t)

	if !strings.Contains(body, `<span class="year" data-year="2026">2026</span>`) {
		t.Error("the year must be rendered prominently and carried as data, not interpolated into the script")
	}
	if !strings.Contains(body, "<title>Billedarkiv 2026") {
		t.Error("the year belongs in the title too, so a tab among many is identifiable")
	}
}

// The concurrency is three, and the reason is in the script's comment. Asserted because it is the number the
// whole "one request per file" design balances on: one is needlessly slow, ten saturates the uplink so every
// file slows together and per-file progress stops meaning anything.
func TestAdminUploaderUsesThreeConcurrentRequests(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	if !strings.Contains(src, "const CONCURRENCY = 3") {
		t.Error("the uploader's concurrency must be 3 — see the script's comment for why not 1 and not 10")
	}
	// One request per file, to the single-file endpoint. A batching request would defeat task 372's whole
	// argument about one bad file failing alone.
	if !strings.Contains(src, "fetch('/api/admin/photos'") {
		t.Error("each file must be its own POST to the single-file endpoint")
	}
}

// The three upload outcomes must look different, because two of them are not successes in the way a green row
// would claim (task 372). A duplicate shown as a plain success makes a duplicated card undiagnosable.
func TestAdminUploaderDistinguishesTheThreeOutcomes(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	for _, want := range []string{"'already'", "'deleted'", "Allerede lagt op", "Slettet tidligere"} {
		if !strings.Contains(src, want) {
			t.Errorf("the uploader must handle and label the %s outcome distinctly", want)
		}
	}
}

// The four position states read differently, and `unknown` must not read as a rejection: PRD 011 §6 requires an
// out-of-bounds coordinate to be visible *as rejected*, while `unknown` is a statement about us — we had no
// race area to judge against — and blaming the photograph for that would be wrong.
func TestAdminUploaderDistinguishesTheFourPositionStates(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	for _, want := range []string{
		"har position",                 // inside
		"uden for området",             // outside
		"position kunne ikke vurderes", // unknown
		".pos.inside", ".pos.outside", ".pos.unknown",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the uploader must distinguish the position state %q", want)
		}
	}

	// And they must not share a colour, which is the way this distinction usually dies.
	css := src[strings.Index(src, ".pos.inside"):strings.Index(src, ".todo")]
	if strings.Count(css, "background:") < 3 {
		t.Errorf("the three position states need distinct styling\n%s", css)
	}
}

// The drop zone is a thin bar when idle and grows during a batch (PRD 022 §7), driven by a class rather than by
// inline styles so the two states are readable in one place.
func TestAdminDropZoneCollapsesWhenIdle(t *testing.T) {
	body := renderAdminPage(t)
	src := adminSource(t, "adminpage.go")

	if !strings.Contains(body, `<section id="drop" class="idle"`) {
		t.Error("the drop zone must start in its idle (collapsed) state")
	}
	for _, rule := range []string{"#drop.idle", "#drop.busy", "#drop.hover"} {
		if !strings.Contains(src, rule) {
			t.Errorf("the drop zone needs a %s rule", rule)
		}
	}
	if !strings.Contains(src, "drop.classList.toggle('busy'") {
		t.Error("the busy state must be driven by a class, not by inline styles")
	}
}

// A dropped **folder** only yields its contents through the entries API — `dataTransfer.files` is empty for a
// directory — and `readEntries` returns at most 100 entries per call. A single read would silently upload the
// first hundred photographs of a card and drop the rest, which is the worst kind of bug here: it looks like it
// worked.
func TestAdminDropZoneReadsAWholeFolder(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	if !strings.Contains(src, "webkitGetAsEntry") {
		t.Error("a dropped folder needs the entries API; dataTransfer.files is empty for a directory")
	}
	if !strings.Contains(src, "createReader()") || !strings.Contains(src, "readEntries") {
		t.Error("directory contents need a reader")
	}
	// The loop is the point. `for (;;)` with a break on an empty batch, rather than one readEntries call.
	walk := src[strings.Index(src, "async function walk"):]
	if !strings.Contains(walk, "for (;;)") {
		t.Error("readEntries must be called in a loop until it returns an empty batch, or only the first " +
			"hundred files in a folder are uploaded")
	}
}

// The preview is the local File, not a fetch of the stored rendition — no round trip, and it shows the
// photographer what they actually selected. The object URL must be revoked, or a 300-file batch holds 300
// decoded bitmaps and the tab dies partway through exactly the batch it was built for.
func TestAdminUploaderRevokesItsPreviewURLs(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	if !strings.Contains(src, "URL.createObjectURL(file)") {
		t.Error("the row preview should come from the local file rather than a server round trip")
	}
	if strings.Count(src, "URL.revokeObjectURL(url)") < 2 {
		t.Error("the object URL must be revoked on both load and error, or a large batch leaks bitmaps")
	}
}

// Failures must not stop the batch. Structural: the per-file promise settles through `finally`, so the pump
// continues regardless of outcome, and the fetch is wrapped so a network error becomes a row rather than an
// unhandled rejection.
func TestAdminUploaderKeepsGoingAfterAFailure(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	if !strings.Contains(src, ".finally(() => {") {
		t.Error("the queue must advance in a finally, so a failed file does not stall the batch")
	}
	upload := src[strings.Index(src, "async function upload(job)"):]
	upload = upload[:strings.Index(upload, "function applyOutcome")]
	if !strings.Contains(upload, "catch (err)") {
		t.Error("a network error must become a row with a reason, not an unhandled rejection")
	}
	// And every failure path writes a Danish sentence rather than a status code alone.
	if !strings.Contains(src, "Forbindelsen blev afbrudt") {
		t.Error("a dropped connection needs a plain Danish reason that invites the recovery that works")
	}
}

// Every HTTP failure the endpoint can produce has a Danish sentence. A curator reading three hundred rows
// should never meet a bare status code.
//
// **507 is the one that matters most** (task 384): it means the volume is full, and the right action — keep
// the card, do not clear it — is the opposite of what somebody would guess from "upload failed". It was
// missing until task 388 went looking.
//
// 429 stays in the list although `requireAdmin` no longer produces one (task 388 dropped the limiter). The
// endpoint is behind Traefik and a proxy-originated 429 is still reachable, so the sentence is cheap
// insurance — and a client that renders a bare code for it would be a worse outcome than one line of dead
// JavaScript.
func TestAdminUploaderExplainsEveryFailureInDanish(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	reason := src[strings.Index(src, "function httpReason"):]
	reason = reason[:strings.Index(reason, "// --- rows")]

	for _, status := range []string{"413", "400", "401", "507", "429", "503"} {
		if !strings.Contains(reason, status) {
			t.Errorf("status %s has no plain-language reason; the endpoint can return it", status)
		}
	}
}

// **The boundary this task must not cross.** No npm dependency, no bundler, no framework, and nothing added
// under `vue/` (PRD 022 §7, §8.10).
//
// Checked against git rather than by inspection, because the failure would be somebody reaching for a helper
// library at the moment the JavaScript gets awkward — which is precisely when nobody re-reads the PRD.
func TestTheAdminToolAddsNothingToTheFrontend(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	// The one permitted external script, and the reason it is permitted.
	//
	// PRD 022 §7 names the map island as the single exception to this page's no-build-step rule: the public
	// patrol page already ships vendored, self-hosted Leaflet (task 342), and reusing those exact files is
	// cheaper than introducing a second way to draw a map. It is **vendored**, not a CDN — which is the part that
	// matters, since a CDN would put a third party in a position to log who looked at the event's photographs.
	//
	// An allowlist rather than dropping the check: everything else still fails, including a second copy of
	// Leaflet, a clustering plugin, or the same library from a CDN.
	const vendoredLeaflet = `<script src="/vendor/leaflet.js" defer></script>`
	if !strings.Contains(src, vendoredLeaflet) {
		t.Error("the map island must load the same vendored Leaflet the public patrol page uses")
	}
	scriptTags := strings.Count(src, "<script src=")
	if scriptTags != 1 {
		t.Errorf("want exactly one external script (the vendored Leaflet), found %d — a second one is a new "+
			"dependency and needs its own decision", scriptTags)
	}
	for _, cdn := range []string{"unpkg", "jsdelivr", "cdn.", "googleapis", "//cdnjs"} {
		if strings.Contains(src, cdn) {
			t.Errorf("the admin page references %q: the map libraries are vendored precisely so no third party "+
				"learns who looked at the event's photographs", cdn)
		}
	}

	// And the page's own script pulls nothing in.
	script := src[strings.Index(src, "<script>"):]
	for _, smell := range []string{"import ", "require(", "importScripts", "eval("} {
		if strings.Contains(script, smell) {
			t.Errorf("the admin page's script references %q: it must have no build step and no dependency", smell)
		}
	}

	// No file was added under vue/ for it. `git status` rather than a directory listing, so this asserts about
	// the change rather than about the tree.
	out, err := exec.Command("git", "status", "--porcelain", "--", "../../../vue").Output()
	if err != nil {
		t.Skipf("git is unavailable, skipping the vue/ check: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// An added file is the thing forbidden. `vite.config.ts` is *modified* by task 370, which routes /admin
		// to the BFF in dev — a routing fix, not a frontend for the tool, and without it the page is unreachable
		// in a dev browser.
		if strings.HasPrefix(line, "A ") || strings.HasPrefix(line, "??") {
			t.Errorf("the admin tool must add no file under vue/: %s", line)
		}
	}
}

// The map is an **enhancement, never a requirement** — the rule the public island already follows. Where Leaflet
// cannot run, the checkpoint picker is a complete way to do the job, so the panel must not depend on the map
// having drawn.
func TestTheAdminPositionPanelWorksWithoutTheMap(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	// The island bails out rather than throwing when Leaflet is absent.
	if !strings.Contains(src, "if (typeof L === 'undefined') return") {
		t.Error("the map must degrade silently when Leaflet is unavailable, not throw and take the panel with it")
	}
	// The container starts hidden and is only revealed once the island draws, as the public page's does.
	if !strings.Contains(src, `<div id="posmap" hidden>`) {
		t.Error("the map container must start hidden, so a blocked island leaves no empty grey box")
	}
	// And the post picker is a real form control, not something the map builds.
	if !strings.Contains(src, `<select id="cppick">`) {
		t.Error("the checkpoint picker must be server-rendered markup, so it works without the map")
	}
}

// **The map actually draws a base layer** (task 389).
//
// # Why this is asserted on the source rather than on a rendered map
//
// The island runs in a browser with Leaflet and two network fetches; nothing in this suite can execute it. What
// *can* be checked is the shape of the lookup — and the shape is precisely what was wrong.
//
// The first version read `layers.layers[0]`. `maplayers.json` keys `layers` by **layer id** (`dtk25`, `dtk50`,
// `orto`), so that expression is permanently `undefined`: no tile layer was ever added and the panel showed an
// empty container. It degraded exactly as `TestTheAdminPositionPanelWorksWithoutTheMap` requires, which is why
// nothing caught it — a map that silently draws nothing passes every test about failing gracefully.
//
// So this asserts the three things that make it render, each of which was missing.
func TestTheAdminPositionMapDrawsARealBaseLayer(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	island := src[strings.Index(src, "async function drawPositionMap"):]
	island = island[:strings.Index(island, "function attachTileRetry")]

	// Indexed by key, never by position. This is the bug, stated as a rule.
	if strings.Contains(island, ".layers[0]") {
		t.Error("the base layer must be looked up by key: maplayers.json keys `layers` by layer id, so an " +
			"index is permanently undefined and the map draws nothing at all (task 389)")
	}

	for _, want := range []struct{ needle, why string }{
		{"cfg.default", "the file names its own default layer; picking one here would drift from the app"},
		{"dataforsyningen_token", "these are Dataforsyningen WMS endpoints and refuse a request with no token"},
		{"layers: base.layer", "a WMS request without a layer parameter is not a tile request"},
		{"format: base.format", "likewise the format"},
		{"attachTileRetry(", "Leaflet has no tile retry, and the shared config ships a retry policy for it"},
	} {
		if !strings.Contains(island, want.needle) {
			t.Errorf("the position map is missing %q: %s", want.needle, want.why)
		}
	}

	// A map that cannot be drawn says so. An empty square with no explanation is the state this task was
	// reported as, and "it degrades gracefully" is not a licence to degrade silently.
	if !strings.Contains(island, "Kortet kan ikke hentes") {
		t.Error("a missing base layer must be said in Danish; a grey rectangle tells the curator nothing")
	}
}

// The retry is the app's, not an approximation of it.
//
// Ported by hand because this page has no build step and cannot import the app's TypeScript. That makes drift
// the risk, so the three properties that matter are pinned here — each one is a thing a simplifying rewrite
// would drop, and each one would leave a retry that appears to work.
func TestTheAdminTileRetryMatchesTheApp(t *testing.T) {
	src := adminSource(t, "adminpage.go")
	retry := src[strings.Index(src, "function attachTileRetry"):]
	retry = retry[:strings.Index(retry, "\n  }\n")+4]

	for _, want := range []struct{ needle, why string }{
		{"tile.src = original + '&_retry=' + attempt", "re-assigning src on the **same** <img> keeps " +
			"Leaflet's own handlers attached, so a late success still fades the tile in normally — and the " +
			"cache-buster defeats negative caching of the failed response. Asserted as the whole assignment: " +
			"a break-test showed that `_retry=` alone also matches the regex that strips it, so the shorter " +
			"needle proved nothing"},
		{"Math.random()", "jitter stops a screen of failed tiles retrying in lockstep and hammering the service"},
		{"tile.isConnected", "a tile discarded by a pan or a layer swap must not be revived"},
		{"Math.pow(2, attempt - 1)", "exponential backoff, as the shared file's comment specifies"},
	} {
		if !strings.Contains(retry, want.needle) {
			t.Errorf("the tile retry is missing %q: %s", want.needle, want.why)
		}
	}

	// The numbers come from the shared file, so a change to the policy reaches this page without an edit.
	for _, k := range []string{"retry.limit", "retry.baseDelayMs", "retry.jitterMs"} {
		if !strings.Contains(retry, k) {
			t.Errorf("%s must be read from maplayers.json rather than hard-coded here, or the two drift", k)
		}
	}
}
