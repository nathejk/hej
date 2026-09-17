package main

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
)

// Serving Glimt media (PRD 019 §8, task 305).
//
// # A blob URL is a capability, and this file is where that stops being true
//
// Media is content-addressed, so `/api/.../<sha256>` would be a bearer token: forwardable,
// unrevokable, and identical for every caller. Anyone holding the string could read the bytes. That
// is precisely how a "min gruppe" photograph of a child becomes public — not through a breach, but
// through a hash being pasted somewhere.
//
// Two things prevent it here:
//
//  1. **The hash never leaves the building.** Media is addressed as `{glimtId}/media/{ordinal}`
//     (task 302), and an ordinal means nothing without the glimt id.
//  2. **This handler applies the same predicate as the feed.** Not a looser one, not none — the
//     identical `users.MaySeeGlimt`. The test that a `group`-scoped item 403s for an outsider is the
//     single most important test in the Glimt backend, because it is the one that proves a media URL
//     is not a bearer token.
//
// # Caching
//
// The bytes are immutable by construction: a different image is a different ref, so a URL's content
// can only change if the *glimt* changes, and a glimt's media list is written once. So the response
// is cacheable for a long time — which is most of the answer to the post-race load spike
// (PRD 019 §0a.3) and costs one header.
//
// But `private`, always, even for public-scope media served through this authenticated route:
// a shared cache keyed on URL alone would serve one member's group-scoped photo to the next caller
// of the same URL. The public page has its own unauthenticated route (task 323) where a shared
// cache is appropriate, and that separation is the reason it exists as a separate route at all.

// glimtMediaCacheControl is a year, immutable, private.
//
// A year rather than "forever" because there is no forever, and `immutable` is what tells the
// browser not to revalidate — the part that actually removes requests on the night.
const glimtMediaCacheControl = "private, max-age=31536000, immutable"

// showGlimtMediaHandler serves one variant of one media item.
//
// @Summary      One Glimt media item
// @Description  Serves the stored bytes for item `ordinal` of glimt `glimtId`. `variant=thumb` serves the 320px thumbnail used by the hold-collection grid; anything else, or an item with no thumbnail, serves the display image. Visibility is re-checked against the same rule the feed uses, so a group-scoped item is refused to a member of another group even though the URL is guessable — media URLs are not bearer tokens. Responses are `private` and `immutable`: the bytes for a given URL never change, but a shared cache must not serve one member's photo to another. 404 when the glimt, the item, or the stored bytes are gone.
// @Tags         glimt
// @Produce      jpeg
// @Param        glimtId  path      string  true   "glimt id"
// @Param        ordinal  path      int     true   "media position within the glimt"
// @Param        variant  query     string  false  "full (default) or thumb"
// @Success      200  {file}    binary
// @Failure      304  "not modified"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not shared with you"
// @Failure      404  {object}  map[string]string
// @Failure      429  {object}  map[string]string  "read rate limit — loose; see allowGlimtRead"
// @Failure      503  {object}  map[string]string
// @Router       /glimt/items/{glimtId}/media/{ordinal} [get]
func (app *application) showGlimtMediaHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if !app.allowGlimtRead(w, r, s.UserID) {
		return
	}
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, "glimt er ikke tilgængelige lige nu")
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	glimtID := params.ByName("glimtId")
	ordinal, err := strconv.Atoi(params.ByName("ordinal"))
	if err != nil {
		app.NotFoundResponse(w, r)
		return
	}

	g, found, err := app.models.Glimt.Get(app.config.eventYear, glimtID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	viewer, vfound := app.models.Users.Get(s.UserID)
	if !vfound {
		app.NotFoundResponse(w, r)
		return
	}

	// The whole point of this file. Same predicate as the feed, no exceptions, no shortcuts for
	// public-scope media (which has its own route).
	if !users.MaySeeGlimt(app.glimtViewer(s.UserID, viewer.Role), glimtSubjectOf(g)) {
		// 403 rather than 404. The glimt id came from *somewhere* — a forwarded link, a
		// shared screenshot — so pretending it does not exist buys nothing, and an honest
		// refusal is what tells a member their photo was not shared with this person. The
		// id itself is not a secret; the bytes are.
		app.ForbiddenResponse(w, r)
		return
	}

	ref, ok := glimtVariantRef(g, ordinal, r.URL.Query().Get("variant"))
	if !ok {
		app.NotFoundResponse(w, r)
		return
	}

	app.streamGlimtMedia(w, r, ref, glimtID)
}

// glimtVariantRef picks the ref for an ordinal and a variant.
//
// Falls back to the full item when a thumbnail was asked for and none exists. That fallback is not
// laziness: task 303 deliberately lets a thumbnail fail without failing the upload, so
// "thumbnail-less item" is a state that really occurs, and the alternative here would be a 404 for a
// tile whose photo is perfectly fine.
func glimtVariantRef(g glimt.Glimt, ordinal int, variant string) (blob.Ref, bool) {
	for _, m := range g.Media {
		if m.Ordinal != ordinal {
			continue
		}
		if strings.EqualFold(variant, "thumb") && m.ThumbRef != "" {
			if ref, ok := glimtBlobRef(m.ThumbRef); ok {
				return ref, true
			}
		}
		ref, ok := glimtBlobRef(m.Ref)
		return ref, ok
	}
	return "", false
}

// streamGlimtMedia writes the bytes with caching headers.
//
// Modelled on streamPortrait. The ETag is the content hash, which makes it a perfect validator:
// same bytes, same value. Combined with `immutable` this is what turns the post-race browse from
// thousands of transfers into thousands of 304s and then, once the browser trusts `immutable`, into
// no requests at all.
func (app *application) streamGlimtMedia(w http.ResponseWriter, r *http.Request, ref blob.Ref, logID string) {
	etag := `"` + string(ref) + `"`

	// Answered before opening the object: a 304 should not cost a filesystem read. The
	// browser sends this on every navigation back into a grid it already has.
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, string(ref)) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", glimtMediaCacheControl)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	reader, err := app.blobs.Get(r.Context(), ref)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			// A row referencing bytes that have gone degrades to 404, per PRD 008 §8: "a
			// replay that finds a missing object must degrade to 'no photo', never fail".
			app.NotFoundResponse(w, r)
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}
	defer reader.Close()

	// Always image/jpeg for now: every stored image is re-encoded (task 303). Task 322 will
	// need the stored content type here for video, which is why `glimt_media.contentType`
	// exists in the schema already.
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", glimtMediaCacheControl)
	w.Header().Set("ETag", etag)

	if _, err := io.Copy(w, reader); err != nil {
		// The response has already begun; there is nothing to say to the client that it
		// would still parse.
		app.Logger.Error("streaming glimt media", "err", err, "glimtId", logID)
	}
}
