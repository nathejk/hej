package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The working year (task 392). See adminyear.go for why it is carried on the request and never defaulted.

// **Every admin route carries a year**, by `atAdminYear` (the pages) or `requireAdminYear` (everything else). A
// route with neither would see `adminYear(r) == ""` — found exactly that way, on the untag route, while this
// task was being built. The vendored libraries are the one exception: static bytes, the same in every year.
func TestEveryAdminRouteCarriesAYear(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "routes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "HandlerFunc" || len(call.Args) < 3 || !wrapsRequireAdmin(call.Args[2]) {
			return true
		}
		path, _ := parseRegisteredPath(t, call.Args[1], fset)
		if path == "/admin/vendor/:asset" {
			return true
		}
		checked++
		if !wrapsGate(call.Args[2], "requireAdminYear") && !wrapsGate(call.Args[2], "atAdminYear") {
			t.Errorf("routes.go:%d %s is an admin route with no working year: wrap it in requireAdminYear",
				fset.Position(call.Pos()).Line, path)
		}
		return true
	})
	if checked < 20 {
		t.Fatalf("only %d admin routes checked; did the registration style change?", checked)
	}
}

// **No admin handler reads the configured year.** With some fifty call sites, one missed is a photograph filed
// in the wrong year with nothing to show for it. adminyear.go is the one file allowed to name it: it is where
// `EVENT_YEAR` becomes one of the workable years.
func TestNoAdminHandlerReadsTheConfiguredYear(t *testing.T) {
	files, _ := filepath.Glob("admin*.go")
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "adminyear.go" {
			continue
		}
		checked++
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			code := line
			if j := strings.Index(code, "//"); j >= 0 {
				code = code[:j]
			}
			if strings.Contains(code, "config.eventYear") || strings.Contains(code, "publicRoot()") {
				t.Errorf("%s:%d reads the configured year; use adminYear(r)", f, i+1)
			}
		}
	}
	if checked < 8 {
		t.Fatalf("only %d admin files checked", checked)
	}
}

// A fragment or API request with no year, or one the tool does not know, is refused — never defaulted.
func TestAnAdminRequestWithoutAWorkableYearIsRefused(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}

	for name, set := range map[string]func(*http.Request){
		"no year":      func(*http.Request) {},
		"unknown year": func(r *http.Request) { r.Header.Set(adminYearHeader, "1999") },
		"not a year":   func(r *http.Request) { r.Header.Set(adminYearHeader, "null") },
	} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/admin/photos", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.SetBasicAuth(testAdminUser, testAdminPass)
		set(req)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
	}
}

// `?year=` works where a header cannot be sent — an <img> thumbnail.
func TestTheYearMayComeAsAQueryParameter(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/admin/photos?year=2026", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

// The workable years are `event_year`'s plus EVENT_YEAR, newest first, and the pages exist for each.
func TestThePagesAreServedForEveryWorkableYear(t *testing.T) {
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass
	app.models.Years = fixedYears{"2025", "2026"}
	srv := httptestServer(t, app)

	if got := app.workableYears; strings.Join(got, ",") != "2026,2025" {
		t.Errorf("want 2026,2025, got %v", got)
	}
	for _, path := range []string{"/2025/photos", "/2025/albums", "/2026/photos"} {
		if got := getAdmin(t, srv, path, testAdminUser, testAdminPass).StatusCode; got != http.StatusOK {
			t.Errorf("%s: want 200, got %d", path, got)
		}
	}
	body := adminBody(t, getAdmin(t, srv, "/2025/photos", testAdminUser, testAdminPass))
	if !strings.Contains(body, `data-year="2025"`) {
		t.Error("the 2025 page must be in 2025")
	}
	if got := getAdmin(t, srv, "/1999/photos", testAdminUser, testAdminPass).StatusCode; got == http.StatusOK {
		t.Error("a year nobody works in has no pages")
	}
}

type fixedYears []string

func (f fixedYears) Years() ([]string, error)                   { return f, nil }
func (f fixedYears) Route(string) (string, string, bool, error) { return "", "", false, nil }

func httptestServer(t *testing.T, app *application) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return srv
}

// The page is in the year of its route, offers the other workable years as links, and marks any year that is not
// EVENT_YEAR with a bar that says so — on every page, since each one renders the header.
func TestWorkingInAnotherYearIsMarked(t *testing.T) {
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.config.adminUser = testAdminUser
	app.config.adminPassword = testAdminPass
	app.models.Years = fixedYears{"2025", "2026"}
	app.models.PhotoCurator = &libraryCurator{}
	srv := httptestServer(t, app)

	current := adminBody(t, getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass))
	if strings.Contains(current, `class="otheryear"`) {
		t.Error("the current year is not marked")
	}
	if !strings.Contains(current, `<a href="/2025/photos">2025</a>`) {
		t.Error("the photos page offers the same view in the other year")
	}

	for _, path := range []string{"/2025/photos", "/2025/albums"} {
		body := adminBody(t, getAdmin(t, srv, path, testAdminUser, testAdminPass))
		if !strings.Contains(body, `class="otheryear"`) || !strings.Contains(body, "ikke i årets løb (2026)") {
			t.Errorf("%s must say it is not the current year", path)
		}
		if !strings.Contains(body, `hx-headers='{"X-Admin-Year": "2025"}'`) {
			t.Errorf("%s must stamp 2025 on its htmx requests", path)
		}
	}
}

// Nothing in the tool's scripts calls fetch directly: ctx.fetch is what adds the working year, and a request
// without it is refused — so a bare fetch is a button that fails, or worse, the start of a default.
func TestTheAdminScriptsFetchOnlyThroughTheYear(t *testing.T) {
	for _, p := range adminPageScripts {
		src := mustReadAdminAsset(p)
		for i, line := range strings.Split(src, "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") {
				continue
			}
			for _, bad := range []string{" fetch(", "(fetch(", "\tfetch(", "XMLHttpRequest"} {
				if strings.Contains(" "+line, bad) && !strings.Contains(line, "window.fetch(url, o)") {
					t.Errorf("%s:%d calls fetch without the working year; use ctx.fetch", p, i+1)
				}
			}
		}
	}
}
