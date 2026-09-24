package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The admin door (PRD 022 §8.2, tasks 369–371).
//
// # Why this file is disproportionately large for one wrapper
//
// Because it is the only inbound credential in the service, and because the thing it protects is a write path
// into the blob store — the one store here that cannot be rebuilt from the event log. The feature is twenty
// lines; the properties that make it acceptable are what need holding down.
//
// Note the tests send `X-Forwarded-Proto: https` for the happy paths. That is not a workaround: it is exactly
// what production looks like, because Traefik terminates TLS and speaks plain HTTP to this service. A test
// that omitted it would be testing the dev exemption instead of the real path.

const (
	testAdminUser = "foto"
	testAdminPass = "a-long-generated-password"
)

// adminApp returns an app with the admin tool configured, and its server.
func adminApp(t *testing.T) (*application, *httptest.Server) {
	t.Helper()

	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, srv
}

// getAdmin issues a request with the credential and the proxy header production sets.
func getAdmin(t *testing.T, srv *httptest.Server, path, user, pass string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// readBody drains a response body. Named distinctly from glimtfeed_test.go's `readBody`, which takes a URL
// and issues the request itself.
func adminBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the response body: %v", err)
	}
	return string(b)
}

// adminSource reads a file in this package, for the structural assertions below.
//
// Several properties here are not observable from a response: a constant-time comparison, the absence of a
// default password, and whether a route sits inside a conditional block. Those are read from the source, in
// the manner of glimtopenapi_test.go and publicprivacy_test.go.
func adminSource(t *testing.T, file string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	return string(b)
}

// adminPageSource returns the assembled admin page — markup, CSS and every script — exactly as it is served.
//
// # Why this exists, and why it is better than reading the Go file
//
// Before task 394 the page was one raw-string literal in `adminpage.go`, so these guards read that **Go source
// file** and grepped it. That worked and had a sharp edge: the needle could match a *Go comment*, so a rule
// could pass against the prose explaining it rather than the code implementing it. It happened twice in one
// session.
//
// This calls the same `mustInjectAdminAssets` the production template is built from, so what the assertions see
// is what the browser gets — no Go, no reconstruction. A rule about the page is now tested against the page.
//
// It takes `adminPageScripts` rather than a list of its own, which is the detail that kept ~30 guards working
// unchanged when task 395 split page.js into nine files: if a script is added to the page it is added here, so no
// assertion can quietly stop covering code that is being served.
//
// Where a rule is genuinely about one file, `adminAsset` reads that file alone.
func adminPageSource(t *testing.T) string {
	t.Helper()
	return mustInjectAdminAssets("adminui/page.html", "adminui/page.css", adminPageScripts...)
}

// adminAsset reads one of the admin tool's real asset files, by name under adminui/.
func adminAsset(t *testing.T, name string) string {
	t.Helper()
	return mustReadAdminAsset("adminui/" + name)
}

// The admin tool's assets are real files (task 394).
//
// # What these guard, and why it is worth guarding
//
// The markup, CSS and JavaScript were one Go raw-string literal until task 394. A backtick anywhere in any of
// the three terminated it — four incidents, each presenting as a Go syntax error pointing at a line of CSS — and
// no editor could help with 1,400 lines of JavaScript inside a string.
//
// Now they are `adminui/page.{html,css}` with the JavaScript in nine per-feature files, plus
// `adminui/album.{html,css,js}`, spliced into one document before parsing. These tests hold the two properties
// that make that arrangement safe rather than merely tidier.

// **The CSS and every script carry no template actions, so they are genuinely valid standalone files.**
//
// This is what separates "real files" from "fragments in a different location". A `{{.Year}}` in one of these
// would make it un-lintable, un-formattable and un-runnable outside the Go template — and it would land inside
// `<script>`, where `html/template` applies **JavaScript** escaping and mangles values in ways nobody notices
// until a curator's browser does something strange.
//
// The rule this preserves predates the extraction: every value the script needs is read from a `data-` attribute
// on an element. That was already true, which is the only reason the extraction was safe.
//
// The script list comes from `adminPageScripts` rather than being repeated, so a tenth file is covered the moment
// it is served.
func TestTheAdminAssetsCarryNoTemplateActions(t *testing.T) {
	names := []string{"page.css", "album.css", "album.js"}
	for _, p := range adminPageScripts {
		names = append(names, strings.TrimPrefix(p, "adminui/"))
	}
	if len(names) < 12 {
		t.Fatalf("expected the page's scripts to be covered, got only %d files: %v", len(names), names)
	}

	for _, name := range names {
		src := adminAsset(t, name)
		if strings.Contains(src, "{{") {
			t.Errorf("%s contains a template action. Pass the value through a data- attribute instead: an "+
				"action here is escaped as JavaScript or CSS by html/template, and it stops this file being "+
				"something a formatter or a linter can read", name)
		}
	}
	// And the HTML is where the actions belong — asserted so a future "tidy-up" that moved them out and
	// reintroduced JS interpolation would fail here rather than silently.
	if !strings.Contains(adminAsset(t, "page.html"), "{{.Year}}") {
		t.Error("page.html should carry the template's actions; if it no longer does, where did they go?")
	}
}

// **Every injection marker resolves.** A typo in one produces a page with no styling or no behaviour, and no
// error anywhere — so `mustInjectAdminAssets` panics, and this is the test that says so out loud.
//
// Asserted through the assembled output rather than by inspecting the markers: what matters is that the CSS and
// the JS actually arrive in the served document.
func TestTheAdminAssetsAreActuallyInjected(t *testing.T) {
	page := adminPageSource(t)

	if strings.Contains(page, "@inject") {
		t.Error("an @inject marker survived into the assembled page, so one of the assets was not spliced in")
	}
	// A distinctive line from each file, so this fails if a marker is replaced with the wrong asset.
	if !strings.Contains(page, ".sheet button:disabled") {
		t.Error("page.css did not reach the assembled page")
	}
	if !strings.Contains(page, "function openSheet(") {
		t.Error("sheetshell.js did not reach the assembled page")
	}
	// One distinctive line from each of the other scripts, because nine markers is nine chances for one of them to
	// be missing — and a missing feature is silent: the page renders, the button just does nothing.
	for _, want := range []struct{ needle, file string }{
		{"function initAdminTool(", "main.js"},
		{"function initContactSheet(", "contactsheet.js"},
		{"function initAlbumAction(", "albumaction.js"},
		{"function initPositionAction(", "positionaction.js"},
		{"function initPatrolAction(", "patrolaction.js"},
		{"function initCreditAction(", "creditaction.js"},
		{"function initDeleteAction(", "deleteaction.js"},
		{"function initUpload(", "upload.js"},
	} {
		if !strings.Contains(page, want.needle) {
			t.Errorf("%s did not reach the assembled page", want.file)
		}
	}
	// The document still closes properly — splicing into the wrong place would produce a page that renders as
	// text, which every other guard in this package would sail straight past.
	if !strings.HasSuffix(strings.TrimSpace(page), "{{end}}") {
		t.Error("the assembled template no longer ends with its define block")
	}
}

// A missing asset or marker fails at **init**, not at the first request.
//
// The whole point of panicking in `mustInjectAdminAssets` is that a broken page cannot be served: a binary that
// refuses to start is a deploy that fails, while a page silently missing its stylesheet is a curator wondering
// why the tool looks broken on the Tuesday after the event.
func TestAMissingAdminAssetPanics(t *testing.T) {
	for _, c := range []struct{ name, html, css, js string }{
		{"missing html", "adminui/nope.html", "adminui/page.css", "adminui/page.js"},
		{"missing css", "adminui/page.html", "adminui/nope.css", "adminui/page.js"},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("want a panic: an unassemblable page must stop the binary, not reach a curator")
				}
			}()
			_ = mustInjectAdminAssets(c.html, c.css, c.js)
		})
	}

	// A marker that does not match is the subtler failure, and the one a rename would cause: the file exists, the
	// splice silently does nothing, and the page loses its styling. Provoked by asking for a CSS file whose
	// marker name differs from the one page.html carries.
	t.Run("marker mismatch", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("want a panic when the marker does not match the asset's filename")
			}
		}()
		_ = mustInjectAdminAssets("adminui/page.html", "adminui/album.css", "adminui/page.js")
	})
}

// The Go handlers no longer carry markup.
//
// The point of the extraction, stated as a rule so it does not creep back one convenient `<div>` at a time —
// which is exactly how 1,700 lines accumulated in the first place.
func TestTheAdminHandlersCarryNoMarkup(t *testing.T) {
	for _, file := range []string{"adminpage.go", "adminalbumpage.go"} {
		// **Comments stripped first.** These doc comments necessarily quote the tags being forbidden — the
		// paragraph about `html/template` escaping actions inside `<script>` is the whole reason the rule
		// exists. The first draft of this test failed against that paragraph, which is the same trap task 386's
		// `foldBody` records: a guard that searches for forbidden text will find it in the prose forbidding it.
		var code []string
		for _, line := range strings.Split(adminSource(t, file), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			code = append(code, line)
		}
		src := strings.Join(code, "\n")

		for _, forbidden := range []string{"<!doctype", "<style>", "<script>", "<div"} {
			if strings.Contains(src, forbidden) {
				t.Errorf("%s contains %q. The markup, CSS and JavaScript live in adminui/ — putting any of it "+
					"back here reopens the backtick trap (task 394)", file, forbidden)
			}
		}
	}
}

// **The vendored libraries match the versions they claim** (task 395).
//
// # Why a test and not just a README
//
// Updating a vendored library is three curl commands and two files to edit, so the failure mode is obvious: the
// bytes change and the table does not, or the reverse. Then `vendor/README.md` describes a version nobody is
// running, and the next person to debug something reads a lie.
//
// Every one of the three announces its own version in its own bytes, so this is checkable without a network or
// a lockfile. **The committed file is the lockfile** — that is the point of vendoring — and this is what keeps
// its label attached.
func TestTheVendoredAssetsMatchTheirPinnedVersions(t *testing.T) {
	pinned := adminAsset(t, "vendor/vendor.txt")

	// How each library states its own version. Not a shared pattern: they differ, and pretending otherwise
	// would mean a loose regex that matches some other number in a minified bundle.
	announces := map[string]func(version string) string{
		"htmx.min.js":   func(v string) string { return `version:"` + v + `"` },
		"alpine.min.js": func(v string) string { return `version:"` + v + `"` },
		"pico.min.css":  func(v string) string { return "v" + v },
	}

	seen := 0
	for _, line := range strings.Split(pinned, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			t.Errorf("vendor.txt line is not file<TAB>version<TAB>url: %q", line)
			continue
		}
		name, version, url := fields[0], fields[1], fields[2]
		seen++

		needle, known := announces[name]
		if !known {
			t.Errorf("vendor.txt pins %s, but this test does not know how that library states its version. "+
				"Add it to `announces` rather than dropping the check", name)
			continue
		}
		if got := needle(version); !strings.Contains(adminAsset(t, "vendor/"+name), got) {
			t.Errorf("%s does not announce %q. vendor.txt claims %s — either the file was re-downloaded at a "+
				"different version, or the table was edited without re-downloading", name, got, version)
		}
		// The URL must name the same version, so a copy-paste that updated one number and not the other fails.
		if !strings.Contains(url, version) {
			t.Errorf("%s: the pinned URL %q does not contain version %s", name, url, version)
		}
		// And the README's table must agree, since that is the file a human reads. Read from disk rather than
		// the embed set: it is a repo document, not a served asset, so there is no reason to ship it in the
		// binary.
		if readme := adminSource(t, "adminui/vendor/README.md"); !strings.Contains(readme, "**"+version+"**") {
			t.Errorf("README.md does not list version %s for %s", version, name)
		}
	}

	if seen != len(announces) {
		t.Errorf("vendor.txt pins %d libraries, %d are embedded and served — a file served but unpinned is a "+
			"dependency nobody is tracking", seen, len(announces))
	}
}

// The vendor route serves each pinned library, and **nothing else**.
//
// The asset name comes from the URL. A handler that joined it to a directory would be one encoded `../` away
// from serving this service's templates out of the embedded filesystem; matching against a fixed map cannot
// express a path that is not in the map. Asserted because "we look it up in a map" is exactly the kind of
// detail a later refactor tidies into a `filepath.Join`.
func TestTheAdminVendorRouteServesOnlyThePinnedAssets(t *testing.T) {
	_, srv := adminApp(t)

	for name, wantType := range map[string]string{
		"htmx.min.js":   "application/javascript",
		"alpine.min.js": "application/javascript",
		"pico.min.css":  "text/css",
	} {
		resp := getAdmin(t, srv, "/admin/vendor/"+name, testAdminUser, testAdminPass)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: want 200, got %d", name, resp.StatusCode)
			continue
		}
		if got := resp.Header.Get("Content-Type"); !strings.Contains(got, wantType) {
			t.Errorf("%s: want %s, got %q", name, wantType, got)
		}
		if len(body) < 10_000 {
			t.Errorf("%s served %d bytes, which is too small to be the real library", name, len(body))
		}
		// Still `no-store`, like every other response on this surface. The libraries are public, but the rule
		// is the rule — see the handler's doc for why no exception was made.
		if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "no-store") {
			t.Errorf("%s: admin responses carry no-store, got %q", name, got)
		}
	}

	// Anything not pinned is a 404, including the traversal shapes.
	for _, name := range []string{
		"vendor.txt", "README.md", "page.js", "nope.js",
		"..%2fpage.js", "..%2f..%2fadminpage.go",
	} {
		resp := getAdmin(t, srv, "/admin/vendor/"+name, testAdminUser, testAdminPass)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("/admin/vendor/%s answered 200; only the pinned libraries may be served", name)
		}
		if strings.Contains(string(body), "package main") || strings.Contains(string(body), "openSheet") {
			t.Errorf("/admin/vendor/%s leaked embedded source", name)
		}
	}
}

// And the route is behind the credential, like the rest of the surface.
func TestTheAdminVendorRouteNeedsTheCredential(t *testing.T) {
	_, srv := adminApp(t)

	resp := getAdmin(t, srv, "/admin/vendor/htmx.min.js", "", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401 with no credential, got %d", resp.StatusCode)
	}
}

func TestAdminPageServesWithTheCredential(t *testing.T) {
	_, srv := adminApp(t)

	resp := getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 with the credential, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("want HTML, got %q", ct)
	}
	// And it is the tool rather than the SPA shell, which also answers 200 for unmatched paths.
	if body := adminBody(t, resp); !strings.Contains(body, adminPageMarker) {
		t.Errorf("want the admin tool, got something else:\n%s", body)
	}
}

// The year is the one thing on the page a curator cannot fix by editing: a photograph uploaded into the wrong
// year is in the wrong event, not mistyped. PRD 022 §5 and §6 both require it to be unmistakable.
func TestAdminPageStatesTheYear(t *testing.T) {
	_, srv := adminApp(t)

	resp := getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass)
	body := adminBody(t, resp)
	if !strings.Contains(body, "2026") {
		t.Errorf("the admin page must state the event year it writes to\ngot: %s", body)
	}
}

// # The core refusal, and the thing it must not leak
//
// One response for a missing credential, a wrong username and a wrong password. For a shared credential the
// username is half the secret, so an error that distinguished them would give away the easier half.
func TestAdminRefusesAndRevealsNothing(t *testing.T) {
	_, srv := adminApp(t)

	cases := map[string][2]string{
		"no credential":    {"", ""},
		"wrong user":       {"nobody", testAdminPass},
		"wrong password":   {testAdminUser, "guess"},
		"both wrong":       {"nobody", "guess"},
		"empty password":   {testAdminUser, ""},
		"empty user":       {"", testAdminPass},
		"password as user": {testAdminPass, testAdminUser},
	}

	var bodies []string
	for name, c := range cases {
		resp := getAdmin(t, srv, "/2026/photos", c[0], c[1])
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: want 401, got %d", name, resp.StatusCode)
		}
		if ch := resp.Header.Get("WWW-Authenticate"); !strings.HasPrefix(ch, "Basic ") {
			t.Errorf("%s: want a Basic challenge so the browser prompts, got %q", name, ch)
		}
		bodies = append(bodies, adminBody(t, resp))
	}

	// Every refusal is byte-identical. This is the assertion that keeps the endpoint from becoming a
	// username oracle, and it is cheap to break by adding a helpful message.
	for i, b := range bodies[1:] {
		if b != bodies[0] {
			t.Errorf("refusal bodies differ, so they distinguish a wrong user from a wrong password:\n%q\nvs\n%q",
				bodies[0], bodies[i+1])
		}
	}
}

// **The most important property in this file.** The credential is not a session.
//
// `requireAuth` is the only place a session enters a request context — routes.go states this as a security
// property and several tests enforce it. If `requireAdmin` set one, the shared password would become a way to
// authenticate *as somebody*, and since there is no person behind it the only way to do that would be to
// invent one.
//
// Asserted from inside a handler, because that is where it would matter.
func TestAdminCredentialIsNotASession(t *testing.T) {
	app := newTestApp(t)
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass

	var sawSession bool
	probe := app.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		_, sawSession = contextGetSession(r)
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(probe)
	defer srv.Close()

	resp := getAdmin(t, srv, "/", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want the handler to run, got %d", resp.StatusCode)
	}
	if sawSession {
		t.Fatal("requireAdmin put a session on the request context: the admin credential must grant exactly " +
			"this tool and must never authenticate as a person")
	}
}

// Both header sets must be present on **every** response from the surface, including the ones that never reach
// a handler. A cached contact sheet on a shared laptop is a leak whatever the status code was.
func TestAdminResponsesAreNeverStoredOrIndexed(t *testing.T) {
	_, srv := adminApp(t)

	for name, creds := range map[string][2]string{
		"authorised": {testAdminUser, testAdminPass},
		"refused":    {"nobody", "guess"},
	} {
		resp := getAdmin(t, srv, "/2026/photos", creds[0], creds[1])
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: want Cache-Control no-store, got %q", name, got)
		}
		if got := resp.Header.Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
			t.Errorf("%s: want X-Robots-Tag noindex, got %q", name, got)
		}
	}
}

// A basic-auth credential travels on every request, so plain HTTP means the password in cleartext repeatedly.
// Refused outside development — and refused *before* the credential is examined, since the credential is in
// the very request being rejected.
func TestAdminRefusesPlainHTTP(t *testing.T) {
	app := newTestApp(t) // env: "testing", so the development exemption does not apply
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// No X-Forwarded-Proto: this is what a request that reached us over cleartext looks like.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+app.publicRoot()+"/photos", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET the photos page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("want 421 for a cleartext admin request, got %d", resp.StatusCode)
	}
	// And the refusal itself must not be cacheable.
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("want no-store on the refusal too, got %q", got)
	}
}

// Development is exempt, because the dev stack is plain HTTP and refusing there would make the tool
// undevelopable. Asserted so the exemption is a known hole rather than an accident.
func TestAdminAllowsPlainHTTPInDevelopment(t *testing.T) {
	app := newTestApp(t)
	app.config.env = envDevelopment
	app.config.eventYear = "2026"
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+app.publicRoot()+"/photos", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET the photos page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("development must serve over plain HTTP, got %d", resp.StatusCode)
	}
}

// **The admin surface does not throttle authenticated requests** (task 388).
//
// This is the regression test for the bug that made the tool unusable, and it is written as the shape of the
// real workload rather than as "the limiter is gone": the contact sheet fetches one thumbnail per photograph,
// so a single page load of a full library is well over a hundred requests through `requireAdmin`, and a
// hand-in is three hundred more. Any per-request ceiling on this surface — a limiter, a semaphore, a
// well-meant "are you sure you are not a script" — breaks PRD 022 §9's success condition, and it breaks it
// silently after the feature looked fine in a test with three photographs.
//
// 400 rather than a token number, because the failure it guards against was **30**. A count that could be
// mistaken for a plausible limit would not have caught it.
func TestTheAdminSurfaceDoesNotThrottleAuthenticatedRequests(t *testing.T) {
	_, srv := adminApp(t)

	for i := 0; i < 400; i++ {
		resp := getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d with the correct credential answered %d. The curator's work must not be "+
				"rationed: one contact sheet is one request per photograph, and a hand-in is three hundred "+
				"more (PRD 022 §9, task 388)", i+1, resp.StatusCode)
		}
	}
}

// A wrong credential is refused every time, and refused the same way.
//
// Without a limiter there is nothing to exhaust, so the interesting property is that repetition changes
// nothing: no lockout, no different status, no leak of which half was wrong. The password's entropy is the
// only control on this surface now (task 388), which is recorded at `requireAdmin` and at
// `config.adminPassword` rather than implied here.
func TestAWrongAdminCredentialIsAlwaysRefusedIdentically(t *testing.T) {
	_, srv := adminApp(t)

	var first string
	for i, c := range [][2]string{
		{"nobody", "guess"},
		{"nobody", "guess"},
		{testAdminUser, "wrong-password"},
		{"wrong-user", testAdminPass},
	} {
		resp := getAdmin(t, srv, "/2026/photos", c[0], c[1])
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i+1, resp.StatusCode)
		}
		if got := resp.Header.Get("WWW-Authenticate"); !strings.Contains(got, "Basic realm=") {
			t.Errorf("attempt %d: want a challenge so the browser re-prompts, got %q", i+1, got)
		}
		// One answer for a wrong username and a wrong password, or the response confirms the username — which
		// for a shared credential is half the secret.
		if i == 0 {
			first = string(body)
		} else if string(body) != first {
			t.Errorf("attempt %d answered differently; a wrong username and a wrong password must be "+
				"indistinguishable\nfirst: %q\nthis:  %q", i+1, first, body)
		}
	}
}

// A comparison that short-circuits on the username leaks "that username exists" through timing. Asserted
// structurally — timing itself is not reliably testable — by requiring both comparisons to be combined
// bitwise rather than with `&&`.
func TestAdminComparisonDoesNotShortCircuit(t *testing.T) {
	src := adminSource(t, "middleware.go")

	if strings.Contains(src, "userOK && passOK") || strings.Contains(src, "passOK && userOK") {
		t.Error("the credential comparison short-circuits on the username, which is a timing oracle for " +
			"'that user exists'. Combine the two with a bitwise & instead.")
	}
	if !strings.Contains(src, "subtle.ConstantTimeCompare") {
		t.Error("the credential comparison must be constant-time")
	}
	// Hashing both sides first is what removes the length leak: ConstantTimeCompare returns early for
	// different lengths, so comparing raw strings discloses how long the configured password is.
	if !strings.Contains(src, "sha256.Sum256") {
		t.Error("both sides must be hashed to a fixed length before comparison, or the password's length " +
			"leaks through timing")
	}
}

// An unset password must refuse rather than match an empty presented one. This should be unreachable — the
// routes are not registered without a password — and is checked because "unreachable" is a property of
// today's routes() and `adminCredentialOK` outlives it.
func TestAdminCredentialRefusesWhenUnconfigured(t *testing.T) {
	app := newTestApp(t)
	app.config.adminUser = ""
	app.config.adminPassword = ""

	for name, c := range map[string][2]string{
		"both empty":     {"", ""},
		"empty password": {"foto", ""},
	} {
		if app.adminCredentialOK(c[0], c[1]) {
			t.Errorf("%s: an unconfigured credential must never match", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Task 370: absent, not open.
// ---------------------------------------------------------------------------

// With no password, the routes do not exist. Not 401, not 403 — **404**, because no handler was registered.
//
// The distinction matters operationally: a misconfigured deploy must not publish an anonymous bulk-upload
// endpoint on the public internet, and "absent" is the only state that cannot be talked into serving.
func TestAdminSurfaceIsAbsentWithoutAPassword(t *testing.T) {
	app := newTestApp(t)
	app.config.adminUser = ""
	app.config.adminPassword = ""

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Every admin path, and every way somebody might try to reach one.
	for _, path := range adminPathsFromRoutes(t) {
		probe := strings.ReplaceAll(path, ":slug", "noget")
		probe = strings.ReplaceAll(probe, ":albumId", "a1")
		probe = strings.ReplaceAll(probe, ":photoId", strings.Repeat("a", 64))

		for _, attempt := range []struct {
			name string
			with func(*http.Request)
		}{
			{"anonymous", func(*http.Request) {}},
			{"with the credential a configured deploy would use", func(r *http.Request) {
				r.SetBasicAuth(testAdminUser, testAdminPass)
			}},
			{"with an empty credential", func(r *http.Request) { r.SetBasicAuth("", "") }},
		} {
			req, _ := http.NewRequest(http.MethodGet, srv.URL+probe, nil)
			req.Header.Set("X-Forwarded-Proto", "https")
			attempt.with(req)

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("GET %s: %v", probe, err)
			}
			body := adminBody(t, resp)
			_ = resp.Body.Close()

			// The decisive assertion: whatever answered, it was not the tool. Checked on content rather than
			// status because an unmatched non-API path falls through to the SPA shell with a 200.
			if strings.Contains(body, adminPageMarker) {
				t.Errorf("%s: %s served the admin tool with no password configured", attempt.name, probe)
			}
			// And it must not have reached the middleware at all: a 401 or a 429 would mean the route was
			// registered and merely refused, which is "open but guarded" rather than "absent".
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests {
				t.Errorf("%s: %s reached the admin middleware with no password configured (%d); "+
					"the routes must not be registered at all", attempt.name, probe, resp.StatusCode)
			}
		}
	}
}

// However the path is spelled, the credential is still required.
//
// # What this found, and why the test changed shape
//
// It was written to assert that `/Admin` does not serve the tool — and it failed, because httprouter enables
// `RedirectFixedPath` by default: a path differing only in case gets a 301 to the canonical one, which the Go
// client then follows. So the path *is* effectively case-insensitive.
//
// That is not a hole, and asserting case-sensitivity would have been asserting an accident of the router
// rather than a property of the design: the redirect lands on the canonical path, which is behind `requireAdmin` like
// any other request to it. The guard is on the handler, not on the spelling.
//
// So the test now pins the property that actually matters, and which *would* be a hole if it broke: no
// spelling of the path reaches the tool without the credential.
func TestNoSpellingOfTheAdminPathSkipsTheCredential(t *testing.T) {
	_, srv := adminApp(t)

	for _, path := range []string{"/2026/photos", "/2026/Photos", "/2026/PHOTOS", "/2026/pHoToS", "/2026/photos/", "//2026/photos"} {
		// No credential at all.
		resp := getAdmin(t, srv, path, "", "")
		body := adminBody(t, resp)

		if strings.Contains(body, adminPageMarker) {
			t.Errorf("%s served the admin tool without a credential", path)
		}
		// And with a wrong one.
		resp = getAdmin(t, srv, path, "nobody", "guess")
		if strings.Contains(adminBody(t, resp), adminPageMarker) {
			t.Errorf("%s served the admin tool with a wrong credential", path)
		}
	}
}

// adminPageMarker is a string the admin tool renders and nothing else does.
//
// Used to tell "the admin tool answered" from "something answered 200" — which matters because the SPA
// fallback answers 200 for any unmatched non-API path.
const adminPageMarker = "Billedarkiv"

// There must be no default, fallback or dev-only admin password anywhere in the tree. A default is the one
// value of the configuration that produces an unprotected upload endpoint.
func TestThereIsNoDefaultAdminPassword(t *testing.T) {
	src := adminSource(t, "env.go")

	// The flag registration must default to the empty string for both fields.
	for _, want := range []string{
		`envStr("ADMIN_USER", "")`,
		`envStr("ADMIN_PASSWORD", "")`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("env.go must register %s — a default admin credential is how a forgotten config "+
				"becomes an anonymous write path into the blob store", want)
		}
	}

	// And nothing anywhere may assign one.
	for _, file := range []string{"admin.go", "middleware.go", "adminpage.go", "routes.go"} {
		body := adminSource(t, file)
		for _, smell := range []string{
			`adminPassword = "`, `adminPassword: "`, `adminUser = "`, `adminUser: "`,
		} {
			if strings.Contains(body, smell) {
				t.Errorf("%s assigns an admin credential (%q); there must be no default in any environment",
					file, smell)
			}
		}
	}
}

// `PUBLIC_ALBUMS=false` hides the public section; it must not disable the curator's tool. Two switches, two
// questions — a curator has to be able to assemble albums *before* the public section is switched on
// (PRD 022 §6).
func TestPublicAlbumsFlagDoesNotDisableTheAdminTool(t *testing.T) {
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.config.publicAlbums = false
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("the admin tool must work with PUBLIC_ALBUMS=false, got %d", resp.StatusCode)
	}

	// And the public side must still be hidden, so this test cannot pass by the flag having stopped working.
	pub, err := srv.Client().Get(srv.URL + "/2026/album/noget")
	if err != nil {
		t.Fatalf("GET the public album page: %v", err)
	}
	defer pub.Body.Close()
	if pub.StatusCode != http.StatusNotFound {
		t.Errorf("PUBLIC_ALBUMS=false must still hide the public album page, got %d", pub.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// The route table: the two wrappers must never mix.
// ---------------------------------------------------------------------------

// allRegisteredRoutes parses routes.go and returns every registration, unscoped.
//
// Reuses the AST helpers the OpenAPI guard already has, but deliberately *not* its `isInScope` filter: the
// point here is to see the whole table, including the routes that guard is not yet responsible for.
func allRegisteredRoutes(t *testing.T) []registeredRoute {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse routes.go: %v", err)
	}

	var out []registeredRoute
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandlerFunc" || len(call.Args) < 3 {
			return true
		}
		path, ok := parseRegisteredPath(t, call.Args[1], fset)
		if !ok {
			return true
		}
		out = append(out, registeredRoute{
			method:        httpMethodName(call.Args[0]),
			path:          path,
			handler:       handlerName(call.Args[2]),
			line:          fset.Position(call.Pos()).Line,
			authenticated: wrapsRequireAuth(call.Args[2]),
			admin:         wrapsRequireAdmin(call.Args[2]),
		})
		return true
	})

	if len(out) == 0 {
		t.Fatal("no routes found — did routes.go move, or the registration style change?")
	}
	return out
}

// isAdminNamespace reports whether a path is in a namespace that belongs to the admin surface alone.
//
// Every route here must be behind `requireAdmin`. The converse no longer holds: since task 396 the curator's
// pages sit under the public year prefix (`/2026/photos`), so being admin is decided by the wrapper and this
// predicate only catches a route added to an admin namespace without it.
func isAdminNamespace(path string) bool {
	return path == "/admin" || strings.HasPrefix(path, "/admin/") ||
		strings.HasPrefix(path, "/api/admin")
}

// adminPathsFromRoutes returns every admin path in the table, read from source.
//
// From source rather than a literal list, which is the point of criterion "a test fails if a new admin route
// is added outside the conditional block": a route added to the table is automatically covered by the
// absence tests above.
func adminPathsFromRoutes(t *testing.T) []string {
	t.Helper()

	var out []string
	for _, r := range allRegisteredRoutes(t) {
		if r.admin {
			out = append(out, r.path)
		}
	}
	if len(out) == 0 {
		t.Fatal("no admin routes found in routes.go; this guard would pass vacuously")
	}
	return out
}

// **The two wrappers must never mix.**
//
// `requireAuth(requireAdmin(h))` would let a participant's session reach the curator's surface;
// `requireAdmin(requireAuth(h))` would let the shared password reach a participant's data. Either is a
// privilege confusion that reads as harmless in a diff.
//
// And the credential must grant only the curator's tool: an admin route is either in an admin namespace or one
// of the curator pages under the year prefix, never anywhere else.
func TestAdminRoutesUseOnlyTheAdminWrapper(t *testing.T) {
	for _, r := range allRegisteredRoutes(t) {
		if isAdminNamespace(r.path) && !r.admin {
			t.Errorf("routes.go:%d %s %s is on the admin surface but is not behind requireAdmin",
				r.line, r.method, r.path)
		}
		if !r.admin {
			continue
		}
		if r.authenticated {
			t.Errorf("routes.go:%d %s %s is behind requireAuth as well as the admin credential; "+
				"the two must not mix (a participant's session must not reach the curator's tool)",
				r.line, r.method, r.path)
		}
		if !isAdminNamespace(r.path) && !looksLikeYearPrefix(r.path) {
			t.Errorf("routes.go:%d %s %s is behind requireAdmin outside the admin namespaces and the year's "+
				"curator pages; the shared credential must grant exactly the admin tool and nothing else",
				r.line, r.method, r.path)
		}
	}
}

// Every admin route must be inside the `adminRoutesEnabled` block. One registered outside it would be served
// with no password configured — the exact failure task 370 exists to prevent — and the absence test above
// would not catch it, because it would then be testing a route that is *supposed* to answer.
func TestEveryAdminRouteIsInsideTheConditionalBlock(t *testing.T) {
	src := adminSource(t, "routes.go")

	start := strings.Index(src, "if adminRoutesEnabled(app.config) {")
	if start < 0 {
		t.Fatal("routes.go no longer has an adminRoutesEnabled block")
	}
	// The block ends at the first line that closes it at the function's indentation level.
	end := strings.Index(src[start:], "\n\t}")
	if end < 0 {
		t.Fatal("could not find the end of the adminRoutesEnabled block")
	}
	// By line rather than by searching the block for the path's literal: the curator pages are registered as
	// `publicRoot + "/photos"`, which has no literal to find.
	first := strings.Count(src[:start], "\n") + 1
	last := strings.Count(src[:start+end], "\n") + 1

	for _, r := range allRegisteredRoutes(t) {
		if r.admin && (r.line < first || r.line > last) {
			t.Errorf("routes.go:%d %s is registered outside the adminRoutesEnabled block, so it would be served "+
				"with no password configured", r.line, r.path)
		}
	}
}

// ---------------------------------------------------------------------------
// The split into per-feature scripts (step 7 of task 395).
// ---------------------------------------------------------------------------

// **Every `ctx` member a feature uses is provided by some other feature.**
//
// # Why this test exists
//
// page.js was 1,436 lines in one closure, and the features inside it shared state by simply being in the same
// scope. Splitting it means that sharing becomes an explicit object, and it introduces exactly one new class of
// bug: a feature reading `ctx.somethingNobodyProvides`. JavaScript gives no compiler to catch it, and the symptom
// is silent — the page renders, and one button does nothing.
//
// So this is the compiler. It reads the served scripts, collects every `ctx.x` used and every `ctx.x =` assigned,
// and fails on the difference. It is the reason the split is safe to make at all.
//
// `main.js` builds the context by assignment rather than as an object literal precisely so this can be exact.
func TestEveryAdminContextMemberIsProvided(t *testing.T) {
	// `ctx.x` anywhere, and `ctx.x =` (but not `==`) as a provider.
	used := regexp.MustCompile(`\bctx\.([A-Za-z_$][\w$]*)`)
	provided := regexp.MustCompile(`\bctx\.([A-Za-z_$][\w$]*)\s*=[^=]`)

	uses := map[string][]string{} // member -> files that read it
	gives := map[string]string{}  // member -> file that provides it

	for _, path := range adminPageScripts {
		name := strings.TrimPrefix(path, "adminui/")
		// Comments are stripped first. Prose about `ctx.openSheet` is not a use of it, and a needle matching the
		// comment that explains a rule rather than the code implementing it is a mistake this package has made
		// three times.
		src := stripJSLineComments(mustReadAdminAsset(path))

		for _, m := range provided.FindAllStringSubmatch(src, -1) {
			gives[m[1]] = name
		}
		for _, m := range used.FindAllStringSubmatch(src, -1) {
			uses[m[1]] = append(uses[m[1]], name)
		}
	}

	if len(uses) == 0 {
		t.Fatal("found no ctx members at all, so this test is checking nothing — has the wiring changed shape?")
	}

	for member, readers := range uses {
		if _, ok := gives[member]; !ok {
			t.Errorf("ctx.%s is read by %v and provided by nobody: that is a button that silently does nothing",
				member, unique(readers))
		}
	}

	// And the other direction, which is not a bug but is worth knowing: a member nobody reads is shared state
	// with no sharer, and the point of writing the surface down was to keep it small.
	for member, giver := range gives {
		if _, ok := uses[member]; !ok {
			t.Errorf("ctx.%s is provided by %s and read by nobody; the shared surface should only hold what is "+
				"actually shared", member, giver)
		}
	}
}

// stripJSLineComments removes `//` comments, leaving the code.
//
// Deliberately naive about `//` inside a string literal: there is none in these files, and a real tokeniser would
// be more machinery than the one property it protects is worth. If one ever appears, this over-strips that line
// and the test can only become *stricter*, never quieter — which is the safe direction for a guard to fail in.
func stripJSLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func unique(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// **Each script is a single function declaration, which is what makes it a real file.**
//
// The alternative arrangements were both worse and are worth naming so neither is drifted back into:
//
//   - **ES modules.** The same split with real imports and no build step, rejected because every asset on this
//     surface answers `no-store` (task 371) — nine module files would be nine uncacheable requests per page load
//     where the injection is zero, and the person waiting is a photographer on a hotel connection.
//   - **Splitting mid-IIFE**, so one file opens a closure another closes. That is what "real files" was supposed
//     to end (task 394): such a file is not a program, and no formatter or linter can read it.
//
// A single `function init…(ctx)` declaration is a complete valid program *and* costs one request. This asserts
// each file really is one, because the temptation under time pressure is to drop a bare statement at the bottom.
func TestEachAdminScriptIsOneFunctionDeclaration(t *testing.T) {
	for _, path := range adminPageScripts {
		name := strings.TrimPrefix(path, "adminui/")
		src := stripJSLineComments(mustReadAdminAsset(path))

		// The top-level statements are the lines with no leading whitespace.
		var top []string
		for _, line := range strings.Split(src, "\n") {
			if line == "" || line[0] == ' ' || line[0] == '\t' {
				continue
			}
			top = append(top, strings.TrimRight(line, " \t"))
		}

		// `function x(ctx) {` … `}`, and nothing else — except main.js, which must also *call* itself, since it is
		// the entry point and something has to start the tool.
		want := []string{"function ", "}"}
		if name == "main.js" {
			want = append(want, "initAdminTool();")
		}
		if len(top) != len(want) {
			t.Errorf("%s has %d top-level statements, want %d: %q", name, len(top), len(want), top)
			continue
		}
		if !strings.HasPrefix(top[0], "function init") || !strings.HasSuffix(top[0], "(ctx) {") {
			if name != "main.js" {
				t.Errorf("%s must be one `function init…(ctx) {` declaration, got %q", name, top[0])
			}
		}
	}
}
