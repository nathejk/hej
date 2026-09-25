package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// The public site's shell (task 332). What matters here is mostly what the page does *not* do: run
// JavaScript, read a session, or tell a visitor whether a patrol number is real.

// **Hiding the albums (task 359).** The maintainer's instruction before the first production deploy: *"hide
// both, we have no photos at the moment — i want to get the rest in prod"*.
//
// What these pin is the difference between *hidden* and *empty*, because the frontpage already had an empty
// state and reaching for it would have been the obvious wrong fix: "Der er ikke lagt billeder op endnu" is a
// promise with a date on it, and there is nothing to promise yet.
func TestTheAlbumSectionIsAbsentWhenSwitchedOff(t *testing.T) {
	app, _, _ := publicApp(t)
	app.config.publicAlbums = false
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/2026", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	page := string(body)

	// No heading, and **not** the empty state either.
	for _, forbidden := range []string{">Billeder<", "Der er ikke lagt billeder op endnu", "/2026/album/"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the hidden album section still renders %q\n%s", forbidden, page)
		}
	}

	// The rest of the page is the point of the deploy, so it must be untouched.
	for _, want := range []string{"Find din patrulje", ">Glimt<"} {
		if !strings.Contains(page, want) {
			t.Errorf("hiding the albums cost the %q section\n%s", want, page)
		}
	}
}

// The intro promised "Billeder fra løbet" — a sentence that has to change with the section, or the page opens by
// advertising something it does not have.
func TestTheIntroDoesNotPromisePhotosThatAreHidden(t *testing.T) {
	app, _, _ := publicApp(t)
	app.config.publicAlbums = false
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	if strings.Contains(page, "Billeder fra løbet") {
		t.Errorf("the intro still promises album photographs\n%s", page)
	}
	if !strings.Contains(page, "Patruljernes egne sider") {
		t.Errorf("the intro should say what the page does have\n%s", page)
	}
}

// **Hidden means unreachable, not unadvertised.** A section removed from the frontpage while its pages keep
// serving is still in every shared link, every old message and every index — so the album page answers 404.
//
// 404 rather than the 503 the missing-projection case gives: that one means "come back later", this one means
// "there is nothing here", and a crawler should be told the second thing.
func TestAnAlbumPageIsGoneWhenTheSectionIsHidden(t *testing.T) {
	app, _, _ := publicApp(t)
	app.config.publicAlbums = false
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := getPublic(t, srv.URL+"/2026/album/natten", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for an album page while the feature is hidden, got %d", resp.StatusCode)
	}
}

// **The gap this rule had, found while verifying task 376.** The frontpage hid the section and the album page
// answered 404 — but `/api/public/albums` went on serving every published album's slug and the coordinates of its
// photographs. That is precisely the "unadvertised but serving" state the test above argues against, one
// directory along.
//
// It mattered more once a curator could *place* those coordinates in bulk (task 376): the endpoint was the one
// public surface still disclosing them while the feature was supposedly switched off.
func TestTheAlbumMapIsGoneWhenTheSectionIsHidden(t *testing.T) {
	app, _, _ := publicApp(t)
	app.config.publicAlbums = false
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/api/public/albums", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for the album map while the feature is hidden, got %d", resp.StatusCode)
	}
	// And nothing leaks in the body either — no slug, no coordinate.
	for _, forbidden := range []string{"\"lat\"", "\"slug\"", "\"album\""} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the hidden album map still returns %s\n%s", forbidden, body)
		}
	}
}

// And the switch is not a one-way door: on, the album that was hidden is a page again. Worth a test because the
// value of a flag over a deletion is precisely that it comes back — an env change, not a release.
func TestTheAlbumsComeBackWhenSwitchedOn(t *testing.T) {
	app, _, _ := publicApp(t)
	app.config.publicAlbums = true
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	if !strings.Contains(string(body), ">Billeder<") {
		t.Errorf("the section should be back\n%s", body)
	}

	// The map read comes back with it, so the fix above cannot become a one-way door of its own.
	//
	// Asserted as "not 404" rather than "200": this fixture has no album projection, so the honest answer here is
	// 503 ("come back later"). What matters is that the *flag* is no longer what stops it — and the two are
	// deliberately different statuses for exactly this reason.
	resp, _ := getPublic(t, srv.URL+"/api/public/albums", nil)
	if resp.StatusCode == http.StatusNotFound {
		t.Error("the album map must not 404 when the section is switched on")
	}
}

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

	resp, body := getPublic(t, srv.URL+"/2026", nil)
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
	for _, path := range []string{"/2026", "/2026/patrulje/42"} {
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

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	for _, want := range []string{"<h1>", "<h2>", "<form method=\"get\"", "<label for=\"nummer\"", "<header class=\"sitehead\">", "<footer class=\"sitefoot\">"} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage is missing %q", want)
		}
	}
}

func TestPublicSitePagesAreNotIndexed(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/2026", "/2026/patrulje/42"} {
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

	for _, path := range []string{"/2026", "/2026/patrulje/42"} {
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
	resp, err := client.Get(srv.URL + "/2026/patrulje?nummer=42")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("want 303 so the browser re-requests with GET, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/2026/patrulje/42" {
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
		resp, err := client.Get(srv.URL + "/2026/patrulje?nummer=" + number)
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
		if loc != "/2026/patrulje/"+number {
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
		resp, err := client.Get(srv.URL + "/2026/patrulje?nummer=" + url.QueryEscape(entry))
		if err != nil {
			t.Fatalf("GET %q: %v", entry, err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Location"); got != "/2026?fejl=nummer" {
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

	_, body := getPublic(t, srv.URL+"/2026?fejl=nummer", nil)
	if !strings.Contains(string(body), "Skriv patruljens nummer med tal.") {
		t.Error("the known flag should render its message")
	}

	_, body = getPublic(t, srv.URL+"/2026?fejl=%3Cb%3Ehallo%3C%2Fb%3E", nil)
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
		resp, body := getPublic(t, srv.URL+"/2026/patrulje/"+number, nil)
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

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	// Minus the artwork, which contains "42" inside a Bezier coordinate — see withoutSVG.
	page := withoutSVG(string(body))

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

	_, body := getPublic(t, srv.URL+"/2026", nil)
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

	resp, body := getPublic(t, srv.URL+"/2026", nil)
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

	_, body := getPublic(t, srv.URL+"/2026", nil)
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

	_, body := getPublic(t, srv.URL+"/2026", nil)
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
// The frontpage links onward to the glimt page and to the site's own privacy page.
//
// **Not to `/privatliv` or `/desktop.html` any more** (task 351). Both were links *into the app*: a browser
// visitor following either got the app shell, which sent them to a placeholder reading "more to come…" — and
// once the desktop gate pointed at the public site, that became a loop. The site now has its own privacy page
// and the placeholder link is gone until PRD 013's content exists here.
func TestFrontpageLinksToTheGlimtPageAndTheSitesOwnPages(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	for _, want := range []string{`href="/2026/glimt"`, `href="/2026/privatliv"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage should link to %s", want)
		}
	}
	// And no link that leaves the public site for a page its audience cannot use.
	for _, forbidden := range []string{`href="/privatliv"`, `href="/desktop.html"`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the frontpage links to %s, which boots the app for a browser visitor", forbidden)
		}
	}
}

// The footer's takedown line is the one piece of text somebody needs when something is wrong, so it is
// on every page of the site rather than only the one that happens to have photographs on it.
func TestEveryPublicSitePageCarriesTheTakedownLine(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/2026", "/2026/patrulje/42"} {
		_, body := getPublic(t, srv.URL+path, nil)
		if !strings.Contains(string(body), "så tager vi det ned") {
			t.Errorf("%s is missing the takedown line", path)
		}
	}
}

// The move to the event-year prefix (PRD 021 §0, task 351).
//
// What matters here is not that `/2026` works — every other test in this file exercises that now — but the
// two things that are easy to get wrong when a surface changes address: the paths *inside* the pages, and
// what happens one directory off target.

// **There is no alias for the public site.** `/offentligt` was its first address for a few days before
// anything linked to it, so it was deleted rather than kept as a redirect: an address nobody holds is not
// compatibility, it is a second thing to remember in every template, test and denylist.
//
// Asserted rather than assumed, because a tidy-up like this is exactly what a later change reintroduces out
// of habit — and because the app derives the year itself now, so an alias would have no caller.
func TestTheFormerAddressIsGone(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	for _, path := range []string{"/offentligt", "/offentligt/glimt", "/offentligt/patrulje/42"} {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusSeeOther {
			t.Errorf("%s still redirects (%d); the alias was meant to be deleted", path, resp.StatusCode)
		}
	}
}

// **Every link inside the pages uses the current prefix.** A page that renders its own address wrongly is a
// page that 301s on every click at best, and 404s at worst — which is why the templates build links from a
// field rather than having the prefix typed into them.
func TestThePagesLinkToThemselvesUnderTheYearPrefix(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	for _, want := range []string{
		`href="/2026"`,            // the wordmark
		`action="/2026/patrulje"`, // the lookup form
		`href="/2026/glimt"`,      // the glimt strip's link
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the frontpage is missing %s", want)
		}
	}
	// And nothing still points at the old prefix, which would work only because of the redirect.
	if strings.Contains(page, `href="/offentligt`) || strings.Contains(page, `action="/offentligt`) {
		t.Error("the page still links to the former address")
	}
}

// **A path under the prefix that is not a page gets the public site's 404, never the app shell.**
//
// The SPA fallback answers anything unmatched with index.html so a client-side route survives a reload. Under
// the public prefix that would boot the app, which on a desktop sends the visitor straight back out to the
// public site — a loop. This is the same class of bug task 332 shipped through the service worker's denylist.
func TestAnUnknownPathUnderTheYearPrefixIsNotTheAppShell(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{
		"/2026/patruljer",   // a typo
		"/2026/album",       // a real prefix, no slug
		"/2025/patrulje/42", // a year this deployment does not serve
		"/1999",             // nonsense, year-shaped
	} {
		resp, body := getPublic(t, srv.URL+path, nil)
		page := string(body)

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, resp.StatusCode)
		}
		if !strings.Contains(page, "Siden findes ikke") {
			t.Errorf("%s: want the public not-found page, got:\n%.400s", path, page)
		}
		// The app shell's marker. Its absence is the property being tested.
		if strings.Contains(page, `id="app"`) || strings.Contains(page, "/assets/index") {
			t.Errorf("%s: served the app shell", path)
		}
		// A 404 must not be cached: a page that appears a minute later — an album published, a patrol
		// finishing — would read as missing to whoever asked early.
		if got := resp.Header.Get("Cache-Control"); strings.Contains(got, "public") {
			t.Errorf("%s: Cache-Control = %q; a failure must not be cached", path, got)
		}
	}
}

// The not-found page offers a way back rather than leaving somebody at a dead end, and names the year it
// actually serves — which is the likeliest thing they got wrong.
func TestThePublicNotFoundPageOffersAWayBack(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/ingenting", nil)
	page := string(body)

	if !strings.Contains(page, `href="/2026"`) {
		t.Error("the not-found page must link back to the frontpage")
	}
	if !strings.Contains(page, "Nathejk 2026") {
		t.Error("the not-found page should name the year this deployment serves")
	}
}

// withoutSVG removes every inline `<svg>…</svg>` from a rendered page.
//
// # Why the substring leak guards need this, and why it is safe
//
// Several guards on this surface ask "does this page contain this string anywhere at all" — a patrol number, a
// name, a decimal distance. Blunt on purpose: `TestAClosedPageLeaksNothingAboutARealPatrol` says so in its own
// comment, having once been failed by a hex colour containing "43", and concludes that the bluntness is worth a
// false positive because a leak hiding in an attribute is exactly what it is for.
//
// Task 423 changed the arithmetic. The header now carries the Nathejk moon inline, which is forty Bézier
// coordinates — `10.4336`, `57.4219`, `45.5854` — so "43", "42" and "5.5" are all present on **every** page of
// the site, whatever it is about. That is no longer an occasional false positive; it is a guard that fails for
// most two-digit patrol numbers regardless of whether anything leaked.
//
// Stripping artwork is the narrowest fix available. It is safe because **no SVG on this surface contains a
// template action** — the marks are constants — so nothing that could be a leak can be inside one.
// `TestNoInlineSvgOnThePublicSiteCarriesATemplateAction` is what keeps that true, and it is the licence for this
// helper: without it, this would be a hole rather than an exception.
func withoutSVG(page string) string {
	var out strings.Builder
	for {
		i := strings.Index(page, "<svg")
		if i < 0 {
			out.WriteString(page)
			return out.String()
		}
		out.WriteString(page[:i])
		rest := page[i:]
		j := strings.Index(rest, "</svg>")
		if j < 0 {
			// An unterminated svg: everything after it is artwork as far as this is concerned.
			return out.String()
		}
		page = rest[j+len("</svg>"):]
	}
}

// No inline SVG on the public site may carry a template action (task 423).
//
// This is the licence for `withoutSVG`, which the leak guards use to skip artwork. The moment a mark renders a
// value — a patrol's initial, a count, anything — artwork stops being a constant and those guards stop covering
// it. Then either this test fails and somebody makes a decision, or a leak walks straight through a hole nobody
// remembers opening.
//
// Read from the source rather than from a rendered page, so it covers every page including ones no test renders.
func TestNoInlineSvgOnThePublicSiteCarriesATemplateAction(t *testing.T) {
	src, err := os.ReadFile("publicsite.go")
	if err != nil {
		t.Fatalf("reading publicsite.go: %v", err)
	}

	rest := string(src)
	for {
		i := strings.Index(rest, "<svg")
		if i < 0 {
			return
		}
		rest = rest[i:]
		j := strings.Index(rest, "</svg>")
		if j < 0 {
			t.Fatal("an unterminated <svg> in publicsite.go")
		}
		if mark := rest[:j]; strings.Contains(mark, "{{") {
			t.Errorf("an inline SVG renders a template value, so the leak guards' withoutSVG no longer skips "+
				"only constants:\n%s", mark)
		}
		rest = rest[j+len("</svg>"):]
	}
}

// Every page of the public site wears the same header and footer, and the footer carries the boilerplate
// (task 423).
//
// # Why this is asserted per page rather than once
//
// The four strings in the footer used to be scattered: two in a shared footer, one on the patrol page, and "back
// to the frontpage" copy-pasted into four templates. The failure mode of that arrangement is not a wrong page, it
// is a **missing** line on the fifth page somebody adds — which nobody notices, because the other four are right.
//
// So this walks the pages and asks each one the same questions. A new page that forgets the layout fails here,
// which is the whole point of having the layout.
func TestEveryPublicPageWearsTheHeaderAndFooter(t *testing.T) {
	app, _, _ := publicApp(t)
	app.config.publicAlbums = true
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/2026", "/2026/privatliv", "/2026/patrulje/42"} {
		t.Run(path, func(t *testing.T) {
			_, body := getPublic(t, srv.URL+path, nil)
			page := string(body)

			for _, want := range []struct{ needle, why string }{
				{`<header class="sitehead">`, "the dark bar with the moon and the wordmark"},
				{`<a class="wordmark" href="/2026">`, "which is also the way home"},
				{`class="moon"`, "the mark itself, inlined so it cannot 404 and needs no asset under vue/"},
				{"<main>", "the page's own content, which is what carries the measure"},
				{`<footer class="sitefoot">`, "the same bar at the bottom"},
				{"Data og privatliv", "the privacy link belongs to the site, not to a page"},
			} {
				if !strings.Contains(page, want.needle) {
					t.Errorf("%s is missing %s — %s", path, want.needle, want.why)
				}
			}

			// The takedown invitation is in the footer in exactly one of its two forms, never both: a footer that
			// asked twice, once with a form and once without, would read as though the first one had not worked.
			form := strings.Contains(page, "Er der noget på denne side")
			line := strings.Contains(page, "Er der noget her, der ikke skal ligge offentligt")
			if form == line {
				t.Errorf("%s must carry exactly one takedown invitation (form=%v, line=%v)", path, form, line)
			}
		})
	}
}

// The frontpage does not offer a link to itself.
//
// Small, and it is the kind of thing a shared footer gets wrong by default: the link is right on every other page
// and pointless on this one, which is why the flag that suppresses it is set positively by the one handler that
// knows rather than inferred from something that happens to correlate.
func TestTheFrontpageFooterHasNoLinkBackToItself(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	if strings.Contains(string(body), "Tilbage til forsiden") {
		t.Error("the frontpage must not link back to the frontpage")
	}

	// And a page that is not the frontpage does.
	_, body = getPublic(t, srv.URL+"/2026/privatliv", nil)
	if !strings.Contains(string(body), "Tilbage til forsiden") {
		t.Error("every other page needs the way back, and the footer is where it lives now")
	}
}
