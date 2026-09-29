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

	// Ref is the photograph's **display** content hash, and its public address (tasks 447, 456).
	//
	// # Why a hash may be here, when the rule was that it may not
	//
	// PRD 022's surrounding rule — no blob hash in a public payload, because content addressing would make a URL
	// a forwardable, unrevokable capability — was narrowed on 2026-09-28 (maintainer, option A) to: **a ref may
	// appear where the route resolving it is album-scoped and publication-checked.**
	//
	// What makes that safe here is in the code rather than in the argument. `albumItemRef` resolves a ref *inside
	// a named album that must be in `Published(year)`*, so a ref reaches exactly what an ordinal reaches:
	// `TestAlbumMediaByRefIsScopedToItsPublishedAlbum` holds that one album's ref 404s in another and in the
	// unpublished one, and unpublishing revokes it. The hash is not a secret in any case — it is the SHA of bytes
	// this page already serves, so any visitor who downloads a photograph can compute it.
	//
	// What was traded is defence in depth: a hash in circulation is a hash a *future* route could be exploited
	// by. The glimt path keeps the unnarrowed rule, where media is per-member scoped and the argument is much
	// stronger (`glimtmediaserve.go`).
	//
	// # Only the display ref
	//
	// Never the thumbnail's or the 800px rendition's. Those are derived, a curator never sees them, and the
	// variant is a query parameter — so one ref addresses all three. `TestTheAlbumPageNeverPutsARenditionRefIn
	// ItsHTML` still forbids the derived two, which is the part of that rule that lost no force.
	Ref string

	// Permalink is this photograph's durable public address: /{year}/album/{slug}/{ref} (task 447).
	//
	// Built by the server because the server is what knows the slug, and handed to the viewer so that **share**
	// mints the durable form. The page's own state stays `?foto={ordinal}` — that is where the window and the
	// scroll target come from — and the distinction is the point: an ordinal says where a photograph *is today*,
	// a ref says which photograph it *is*.
	Permalink string

	// Credit is the photographer's credit line, or "" when there is none (task 393).
	//
	// **The only field on the public surface that names a human being**, and the one exception to the claim in
	// publicprivacy_test.go's header. It names somebody who asked to be named, as the author of the
	// photograph, from text a curator typed — never resolved from the `person` projection. The guard flags
	// `credit` and excepts this exact field, so the exception is recorded rather than invisible.
	Credit string

	Width  int
	Height int

	// HasMedium says whether the photograph has an 800px rendition (task 409), so the tile can emit
	// `data-medium` only when one exists.
	//
	// **A boolean, not the ref.** No blob hash may appear in a public payload: content addressing would make
	// it a forwardable, unrevokable capability, which is the rule `glimtpublic_test.go` asserts structurally.
	// The page addresses photographs by ordinal for exactly that reason, and this field carries the one bit
	// the template actually needs rather than the string it would be tempting to pass.
	HasMedium bool
}

// ShareCard is the album's own preview: its cover, its title, its description (PRD 026, task 467).
//
// # The cover, and what that decision cost
//
// The maintainer chose the cover over a neutral card, against the recommendation, and the reasoning is in PRD 026
// §6: the cover is the one photograph an organizer **picked** to represent the album, with PRD 011 §0b.2's consent
// gate upstream of it — and a card with no picture is a card nobody clicks, which defeats a public gallery.
//
// What it costs is written down where it can be acted on: a photograph Facebook scrapes is copied to Facebook's
// infrastructure and no takedown of ours reaches it. That is why task 470 puts the warning next to the control that
// chooses the cover rather than only in a document.
//
// # Where the text comes from, and where it must not come from
//
// The title is the album's title and the description is the album's own description — both written by a curator
// *about the album*. **Never a caption and never a credit.** Those are the two free-text fields on this surface that
// could carry a person's name, and a credit is the documented exception that certainly does (task 393).
// `TestNoShareCardNamesAPerson` holds it.
func (d publicAlbumPageData) ShareCard() shareCard {
	card := shareCard{
		Title:       d.Album.Title,
		Description: d.Album.Description,
	}
	// The count is worth having and worth being relaxed about: Facebook caches what it scrapes for days, so a share
	// from before an album grew will read low. A slightly stale count on an old post is harmless; a card that says
	// nothing about how much is in there is just less useful.
	if n := d.Album.ItemCount; n > 0 {
		count := photoCount(n)
		if card.Description == "" {
			card.Description = count + " fra " + publicSiteTitle + " " + d.Year + "."
		} else {
			card.Description += " \u00b7 " + count
		}
	}

	// The cover, at the 800px rendition — nearest the size a card is rendered at, and already the one the frontpage's
	// grid asks for (task 461). Addressed by **ref**, like everything else on this page since task 462.
	//
	// **No cover, no image**: `resolve` then falls back to the branded card. An og:image that 404s is worse than
	// none — Facebook caches the failure, so the card stays blank for days after the album has photographs.
	if cover, ok := d.coverRef(); ok {
		card.Image = "/api/public/albums/" + url.PathEscape(d.Album.ID) + "/media/" + cover + "?variant=medium"
		card.ImageAlt = "Forsidebillede fra albummet " + d.Album.Title
	}
	return card
}

// coverRef is the album's cover photograph.
//
// `album.Album` carries `CoverRef` since task 461 and `BySlug` — this page's read — computes it, so this is a
// pass-through with a fallback to the first item rather than a second cover rule. There is exactly one cover rule and
// it lives in `album.pickCover`.
func (d publicAlbumPageData) coverRef() (string, bool) {
	if d.Album.CoverRef != "" {
		return d.Album.CoverRef, true
	}
	if len(d.Items) > 0 && d.Items[0].Ref != "" {
		return d.Items[0].Ref, true
	}
	return "", false
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

	// Credits resolved for **this window only** (PRD 025, task 452). The album may hold hundreds of
	// photographs and the page shows a side of them, so resolving the whole album would read names nobody
	// is about to see — and this is the one read in the service that touches the person projection on a
	// public path. It does as little of it as the page needs.
	//
	// `app.config.eventYear` rather than the album's: the public site serves one year, and the photograph's
	// year is that year. See `person.CreditNames` for why resolution is year-scoped at all.
	var creditIDs []string
	for _, it := range window {
		if it.CreditCrewID != "" {
			creditIDs = append(creditIDs, it.CreditCrewID)
		}
	}
	creditNames := app.creditNames(app.config.eventYear, creditIDs)

	for _, it := range window {
		data.Items = append(data.Items, publicAlbumItem{
			Ordinal: it.Ordinal,
			Caption: it.Caption,
			// The **name**, never the reference (PRD 025 §6 R6): the id is a handle to a person record, and
			// `publicAlbumItem` is a type this page renders into HTML. An unresolvable reference is "" here,
			// which the template already renders as no credit line at all.
			Ref:       it.Ref,
			Permalink: albumPhotoPermalink(app.publicRoot(), a.Slug, it.Ref),
			Credit:    resolvedCredit(it.Credit, it.CreditCrewID, creditNames),
			Width:     it.Width,
			Height:    it.Height,
			HasMedium: it.MediumRef != "",
		})
	}

	app.renderPublicPage(w, r, "album", &data)
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

// albumPhotoPermalinkHandler resolves a photograph's durable address onto the album page (task 447).
//
// # Why this redirects rather than rendering
//
// The album page's state is `?foto={ordinal}` — the ordinal is what selects the window, what the fragment
// scrolls to, and what the viewer reflects as a visitor moves through the album (task 401). A permalink that
// rendered the page directly would either duplicate that machinery or leave the viewer rewriting the address
// into a form the permalink was meant to replace.
//
// So the two have different jobs and this is the seam: **the ref is the address, the ordinal is the state.**
// A permalink is resolved fresh on every visit, which is exactly what makes it durable — the album can be
// re-sorted a dozen times and the same link keeps landing on the same photograph.
//
// # 302, never 301
//
// The mapping from ref to ordinal changes whenever the album is re-sorted. A permanent redirect would invite
// every cache to pin one, and the link would then rot in precisely the way this feature exists to prevent.
//
// @Summary      A photograph's permalink (HTML redirect)
// @Description  The durable public address of one photograph: `/{year}/album/{slug}/foto/{ref}`, where `ref` is the photograph's content hash. **302 onto the album page** with `?foto={ordinal}` and the matching fragment, resolved fresh on every visit — which is what makes the link durable: the ordinal moves whenever the album is re-sorted (PRD 024) and the ref does not. Never a permanent redirect, because a cache that pinned one would rot the link in exactly the way this route exists to prevent. A ref that is not in the album, or no longer is, redirects to the **album** rather than answering 404: a taken-down photograph must not make an album look deleted (PRD 023 §8). An unknown, unpublished or deleted album answers 404 identically, and so does everything here when the public album section is switched off. Unauthenticated; ignores the session cookie.
// @Tags         public-site
// @Produce      html
// @Param        slug  path      string  true   "album slug"
// @Param        ref   path      string  true   "the photograph's content hash"
// @Success      302  "redirect to the album page, on the photograph"
// @Failure      404  "unknown, unpublished or deleted album — or the section is switched off"
// @Failure      503  {object}  map[string]string  "the albums are unavailable"
// @Router       /{year}/album/{slug}/foto/{ref} [get]
//
// # A ref that is not there lands on the album
//
// PRD 023 §8's rule, and it is the better answer: "a link that has half-rotted — because the photograph it
// pointed at was taken down — should land on the album rather than on an error page". A takedown must not make
// an album look deleted. An unknown *album* is still a 404, because that address names nothing at all.
func (app *application) albumPhotoPermalinkHandler(w http.ResponseWriter, r *http.Request) {
	if !app.config.publicAlbums {
		// The same refusal the album page makes when the section is switched off (task 359): every surface of a
		// hidden feature has to be hidden, and a redirect that still worked would be a link that still worked.
		app.NotFoundResponse(w, r)
		return
	}
	if app.models.Albums == nil {
		app.ServiceUnavailableResponse(w, r, "billederne er ikke tilgængelige lige nu")
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	slug := params.ByName("slug")
	ref := params.ByName("ref")

	_, items, found, err := app.models.Albums.BySlug(app.config.eventYear, slug)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	album := app.publicRoot() + "/album/" + slug
	for _, it := range items {
		if it.Ref != ref {
			continue
		}
		// Query **and** fragment, for task 401's reason: the query is what lets the server render the page
		// holding the item, the fragment is what scrolls the recipient to the tile, and neither can do the
		// other's job.
		ordinal := strconv.Itoa(it.Ordinal)
		http.Redirect(w, r, album+"?foto="+ordinal+"#foto-"+ordinal, http.StatusFound)
		return
	}

	// Not in this album, or no longer: the album itself, without a photograph named.
	http.Redirect(w, r, album, http.StatusFound)
}

// albumPhotoPermalink builds a photograph's durable public address (task 447).
//
// `/{year}/album/{slug}/foto/{ref}`, which is the maintainer's shape with one segment added. The shape was
// chosen for a reason worth keeping visible: **every public photograph is in an album**, so the album is part of
// what a photograph is publicly, and the ref is the part that does not move when the album is re-sorted.
//
// The extra `foto` is not taste. `{year}/album/{slug}/edit` is the admin album editor, registered under the same
// root, and httprouter refuses a wildcard sibling of a literal segment — `/album/{slug}/{ref}` panics at
// registration. `foto` also matches the vocabulary of the `?foto=` parameter it resolves onto.
//
// One place, because the page renders it into an attribute and the route below parses it back, and a builder
// and a parser that disagree about a URL shape produce links that resolve to the wrong photograph rather than
// to none.
func albumPhotoPermalink(root, slug, ref string) string {
	if slug == "" || ref == "" {
		return ""
	}
	return root + "/album/" + slug + "/foto/" + ref
}

// albumItemIs reports whether an item is the one a selector names.
//
// **By ref first**, because that is the form the page mints and the one that survives a re-sort. The ordinal
// comparison is second and exists for links made before task 456 — a shared address, a crawler's index, a
// browser history.
//
// An empty selector matches nothing: it is a URL with no photograph in it, and matching the first item would
// turn a malformed link into a confident wrong answer.
func albumItemIs(it album.Item, selector string) bool {
	if selector == "" {
		return false
	}
	// A ref is the photograph's **display** ref, never a thumbnail's or an 800px rendition's. Those are
	// derived and a curator never sees them; addressing by one would make the address depend on which
	// rendition happened to exist, which is the opposite of durable.
	if it.Ref == selector {
		return true
	}
	ordinal, err := strconv.Atoi(selector)
	return err == nil && it.Ordinal == ordinal
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
// @Description  Serves the stored bytes for item `ordinal` of a **published, non-deleted** album. `variant=thumb` serves the 320px thumbnail, which is what the grid requests; `variant=medium` serves the 800px rendition, which is what the viewer's `srcset` offers a phone; anything else serves the 1600px display image. A variant whose rendition was never produced — including every photograph uploaded before the 800px rendition existed — falls back to the display image rather than answering 404, so no page ever renders a gap. Unauthenticated and it ignores the session cookie; the album's publication state is re-checked here, so this route cannot be used to reach a draft album's photographs by id. Answers 404 when `PUBLIC_ALBUMS=false` hides the feature — the bytes go with the pages, or hiding the section would only unadvertise it. Cached `public` and `immutable`, which is safe precisely because the answer does not depend on who asked. The photograph is named either by its **ref** — the content hash, which does not change when the album is re-sorted — or by its **ordinal**, which does. Only the ref form is served `immutable`: an ordinal's meaning moves under a re-sort (PRD 024), and a cache told not to revalidate would keep serving the wrong photograph for a year. Ordinal URLs keep working, with a short lifetime, because they are what links minted earlier carry.
// @Tags         public-site
// @Produce      jpeg
// @Param        albumId  path      string  true   "album id"
// @Param        selector  path      string  true   "the photograph: its ref (content hash), or its ordinal within the album"
// @Param        variant  query     string  false  "full (default), medium (800px) or thumb (320px)"
// @Success      200  {file}    binary
// @Failure      304  "not modified"
// @Failure      404  {object}  map[string]string  "unknown album, unpublished, deleted, gone, or the albums section is switched off"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      503  {object}  map[string]string  "albums are unavailable"
// @Router       /public/albums/{albumId}/media/{selector} [get]
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
	selector := params.ByName("selector")

	ref, plan, ok, err := app.albumItemRef(albumID, selector, r.URL.Query().Get("variant"))
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

	// **`immutable` only for a content-addressed URL** (task 456), and this is the fix rather than a nicety.
	//
	// The route was addressed by ordinal and served `immutable, max-age=1y`. PRD 024 then made ordinals mutable:
	// a non-manual album re-sorts itself whenever photographs are added. So the header was a promise the server
	// could no longer keep — `immutable` tells caches **not to revalidate at all**, and after a re-sort every
	// cache in the world would keep serving the old photograph at that URL for a year, under captions the page
	// (`max-age=60`) had already updated.
	//
	// A ref is the hash of the bytes, so at a ref the promise is true again. An ordinal keeps working, because
	// those URLs are already cached and shared — but it gets a short lifetime, because what it names can change.
	app.streamGlimtMedia(w, r, ref, albumID, albumMediaCacheControl(selector), plan)
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
// albumMediaCacheControl decides how long a media URL may be trusted, from how it addresses the photograph.
//
// Content-addressed: a year, immutable, public — the bytes at a hash cannot become other bytes. Position-
// addressed: a minute, matching the album page that references it, because a re-sort moves what the position
// names and a cache that has been told not to revalidate would never find out.
func albumMediaCacheControl(selector string) string {
	if blob.Ref(selector).Valid() {
		return publicGlimtMediaCacheControl
	}
	return "public, max-age=60"
}

// albumItemRef finds a published album's photograph and the rendition to serve for it.
//
// `selector` is a **ref or an ordinal** (task 456). The ref is the durable address — it is the hash of the
// photograph's own bytes, so it does not move when the album is re-sorted — and the ordinal is what links
// minted before that change carry. A ref is recognised by being a valid ref, which an ordinal can never be:
// ordinals are short decimal numbers and a ref is a hash, so the two cannot be confused.
func (app *application) albumItemRef(
	albumID string, selector string, variant string,
) (blob.Ref, renditionRepair, bool, error) {
	published, err := app.models.Albums.Published(app.config.eventYear)
	if err != nil {
		return "", renditionRepair{}, false, err
	}

	slug := ""
	for _, a := range published {
		if a.ID == albumID {
			slug = a.Slug
			break
		}
	}
	if slug == "" {
		return "", renditionRepair{}, false, nil
	}

	_, items, found, err := app.models.Albums.BySlug(app.config.eventYear, slug)
	if err != nil || !found {
		return "", renditionRepair{}, false, err
	}

	for _, it := range items {
		if !albumItemIs(it, selector) {
			continue
		}
		// The requested rendition when present; otherwise the full image. Falling back rather than 404ing on
		// a missing rendition is the glimt grid's rule too: a rendition is an optimisation, and losing one
		// should cost bandwidth rather than the photograph. It is also what lets the 800px rendition (task
		// 409) ship with no backfill — every photograph uploaded before it has `mediumRef = ""` and serves
		// the display image, which is correct rather than degraded.
		ref := it.Ref
		edge := 0
		switch variant {
		case "thumb":
			if it.ThumbRef != "" {
				ref, edge = it.ThumbRef, glimtThumbEdges[0]
			}
		case "medium":
			if it.MediumRef != "" {
				ref, edge = it.MediumRef, mediumEdge
			}
		}
		r := blob.Ref(ref)
		if !r.Valid() {
			// Not a hash, so not something this store put there. Refused rather than passed to the
			// blob store, which is the one place a bad ref could become a filesystem path.
			return "", renditionRepair{}, false, nil
		}

		// The repair plan (task 430), only when a **derived** rendition is being served — `edge` is non-zero
		// exactly in those cases. An album upload keeps no separate original (PRD 022 §8.5), so the item's
		// full rendition is the source; and the full rendition itself is therefore unrebuildable, which is
		// exactly why it is the half that is backed up.
		plan := renditionRepair{}
		if edge > 0 {
			if full := blob.Ref(it.Ref); full.Valid() {
				plan = renditionRepair{
					Target:  r,
					Source:  full,
					Edge:    edge,
					Quality: glimtJPEGQuality,
				}
			}
		}
		return r, plan, true, nil
	}
	return "", renditionRepair{}, false, nil
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
			CoverRef:     a.CoverRef,
			// The **presence** of the rendition, not its ref: see publicAlbumSummary.CoverHasMedium.
			CoverHasMedium: a.CoverMediumRef != "",
			HasCover:       a.HasCover,
			Count:          a.ItemCount,
		})
	}
	return out
}
