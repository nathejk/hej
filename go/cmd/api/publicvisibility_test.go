package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// Nothing a curator did not publish reaches a public page (PRD 022 §9, PRD 011 §0b; task 382).
//
// # Why this test exists at all, and why it did not need to before
//
// PRD 011 §0b leans on publication as a **safety control** rather than a convenience: *a photograph is in
// an album because somebody decided it may be shown.* Under the old model that was nearly self-enforcing,
// because a photograph came into existence by being put in an album — existence and publication were one
// act, so the bug "a row leaked before anybody curated it" had nowhere to live.
//
// PRD 022 changes that. A photograph now exists in the library **before** anybody decides it may be shown,
// and the whole bulk is that bug's habitat. So PRD 022 §9 states the bar as a number: *"No photograph on a
// public page that a curator did not publish. Verified by test, not observed; zero is the only acceptable
// number."*
//
// # And why the fixture is a row store rather than a set of canned answers
//
// Every public album read funnels through three methods, and each applies its filter **in SQL** — the
// clauses `album/querysafety_test.go` guards. A stub that returned prepared lists would make this file pass
// by construction: it would assert that a fixture containing no draft photographs contains no draft
// photographs.
//
// So `visibilityStore` holds rows — albums, memberships, photographs — and applies the same joins the
// querier does, in the same order, with the same conditions. That is the pattern `albumStore` already set
// ("implemented for real … because the property under test is that an unpublished album is unreachable"),
// taken one step further because there are now three tables to get wrong instead of two.
//
// The two halves are complementary and neither is sufficient: `querysafety_test.go` proves the clauses are
// in the statement, and this proves that a read obeying those clauses leaks nothing through any handler,
// template or cache header on the way out.

// The markers. Each awkward case carries values distinctive enough that finding one anywhere in a public
// response is proof of a leak rather than a coincidence in Danish prose or CSS.
//
// Every case in PRD 022 §9's list gets its own marker, so a failure says *which* control lapsed rather than
// that something leaked.
const (
	// A photograph in the library and in no album at all. The bulk: the case that did not exist before
	// PRD 022 and is now the largest population in the table.
	hiddenUncurated = "UNCURATED-never-in-an-album"
	// A photograph in an album the curator has not published.
	hiddenDraft = "DRAFT-album-not-published"
	// A photograph in an album that was taken down.
	hiddenDeletedAlbum = "DELETEDALBUM-album-was-removed"
	// A membership the curator removed from a published album.
	hiddenRemovedItem = "REMOVEDITEM-taken-out-of-the-album"
	// A photograph the curator deleted from the library that is *still* a member of a published album.
	// The one that is invisible without the join: the membership row is live and correct.
	hiddenDeletedPhoto = "DELETEDPHOTO-still-a-member"
	// Coordinates that were judged out of the race area, or never judged at all.
	hiddenOutside = "OUTSIDE-verdict-outside"
	hiddenUnknown = "UNKNOWN-verdict-unchecked"

	// What a visitor is entitled to see. Asserted as well, because every control in this file can be
	// satisfied by showing nothing, and a test that only looks for absences passes loudest when the
	// feature is broken.
	shownPublic = "SHOWN-published-and-live"
)

// The coordinates. Distinctive to six decimal places so a substring match is unambiguous, and far enough
// apart that a rounded one is still recognisable.
const (
	latShown   = 55.733201
	lngShown   = 12.264801
	latOutside = 54.111102
	lngOutside = 11.111102
	latUnknown = 56.222203
	lngUnknown = 13.222203
	latDraft   = 57.333304
	lngDraft   = 14.333304
)

// visibilityPhoto is a row in the library.
type visibilityPhoto struct {
	id       string
	ref      string
	thumbRef string
	caption  string
	lat, lng *float64
	verdict  string
	// deleted is the curator's takedown. The membership rows pointing at it stay live — that is
	// deliberate, and it is what makes the join the only thing standing between a deleted photograph and
	// an album page. See photo/consumer.go's handleDeleted.
	deleted bool
}

// visibilityMembership is a row in album_item: a position and the photograph at it, and nothing else.
type visibilityMembership struct {
	ordinal int
	photoID string
	// removed is the curator taking the photograph out of *this* album. The photograph is untouched.
	removed bool
}

// visibilityAlbum is a row in album, with its memberships.
type visibilityAlbum struct {
	id, slug, title string
	sortOrder       int
	published       bool
	deleted         bool
	items           []visibilityMembership
}

// visibilityStore answers the three public reads from rows, applying the querier's joins.
type visibilityStore struct {
	albums []visibilityAlbum
	photos map[string]visibilityPhoto
}

// live reports whether a membership resolves to a photograph a public read may see.
//
// The join, in one place, so the three reads below cannot disagree about it — which is exactly the failure
// `TestTheCountAndTheCoverSeeTheSamePhotographsAsThePage` guards against in the real SQL.
func (s *visibilityStore) live(m visibilityMembership) (visibilityPhoto, bool) {
	if m.removed {
		return visibilityPhoto{}, false
	}
	p, found := s.photos[m.photoID]
	if !found || p.deleted {
		return visibilityPhoto{}, false
	}
	return p, true
}

// items renders one album's live memberships as the join produces them.
func (s *visibilityStore) items(a visibilityAlbum) []album.Item {
	var out []album.Item
	for _, m := range a.items {
		p, ok := s.live(m)
		if !ok {
			continue
		}
		out = append(out, album.Item{
			Ordinal: m.ordinal, PhotoID: p.id, Ref: p.ref, ThumbRef: p.thumbRef,
			Caption: p.caption, Width: 1600, Height: 1200,
			Lat: p.lat, Lng: p.lng, BoundsVerdict: p.verdict,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ordinal < out[j].Ordinal })
	return out
}

func (s *visibilityStore) summary(a visibilityAlbum) album.Album {
	items := s.items(a)
	out := album.Album{ID: a.id, Slug: a.slug, Title: a.title, SortOrder: a.sortOrder,
		ItemCount: len(items)}
	if len(items) > 0 {
		out.CoverOrdinal = items[0].Ordinal
		out.HasCover = true
	}
	return out
}

func (s *visibilityStore) Published(string) ([]album.Album, error) {
	out := []album.Album{}
	for _, a := range s.albums {
		if !a.published || a.deleted {
			continue
		}
		out = append(out, s.summary(a))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

func (s *visibilityStore) BySlug(_, slug string) (album.Album, []album.Item, bool, error) {
	for _, a := range s.albums {
		if a.slug != slug {
			continue
		}
		// One answer for unpublished, deleted and unknown.
		if !a.published || a.deleted {
			return album.Album{}, nil, false, nil
		}
		return s.summary(a), s.items(a), true, nil
	}
	return album.Album{}, nil, false, nil
}

func (s *visibilityStore) Plottable(string) ([]album.PlottableItem, error) {
	out := []album.PlottableItem{}
	for _, a := range s.albums {
		if !a.published || a.deleted {
			continue
		}
		for _, m := range a.items {
			p, ok := s.live(m)
			if !ok {
				continue
			}
			// Only `inside`, and only with a coordinate. Both conditions, because "has a verdict" and
			// "has a position" are different facts and a photograph can carry one without the other.
			if p.verdict != album.BoundsInside || p.lat == nil || p.lng == nil {
				continue
			}
			out = append(out, album.PlottableItem{AlbumID: a.id, AlbumSlug: a.slug,
				Ordinal: m.ordinal, Lat: *p.lat, Lng: *p.lng})
		}
	}
	return out, nil
}

// visibilityApp wires a public app with every awkward case in PRD 022 §9's list present in the data.
func visibilityApp(t *testing.T) (*application, *visibilityStore, *httptest.Server) {
	t.Helper()

	app, _, _ := publicApp(t)
	app.config.eventYear = "2026"

	put := func(body string) string {
		ref, err := app.blobs.Put(context.Background(), []byte(body))
		if err != nil {
			t.Fatalf("seeding the blob store: %v", err)
		}
		return ref.String()
	}

	at := func(lat, lng float64) (*float64, *float64) { return &lat, &lng }
	shownLat, shownLng := at(latShown, lngShown)
	outsideLat, outsideLng := at(latOutside, lngOutside)
	unknownLat, unknownLng := at(latUnknown, lngUnknown)
	draftLat, draftLng := at(latDraft, lngDraft)

	photos := map[string]visibilityPhoto{}
	add := func(p visibilityPhoto) {
		// Real bytes, so the media route can actually serve what it is allowed to serve and the refusals
		// are refusals rather than missing objects. A 404 caused by an absent blob would look exactly like
		// a 404 caused by the control under test, and would pass this file while the control was gone.
		p.ref = put("full-" + p.id)
		p.thumbRef = put("thumb-" + p.id)
		photos[p.id] = p
	}

	add(visibilityPhoto{id: "p-shown", caption: shownPublic,
		lat: shownLat, lng: shownLng, verdict: album.BoundsInside})
	add(visibilityPhoto{id: "p-uncurated", caption: hiddenUncurated, verdict: album.BoundsNone})
	add(visibilityPhoto{id: "p-draft", caption: hiddenDraft,
		lat: draftLat, lng: draftLng, verdict: album.BoundsInside})
	add(visibilityPhoto{id: "p-deletedalbum", caption: hiddenDeletedAlbum, verdict: album.BoundsInside})
	add(visibilityPhoto{id: "p-removeditem", caption: hiddenRemovedItem, verdict: album.BoundsInside})
	add(visibilityPhoto{id: "p-deletedphoto", caption: hiddenDeletedPhoto, deleted: true,
		lat: shownLat, lng: shownLng, verdict: album.BoundsInside})
	add(visibilityPhoto{id: "p-outside", caption: hiddenOutside,
		lat: outsideLat, lng: outsideLng, verdict: album.BoundsOutside})
	add(visibilityPhoto{id: "p-unknown", caption: hiddenUnknown,
		lat: unknownLat, lng: unknownLng, verdict: album.BoundsUnknown})

	store := &visibilityStore{
		photos: photos,
		albums: []visibilityAlbum{
			{
				id: "al-open", slug: "ved-maalet", title: "Ved målet", sortOrder: 10, published: true,
				items: []visibilityMembership{
					{ordinal: 0, photoID: "p-shown"},
					// The three that must vanish from a published album: a removed membership, a deleted
					// photograph whose membership is live, and two unplottable coordinates that may be on
					// the page but never on the map.
					{ordinal: 1, photoID: "p-removeditem", removed: true},
					{ordinal: 2, photoID: "p-deletedphoto"},
					{ordinal: 3, photoID: "p-outside"},
					{ordinal: 4, photoID: "p-unknown"},
				},
			},
			{
				id: "al-draft", slug: "kladde", title: "Kladde", sortOrder: 20, published: false,
				items: []visibilityMembership{{ordinal: 0, photoID: "p-draft"}},
			},
			{
				id: "al-gone", slug: "fjernet", title: "Fjernet", sortOrder: 30,
				published: true, deleted: true,
				items: []visibilityMembership{{ordinal: 0, photoID: "p-deletedalbum"}},
			},
		},
	}

	app.models.Albums = store
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, store, srv
}

// hiddenMarkers is every value that must not appear on any public surface, with what its presence means.
//
// # What is deliberately absent from this list
//
// The captions of the `outside` and `unknown` photographs. **A bad coordinate is not a reason to hide a
// photograph** — it is a reason not to plot it, and PRD 022 §9's two sentences are separate for exactly that
// reason: "no photograph a curator did not publish" and "no coordinate with a verdict other than `inside`".
// The curator published those two, so they belong on the page; only their positions are withheld. Their
// coordinates are in the list below, and `TestThePublishedPhotographIsStillShown` asserts the photographs
// themselves are present — otherwise "withhold the coordinate" could be implemented as "drop the
// photograph" and every test here would still pass.
func hiddenMarkers() map[string]string {
	return map[string]string{
		hiddenUncurated:    "a photograph in the library and in no album — the bulk, which nobody has curated",
		hiddenDraft:        "a photograph from an unpublished album",
		hiddenDeletedAlbum: "a photograph from an album that was taken down",
		hiddenRemovedItem:  "a membership the curator removed from this album",
		hiddenDeletedPhoto: "a photograph the curator deleted from the library",
		// Identities, not just captions: a slug or an id is enough to enumerate drafts even with no
		// caption rendered.
		"kladde":  "an unpublished album's slug",
		"Kladde":  "an unpublished album's title",
		"fjernet": "a deleted album's slug",
		"Fjernet": "a deleted album's title",
		"al-gone": "a deleted album's id",
		// Coordinates. The draft's is the sharp case: PRD 022 §9 forbids a *coordinate* on the public map
		// from an album nobody published, independently of any photograph being visible.
		"57.3333": "a coordinate from an unpublished album",
		"54.1111": "a coordinate judged outside the race area",
		"56.2222": "an unchecked coordinate",
	}
}

// **The walk.** Every public GET route, every media address in the fixture, every marker.
func TestNoUnpublishedPhotographReachesAPublicSurface(t *testing.T) {
	_, _, srv := visibilityApp(t)

	routes := publicRoutePaths(t)
	if len(routes) == 0 {
		t.Fatal("no public routes found: the enumeration is broken, which would make this file pass " +
			"while asserting nothing")
	}

	forbidden := hiddenMarkers()
	check := func(t *testing.T, where, body string) {
		t.Helper()
		for marker, what := range forbidden {
			if strings.Contains(body, marker) {
				t.Errorf("%s leaks %s (%q).\nPRD 022 §9: no photograph on a public page that a curator "+
					"did not publish, and zero is the only acceptable number. PRD 011 §0b makes "+
					"publication a safety control, not a convenience — a photograph is in an album "+
					"because somebody decided it may be shown.", where, what, marker)
			}
		}
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			url := srv.URL + visibilityURL(route.path)
			resp, body := getPublic(t, url, nil)
			if resp.StatusCode >= 500 && resp.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("%s answered %d; a server error is not a pass for this test", url, resp.StatusCode)
			}
			check(t, route.method+" "+route.path, string(body))
		})
	}

	// And every media address the fixture can express, including the ones that must refuse. Walked
	// separately because the route table has one media path and the interesting thing is which
	// (albumId, ordinal) pairs it answers.
	for _, media := range visibilityMediaCases() {
		t.Run("media "+media.url, func(t *testing.T) {
			resp, body := getPublic(t, srv.URL+media.url, nil)
			if media.serves {
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("%s: want 200 — every control here can be satisfied by serving nothing, so "+
						"the one photograph a visitor is entitled to must arrive; got %d",
						media.url, resp.StatusCode)
				}
				return
			}
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s: want 404 (%s), got %d", media.url, media.why, resp.StatusCode)
			}
			check(t, "media "+media.url, string(body))
		})
	}
}

// The photographs a visitor is entitled to must actually be on the pages.
//
// Every assertion above can be satisfied by publishing nothing at all, which is the failure mode of a
// test written only as a list of absences: it is loudest exactly when the feature is broken.
//
// This is also where the `outside`/`unknown` distinction is made real. Those two photographs must be
// **present**, because withholding a coordinate must not be implemented as dropping the photograph — and if
// it were, the walk above would go green.
func TestThePublishedPhotographIsStillShown(t *testing.T) {
	_, _, srv := visibilityApp(t)

	_, front := getPublic(t, srv.URL+"/2026", nil)
	for _, want := range []string{"Ved målet", `href="/2026/album/ved-maalet"`} {
		if !strings.Contains(string(front), want) {
			t.Errorf("the frontpage is missing %q\n%s", want, front)
		}
	}
	// Three live items out of five memberships: the count must agree with what the page shows, or "5
	// billeder" on a page with three photographs is a bug that ships easily. It is the same disagreement
	// `album.TestTheCountAndTheCoverSeeTheSamePhotographsAsThePage` guards in SQL, asserted end to end.
	if !strings.Contains(string(front), "3 billeder") {
		t.Errorf("the frontpage must count only the live items\n%s", front)
	}

	_, page := getPublic(t, srv.URL+"/2026/album/ved-maalet", nil)
	for _, want := range []string{shownPublic, hiddenOutside, hiddenUnknown} {
		if !strings.Contains(string(page), want) {
			t.Errorf("the album page is missing %q. A coordinate the bounds check rejected is withheld from "+
				"the map; the photograph is still one the curator published\n%s", want, page)
		}
	}
	if got := strings.Count(string(page), "<img "); got != 3 {
		t.Errorf("want three images on the album page, got %d", got)
	}
}

// **The map.** Only `inside`, and only from a published, undeleted album.
//
// Asserted on the endpoint's own response rather than only through the marker walk, because the map is the
// one public surface whose whole payload is coordinates — so "no forbidden string appears" is a weaker
// statement here than "exactly one marker, at the expected place".
func TestOnlyInsideVerdictsReachTheAlbumMap(t *testing.T) {
	app, _, srv := visibilityApp(t)
	if !app.config.publicAlbums {
		t.Skip("the album map is gated off in this app; see TestTheAlbumMapIsGoneWhenTheSectionIsHidden")
	}

	resp, body := getPublic(t, srv.URL+"/api/public/albums", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, body)
	}
	payload := string(body)

	if !strings.Contains(payload, "55.7332") {
		t.Errorf("the one plottable photograph is missing from the map\n%s", payload)
	}
	for marker, what := range hiddenMarkers() {
		if strings.Contains(payload, marker) {
			t.Errorf("the album map leaks %s (%q)", what, marker)
		}
	}
	// A coordinate count, so a second marker cannot hide behind the first being right.
	if got := strings.Count(payload, "56."); got != 0 {
		t.Errorf("the map carries an unchecked coordinate (%d occurrences of 56.)\n%s", got, payload)
	}
}

// **The gate** (task 359, PRD 022 §9's last row). With `PUBLIC_ALBUMS=false` the whole feature is hidden,
// and "hidden" has to mean *every* surface — not the section on the frontpage while the album page, the
// media bytes and the map still answer.
//
// A published, live, `inside` photograph is the fixture for this one. Everything else in this file asks
// whether a control lapsed for a photograph nobody cleared; this asks whether a photograph that *is*
// cleared still disappears when the section is switched off. Those are opposite questions, and passing the
// first says nothing about the second — which is how task 376 found `/api/public/albums` still serving
// published slugs and coordinates while the section was hidden.
func TestNothingAlbumShapedSurvivesTheSectionBeingHidden(t *testing.T) {
	app, _, srv := visibilityApp(t)
	app.config.publicAlbums = false

	// The frontpage must not mention the album, and the album's own surfaces must refuse.
	_, front := getPublic(t, srv.URL+"/2026", nil)
	for _, forbidden := range []string{shownPublic, "Ved målet", "ved-maalet", "al-open", "55.7332"} {
		if strings.Contains(string(front), forbidden) {
			t.Errorf("the frontpage leaks %q while PUBLIC_ALBUMS=false", forbidden)
		}
	}

	for _, url := range []string{
		"/2026/album/ved-maalet",
		"/api/public/albums",
		"/api/public/albums/al-open/media/0",
	} {
		resp, body := getPublic(t, srv.URL+url, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s answered %d with the section hidden; want 404", url, resp.StatusCode)
		}
		for _, forbidden := range []string{shownPublic, "ved-maalet", "55.7332"} {
			if strings.Contains(string(body), forbidden) {
				t.Errorf("%s leaks %q in its refusal", url, forbidden)
			}
		}
	}
}

// visibilityMediaCase is one address on the public media route and what must happen at it.
type visibilityMediaCase struct {
	url    string
	serves bool
	why    string
}

// visibilityMediaCases enumerates every media address the fixture can express.
//
// The refusals are the point. Each one must be a 404 **because the row is not in the read**, not because a
// handler remembered to check something: `albumItemRef` resolves through `Published`, so an album that is
// not published has no items to resolve against at all. See its comment — that funnelling is deliberate,
// precisely so there is no second place the publication filter has to be remembered.
func visibilityMediaCases() []visibilityMediaCase {
	const base = "/api/public/albums/"
	return []visibilityMediaCase{
		{url: base + "al-open/media/0", serves: true},
		{url: base + "al-open/media/0?variant=thumb", serves: true},
		// The page may show an out-of-bounds photograph — a bad coordinate is not a reason to hide a
		// photograph, only a reason not to plot it — so these two serve, and the map test is what proves
		// their coordinates stay off it.
		{url: base + "al-open/media/3", serves: true},
		{url: base + "al-open/media/4", serves: true},

		{url: base + "al-open/media/1", why: "the curator removed this membership"},
		{url: base + "al-open/media/2", why: "the curator deleted the photograph from the library"},
		{url: base + "al-draft/media/0", why: "the album is not published"},
		{url: base + "al-gone/media/0", why: "the album was taken down"},
		{url: base + "al-open/media/99", why: "no such ordinal"},
		{url: base + "al-unknown/media/0", why: "no such album"},
	}
}

// visibilityURL fills httprouter's parameters with values this fixture has.
func visibilityURL(path string) string {
	for param, value := range map[string]string{
		":slug":    "ved-maalet",
		":albumId": "al-open",
		":ordinal": "0",
		":glimtId": "g-none",
		":number":  "42",
	} {
		path = strings.ReplaceAll(path, param, value)
	}
	return path
}

// **The enumeration is self-checking.** A new public route that serves photographs and is not covered
// would make the walk above pass by not looking.
//
// Two halves, because the walk has two enumerations and they fail differently:
//
//  1. `publicRoutePaths` must still find the known surface. Shared with task 337's
//     `TestPublicRouteEnumerationCoversTheKnownSurface`; repeated here on the routes *this* file depends
//     on, so a change that narrows the parser fails next to the test that relied on it.
//  2. **Any public route whose path mentions media, albums or photos must appear in the media case list.**
//     That is the half that catches a new route: adding `/api/public/albums/:albumId/original/:ordinal`
//     would fail here rather than silently going untested.
func TestTheVisibilityWalkCoversEveryPublicPhotographRoute(t *testing.T) {
	routes := publicRoutePaths(t)

	found := map[string]bool{}
	for _, route := range routes {
		found[route.path] = true
	}
	for _, want := range []string{
		guardYear,
		guardYear + "/album/:slug",
		"/api/public/albums",
		"/api/public/albums/:albumId/media/:ordinal",
	} {
		if !found[want] {
			t.Errorf("the enumeration missed %s, which this file's assertions depend on", want)
		}
	}

	// Which media routes the case list speaks for. Matched on the route's path template rather than on a
	// concrete URL, so the check is about coverage rather than about any one address.
	covered := map[string]bool{}
	for _, c := range visibilityMediaCases() {
		url := strings.SplitN(c.url, "?", 2)[0]
		parts := strings.Split(strings.Trim(url, "/"), "/")
		// /api/public/albums/{id}/media/{ordinal} → the template with the parameters put back.
		if len(parts) == 6 {
			covered[fmt.Sprintf("/%s/%s/%s/:albumId/%s/:ordinal",
				parts[0], parts[1], parts[2], parts[4])] = true
		}
	}

	for _, route := range routes {
		lower := strings.ToLower(route.path)
		servesPhotographs := strings.Contains(lower, "/media/") ||
			strings.Contains(lower, "album") && strings.Contains(lower, "/api/")
		if !servesPhotographs {
			continue
		}
		// The map endpoint has no per-photograph address; it is asserted whole in
		// TestOnlyInsideVerdictsReachTheAlbumMap.
		if route.path == "/api/public/albums" {
			continue
		}
		// The glimt media route is a different feature with its own visibility rules (PRD 019) and its own
		// tests. Named explicitly rather than matched loosely, so a *new* album-side route cannot slip
		// through the same gap.
		if strings.Contains(route.path, "/glimt/") {
			continue
		}
		if !covered[route.path] {
			t.Errorf("%s serves photographs but visibilityMediaCases() says nothing about it. Add its "+
				"cases — a public media route that this file does not address makes the walk pass by "+
				"not looking, which is worse than no walk (PRD 022 §9)", route.path)
		}
	}
}
