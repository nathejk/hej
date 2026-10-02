package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// The share cards (PRD 026): what Facebook, Messenger and iMessage show for a shared link.
//
// # Why these are worth more than they look
//
// Every failure in this feature is **invisible from the site**. The tags are in `<head>`, nothing on the page
// changes, and the only reader is somebody else's scraper — so a card that is wrong stays wrong until it appears in
// a post, days later, after Facebook has cached it. Three bugs during the work proved the point: the origin came out
// as the container's own address, a per-page override was silently never called, and adding `og:url` broke the
// patrol pages' byte-identity guarantee. Every one of them rendered a page that looked perfect.

// shareTagsOf returns the `content` values of a page's share tags, by property name.
func shareTagsOf(t *testing.T, page string) map[string]string {
	t.Helper()

	tag := regexp.MustCompile(`<meta (?:property|name)="((?:og|twitter):[a-z_:]+)" content="([^"]*)">`)
	out := map[string]string{}
	for _, m := range tag.FindAllStringSubmatch(page, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// Every public page carries a complete card, and a page that never thought about sharing gets the branded one.
//
// The defaulting is the point. The alternative — tags only where a handler remembered them — fails by omission, and
// omission here means Facebook goes back to guessing, which is the state this feature exists to end.
func TestEveryPublicPageCarriesACompleteShareCard(t *testing.T) {
	app, _ := albumApp(t)
	app.models.Albums = seedAlbums(t, app)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{
		"/2026",
		"/2026/album/loerdag-morgen",
		"/2026/privatliv",
		"/2026/glimt",
	} {
		_, body := getPublic(t, srv.URL+path, nil)
		tags := shareTagsOf(t, string(body))

		for _, want := range []string{
			"og:type", "og:site_name", "og:locale",
			"og:title", "og:description", "og:image", "og:image:alt",
			"twitter:card", "twitter:image",
		} {
			if tags[want] == "" {
				t.Errorf("%s: %s is missing or empty — an incomplete card is one the platform fills in by "+
					"guessing\n%v", path, want, tags)
			}
		}
		if tags["og:locale"] != "da_DK" {
			t.Errorf("%s: og:locale = %q, want da_DK", path, tags["og:locale"])
		}
		if tags["twitter:card"] != "summary_large_image" {
			t.Errorf("%s: twitter:card = %q, want the large form", path, tags["twitter:card"])
		}
	}
}

// **The absolute URLs, which are the whole mechanism.**
//
// A relative `og:image` is silently ignored and the card comes back with no picture. And the host must be the one the
// visitor asked for: the first implementation used `r.Host`, which behind Traefik is the container's own address, so
// every card pointed at `https://api:4000/…` — a URL no scraper on the internet can resolve. Nothing on the page
// looked wrong, because every other link is relative.
func TestTheShareCardsURLsAreAbsoluteAndNameTheVisitorsHost(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/2026", nil)
	if err != nil {
		t.Fatal(err)
	}
	// What the proxy sends: the service is reached by its internal name, and the visitor's host and scheme arrive in
	// headers. Both are trusted only because our own proxy is the sole route in — see publicOrigin.
	req.Host = "api:4000"
	req.Header.Set("X-Forwarded-Host", "hej.nathejk.dk")
	req.Header.Set("X-Forwarded-Proto", "https")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)

	tags := shareTagsOf(t, body)
	for _, name := range []string{"og:image", "twitter:image"} {
		if got := tags[name]; !strings.HasPrefix(got, "https://hej.nathejk.dk/") {
			t.Errorf("%s = %q, want an absolute URL at the visitor's host: a relative one is ignored, and the "+
				"container's own address is unreachable", name, got)
		}
	}
	if !strings.Contains(body, `<meta property="og:url" content="https://hej.nathejk.dk/2026">`) {
		t.Errorf("og:url must be the absolute page address\n%s", body[:min(len(body), 1500)])
	}
	if strings.Contains(body, "api:4000") {
		t.Error("the container's internal address reached the page")
	}
}

// Each surface previews as what the maintainer decided it should (PRD 026 §6).
func TestEachSurfacePreviewsAsItsOwnThing(t *testing.T) {
	app, store := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	t.Run("the frontpage is branded, with no photograph", func(t *testing.T) {
		_, body := getPublic(t, srv.URL+"/2026", nil)
		tags := shareTagsOf(t, string(body))
		if !strings.HasSuffix(tags["og:image"], "/2026/share-card.png") {
			t.Errorf("og:image = %q, want the branded card", tags["og:image"])
		}
		// The frontpage lists several albums, and picking one of their covers would be our choice rather than a
		// curator's — on the page most likely to be shared by somebody unrelated to anybody in the pictures.
		if strings.Contains(tags["og:image"], "/media/") {
			t.Error("the frontpage must not preview as one album's photograph")
		}
	})

	t.Run("an album is its cover", func(t *testing.T) {
		_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
		tags := shareTagsOf(t, string(body))

		want := "/api/public/albums/al-1/media/" + itemRef(t, store, 0) + "?variant=medium"
		if !strings.HasSuffix(tags["og:image"], want) {
			t.Errorf("og:image = %q, want the cover at %q", tags["og:image"], want)
		}
		// The 800px rendition, not the 320px thumbnail: a card is rendered at ~1200 wide, and the thumbnail would
		// be visibly soft — the same arithmetic as task 461's.
		if strings.Contains(tags["og:image"], "variant=thumb") {
			t.Error("a card built from the thumbnail is the blurry-cover bug in a new place")
		}
		if tags["og:title"] != "Lørdag morgen" {
			t.Errorf("og:title = %q, want the album's title", tags["og:title"])
		}
		if !strings.Contains(tags["og:description"], "Da solen kom") {
			t.Errorf("og:description = %q, want the album's own description", tags["og:description"])
		}
		// The count, which is what makes the card say how much is in there.
		if !strings.Contains(tags["og:description"], "billeder") {
			t.Errorf("og:description = %q, want the photograph count", tags["og:description"])
		}
	})

	// A shared photograph previews as itself (task 490). The share button sends the permalink, which lands on the album
	// page at `?foto=`; that page must show the photograph and name the permalink as og:url, because a platform
	// re-scrapes og:url — an album address there would bring the cover back.
	t.Run("a shared photograph is itself, at its permalink", func(t *testing.T) {
		ref := itemRef(t, store, 1)
		resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}).
			Get(srv.URL + "/2026/album/loerdag-morgen/f/" + ref[:12])
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		tags := shareTagsOf(t, string(raw))

		if want := "/api/public/albums/al-1/media/" + ref + "?variant=medium"; !strings.HasSuffix(tags["og:image"], want) {
			t.Errorf("og:image = %q, want the shared photograph at %q", tags["og:image"], want)
		}
		if want := "/2026/album/loerdag-morgen/f/" + ref[:12]; !strings.Contains(string(raw),
			`<meta property="og:url" content="`+srv.URL+want+`">`) {
			t.Errorf("og:url must be the photograph's permalink %q, not the album", want)
		}
		if tags["og:title"] != "Lørdag morgen" {
			t.Errorf("og:title = %q, want the album's title", tags["og:title"])
		}
	})

	t.Run("an unknown ordinal is the album's own card", func(t *testing.T) {
		_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen?foto=999", nil)
		want := "/api/public/albums/al-1/media/" + itemRef(t, store, 0) + "?variant=medium"
		if got := shareTagsOf(t, string(body))["og:image"]; !strings.HasSuffix(got, want) {
			t.Errorf("og:image = %q, want the cover", got)
		}
	})

	t.Run("an album with no cover falls back to the branded card", func(t *testing.T) {
		empty := &albumStore{albums: []albumStoreEntry{{
			album:     album.Album{ID: "al-empty", Slug: "tomt", Title: "Tomt album"},
			published: true,
		}}}
		app, _ := albumApp(t)
		app.models.Albums = empty
		emptySrv := httptest.NewServer(app.routes())
		defer emptySrv.Close()

		_, body := getPublic(t, emptySrv.URL+"/2026/album/tomt", nil)
		tags := shareTagsOf(t, string(body))
		if !strings.HasSuffix(tags["og:image"], "/share-card.png") {
			t.Errorf("og:image = %q — an album with no photographs must fall back to the branded card, never to "+
				"a media URL that 404s: Facebook caches the failure and the card stays blank for days after the "+
				"album fills up", tags["og:image"])
		}
	})
}

// A patrol page previews as its diploma, with the diploma's own wording.
func TestThePatrolPagePreviewsAsItsDiploma(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	tags := shareTagsOf(t, string(body))

	if !strings.HasSuffix(tags["og:image"], "/api/public/patrol/42/diploma/thumb") {
		t.Errorf("og:image = %q, want the diploma thumbnail", tags["og:image"])
	}
	// Not the PDF: og:image must be an image, and a PDF is silently ignored.
	if strings.HasSuffix(tags["og:image"], "/diploma") {
		t.Error("og:image must be the thumbnail, not the PDF")
	}
	if !strings.Contains(tags["og:title"], "42") {
		t.Errorf("og:title = %q, want the patrol as the page names it", tags["og:title"])
	}
}

// **No card names a person.**
//
// PRD 026's non-functional requirement, and the reason it needs its own test rather than a note: the album page holds
// two free-text strings that could contain a name — a caption and a credit — and a credit is the one documented case
// that certainly does (task 393). Either would be a tempting thing to build a description from.
//
// The fixture's credit is the needle, which makes this exact rather than heuristic: it is the one name the public
// surface is allowed to render, and it must appear in the page's body and in none of its tags.
func TestNoShareCardNamesAPerson(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)

	// The credit is on the page — otherwise this test would pass by the album having no credit at all.
	if !strings.Contains(page, fixtureCredit) {
		t.Fatalf("the fixture's credit is not on the page, so this test proves nothing\n%s",
			page[:min(len(page), 800)])
	}

	for name, content := range shareTagsOf(t, page) {
		if strings.Contains(content, fixtureCredit) {
			t.Errorf("%s carries the credit %q: a card is quoted into a chat by whoever shares it, and a "+
				"photographer's name belongs on the photograph rather than in a preview", name, content)
		}
		// And the caption, the other free-text field.
		if strings.Contains(content, "Ved målstregen") {
			t.Errorf("%s carries a caption: captions are free text and the place a name would most plausibly "+
				"end up", name)
		}
	}
}

// The "not yet" patrol page stays byte-identical for every number, with the tags on it.
//
// This is the invariant `og:url` broke. A closed page that differed from an unknown one would say which patrols exist
// and which have finished, which is exactly what `openPatrol` is for — so that page has no canonical URL at all.
// Asserted here as well as in the patrol tests, because the next person to add a tag to `<head>` will read this file.
func TestTheNotYetPageCarriesNoAddress(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, first := getPublic(t, srv.URL+"/2026/patrulje/43", nil)
	_, second := getPublic(t, srv.URL+"/2026/patrulje/999999", nil)

	if string(first) != string(second) {
		t.Error("the closed page must be byte-identical for every number")
	}
	if strings.Contains(string(first), "og:url") || strings.Contains(string(first), "rel=\"canonical\"") {
		t.Error("the closed page must not name the address it was reached at: the number is in it")
	}
	// It still has a card, because a shared closed page should still look like something.
	if tags := shareTagsOf(t, string(first)); tags["og:image"] == "" {
		t.Error("the closed page should still preview as the branded card")
	}
}
