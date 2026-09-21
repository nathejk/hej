package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every Glimt endpoint carries a complete OpenAPI annotation block (`.rules`, task 312).
//
// # Why this is a test and not a checklist
//
// The annotations were written as each handler landed, so a one-off sweep would have found almost
// nothing. What the task actually asks for is the last criterion — *"no endpoint added by PRD 019 is
// missing annotations"* — and that is a statement about the future, which only a test can hold.
//
// swaggo annotations are **comments**: nothing compiles them, nothing runs them, and a handler
// registered without them behaves identically. That makes them the one kind of documentation that
// can go stale without a single symptom, and the one worth spending an AST walk on. This reads
// `routes.go` for what is actually registered and cross-checks each handler's doc comment, so adding
// a route is what fails — not remembering to update a list.
//
// # What it deliberately does not do
//
// It is scoped to Glimt (plus the dev fixture, which is an endpoint like any other). Widening it to
// the whole API is a one-line change to `isInScope`, and worth doing — but it would fail on
// pre-existing handlers from other PRDs, and turning this red on work nobody in this task touched is
// how a guard gets commented out. Left as a note rather than done silently.

// The tags PRD 019 §8 and task 312 allow. A typo'd tag scatters an endpoint into its own group in the
// rendered spec, which is invisible until somebody reads it.
var allowedGlimtTags = map[string]bool{
	"glimt":            true,
	"glimt-moderation": true,
	"glimt-public":     true,
	// The public site (PRD 011, task 332). A separate group from `glimt-public` on purpose: that one is
	// the glimt feed's public API, this one is the surrounding site — frontpage, albums, patrol pages.
	// Folding them together would put "here are the photographs somebody shared" and "here is a patrol's
	// route" in one section of the spec, which are different things with different rules.
	"public-site": true,
	"dev":         true,
}

// registeredRoute is one `router.HandlerFunc(...)` call in routes.go.
type registeredRoute struct {
	method  string
	path    string
	handler string
	line    int
	// authenticated is whether the registration wraps the handler in `requireAuth`.
	//
	// Read from the AST rather than assumed, because the public page and its API (task 323) are
	// deliberately **not** wrapped, and that is a security property rather than an omission — see
	// TestPublicGlimtRoutesAreNotBehindAuth.
	authenticated bool
}

// isInScope selects the routes this guard checks.
//
// Glimt (task 312, the guard's original scope) plus **the whole public site** (task 332). The public
// site was brought in immediately rather than waiting for task 328's repo-wide widening, because it is
// the surface where an undocumented failure mode costs most: these routes are unauthenticated, so the
// spec is the only description of them anybody outside this repo will ever read, and a `@Failure` that
// disagrees with the code is how a rate limit or a closed gate becomes a surprise.
//
// Widening this to the rest of the API remains task 328.
// isInScope reports whether a route belongs to the surface this guard covers.
//
// The **year prefix** is in scope since task 351: the public pages moved there, and without it
// `/2026/patrulje/{number}` would quietly leave the annotation check — a predicate matching "contains
// glimt" would keep only the glimt page.
func isInScope(path string) bool {
	return strings.HasPrefix(path, "/api/glimt") || strings.Contains(path, "glimt") ||
		strings.HasPrefix(path, "/api/public/") || looksLikeYearPrefix(path)
}

// glimtRoutes parses routes.go and returns every in-scope registration.
//
// Read from the source rather than from a hand-written list, which is the whole point: a route added
// without annotations must fail this test without anybody remembering to add it here.
func glimtRoutes(t *testing.T) []registeredRoute {
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

		method := httpMethodName(call.Args[0])
		path, ok := parseRegisteredPath(t, call.Args[1], fset)
		if !ok || method == "" || !isInScope(path) {
			return true
		}

		out = append(out, registeredRoute{
			method:        method,
			path:          path,
			handler:       handlerName(call.Args[2]),
			line:          fset.Position(call.Pos()).Line,
			authenticated: wrapsRequireAuth(call.Args[2]),
		})
		return true
	})

	if len(out) == 0 {
		t.Fatal("no glimt routes found — did routes.go move, or the registration style change?")
	}
	return out
}

// httpMethodName turns `http.MethodGet` into "GET".
func httpMethodName(e ast.Expr) string {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return strings.ToUpper(strings.TrimPrefix(sel.Sel.Name, "Method"))
}

// stringLit unquotes a string literal, or resolves the one non-literal path form routes.go uses.
//
// # Why this has to understand more than a literal
//
// The public pages are registered as `publicRoot + "/patrulje/:number"` (task 351), because httprouter cannot
// take a `:year` parameter as a sibling of static segments and the prefix therefore comes from configuration.
// A parser that only accepted literals would quietly stop seeing those routes — and since **both guards that
// read this file exist to catch what nobody remembered to check**, a silent gap in them is worse than no guard
// at all: the privacy walk would report success over an empty list.
//
// So `publicRoot + "…"` resolves to a representative year. The year's value does not matter to either guard
// (one asks what a route leaks, the other whether it is annotated); what matters is that the route is *seen*.
//
// Anything else non-literal returns false, and `parseRegisteredPath` turns that into a test failure rather
// than a shrug.
func stringLit(e ast.Expr) (string, bool) {
	// The bare prefix: `router.HandlerFunc(http.MethodGet, publicRoot, …)` for the frontpage.
	if root, ok := e2PublicRoot(e); ok {
		return root, true
	}
	if bin, ok := e.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		left, lok := e2PublicRoot(bin.X)
		right, rok := stringLit(bin.Y)
		if lok && rok {
			return left + right, true
		}
		return "", false
	}

	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// guardYear is the event year the guards resolve `publicRoot` to.
//
// A constant rather than the app's configured value, because these guards read *source text* and never build
// an application. It only has to be year-shaped: `looksLikeYearPrefix` and `isPublicSurface` both match the
// shape, not the value.
const guardYear = "/2026"

// e2PublicRoot recognises the `publicRoot` identifier from routes.go.
func e2PublicRoot(e ast.Expr) (string, bool) {
	ident, ok := e.(*ast.Ident)
	if !ok || ident.Name != "publicRoot" {
		return "", false
	}
	return guardYear, true
}

// parseRegisteredPath is stringLit with a **loud** failure for a form it does not understand.
//
// The guards' whole value is that a route added later is covered without anybody remembering. A route whose
// path this parser cannot read is therefore not a route to skip — it is a hole in both guards, and the only
// safe response is to stop the build and make somebody teach the parser.
func parseRegisteredPath(t *testing.T, e ast.Expr, fset *token.FileSet) (string, bool) {
	t.Helper()
	if path, ok := stringLit(e); ok {
		return path, true
	}
	t.Errorf("routes.go:%d registers a route whose path this guard cannot parse. Teach stringLit about it "+
		"— an unreadable route is invisible to the privacy walk and to the OpenAPI check, which is exactly "+
		"what they exist to prevent.", fset.Position(e.Pos()).Line)
	return "", false
}

// handlerName digs the handler method out of whatever wraps it.
//
// **Only names ending in `Handler` are recognised**, which makes that suffix a convention this guard
// enforces rather than merely observes: a registration pointing at `app.doSomething` is invisible here, and an
// invisible route is an unannotated one. Task 351 found this out by naming a handler without the suffix.
//
// Registrations look like `app.requireAuth(app.listGlimtHandler)`, sometimes with more than one
// wrapper, so this takes the **innermost** `app.X` selector rather than the first one it meets —
// otherwise every authenticated route would report `requireAuth` and the test would check one
// non-existent doc comment twelve times and pass.
func handlerName(e ast.Expr) string {
	name := ""
	ast.Inspect(e, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "app" {
			return true
		}
		if strings.HasSuffix(sel.Sel.Name, "Handler") {
			name = sel.Sel.Name
		}
		return true
	})
	return name
}

// wrapsRequireAuth reports whether a registration puts the handler behind the auth middleware.
func wrapsRequireAuth(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "requireAuth" {
			found = true
		}
		return true
	})
	return found
}

// packageFiles parses every non-test .go file in this directory.
//
// `os.ReadDir` plus `parser.ParseFile` rather than `parser.ParseDir`, which staticcheck rejects as
// deprecated (SA1019) — and staticcheck is a gate in the dev container's build loop, so using it
// wedges the loop and leaves the API serving a stale binary. Found the hard way.
func packageFiles(t *testing.T) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	var out []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		out = append(out, file)
	}
	if len(out) == 0 {
		t.Fatal("parsed no package files — has the layout changed?")
	}
	return out
}

// handlerDocs maps every `func (app *application) xHandler` in the package to its doc comment.
func handlerDocs(t *testing.T) map[string]string {
	t.Helper()

	docs := map[string]string{}
	for _, file := range packageFiles(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Doc == nil {
				continue
			}
			docs[fn.Name.Name] = fn.Doc.Text()
		}
	}
	return docs
}

// The annotations every endpoint must carry, whatever it does.
var requiredAnnotations = []string{"@Summary", "@Description", "@Tags", "@Success", "@Router"}

var routerLine = regexp.MustCompile(`@Router\s+(\S+)\s+\[(\w+)\]`)
var tagsLine = regexp.MustCompile(`@Tags\s+(\S+)`)
var failureCode = regexp.MustCompile(`@Failure\s+(\d+)`)

func TestGlimtEndpointsAreAnnotated(t *testing.T) {
	docs := handlerDocs(t)

	for _, route := range glimtRoutes(t) {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if route.handler == "" {
				t.Fatalf("could not identify the handler for %s %s (routes.go:%d)",
					route.method, route.path, route.line)
			}
			doc, found := docs[route.handler]
			if !found {
				t.Fatalf("%s has no doc comment at all", route.handler)
			}

			for _, annotation := range requiredAnnotations {
				if !strings.Contains(doc, annotation) {
					t.Errorf("%s is missing %s", route.handler, annotation)
				}
			}

			// @Produce, unless the endpoint answers with no body at all. 204-only handlers exist
			// here (hide/unhide) and still declare one, so this is not a licence to skip it — it is
			// the media handler, which answers with image bytes rather than JSON.
			if !strings.Contains(doc, "@Produce") {
				t.Errorf("%s is missing @Produce", route.handler)
			}
		})
	}
}

// The annotation must describe the route that is actually registered. A path that drifted is worse
// than a missing one: the spec then documents an endpoint nobody can call, and the real one is
// undocumented, and both look fine.
func TestGlimtRouterAnnotationsMatchTheRegisteredPaths(t *testing.T) {
	docs := handlerDocs(t)

	for _, route := range glimtRoutes(t) {
		doc := docs[route.handler]
		// **All** @Router lines, not the first. swag allows a handler to document several paths, and one
		// handler answering two addresses is a real case here: the public site's former address is served
		// both bare and as a catch-all (task 351). Matching only the first line would have forced a second
		// identical handler to exist purely to satisfy this test.
		matches := routerLine.FindAllStringSubmatch(doc, -1)
		if len(matches) == 0 {
			// Reported by the test above; nothing to compare here.
			continue
		}

		// swaggo paths are relative to /api and use {braces} where httprouter uses :colons.
		//
		// A route that is *not* under /api — the server-rendered public page — documents its literal
		// path instead, and `TrimPrefix` is a no-op for it. There is no tidy alternative: swaggo has
		// one basePath, and writing `/../2026/glimt` to satisfy the arithmetic would be a lie in the
		// rendered spec.
		want := strings.TrimPrefix(route.path, "/api")
		for _, segment := range strings.Split(want, "/") {
			if strings.HasPrefix(segment, ":") {
				want = strings.Replace(want, segment, "{"+strings.TrimPrefix(segment, ":")+"}", 1)
			}
			if strings.HasPrefix(segment, "*") {
				// httprouter's catch-all. swag has no notation for one, so the documented form is a
				// plain parameter with the same name.
				want = strings.Replace(want, segment, "{"+strings.TrimPrefix(segment, "*")+"}", 1)
			}
		}
		// The **event year** is a path parameter as far as the documentation is concerned, even though
		// httprouter cannot express it as one (see routes.go): the annotation says `/{year}/patrulje/{number}`
		// because next year the same handler answers at `/2027/…`, and a spec naming one year would be wrong
		// twelve months later. So the registered path's year segment is normalised before comparing.
		if looksLikeYearPrefix(want) {
			want = "/{year}" + strings.TrimPrefix(want, guardYear)
		}

		documentedPath := false
		for _, match := range matches {
			if match[1] == want {
				documentedPath = true
				if !strings.EqualFold(match[2], route.method) {
					t.Errorf("%s documents method %s for %s but is registered as %s",
						route.handler, match[2], want, route.method)
				}
			}
		}
		if !documentedPath {
			var documented []string
			for _, match := range matches {
				documented = append(documented, match[1])
			}
			t.Errorf("%s is registered at %s but documents @Router %s",
				route.handler, want, strings.Join(documented, ", "))
		}
	}
}

func TestGlimtTagsGroupCoherently(t *testing.T) {
	docs := handlerDocs(t)
	seen := map[string][]string{}

	for _, route := range glimtRoutes(t) {
		match := tagsLine.FindStringSubmatch(docs[route.handler])
		if match == nil {
			continue
		}
		tag := match[1]
		if !allowedGlimtTags[tag] {
			t.Errorf("%s uses @Tags %q, which is not one of the agreed groups — a typo'd tag "+
				"puts an endpoint in its own section of the rendered spec, which nothing else reveals",
				route.handler, tag)
		}
		seen[tag] = append(seen[tag], route.handler)
	}

	// Moderation is its own group, and that is not cosmetic: it is the section a reader of the spec
	// should be able to find in one place, because those three endpoints are the only ones that can
	// read across every audience.
	if len(seen["glimt-moderation"]) != 3 {
		t.Errorf("expected 3 endpoints tagged glimt-moderation (queue, hide, unhide), got %v",
			seen["glimt-moderation"])
	}
}

// Every authenticated Glimt endpoint documents a 401.
//
// Not pedantry: a client author reading the spec needs to know an expired session is a *documented*
// outcome on this endpoint rather than a bug to report.
//
// Conditional on the route actually being wrapped in `requireAuth`, read from the AST — the public
// routes are not, and a 401 documented on one of them would describe an outcome that cannot happen
// and, worse, imply the endpoint looks at a session.
func TestGlimtEndpointsDocumentAuthFailure(t *testing.T) {
	docs := handlerDocs(t)

	for _, route := range glimtRoutes(t) {
		doc := docs[route.handler]
		codes := map[string]bool{}
		for _, m := range failureCode.FindAllStringSubmatch(doc, -1) {
			codes[m[1]] = true
		}

		if route.authenticated && !codes["401"] {
			t.Errorf("%s (%s %s) does not document a 401, though it is behind requireAuth",
				route.handler, route.method, route.path)
		}
		if !route.authenticated && codes["401"] {
			t.Errorf("%s (%s %s) documents a 401 but is not behind requireAuth — which implies it "+
				"reads a session, and the public routes must not",
				route.handler, route.method, route.path)
		}

		// A route with a path parameter can always be given an id that does not exist.
		if strings.Contains(route.path, ":glimtId") && !codes["404"] {
			t.Errorf("%s takes a glimt id but documents no 404", route.handler)
		}
	}
}

// The public routes are unauthenticated, and they cannot read a session even if one is sent.
//
// # This is the trap PRD 019 §8 names, asserted structurally
//
// A logged-in member's browser **will** send `hej_session` to the public page. If any of these
// handlers ever read it, the public page silently becomes a different page for members than for
// parents — and "is this public-safe?" stops being a testable question, because the answer would
// depend on who asked.
//
// Two halves, and together they make it impossible rather than merely wrong:
//
//  1. **Not wrapped in `requireAuth`.** That middleware is the *only* place a session enters the
//     request context (middleware.go), so a bare handler's `contextGetSession` returns false
//     unconditionally. Wrapping one of these would be the change that undoes everything else.
//  2. **No handler in the chain calls `contextGetSession`.** Belt and braces: if somebody ever adds
//     a global session middleware, half 1 stops protecting anything and this half still fires.
//
// `glimtpublic_test.go` covers the behaviour with a real authenticated cookie attached. This covers
// the structure, which is what stops the behaviour from being an accident.
func TestPublicGlimtRoutesAreNotBehindAuth(t *testing.T) {
	bodies := handlerBodies(t)
	saw := 0

	for _, route := range glimtRoutes(t) {
		if !strings.Contains(route.path, "/public/") && !looksLikeYearPrefix(route.path) {
			continue
		}
		saw++

		if route.authenticated {
			t.Errorf("%s (%s %s) is wrapped in requireAuth — the public surface must not be",
				route.handler, route.method, route.path)
		}
		if reads := readsSession(bodies, route.handler, 2); reads != "" {
			t.Errorf("%s reads the session (via %s) — the public page must answer the same thing to "+
				"a member and to a parent, whatever cookie arrives", route.handler, reads)
		}
	}

	if saw < 4 {
		t.Fatalf("found only %d public routes, want the 3 API routes plus the page — has the public "+
			"surface moved?", saw)
	}
}

// readsSession returns the name of the function that reads the session, or "".
func readsSession(bodies map[string]*ast.FuncDecl, name string, depth int) string {
	fn, found := bodies[name]
	if !found || depth < 0 {
		return ""
	}

	culprit := ""
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if fun.Name == "contextGetSession" {
				culprit = name
			}
		case *ast.SelectorExpr:
			if ident, ok := fun.X.(*ast.Ident); ok && ident.Name == "app" {
				// `app.sessions.Read` would be the other way in.
				if fun.Sel.Name == "sessions" {
					culprit = name
				}
				if deeper := readsSession(bodies, fun.Sel.Name, depth-1); deeper != "" {
					culprit = deeper
				}
			}
			if sel, ok := fun.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "sessions" {
				culprit = name
			}
		}
		return true
	})
	return culprit
}

// responseStatus maps each error helper in cmd/api/app/errors.go to the status it writes.
//
// This is the table that turns "is it documented?" into "does the documentation match the code?",
// which is the only version of this question worth asking. Annotations are comments: nothing
// compiles them, so the *only* thing that can make them true is a comparison against the branches
// that actually exist.
var responseStatus = map[string]string{
	"AuthenticationRequiredResponse": "401",
	"InvalidCredentialsResponse":     "401",
	"ForbiddenResponse":              "403",
	"NotFoundResponse":               "404",
	"BadRequestResponse":             "400",
	"BadRequestMessageResponse":      "400",
	"ConflictResponse":               "409",
	"RateLimitResponse":              "429",
	"RateLimitMessageResponse":       "429",
	"PayloadTooLargeResponse":        "413",
	"ServiceUnavailableResponse":     "503",
	"InsufficientStorageResponse":    "507",
	"ServerErrorResponse":            "500",
}

// statusConstant maps the `http.Status*` constants to their codes, for statuses written straight to
// the ResponseWriter rather than through an error helper.
//
// Success codes are deliberately absent: this table feeds a comparison against `@Failure` lines, so a
// 200 or 204 in it would be reported as an undocumented failure.
var statusConstant = map[string]string{
	"StatusNotModified":           "304",
	"StatusBadRequest":            "400",
	"StatusUnauthorized":          "401",
	"StatusForbidden":             "403",
	"StatusNotFound":              "404",
	"StatusConflict":              "409",
	"StatusRequestEntityTooLarge": "413",
	"StatusTooManyRequests":       "429",
	"StatusInternalServerError":   "500",
	"StatusServiceUnavailable":    "503",
}

// handlerBodies maps every method on *application to its AST body, so a handler's branches can be
// walked — including through the guards it delegates to.
func handlerBodies(t *testing.T) map[string]*ast.FuncDecl {
	t.Helper()

	out := map[string]*ast.FuncDecl{}
	for _, file := range packageFiles(t) {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Body != nil {
				out[fn.Name.Name] = fn
			}
		}
	}
	return out
}

// statusesWritten collects every HTTP status a handler can actually answer with.
//
// It follows calls to other `app.` methods, because the interesting branches are deliberately not in
// the handler: `requireGlimtModerator` owns the 401/403/503 for all three moderation endpoints, and
// `moderateGlimt` owns hide and unhide entirely. A walk that stopped at the handler body would
// conclude that `hideGlimtHandler` can only answer 204 and would then "helpfully" report every one of
// its documented failures as wrong.
//
// Depth-limited rather than cycle-tracked because the depth needed is 2 and an unbounded walk into
// arbitrary helpers would drag in every status the package can produce, which answers nothing.
func statusesWritten(bodies map[string]*ast.FuncDecl, name string, depth int) map[string]bool {
	out := map[string]bool{}
	fn, found := bodies[name]
	if !found || depth < 0 {
		return out
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// A status written straight to the ResponseWriter, bypassing the error helpers. The media
		// handler's conditional-GET 304 is the case that matters: it is a documented outcome with no
		// `app.*Response` call behind it, and a walk that only knew about the helpers reported the
		// annotation as describing something impossible.
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WriteHeader" {
				for _, arg := range call.Args {
					if s, ok := arg.(*ast.SelectorExpr); ok {
						if status, known := statusConstant[s.Sel.Name]; known {
							out[status] = true
						}
					}
				}
			}
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if ident, ok := sel.X.(*ast.Ident); !ok || ident.Name != "app" {
			return true
		}
		if status, isResponse := responseStatus[sel.Sel.Name]; isResponse {
			out[status] = true
			return true
		}
		// Another method on app: follow it.
		for status := range statusesWritten(bodies, sel.Sel.Name, depth-1) {
			out[status] = true
		}
		return true
	})
	return out
}

// The annotations agree with the branches the code actually has.
//
// # Both directions, and the second one is the useful one
//
// A **missing** `@Failure` is the ordinary omission: a client author reads the spec, does not learn
// that an expired session is a documented outcome, and treats a 401 as a bug to report.
//
// A **documented failure the code cannot produce** is worse and much harder to notice. It is how this
// test earned its keep: a first draft asserted "every write documents 503, because the event stream
// can be down by design" and flagged `uploadGlimtMediaHandler` — which turned out to be *correct*,
// because uploading stores bytes in the blob store and never publishes, so it has no 503 branch at
// all. The heuristic was wrong, not the annotation. Comparing against the code cannot make that
// mistake, which is the whole argument for doing it this way.
//
// 500 is exempt in both directions. Nearly every handler can produce one, documenting it says nothing
// a client can act on, and requiring it would add a line of noise to twelve annotation blocks.
func TestGlimtFailureCodesMatchTheHandlers(t *testing.T) {
	docs := handlerDocs(t)
	bodies := handlerBodies(t)

	for _, route := range glimtRoutes(t) {
		t.Run(route.handler, func(t *testing.T) {
			documented := map[string]bool{}
			for _, m := range failureCode.FindAllStringSubmatch(docs[route.handler], -1) {
				documented[m[1]] = true
			}

			written := statusesWritten(bodies, route.handler, 2)
			if len(written) == 0 && len(documented) > 0 {
				t.Fatalf("documents @Failure but has no error branches — did the handler move, or the "+
					"response helpers get renamed? (%s %s)", route.method, route.path)
			}
			if len(written) == 0 {
				// Coherent: a handler that cannot fail and documents no failure. The redirect from the
				// public site's former address is the case that makes this legitimate — it parses nothing
				// and reads nothing, so there is no branch to answer with.
				//
				// Kept narrow on purpose: the loud version above still fires for the failure this check
				// exists to catch, which is an annotation left behind by a handler that was rewritten.
				return
			}

			var undocumented, unreachable []string
			for status := range written {
				if status != "500" && !documented[status] {
					undocumented = append(undocumented, status)
				}
			}
			for status := range documented {
				if status != "500" && !written[status] {
					unreachable = append(unreachable, status)
				}
			}
			sort.Strings(undocumented)
			sort.Strings(unreachable)

			if len(undocumented) > 0 {
				t.Errorf("answers %v but does not document it — a client will read those as bugs",
					undocumented)
			}
			if len(unreachable) > 0 {
				t.Errorf("documents %v but has no branch that answers it — the spec describes an "+
					"outcome that cannot happen", unreachable)
			}
		})
	}
}
