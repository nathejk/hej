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
	"dev":              true,
}

// registeredRoute is one `router.HandlerFunc(...)` call in routes.go.
type registeredRoute struct {
	method  string
	path    string
	handler string
	line    int
}

func isInScope(path string) bool {
	return strings.HasPrefix(path, "/api/glimt") || strings.Contains(path, "glimt")
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
		path, ok := stringLit(call.Args[1])
		if !ok || method == "" || !isInScope(path) {
			return true
		}

		out = append(out, registeredRoute{
			method:  method,
			path:    path,
			handler: handlerName(call.Args[2]),
			line:    fset.Position(call.Pos()).Line,
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

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// handlerName digs the handler method out of whatever wraps it.
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

// handlerDocs maps every `func (app *application) xHandler` in the package to its doc comment.
func handlerDocs(t *testing.T) map[string]string {
	t.Helper()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	docs := map[string]string{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Doc == nil {
					continue
				}
				docs[fn.Name.Name] = fn.Doc.Text()
			}
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
		match := routerLine.FindStringSubmatch(doc)
		if match == nil {
			// Reported by the test above; nothing to compare here.
			continue
		}

		// swaggo paths are relative to /api and use {braces} where httprouter uses :colons.
		want := strings.TrimPrefix(route.path, "/api")
		for _, segment := range strings.Split(want, "/") {
			if strings.HasPrefix(segment, ":") {
				want = strings.Replace(want, segment, "{"+strings.TrimPrefix(segment, ":")+"}", 1)
			}
		}

		if match[1] != want {
			t.Errorf("%s documents @Router %s but is registered at %s",
				route.handler, match[1], want)
		}
		if !strings.EqualFold(match[2], route.method) {
			t.Errorf("%s documents method %s but is registered as %s",
				route.handler, match[2], route.method)
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
// outcome on this endpoint rather than a bug to report, and every one of these sits behind
// `requireAuth`.
func TestGlimtEndpointsDocumentAuthFailure(t *testing.T) {
	docs := handlerDocs(t)

	for _, route := range glimtRoutes(t) {
		doc := docs[route.handler]
		codes := map[string]bool{}
		for _, m := range failureCode.FindAllStringSubmatch(doc, -1) {
			codes[m[1]] = true
		}

		if !codes["401"] {
			t.Errorf("%s (%s %s) does not document a 401, though it is behind requireAuth",
				route.handler, route.method, route.path)
		}

		// A route with a path parameter can always be given an id that does not exist.
		if strings.Contains(route.path, ":glimtId") && !codes["404"] {
			t.Errorf("%s takes a glimt id but documents no 404", route.handler)
		}
	}
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

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	out := map[string]*ast.FuncDecl{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Body != nil {
					out[fn.Name.Name] = fn
				}
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
			if len(written) == 0 {
				t.Fatalf("found no error branches at all — did the handler move, or the response "+
					"helpers get renamed? (%s %s)", route.method, route.path)
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
