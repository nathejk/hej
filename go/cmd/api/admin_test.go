package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"nathejk.dk/internal/ratelimit"
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
	// Generous, so no test trips a limit it is not testing. The tests that DO test the limit build their own.
	app.adminAuthLimiter = ratelimit.New(1000, time.Minute)

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

func TestAdminPageServesWithTheCredential(t *testing.T) {
	_, srv := adminApp(t)

	resp := getAdmin(t, srv, "/admin", testAdminUser, testAdminPass)
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

	resp := getAdmin(t, srv, "/admin", testAdminUser, testAdminPass)
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
		resp := getAdmin(t, srv, "/admin", c[0], c[1])
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
	app.adminAuthLimiter = ratelimit.New(1000, time.Minute)

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
		resp := getAdmin(t, srv, "/admin", creds[0], creds[1])
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
	app.adminAuthLimiter = ratelimit.New(1000, time.Minute)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// No X-Forwarded-Proto: this is what a request that reached us over cleartext looks like.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /admin: %v", err)
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
	app.adminAuthLimiter = ratelimit.New(1000, time.Minute)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin", nil)
	req.SetBasicAuth(testAdminUser, testAdminPass)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /admin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("development must serve over plain HTTP, got %d", resp.StatusCode)
	}
}

// Guessing is rate limited by IP, and **every** attempt counts — not only the failures.
//
// A limiter that exempted successes would let an attacker who guessed correctly continue unthrottled; one
// counting only failures still has to run the comparison to find out which it was.
func TestAdminCredentialAttemptsAreRateLimited(t *testing.T) {
	app := newTestApp(t)
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass
	app.adminAuthLimiter = ratelimit.New(3, time.Minute)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for i := 0; i < 3; i++ {
		resp := getAdmin(t, srv, "/admin", "nobody", "guess")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401 while within the limit, got %d", i+1, resp.StatusCode)
		}
	}

	resp := getAdmin(t, srv, "/admin", "nobody", "guess")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("want 429 once the limit is reached, got %d", resp.StatusCode)
	}

	// And the correct credential is throttled too, which is the documented tradeoff rather than an oversight:
	// the limiter is in front of the comparison, so it cannot know the credential was right without doing the
	// work it exists to ration. `allowAdminAttempt` records why that is the safer direction.
	resp = getAdmin(t, srv, "/admin", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("the limiter sits in front of the comparison, so a correct credential is also throttled; "+
			"got %d — if this changed deliberately, update allowAdminAttempt's doc", resp.StatusCode)
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
// rather than a property of the design: the redirect lands on `/admin`, which is behind `requireAdmin` like
// any other request to it. The guard is on the handler, not on the spelling.
//
// So the test now pins the property that actually matters, and which *would* be a hole if it broke: no
// spelling of the path reaches the tool without the credential.
func TestNoSpellingOfTheAdminPathSkipsTheCredential(t *testing.T) {
	_, srv := adminApp(t)

	for _, path := range []string{"/admin", "/Admin", "/ADMIN", "/aDmIn", "/admin/", "//admin"} {
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
	app.adminAuthLimiter = ratelimit.New(1000, time.Minute)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := getAdmin(t, srv, "/admin", testAdminUser, testAdminPass)
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
		})
		return true
	})

	if len(out) == 0 {
		t.Fatal("no routes found — did routes.go move, or the registration style change?")
	}
	return out
}

// wrapsRequireAdmin reports whether a registration puts the handler behind the admin credential.
func wrapsRequireAdmin(t *testing.T, path string) bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse routes.go: %v", err)
	}

	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandlerFunc" || len(call.Args) < 3 {
			return true
		}
		p, ok := parseRegisteredPath(t, call.Args[1], fset)
		if !ok || p != path {
			return true
		}
		ast.Inspect(call.Args[2], func(m ast.Node) bool {
			if s, ok := m.(*ast.SelectorExpr); ok && s.Sel.Name == "requireAdmin" {
				found = true
			}
			return true
		})
		return true
	})
	return found
}

// isAdminPath reports whether a route belongs to the admin surface.
func isAdminPath(path string) bool {
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
		if isAdminPath(r.path) {
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
func TestAdminRoutesUseOnlyTheAdminWrapper(t *testing.T) {
	for _, r := range allRegisteredRoutes(t) {
		switch {
		case isAdminPath(r.path):
			if !wrapsRequireAdmin(t, r.path) {
				t.Errorf("routes.go:%d %s %s is on the admin surface but is not behind requireAdmin",
					r.line, r.method, r.path)
			}
			if r.authenticated {
				t.Errorf("routes.go:%d %s %s is behind requireAuth as well as the admin credential; "+
					"the two must not mix (a participant's session must not reach the curator's tool)",
					r.line, r.method, r.path)
			}
		default:
			if wrapsRequireAdmin(t, r.path) {
				t.Errorf("routes.go:%d %s %s is not an admin path but is behind requireAdmin; "+
					"the shared credential must grant exactly the admin tool and nothing else",
					r.line, r.method, r.path)
			}
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
	block := src[start : start+end]

	for _, path := range adminPathsFromRoutes(t) {
		if !strings.Contains(block, `"`+path+`"`) {
			t.Errorf("%s is registered outside the adminRoutesEnabled block, so it would be served with no "+
				"password configured", path)
		}
	}
}
