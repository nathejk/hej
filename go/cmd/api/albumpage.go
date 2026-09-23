package main

import (
	"net/http"
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
}

// publicAlbumItem is one photograph as the page renders it.
//
// A view type rather than `album.Item` directly, and the difference is the point: this carries **no
// coordinate**. An album page is a list of photographs, and a latitude in the HTML would be a position
// published to the open web by a template nobody reviewed for that. Positions reach the public only
// through the map endpoint (task 342), which is the one place that decision is made and tested.
type publicAlbumItem struct {
	Ordinal int
	Caption string
	Width   int
	Height  int
}

// albumPageHandler renders one album.
//
// @Summary      One public album (HTML)
// @Description  A curated album: its title, description and photographs in the curator's order. **Every photograph is its own img tag — there is no carousel** — so every one is reachable on a desktop without a swipe and without script. Thumbnails are served, lazily. Unpublished, deleted and unknown albums all answer 404 identically, so the open web cannot enumerate drafts. Unauthenticated and it ignores the session cookie entirely. Not indexed.
// @Tags         public-site
// @Produce      html
// @Param        slug  path      string  true  "album slug"
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
	for _, it := range items {
		data.Items = append(data.Items, publicAlbumItem{
			Ordinal: it.Ordinal,
			Caption: it.Caption,
			Width:   it.Width,
			Height:  it.Height,
		})
	}

	app.renderPublicPage(w, "album", data)
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
