package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// The shared photo viewer's assets (task 402, PRD 023 §7.3).
//
// # Why this is a third delivery shape, chosen rather than drifted into
//
// There are now three ways a front-end asset reaches a browser from this binary, and the differences are
// deliberate:
//
//   - `adminui/*` is **injected into its template's source** before parsing, so `html/template` still parses one
//     document and still contextually escapes the markup's actions. Right for a page's own markup, CSS and JS.
//   - the public site's CSS is **inline in its template**, because a separate stylesheet is a second request
//     that can fail and that page must render from one response.
//   - the viewer is **a separate, cached request**, because it is *shared*. One URL for both surfaces means one
//     cache entry and one file to fix a bug in — which is the whole point of PRD 023's "one implementation".
//
// It is allowed to be a second request precisely because it is an *enhancement*. If it fails to load, the page
// it belongs to is the plain page: a grid of thumbnails, each a link to its photograph, and a "Vis flere" link
// for the rest. That is a state PRD 011 §8 requires to work anyway.
//
// # Why the route is outside /admin
//
// Every `/admin/*` response carries `Cache-Control: no-store` (task 371), and keeping that rule true without
// exceptions was judged worth more than saving a download. An asset the admin tool shares with a public page
// cannot live under that rule and still be cached, so it lives somewhere else entirely — no exception needed,
// nothing for the next thing that wants one to cite.
//
//go:embed viewer/viewer.js viewer/viewer.css
var viewerFS embed.FS

// viewerAssets are the only names the route will serve.
//
// The content type is declared here rather than sniffed, for the reason `adminVendorAssets` gives: an explicit
// map means a mis-served stylesheet is a wrong line in this table rather than a subtle behaviour of the
// standard library.
var viewerAssets = map[string]string{
	"viewer.js":  "application/javascript; charset=utf-8",
	"viewer.css": "text/css; charset=utf-8",
}

// viewerAssetCacheControl is a year, immutable, and that is honest because the path carries a content hash.
//
// The thing to know if this is ever revisited: a *stale* page — one served inside the public site's 60-second
// window, just before a deploy — can ask for the previous hash and be answered with the current bytes, because
// the handler serves by name and treats the version as a cache key rather than as a lookup. For two static
// assets whose contract with each other is "class names in the CSS match class names in the JS", the worst case
// is one minute of a slightly mismatched pair on a page whose plain form works regardless. Storing every past
// version to avoid that would be a build system, which is the thing PRD 023 is written to avoid.
const viewerAssetCacheControl = "public, max-age=31536000, immutable"

// viewerVersion is the first 10 hex characters of the two assets' combined hash.
//
// Derived from the bytes rather than bumped by hand, because a version somebody has to remember to change is a
// version that is wrong exactly when it matters: after a one-line fix to a file that is cached for a year.
var viewerVersion = func() string {
	sum := sha256.New()
	// In a fixed order, so the same two files always hash the same way. Ranging over the map would make the
	// version depend on Go's map iteration order and change on every boot, which would defeat the caching this
	// exists for — cheap to get wrong, silent when wrong, so it is spelled out.
	for _, name := range []string{"viewer.css", "viewer.js"} {
		body, err := viewerFS.ReadFile("viewer/" + name)
		if err != nil {
			// Unreachable: the embed directive is compile-time. Panicking rather than degrading, because an
			// unversioned asset would be cached for a year under a path that means nothing.
			panic("reading the embedded viewer asset " + name + ": " + err.Error())
		}
		sum.Write(body)
	}
	return hex.EncodeToString(sum.Sum(nil))[:10]
}()

// viewerAssetPath is the URL a page links, e.g. `/viewer/8f14e45fce/viewer.js`.
//
// Exposed to both surfaces' templates as a function rather than written into their markup, so neither page can
// hold a stale version and neither has to know how versioning works here.
func viewerAssetPath(name string) string {
	return "/viewer/" + viewerVersion + "/" + name
}

// serveViewerAssetHandler serves one of the viewer's two files.
//
// # The name is matched against a map, never joined to a path
//
// Both parameters come from the URL. Joining one to a directory and reading the result is how a path traversal
// happens — `..%2f..%2fetc%2fpasswd` — and while `embed.FS` is not the host filesystem, `viewerFS` sits in a
// binary that also embeds this service's templates. A lookup in a fixed map cannot express a name that is not
// in the map. This is `serveAdminVendorHandler`'s shape and its reasoning, and it is asserted by test because
// "we look it up in a map" is exactly the kind of detail a later refactor tidies into a `filepath.Join`.
//
// The version segment is deliberately **not** validated: see viewerAssetCacheControl. It is a cache key, and a
// request for an old one must be answered rather than 404'd, or a stale page would lose its enhancement.
//
// @Summary      The shared photo viewer's JavaScript or CSS
// @Description  Serves the photo viewer's two static assets (PRD 023 §7.3), used unchanged by the public album page and by the photographer admin tool. The path carries a content hash so the response can be cached for a year; the hash is a cache key rather than a lookup, so a request naming an older one is answered with the current bytes rather than 404. Only `viewer.js` and `viewer.css` are served, matched against a fixed map — any other name, including an encoded path traversal, is 404. Unauthenticated, because a public page loads it, and deliberately **outside** `/admin` so the admin tool's `no-store` rule (task 371) needs no exception. Contains no data of any kind.
// @Tags         public-site
// @Produce      javascript
// @Param        version  path      string  true  "content hash, any value is accepted"
// @Param        asset    path      string  true  "viewer.js or viewer.css"
// @Success      200  {string}  string  "the asset"
// @Failure      404  {object}  map[string]string  "not one of the two assets"
// @Router       /viewer/{version}/{asset} [get]
func (app *application) serveViewerAssetHandler(w http.ResponseWriter, r *http.Request) {
	name := httprouter.ParamsFromContext(r.Context()).ByName("asset")
	contentType, known := viewerAssets[name]
	if !known {
		app.NotFoundResponse(w, r)
		return
	}

	body, err := viewerFS.ReadFile("viewer/" + name)
	if err != nil {
		// Unreachable: the map and the embed directive are both compile-time, so a name in one is in the other.
		// Answered rather than asserted, because "unreachable" is a property of today's code.
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", viewerAssetCacheControl)
	if _, err := w.Write(body); err != nil {
		app.Logger.Debug("viewer asset write failed", "asset", name, "err", err)
	}
}
