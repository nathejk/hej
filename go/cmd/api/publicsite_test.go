package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// The public site's shell (task 332). What matters here is mostly what the page does *not* do: run
// JavaScript, read a session, or tell a visitor whether a patrol number is real.

// noRedirectClient follows nothing, so a 303 can be asserted rather than chased.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func TestPublicFrontpageRendersThreeSections(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/offentligt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	page := string(body)

	for _, want := range []string{"Billeder", "Find din patrulje", "Glimt"} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage is missing the %q section\n%s", want, page)
		}
	}
	// The order is the PRD's and is deliberate: albums invite, the lookup serves whoever arrived with a
	// number, the glimt strip is a taste with a link.
	albums := strings.Index(page, "Billeder")
	find := strings.Index(page, "Find din patrulje")
	glimt := strings.Index(page, ">Glimt<")
	if !(albums < find && find < glimt) {
		t.Errorf("sections are out of order: albums %d, find %d, glimt %d", albums, find, glimt)
	}
}

// The whole justification for server-rendering this surface.
//
// # The property is "complete without script", not "contains no script"
//
// An earlier version of this test forbade a `<script` tag anywhere on the public site. That was the right
// instinct stated too strongly: task 342's map is a **progressive enhancement**, which is a script tag by
// definition, and forbidding one would have forbidden the enhancement rather than protecting the property.
//
// So the frontpage stays script-free — nothing on it needs one — and the patrol page is allowed exactly the
// three lines that load the map, with `TestPatrolPageScriptIsOnlyTheMapIsland` pinning what they may be.
// The substance of every page is asserted to be present without them.
func TestPublicSitePagesCarryNoScript(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// The frontpage and a closed patrol page: neither has anything to enhance.
	for _, path := range []string{"/offentligt", "/offentligt/patrulje/42"} {
		_, body := getPublic(t, srv.URL+path, nil)
		page := strings.ToLower(string(body))
		for _, forbidden := range []string{"<script", "onclick=", "onload=", "javascript:"} {
			if strings.Contains(page, forbidden) {
				t.Errorf("%s contains %q: this surface must work with JavaScript disabled", path, forbidden)
			}
		}
	}
}

// Semantic HTML, so the content survives with CSS disabled — part of the same reach argument.
func TestPublicFrontpageUsesRealHeadingsAndAForm(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/offentligt", nil)
	page := string(body)

	for _, want := range []string{"<h1>", "<h2>", "<form method=\"get\"", "<label for=\"nummer\"", "<footer>"} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage is missing %q", want)
		}
	}
}

func TestPublicSitePagesAreNotIndexed(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/offentligt", "/offentligt/patrulje/42"} {
		resp, body := getPublic(t, srv.URL+path, nil)
		if got := resp.Header.Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
			t.Errorf("%s: want a noindex X-Robots-Tag, got %q", path, got)
		}
		if !strings.Contains(string(body), `name="robots"`) {
			t.Errorf("%s: want a robots meta tag in the document", path)
		}
		// Task 335 depends on this bound: a removed photograph must leave the public page promptly.
		if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "max-age=60") {
			t.Errorf("%s: want the short 60s cache window, got %q", path, got)
		}
	}
}

// The same structural property glimtpublic_test.go asserts, extended to the new routes: an
// authenticated browser gets exactly the page a parent gets.
func TestPublicSiteIgnoresTheSessionCookie(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/offentligt", "/offentligt/patrulje/42"} {
		_, anonymous := getPublic(t, srv.URL+path, nil)
		_, signedIn := getPublic(t, srv.URL+path, authedCookies(t, app, srv, "30000001", "+4530000001"))
		if string(anonymous) != string(signedIn) {
			t.Errorf("%s differs for a signed-in member; the public site must not read the session", path)
		}
	}
}

// The patrol lookup is a bridge from a no-JS form to a path URL, and nothing more.
func TestPatrolLookupRedirectsToThePathForm(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	client := noRedirectClient()
	resp, err := client.Get(srv.URL + "/offentligt/patrulje?nummer=42")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("want 303 so the browser re-requests with GET, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/offentligt/patrulje/42" {
		t.Fatalf("want a redirect to the shareable path form, got %q", got)
	}
}

// **The property the whole lookup design rests on.** An unknown number must redirect exactly like a
// known one: a validity check here would reintroduce the distinction the not-yet page removes, on the
// one route a visitor can hammer with guesses.
func TestPatrolLookupDoesNotRevealWhetherAPatrolExists(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	client := noRedirectClient()
	locations := map[string]string{}
	for _, number := range []string{"42", "999999"} {
		resp, err := client.Get(srv.URL + "/offentligt/patrulje?nummer=" + number)
		if err != nil {
			t.Fatalf("GET %s: %v", number, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("%s: want 303 regardless of whether the patrol exists, got %d", number, resp.StatusCode)
		}
		locations[number] = resp.Header.Get("Location")
	}
	if locations["42"] == locations["999999"] {
		t.Fatal("the two should differ only in the number itself")
	}
	for number, loc := range locations {
		if loc != "/offentligt/patrulje/"+number {
			t.Errorf("%s: want a plain redirect, got %q", number, loc)
		}
	}
}

func TestPatrolLookupRejectsNonNumericWithoutAnErrorPage(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	client := noRedirectClient()
	// Escaped, because some of these are not legal in a raw URL — and the point is what the *handler*
	// does with the value, not what a malformed request line does.
	for _, entry := range []string{"", "   ", "abc", "42a", "../../etc/passwd", "999999999"} {
		resp, err := client.Get(srv.URL + "/offentligt/patrulje?nummer=" + url.QueryEscape(entry))
		if err != nil {
			t.Fatalf("GET %q: %v", entry, err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Location"); got != "/offentligt?fejl=nummer" {
			t.Errorf("%q: want a redirect back to the form, got %q", entry, got)
		}
	}
}

func TestNormalizePatrolNumber(t *testing.T) {
	ok := map[string]string{
		"42":    "42",
		" 42 ":  "42",
		"042":   "42", // one page per patrol, and the canonical URL is the number on the sign
		"0":     "0",
		"00":    "0",
		"12345": "12345",
	}
	for in, want := range ok {
		got, valid := normalizePatrolNumber(in)
		if !valid || got != want {
			t.Errorf("normalizePatrolNumber(%q) = %q, %v; want %q, true", in, got, valid, want)
		}
	}

	// Rejected because this string becomes a path segment on an unauthenticated route. A patrol whose
	// number is not digits is still reachable by its URL directly.
	for _, in := range []string{"", " ", "abc", "4 2", "42-a", "٤٢", "123456789", "42%2f"} {
		if got, valid := normalizePatrolNumber(in); valid {
			t.Errorf("normalizePatrolNumber(%q) = %q, true; want rejected", in, got)
		}
	}
}

// The form's error flag must never become a channel for text from the URL.
func TestFrontpageErrorFlagIsNotReflectedText(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/offentligt?fejl=nummer", nil)
	if !strings.Contains(string(body), "Skriv patruljens nummer med tal.") {
		t.Error("the known flag should render its message")
	}

	_, body = getPublic(t, srv.URL+"/offentligt?fejl=%3Cb%3Ehallo%3C%2Fb%3E", nil)
	page := string(body)
	if strings.Contains(page, "hallo") {
		t.Errorf("an unknown flag must render nothing at all, not its own text\n%s", page)
	}
}

// Until the patrol page exists, every number answers "not yet" — which is also what this route answers
// for most of the year once it does.
func TestPatrolPageAnswersNotYetIdenticallyForEveryNumber(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	var first string
	for i, number := range []string{"42", "43", "999999"} {
		resp, body := getPublic(t, srv.URL+"/offentligt/patrulje/"+number, nil)
		if resp.StatusCode != http.StatusOK {
			// 200 rather than 404 on purpose: "404" in a browser reads as broken, and a parent who
			// followed a link would go and ask a leader why. Equal friendly answers achieve the same
			// indistinguishability as equal errors.
			t.Fatalf("%s: want 200 with a real page, got %d", number, resp.StatusCode)
		}
		if !strings.Contains(string(body), "ikke klar endnu") {
			t.Errorf("%s: want the not-yet page", number)
		}
		if i == 0 {
			first = string(body)
		} else if string(body) != first {
			t.Errorf("%s: the not-yet page must be byte-identical for every number", number)
		}
	}
}

// No patrol data may appear on a closed page — not a name, not a number, not even the one asked for.
func TestNotYetPageCarriesNoPatrolData(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/offentligt/patrulje/42", nil)
	page := string(body)

	// The requested number itself is patrol data on a closed page: echoing it back is how "is 42 real?"
	// becomes answerable by comparing two responses.
	for _, forbidden := range []string{"Ørnene", "Ulvene", "42"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the not-yet page must carry no patrol data, found %q\n%s", forbidden, page)
		}
	}
}

func TestFrontpageEmptyStatesReadAsNotYet(t *testing.T) {
	app, store, _ := publicApp(t)
	store.rows = nil
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/offentligt", nil)
	page := string(body)

	for _, want := range []string{
		"Der er ikke lagt billeder op endnu.",
		// Matches the existing public page's wording rather than inventing a second phrasing.
		"Der er ikke delt nogen offentlige billeder endnu.",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("want the empty state %q\n%s", want, page)
		}
	}
}

// A section being down must not take the page down: the albums and the lookup are unaffected.
func TestFrontpageSurvivesGlimtBeingUnavailable(t *testing.T) {
	app, _, _ := publicApp(t)
	app.models.Glimt = nil
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/offentligt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 with a degraded section, got %d", resp.StatusCode)
	}
	page := string(body)
	if !strings.Contains(page, "kan ikke vises lige nu") {
		t.Errorf("want the section to say it is unavailable\n%s", page)
	}
	if !strings.Contains(page, "Find din patrulje") {
		t.Error("the rest of the page must still render")
	}
}

// The glimt strip must inherit PRD 019's filtering rather than re-implement it: a group-scoped or
// hidden glimt appearing on the frontpage would be a leak the glimt page itself does not have.
func TestFrontpageGlimtStripShowsOnlyPubliclyVisibleGlimt(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/offentligt", nil)
	page := string(body)

	if !strings.Contains(page, "/api/public/glimt/g-public/media/0") {
		t.Errorf("want the public glimt's media on the strip\n%s", page)
	}
	for _, forbidden := range []string{"g-group", "g-nathejk", "g-public-hidden"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the strip must not show %s", forbidden)
		}
	}
}

// Thumbnails always, and lazily: this page is opened by a lot of people at once on whatever connection
// they have.
func TestFrontpageGlimtStripUsesLazyThumbnails(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/offentligt", nil)
	page := string(body)

	if !strings.Contains(page, "variant=thumb") {
		t.Error("the strip must request thumbnails, not full-size images")
	}
	if !strings.Contains(page, `loading="lazy"`) {
		t.Error("the strip must lazy-load")
	}
	if !strings.Contains(page, `alt="Glimt fra Patrulje 42`) {
		t.Error("every image needs alt text attributing the hold")
	}
}

// The two public surfaces must link to each other rather than duplicate content (PRD 013).
func TestFrontpageLinksToTheGlimtPageAndTheManual(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/offentligt", nil)
	page := string(body)

	for _, want := range []string{`href="/offentligt/glimt"`, `href="/desktop.html"`, `href="/privatliv"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage should link to %s", want)
		}
	}
}

// The footer's takedown line is the one piece of text somebody needs when something is wrong, so it is
// on every page of the site rather than only the one that happens to have photographs on it.
func TestEveryPublicSitePageCarriesTheTakedownLine(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/offentligt", "/offentligt/patrulje/42"} {
		_, body := getPublic(t, srv.URL+path, nil)
		if !strings.Contains(string(body), "så tager vi det ned") {
			t.Errorf("%s is missing the takedown line", path)
		}
	}
}
