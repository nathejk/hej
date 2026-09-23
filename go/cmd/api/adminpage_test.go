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
func renderAdminPage(t *testing.T) string {
	t.Helper()

	_, srv := adminApp(t)
	resp := getAdmin(t, srv, "/admin", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	return adminBody(t, resp)
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
func TestAdminUploaderExplainsEveryFailureInDanish(t *testing.T) {
	src := adminSource(t, "adminpage.go")

	reason := src[strings.Index(src, "function httpReason"):]
	reason = reason[:strings.Index(reason, "// --- rows")]

	for _, status := range []string{"413", "400", "401", "429", "503"} {
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
	// The page's script must not pull anything in.
	//
	// Scoped to the template rather than the whole file, because the file is Go source and its own `import`
	// block is not a frontend dependency — which the first version of this test cheerfully flagged.
	src := adminSource(t, "adminpage.go")
	script := src[strings.Index(src, "<script>"):]
	for _, smell := range []string{
		"<script src=", "import ", "require(", "cdn.", "unpkg", "jsdelivr",
	} {
		if strings.Contains(script, smell) {
			t.Errorf("the admin page's script references %q: it must have no build step and no dependency", smell)
		}
	}

	// And no file was added under vue/ for it. `git status` rather than a directory listing, so this asserts
	// about the change rather than about the tree.
	out, err := exec.Command("git", "status", "--porcelain", "--", "../../../vue").Output()
	if err != nil {
		t.Skipf("git is unavailable, skipping the vue/ check: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// An added file is the thing forbidden. `vite.config.ts` is *modified* by task 370, which routes
		// /admin to the BFF in dev — a routing fix, not a frontend for the tool, and without it the page is
		// unreachable in a dev browser.
		if strings.HasPrefix(line, "A ") || strings.HasPrefix(line, "??") {
			t.Errorf("the admin tool must add no file under vue/: %s", line)
		}
	}
}
