package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/publicgate"
	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/scan"
)

// The diploma routes (task 345).
//
// The gate is the substance here: **two** conditions, and the second is the one that is easy to lose. A patrol
// whose page the backstop opened has a page and no diploma, so a route checking only "is the page open?" would
// hand a certificate saying *har gennemført* to a patrol that did not finish.

func TestTheDiplomaIsServedForAPatrolThatFinished(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	resp, body := getPublic(t, srv.URL+"/api/public/patrol/42/diploma", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/pdf" {
		t.Errorf("Content-Type = %q, want application/pdf", got)
	}
	if !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Errorf("body is not a PDF: % x", body[:8])
	}
	// Inline with a filename naming the patrol: a folder of files called diploma.pdf is no use to anybody.
	if got := resp.Header.Get("Content-Disposition"); !strings.Contains(got, "inline") ||
		!strings.Contains(got, "patrulje-42") {
		t.Errorf("Content-Disposition = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != diplomaCacheControl {
		t.Errorf("Cache-Control = %q, want %q", got, diplomaCacheControl)
	}
}

// **The second gate.** 43 never reached the finish; the backstop opens its page. It must still have no diploma.
func TestABackstopOpenedPatrolHasNoDiploma(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	app.publicGate = publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{
			{ID: "cg-1", SortOrder: 10}, {ID: "cg-mål", SortOrder: 20},
		}},
		gateScans{byTeam: map[string][]scan.Scan{
			"team-43": {{QrID: "q", Uts: nightAt(120).Unix(), CheckgroupID: "cg-1"}},
		}},
		gateClosing{uts: time.Now().Add(-time.Hour).Unix(), ok: true},
	)

	// The page is open …
	if resp, _ := getPublic(t, srv.URL+"/2026/patrulje/43", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("the page should be open, got %d", resp.StatusCode)
	}
	// … and there is no diploma behind it, on either route.
	for _, path := range []string{
		"/api/public/patrol/43/diploma",
		"/api/public/patrol/43/diploma/thumb",
	} {
		resp, _ := getPublic(t, srv.URL+path, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404 for a patrol that did not finish", path, resp.StatusCode)
		}
	}
}

// A closed patrol and an unknown number answer identically, as everywhere else on this surface: the diploma
// routes must not become the oracle the page refuses to be.
func TestTheDiplomaRoutesRevealNothingAboutAClosedOrUnknownPatrol(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	var first string
	for i, number := range []string{"43", "999999", "ikke-et-nummer"} {
		resp, body := getPublic(t, srv.URL+"/api/public/patrol/"+number+"/diploma", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", number, resp.StatusCode)
		}
		if i == 0 {
			first = string(body)
		} else if string(body) != first {
			t.Errorf("%s: the refusal differs from a closed patrol's", number)
		}
	}
}

func TestTheDiplomaThumbnailIsAnImageRenderedOnce(t *testing.T) {
	app, _, srv := patrolPageApp(t)

	resp, body := getPublic(t, srv.URL+"/api/public/patrol/42/diploma/thumb", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
	if len(body) < 1000 {
		t.Errorf("thumbnail is %d bytes, which is not an image", len(body))
	}

	// Rendered once and kept: the artwork is identical for every patrol and embedded in the binary, so
	// decoding and scaling it per request would be pure waste on an unauthenticated route.
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			getPublic(t, srv.URL+"/api/public/patrol/42/diploma/thumb", nil)
		}()
	}
	wg.Wait()

	// The same bytes every time, which is what "rendered once" means observably.
	_, again := getPublic(t, srv.URL+"/api/public/patrol/42/diploma/thumb", nil)
	if !bytes.Equal(body, again) {
		t.Error("the thumbnail changed between requests")
	}
	if app.diplomaThumbErr != nil {
		t.Errorf("the thumbnail render recorded an error: %v", app.diplomaThumbErr)
	}
}

// The page's slot links to the PDF and shows the thumbnail — and needs no script to do it.
func TestThePatrolPageLinksTheDiploma(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	for _, want := range []string{
		`href="/api/public/patrol/42/diploma"`,
		`src="/api/public/patrol/42/diploma/thumb"`,
		`alt="Patruljens diplom"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the diploma slot is missing %s", want)
		}
	}
	// No script was added to the page by this feature (the map island's three remain).
	if strings.Count(page, "<script") != 0 && !strings.Contains(page, "/publicmap.js") {
		t.Error("the diploma must not introduce script")
	}
}

// The finish time on the diploma is the **gate's** value, rendered in the event's timezone — so the page, the
// gate and the certificate cannot disagree about when a patrol finished.
func TestTheDiplomaUsesTheGatesFinishTimeInLocalTime(t *testing.T) {
	app, _, _ := patrolPageApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	d, ok := app.patrolDiploma(mustRequest(t, srv.URL+"/api/public/patrol/42/diploma", "42"))
	if !ok {
		t.Fatal("want a diploma for a patrol that finished")
	}
	if d.FinishedAt == nil {
		t.Fatal("want a finish time")
	}

	// The fixture's finish is 07:00 UTC on 20 September, which is 09:00 in Copenhagen (CEST).
	if got := d.FinishedAt.Hour(); got != 9 {
		t.Errorf("finish hour = %d, want 9 (07:00 UTC in Copenhagen)", got)
	}
	if got, want := d.Title, "Nathejk 2026"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	// Unset by default — no override, and the test app has no year projection — so no route is printed rather
	// than 2024's villages.
	if d.Route != "" {
		t.Errorf("route = %q, want empty by default", d.Route)
	}
}

// The route line's two sources (task 357).
//
// hq's year entity carries the two cities, and this app now folds them (`nathejk/table/year`). `EVENT_ROUTE`
// survives as an operator override and wins, because it is the only one of the two that can fix a wrong line
// without waiting for upstream data and a replay.
func TestTheRouteLineComesFromTheYearProjection(t *testing.T) {
	app, _, _ := patrolPageApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	app.models.Years = fixedRoute{from: "Lundby", to: "Glumsø"}

	d, ok := app.patrolDiploma(mustRequest(t, srv.URL+"/api/public/patrol/42/diploma", "42"))
	if !ok {
		t.Fatal("want a diploma")
	}
	if got, want := d.Route, "fra Lundby til Glumsø"; got != want {
		t.Errorf("route = %q, want %q", got, want)
	}
}

func TestTheOperatorOverrideBeatsTheProjection(t *testing.T) {
	app, _, _ := patrolPageApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	app.models.Years = fixedRoute{from: "Lundby", to: "Glumsø"}
	app.config.eventRoute = "fra Sorø til Ringsted"

	d, _ := app.patrolDiploma(mustRequest(t, srv.URL+"/api/public/patrol/42/diploma", "42"))
	if got, want := d.Route, "fra Sorø til Ringsted"; got != want {
		t.Errorf("route = %q, want the override %q", got, want)
	}
}

// **Half a route is no route.** One city without the other must not print "fra Lundby til " on something a
// family frames — the projection reports it as absent, and this pins that the handler does not paper over it.
func TestHalfARouteIsOmitted(t *testing.T) {
	app, _, _ := patrolPageApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	app.models.Years = fixedRoute{from: "Lundby"}

	d, _ := app.patrolDiploma(mustRequest(t, srv.URL+"/api/public/patrol/42/diploma", "42"))
	if d.Route != "" {
		t.Errorf("route = %q, want empty", d.Route)
	}
}

// A broken projection costs the line, not the diploma.
func TestAFailingYearReadOmitsTheLine(t *testing.T) {
	app, _, _ := patrolPageApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	app.models.Years = fixedRoute{err: errors.New("database on fire")}

	d, ok := app.patrolDiploma(mustRequest(t, srv.URL+"/api/public/patrol/42/diploma", "42"))
	if !ok {
		t.Fatal("a failed route read must not cost the diploma")
	}
	if d.Route != "" {
		t.Errorf("route = %q, want empty", d.Route)
	}
}

// fixedRoute is a year projection with one answer.
type fixedRoute struct {
	from, to string
	err      error
}

// Mirrors the real querier's contract: both cities or nothing (see year.Queries.Route).
func (f fixedRoute) Route(string) (string, string, bool, error) {
	if f.err != nil {
		return "", "", false, f.err
	}
	if f.from == "" || f.to == "" {
		return "", "", false, nil
	}
	return f.from, f.to, true, nil
}

// mustRequest builds a request carrying the httprouter param the handler reads.
//
// Built by hand rather than routed, because this test is about `patrolDiploma`'s *value* — the finish time and
// the timezone — which the HTTP response only carries inside a compressed PDF stream.
func mustRequest(t *testing.T, url, number string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	params := httprouter.Params{{Key: "number", Value: number}}
	return req.WithContext(context.WithValue(req.Context(), httprouter.ParamsKey, params))
}
