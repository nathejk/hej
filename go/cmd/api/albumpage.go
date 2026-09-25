package main

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/album"
)

// The public album pages (PRD 011 §6 section 1, §7; task 334).
//
// # No carousel. Every photograph is its own `<img>`
//
// This is PRD 019's decision (task 323) and its reasoning transfers without change: the public surface
// has to work on a **desktop without a swipe** and with **no script at all**, and the app's strip is a
// Vue component driven by Embla that this page deliberately does not load. So an album with twelve
// photographs renders twelve img tags, in the curator's order, and every one of them is reachable by
// scrolling. The alternatives are recorded in task 323 — CSS scroll-snap with anchor links (works, but
// anchor navigation moves the *page*, and containing it needs per-browser care for no benefit) and a
// small inline script (fails the no-JavaScript requirement, and gives some visitors a different page).
//
// # Thumbnails always
//
// Same reason as the glimt page: this is read by a lot of people at once on whatever connection they
// have, and a grid of full-size images is the difference between usable and not. `loading="lazy"` and
// `decoding="async"` are plain attributes, not script, and intrinsic `width`/`height` stop the page
// reflowing as it loads — which on a slow connection is the difference between reading a caption and
// losing your place.
//
// # An unpublished album is a 404, not an empty page
//
// And so is a deleted one, and a slug that never existed. One answer for all three, decided in the
// projection's `BySlug` rather than here: distinguishing them would let the open web enumerate drafts,
// and a draft album is exactly the thing a curator has not decided to show yet.

// publicAlbumPageData is one album's page.
type publicAlbumPageData struct {
	publicPageData

	Album album.Album
	Items []publicAlbumItem

	// HasMore and NextSide carry the "Vis flere" link (task 399).
	//
	// Computed by the handler rather than by the template, because only the handler knows how many items the
	// album holds — the template is handed the window and cannot tell a full last page from a full first one.
	// That is the same reasoning the admin contact sheet's "Hent flere" follows: the server renders the button
	// because the server is what knows whether there is another page, so it cannot be left behind promising a
	// page that does not exist.
	HasMore  bool
	NextSide int
}

// albumPageCap is how many photographs one response carries (task 399, PRD 023 §2a.3).
//
// # Deliberately generous, and not a pagination feature
//
// PRD 023 §2a is honest about what this buys and what it does not. It buys **bounded server work per
// request** — `BySlug` materialises every row of an album on every request and there is no server-side cache,
// `max-age=60` being a header rather than one — plus a bounded DOM on a weak device and a bounded list for
// the viewer to walk. It buys approximately **nothing** on transferred bytes: `loading="lazy"` already does
// that, which is why it stays on every tile.
//
// So 200 is insurance against somebody emptying a 2000-photograph memory card into one album, not a page size
// chosen for reading. **Do not tune it downward because a page looks long.** An album of 40 photographs — the
// ordinary case PRD 011 described — must still be one page, one address, one scroll, with no control to
// press: a "next" button on a two-screen album is a worse page than a long one.
//
// If 200 is the wrong number, task 400 is the measurement that says so.
const albumPageCap = 200

// publicAlbumItem is one photograph as the page renders it.
//
// A view type rather than `album.Item` directly, and the difference is the point: this carries **no
// coordinate**. An album page is a list of photographs, and a latitude in the HTML would be a position
// published to the open web by a template nobody reviewed for that. Positions reach the public only
// through the map endpoint (task 342), which is the one place that decision is made and tested.
type publicAlbumItem struct {
	Ordinal int
	Caption string

	// Credit is the photographer's credit line, or "" when there is none (task 393).
	//
	// **The only field on the public surface that names a human being**, and the one exception to the claim in
	// publicprivacy_test.go's header. It names somebody who asked to be named, as the author of the
	// photograph, from text a curator typed — never resolved from the `person` projection. The guard flags
	// `credit` and excepts this exact field, so the exception is recorded rather than invisible.
	Credit string

	Width  int
	Height int
}

// albumPageHandler renders one album.
//
// @Summary      One public album (HTML)
// @Description  A curated album: its title, description and photographs in the curator's order. **Every photograph is its own img tag — there is no carousel** — so every one is reachable on a desktop without a swipe and without script. Thumbnails are served, lazily. At most 200 photographs per response (task 399): a larger album carries a plain "Vis flere" link to `?side=2`, which is a real link and not script, so every photograph stays reachable with JavaScript disabled. `foto` is a deep link to one photograph by ordinal (task 401): it renders whichever page holds that item, and it **wins over `side`** when both are given, because a photograph is what a sender meant and a window is only how the page is cut up today. Both parameters are clamped or ignored rather than validated — a nonsense, out-of-range or taken-down value lands on the album's first page, never a 404 or a 400, because the address still names a real album. Every tile carries `id="foto-{ordinal}"`, so a link with the matching fragment scrolls to it with no script at all. Unpublished, deleted and unknown albums all answer 404 identically, so the open web cannot enumerate drafts. Unauthenticated and it ignores the session cookie entirely. Not indexed.
// @Tags         public-site
// @Produce      html
// @Param        slug  path      string  true   "album slug"
// @Param        side  query     int     false  "which window of 200 photographs, 1-based (default 1)"
// @Param        foto  query     int     false  "open on this ordinal: renders the page holding it, and overrides side"
// @Success      200  {string}  string  "the page"
// @Failure      404  {object}  map[string]string  "unknown, unpublished or deleted"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      503  {object}  map[string]string  "albums are unavailable"
// @Router       /{year}/album/{slug} [get]
func (app *application) albumPageHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicSiteRead(w, r) {
		return
	}
	// Switched off (task 359): the whole feature is hidden, so an album has no page.
	//
	// **404, not 503.** The reasoning differs from the nil case below, which says "come back later" about a
	// dependency that is down. This says "there is nothing here", which is the truth a visitor and a crawler
	// should both get — and it is what makes hiding the frontpage section actually hide something. A section
	// removed from a page while its links keep serving is not hidden, it is unadvertised, and old links, shared
	// links and indexes all still work.
	if !app.config.publicAlbums {
		app.NotFoundResponse(w, r)
		return
	}
	if app.models.Albums == nil {
		// 503 rather than 404 here, unlike every gate in this feature. The distinction is what the
		// answer would reveal: a closed patrol page must be indistinguishable from a nonexistent one
		// because the difference leaks who has finished, while "albums are down" leaks nothing — and a
		// 404 would tell a curator their album had vanished.
		app.ServiceUnavailableResponse(w, r, "billederne er ikke tilgængelige lige nu")
		return
	}

	slug := httprouter.ParamsFromContext(r.Context()).ByName("slug")

	a, items, found, err := app.models.Albums.BySlug(app.config.eventYear, slug)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	data := publicAlbumPageData{
		publicPageData: publicPageData{Year: app.config.eventYear, Title: a.Title, Root: app.publicRoot()},
		Album:          a,
	}

	window, side, hasMore := albumPageWindow(items, albumRequestedSide(items, r.URL.Query()))
	data.HasMore = hasMore
	data.NextSide = side + 1
	for _, it := range window {
		data.Items = append(data.Items, publicAlbumItem{
			Ordinal: it.Ordinal,
			Caption: it.Caption,
			Credit:  it.Credit,
			Width:   it.Width,
			Height:  it.Height,
		})
	}

	app.renderPublicPage(w, "album", data)
}

// albumRequestedSide decides which window a request asks for, from `foto` if it names an item and `side`
// otherwise (task 401).
//
// # Why `foto` beats `side`
//
// They are different kinds of request and only one of them was typed by a human. `side` names a window — it
// comes from the "Vis flere" link this page rendered a moment ago. `foto` names **a specific photograph**, and
// it comes from somebody having pressed share in the viewer (task 405). When a link carries both and they
// disagree, the photograph is what the sender meant; the window is an implementation detail of how this page
// happens to be cut up today, and the cut can move when a curator adds photographs.
//
// So a stale `?side=2&foto=17` still lands on the photograph.
func albumRequestedSide(items []album.Item, query url.Values) string {
	if side, ok := albumSideHolding(items, query.Get("foto")); ok {
		return strconv.Itoa(side)
	}
	return query.Get("side")
}

// albumSideHolding finds which 1-based side holds the item with this ordinal.
//
// # An ordinal is not an index, and this is the bug that division would have shipped
//
// `ordinal / albumPageCap + 1` reads correctly and is wrong. An ordinal identifies a **slot in this album**,
// and the slots are sparse: `album_item` rows are soft-deleted, and a photograph deleted from the library stops
// satisfying the join in `BySlug` — which is exactly how the projection intends a deletion to take effect
// everywhere at once. So an album that has had items removed can hand back ordinals 0, 1, 5, 9, 400, and
// dividing 400 by the cap would send a visitor to page 3 of a one-page album.
//
// The position in the slice `BySlug` returned is the only thing that knows where an item actually sits, so that
// is what this counts. Linear, over a few hundred items, once per request that carries the parameter.
//
// # Not found is not an error
//
// A missing, non-numeric, negative or unknown ordinal returns false and the caller falls back to `side`, which
// means the album's first page. PRD 023 §8: the address still names a real, published album, and a link that
// has half-rotted — because the photograph it pointed at was taken down — should land on the album rather than
// on an error page. **Ordinal 0 is a perfectly good ordinal**, so it must not be lumped in with the rubbish;
// `strconv.Atoi` plus a lookup keeps that distinction without a special case.
func albumSideHolding(items []album.Item, rawFoto string) (int, bool) {
	if rawFoto == "" {
		return 0, false
	}
	ordinal, err := strconv.Atoi(rawFoto)
	if err != nil {
		return 0, false
	}
	for i, it := range items {
		if it.Ordinal == ordinal {
			return i/albumPageCap + 1, true
		}
	}
	return 0, false
}

// albumSide reads the `side` query parameter as a 1-based window number.
//
// # Every wrong value is page one
//
// Not a 404, and not an error: `?side=abc`, `?side=0`, `?side=-3` and `?side=99` on a three-page album all
// land on the first page. The address still names a real, published album, and the visitor did not type the
// query string — a link did, possibly one that was correct when it was sent and is not now because the curator
// removed photographs.
//
// That is the same instinct as `patrolSearchLookupHandler`'s redirect: when the input is wrong and the thing
// asked for exists, answer with the page rather than with the mistake. A 404 here would turn a stale link into
// a dead album.
func albumSide(raw string) int {
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// albumPageWindow returns the slice of items `side` asks for, the side it resolved to, and whether another
// page follows.
//
// It returns the **resolved** side rather than leaving the caller to re-derive it, because the caller needs it
// for the "next" link and calling `albumSide` twice is how the link ends up one page off the window beside it.
//
// A free function over the item slice rather than a method on the handler, so the windowing is testable
// without an HTTP server — the off-by-one at the last page is exactly the kind of thing that wants a table
// test, and exactly the kind of thing an end-to-end test reports as "the link is missing" without saying why.
//
// A `side` past the end clamps to the **last** page rather than returning nothing. An empty grid under a real
// album's title reads as "the photographs are gone", which is a much worse lie than "here is the end of the
// album".
func albumPageWindow(items []album.Item, rawSide string) ([]album.Item, int, bool) {
	if len(items) <= albumPageCap {
		// The ordinary album, and the one the feature was described for: one page, one address, no control to
		// press. Answered before any arithmetic so that `?side=7` on a 40-photograph album is simply the album.
		return items, 1, false
	}

	lastSide := (len(items) + albumPageCap - 1) / albumPageCap
	side := albumSide(rawSide)
	if side > lastSide {
		side = lastSide
	}

	start := (side - 1) * albumPageCap
	end := start + albumPageCap
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], side, side < lastSide
}

// albumMediaHandler serves one album photograph.
//
// Mirrors the public glimt media route, including the cache header, and shares
// `streamGlimtMedia` so the ETag handling and the missing-object degradation cannot diverge between
// two routes that do the same job.
//
// # The visibility check is re-done here, per item
//
// The bytes are addressed by album id and ordinal, so without this an id would be enough to pull a
// photograph out of an **unpublished** album off an unauthenticated route. That is the same trap the
// glimt media route documents, and the same answer: the read goes through the projection's
// publication filter rather than fetching the item directly.
//
// @Summary      One album photograph
// @Description  Serves the stored bytes for item `ordinal` of a **published, non-deleted** album. `variant=thumb` serves the 320px thumbnail, which is what the pages request. Unauthenticated and it ignores the session cookie; the album's publication state is re-checked here, so this route cannot be used to reach a draft album's photographs by id. Answers 404 when `PUBLIC_ALBUMS=false` hides the feature — the bytes go with the pages, or hiding the section would only unadvertise it. Cached `public` and `immutable`, which is safe precisely because the answer does not depend on who asked.
// @Tags         public-site
// @Produce      jpeg
// @Param        albumId  path      string  true   "album id"
// @Param        ordinal  path      int     true   "position within the album"
// @Param        variant  query     string  false  "full (default) or thumb"
// @Success      200  {file}    binary
// @Failure      304  "not modified"
// @Failure      404  {object}  map[string]string  "unknown album, unpublished, deleted, gone, or the albums section is switched off"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      503  {object}  map[string]string  "albums are unavailable"
// @Router       /public/albums/{albumId}/media/{ordinal} [get]
func (app *application) albumMediaHandler(w http.ResponseWriter, r *http.Request) {
	// The media budget, not the page one (task 347): an album page asks for up to sixty of these, and a
	// visitor scrolling two albums must not spend the allowance their next page load needs.
	if !app.allowPublicMediaRead(w, r) {
		return
	}
	// Switched off (task 359) — the **bytes** too, not only the pages.
	//
	// Found by task 382's walk, and it is the same hole task 376 found in `/api/public/albums`: hiding a
	// feature has to mean every surface of it, and a media route is the one that gets forgotten because it
	// serves no HTML and appears in no navigation. Anybody holding an album id and an ordinal — from a
	// shared link, a crawler's index, a browser history — could still fetch a photograph after the section
	// was switched off, which is precisely what an organizer switching it off is trying to stop.
	//
	// 404 for the same reason the album page answers 404: a section whose links keep serving is not hidden,
	// it is unadvertised.
	if !app.config.publicAlbums {
		app.NotFoundResponse(w, r)
		return
	}
	if app.models.Albums == nil {
		app.ServiceUnavailableResponse(w, r, "billederne er ikke tilgængelige lige nu")
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	albumID := params.ByName("albumId")
	ordinal, err := strconv.Atoi(params.ByName("ordinal"))
	if err != nil {
		app.NotFoundResponse(w, r)
		return
	}

	ref, ok, err := app.albumItemRef(albumID, ordinal, r.URL.Query().Get("variant"))
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !ok {
		// 404 for every reason: unknown album, unpublished album, missing ordinal, removed item. The
		// caller is anonymous, so a more specific refusal would only tell somebody probing which of
		// those it was.
		app.NotFoundResponse(w, r)
		return
	}

	app.streamGlimtMedia(w, r, ref, albumID, publicGlimtMediaCacheControl)
}

// albumItemRef resolves an album id and ordinal to the blob ref for the requested variant.
//
// # Why this goes through the published read
//
// `BySlug` is the only read that applies the publication filter, so resolving by **id** has to be
// funnelled through it — otherwise this route would reach a draft album's bytes. The cost is a slug
// lookup we do not have: the id is matched against the published set instead, which is a handful of
// albums.
//
// That is deliberately the cheap, obviously-correct shape rather than a new `ByID` read. A second read
// returning items would be a second place the publication filter has to be remembered, and this route
// is precisely where forgetting it would matter.
func (app *application) albumItemRef(albumID string, ordinal int, variant string) (blob.Ref, bool, error) {
	published, err := app.models.Albums.Published(app.config.eventYear)
	if err != nil {
		return "", false, err
	}

	slug := ""
	for _, a := range published {
		if a.ID == albumID {
			slug = a.Slug
			break
		}
	}
	if slug == "" {
		return "", false, nil
	}

	_, items, found, err := app.models.Albums.BySlug(app.config.eventYear, slug)
	if err != nil || !found {
		return "", false, err
	}

	for _, it := range items {
		if it.Ordinal != ordinal {
			continue
		}
		// The thumbnail when asked for and present; otherwise the full image. Falling back rather than
		// 404ing on a missing thumbnail is the glimt grid's rule too: a thumbnail is an optimisation,
		// and losing one should cost bandwidth rather than the photograph.
		ref := it.Ref
		if variant == "thumb" && it.ThumbRef != "" {
			ref = it.ThumbRef
		}
		r := blob.Ref(ref)
		if !r.Valid() {
			// Not a hash, so not something this store put there. Refused rather than passed to the
			// blob store, which is the one place a bad ref could become a filesystem path.
			return "", false, nil
		}
		return r, true, nil
	}
	return "", false, nil
}

// frontpageAlbums reads the album summaries the frontpage lists.
//
// Nil projection yields no albums and no error: the frontpage's albums section then shows its empty
// state, which reads as "nothing has been curated yet". That is the honest answer for a visitor either
// way, and unlike the glimt strip there is no takedown timing to be wrong about.
func (app *application) frontpageAlbums() []publicAlbumSummary {
	if app.models.Albums == nil {
		return nil
	}

	published, err := app.models.Albums.Published(app.config.eventYear)
	if err != nil {
		// Logged, and the section degrades to empty. One section failing must not take the page down —
		// the patrol lookup and the glimt strip are unaffected.
		app.Logger.Error("reading albums for the public frontpage", "err", err)
		return nil
	}

	out := make([]publicAlbumSummary, 0, len(published))
	for _, a := range published {
		out = append(out, publicAlbumSummary{
			Slug:         a.Slug,
			Title:        a.Title,
			Description:  a.Description,
			CoverAlbumID: a.ID,
			CoverOrdinal: a.CoverOrdinal,
			HasCover:     a.HasCover,
			Count:        a.ItemCount,
		})
	}
	return out
}
