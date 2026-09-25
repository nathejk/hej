package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// The admin tool's page (PRD 022 §7, tasks 369–373).
//
// # Why this is a Go template and not a Vue view
//
// PRD 022 §8.1: the PWA runs a device-class gate *ahead* of its auth gate and `window.location.replace`s a
// desktop visitor out to the public website. Its own comment says revisiting that gate needs a new PRD. Bulk
// upload from an SD card is desktop access by definition, so the tool cannot live there — and the job is a
// filesystem job anyway: three hundred files, a folder, a keyboard, a screen big enough to judge photographs
// on.
//
// The absence of a build step is the reason not to start a *second* frontend for it either. No npm dependency,
// no bundler, no framework: an `html/template`, inline CSS, and hand-written JavaScript.
//
// # Why this page does not share the public site's template
//
// `publicsite.go` renders with a deliberately low browser floor — no `oklch()`, no nesting, no custom
// properties — and must work without JavaScript, because those pages go to whatever device a family owns.
//
// None of that applies here and inheriting it would be the wrong kind of consistency. This page is served to
// two or three organizers on laptops, behind a password, and the work it does *requires* JavaScript. §7 records
// the trade: the admin page may use modern JavaScript freely; the public pages may not gain any. What it must
// not do is drag the public pages up or down with it, which is why the two do not share a template.
//
// # Icons
//
// Inline SVG copied from Lucide, matching the repo's icon rule, but not the `@lucide/vue` package — there is no
// build step here to tree-shake it.

// adminPageData is what the shell renders.
type adminPageData struct {
	// View is which of the tool's pages this is: "albums" (the landing page), "photos" (upload and the contact
	// sheet) or "album" (one album's editor over its photographs). One template renders both, so the header, the counts and the styling cannot drift apart (task 396).
	View string

	// Root is the year's path prefix, `/2026`. The curator's pages hang off it beside the public ones (task 396).
	Root string

	// Filters are the contact sheet's presets, with the one this page's URL names switched on.
	Filters []adminFilterView
	// Query is the switched-on preset's query string, empty for "Alle". Passed to the first fragment request so
	// the grid a curator lands on already matches the button that is lit.
	Query string

	// Years are the workable years, for the header's year switch; EventYear is `EVENT_YEAR`, so the page can
	// mark working in any other year as the abnormal thing it is (task 392).
	Years     []string
	EventYear string

	// Album is the album view's editor card; nil on the other two views.
	Album *adminAlbumPageData

	// Year is the event year every write lands in, shown prominently because PRD 022 §5 makes it the one
	// thing a curator cannot undo by editing: a photograph uploaded into the wrong year is not a typo, it is
	// in the wrong event.
	Year string

	// Counts is the library's summary. Zero-valued when the projection is unavailable, with Unavailable set
	// so the page can say so rather than claiming an empty library.
	Counts adminCountsView

	// Unavailable reports that the curator reads could not be reached.
	//
	// A flag rather than a 503 for the page as a whole: the tool's own chrome still renders, which tells the
	// curator they are logged in and that the problem is downstream. A bare 503 would be indistinguishable
	// from a wrong password.
	Unavailable bool

	// MaxUploadMB is the per-file ceiling, printed so a photographer knows before they drag rather than after
	// a row turns red.
	MaxUploadMB int
}

// adminCountsView is the header's numbers.
//
// # Why these carry JSON tags
//
// The type is rendered by the page template *and* serialised by `GET /api/admin/photos`. Without tags the API
// exposes Go field names — `Total`, `InNoAlbum` — while the page's script reads `total`, so the header silently
// stopped updating after a batch. The template is indifferent either way, which is exactly why nothing caught
// it: the Go tests decode into this same struct, so the casing round-trips and only a browser notices.
//
// Found by looking at the live endpoint's output. Worth the note because the next person to add a field here
// will be looking at the template, not at the wire.
type adminCountsView struct {
	Total        int `json:"total"`
	InNoAlbum    int `json:"inNoAlbum"`
	WithLocation int `json:"withLocation"`
	Plottable    int `json:"plottable"`
	OutOfBounds  int `json:"outOfBounds"`
	Unknown      int `json:"unknown"`
	Tagged       int `json:"tagged"`
	Deleted      int `json:"deleted"`
}

// adminFilterView is one preset button over the contact sheet.
type adminFilterView struct {
	Label string
	Q     string
	On    bool
}

// adminFilters are the contact sheet's presets, in the order they are shown.
//
// In Go rather than written out in the markup since task 396, because the page now has to know which one its URL
// names: the album list links to `photos?album=none`, and a reload must keep what was chosen.
var adminFilters = []adminFilterView{
	{Label: "Alle", Q: ""},
	{Label: "Uden album", Q: "album=none"},
	{Label: "Uden position", Q: "location=no"},
	{Label: "Med position", Q: "location=yes"},
	{Label: "Uden for området", Q: "verdict=outside"},
	{Label: "Ikke vurderet", Q: "verdict=unknown"},
	{Label: "Med patrulje", Q: "tagged=yes"},
	{Label: "Uden patrulje", Q: "tagged=no"},
	{Label: "Inkl. slettede", Q: "deleted=1"},
}

// adminFiltersFor switches on the preset a query string names, and returns it.
//
// **Only an exact preset is honoured**; anything else lands on "Alle". The page never shows a grid narrowed by a
// query no button represents — the curator would be looking at a subset with nothing on screen saying so, and the
// action bar acts on what they see.
func adminFiltersFor(rawQuery string) ([]adminFilterView, string) {
	out := make([]adminFilterView, len(adminFilters))
	copy(out, adminFilters)
	on := 0
	for i, f := range out {
		if f.Q != "" && f.Q == rawQuery {
			on = i
		}
	}
	out[on].On = true
	return out, out[on].Q
}

// adminAlbumsPageHandler serves the tool's landing page: the counts and the album list (task 396).
//
// No OpenAPI annotations: an HTML page. See adminPhotosPageHandler.
func (app *application) adminAlbumsPageHandler(w http.ResponseWriter, r *http.Request) {
	app.renderAdminPage(w, r, adminPageData{View: "albums"})
}

// adminPhotosPageHandler serves upload and the contact sheet (task 396; before that, all of `/admin`).
//
// No OpenAPI annotations: this is an HTML page, and the annotation guard's scope is the JSON API (see
// glimtopenapi_test.go's isInScope, which task 380 widens to `/api/admin`).
func (app *application) adminPhotosPageHandler(w http.ResponseWriter, r *http.Request) {
	filters, query := adminFiltersFor(r.URL.RawQuery)
	app.renderAdminPage(w, r, adminPageData{View: "photos", Filters: filters, Query: query})
}

// renderAdminPage fills in what every view shows — the year, the counts — and renders it.
func (app *application) renderAdminPage(w http.ResponseWriter, r *http.Request, data adminPageData) {
	data.Year = adminYear(r)
	// The working year's prefix, not the public site's: a curator in 2025 is at `/2025/…` (task 392).
	data.Root = "/" + data.Year
	data.Years = app.workableYears
	data.EventYear = app.currentEventYear()
	data.MaxUploadMB = maxAdminUpload >> 20

	// Nil is the normal degraded state, not an error: no database means no library, and the curator should be
	// told that rather than shown a zero that looks like "nobody has uploaded anything".
	if app.models.PhotoCurator == nil {
		data.Unavailable = true
	} else {
		counts, err := app.models.PhotoCurator.Counts(adminYear(r))
		if err != nil {
			app.Logger.Error("reading the library counts for the admin page", "err", err)
			data.Unavailable = true
		} else {
			data.Counts = adminCountsView{
				Total:        counts.Total,
				InNoAlbum:    counts.InNoAlbum,
				WithLocation: counts.WithLocation,
				Plottable:    counts.Plottable,
				OutOfBounds:  counts.OutOfBounds,
				Unknown:      counts.Unknown,
				Tagged:       counts.Tagged,
				Deleted:      counts.Deleted,
			}
		}
	}

	// The security headers are already set by requireAdmin, which is where they belong: every response from
	// this surface carries them, including the ones that never reach a handler.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminTemplates.ExecuteTemplate(w, "admin", data); err != nil {
		// Logged rather than answered: ExecuteTemplate may have written a partial body already, so there is no
		// status left to set. Same reasoning as renderPublicPage's.
		app.Logger.Error("rendering the admin page", "err", err)
	}
}

// adminTemplates holds the tool's markup.
//
// # The markup, the CSS and the JavaScript live in adminui/ (task 394)
//
// They used to be one Go raw-string literal in this file — 1,700 lines of HTML, CSS and JavaScript inside
// backticks. That cost more than it looked:
//
//   - **A backtick anywhere in the CSS, JS or HTML terminated the literal.** Four separate incidents, each
//     presenting as a Go syntax error pointing at a line of CSS. Quoting an identifier in a comment, the most
//     natural thing to write, broke the build.
//   - No editor support for any of the three languages: no highlighting, no formatting, no linting, no
//     go-to-definition inside 1,400 lines of JavaScript.
//
// So `page.html`, `page.css` and `page.js` are now real files, embedded. The two `@inject` markers in
// page.html are replaced with the other two files' **source** before parsing, which is the detail that makes
// this safe: `html/template` still parses one document, so it still contextually escapes the fourteen actions
// in the markup. Splicing after parsing, or passing the CSS and JS in as template data, would have needed
// `template.CSS`/`template.JS` and thrown that escaping away.
//
// # Why page.css and page.js are valid on their own
//
// Neither contains a single template action — checked, and now asserted by
// `TestTheAdminAssetsCarryNoTemplateActions`. Every value the script needs is read from a `data-` attribute on
// an element, which was already the rule this file followed. That is what makes them real files rather than
// fragments: a formatter or linter can read them, because they are exactly what the browser gets.
//
// # The one Go-template hazard that remains
//
// `html/template` contextually escapes actions inside `<script>`, so interpolating template values into
// JavaScript is a way to get subtly mangled output. Nothing does, and nothing may: the assertion above is what
// keeps it that way. That is also why the year appears in the markup twice — once for a human and once for the
// uploader — rather than being passed through a variable.

//go:embed adminui/page.html adminui/page.css adminui/albumeditor.js adminui/albumorder.js adminui/captionaction.js
//go:embed adminui/main.js adminui/sheetshell.js adminui/contactsheet.js adminui/upload.js
//go:embed adminui/albumaction.js adminui/positionaction.js adminui/patrolaction.js
//go:embed adminui/creditaction.js adminui/deleteaction.js adminui/viewaction.js adminui/vieweredit.js
//go:embed adminui/fragments.html
//go:embed adminui/vendor/htmx.min.js adminui/vendor/alpine.min.js adminui/vendor/pico.min.css adminui/vendor/vendor.txt
var adminUIFS embed.FS

// adminPageScripts are the tool's own JavaScript files, spliced into page.html in this order (task 395).
//
// # Why there are nine files and one script tag
//
// This was 1,436 lines in one closure. Each file is now one `function init…(ctx)` declaration — a complete,
// valid JavaScript program a formatter or linter can read on its own, which was task 394's whole reason for
// making these real files rather than a Go string.
//
// They are still **spliced into one `<script>`**, not served as ES modules, for one reason: every asset on this
// surface answers with `no-store` (task 371), because it sits behind the shared credential. Nine module files
// would be nine uncacheable requests on every page load where this is zero, and the person waiting is a
// photographer on a hotel connection the day after the event.
//
// The order here is the order they appear in the script. It does not decide anything — function declarations
// hoist, and main.js decides what runs when — but main.js comes first so the file that explains the arrangement
// is the first thing read.
var adminPageScripts = []string{
	"adminui/main.js",
	"adminui/sheetshell.js",
	"adminui/contactsheet.js",
	"adminui/viewaction.js",
	"adminui/vieweredit.js",
	"adminui/albumaction.js",
	"adminui/positionaction.js",
	"adminui/patrolaction.js",
	"adminui/captionaction.js",
	"adminui/creditaction.js",
	"adminui/deleteaction.js",
	"adminui/upload.js",
	"adminui/albumeditor.js",
	"adminui/albumorder.js",
}

// adminTemplates is the page plus the htmx fragments (task 395), parsed as one set.
//
// One set rather than two, because a fragment is a piece of this page: parsing them together means the page's
// shell and the fragment it swaps in cannot drift onto different template syntax, and a broken fragment is a
// panic at init rather than a 500 the first time a curator presses a button.
var adminTemplates = template.Must(template.Must(template.New("admin").Funcs(adminTemplateFuncs).Parse(
	mustInjectAdminAssets("adminui/page.html", "adminui/page.css", adminPageScripts...),
)).Parse(mustReadAdminAsset("adminui/fragments.html")))

// adminTemplateFuncs are the Danish counts (task 387).
//
// The same definitions the public site registers and the Go sentences call, so a count cannot read one way in the
// curator's list and another on the frontpage — which is the failure that produced "1 billeder" there. See
// plural.go for why these are per-noun rather than one pluralise().
var adminTemplateFuncs = template.FuncMap{
	"photos": photoCount,
	"albums": albumCount,
	// The shared photo viewer's assets (task 402). The same function the public site's templates use, because it
	// is the same asset at the same URL — which is the entire point of PRD 023's "one implementation".
	"viewer": viewerAssetPath,
}

// mustInjectAdminAssets splices a page's CSS and its scripts into its HTML, ready to be parsed.
//
// The markers are `/* @inject page.css */` and one `// @inject <name>.js` per script, each a comment in its own
// language so the HTML file stays valid on its own. Every marker must be found: a typo in one would otherwise
// produce a page that renders with no styling or a feature silently missing, with no error anywhere — which is the
// failure mode this panics rather than tolerate. It runs once, at init, so a panic here is a binary that refuses to
// start, the right direction for an asset that cannot be assembled.
//
// The scripts go in **in the order given**, which is the order they will appear inside the one `<script>`.
func mustInjectAdminAssets(htmlPath, cssPath string, jsPaths ...string) string {
	html := mustReadAdminAsset(htmlPath)
	injects := []struct{ marker, path string }{
		{"/* @inject " + filepath.Base(cssPath) + " */", cssPath},
	}
	for _, js := range jsPaths {
		injects = append(injects, struct{ marker, path string }{"// @inject " + filepath.Base(js), js})
	}
	for _, inject := range injects {
		if !strings.Contains(html, inject.marker) {
			panic(fmt.Sprintf("adminui: %s has no %q marker", htmlPath, inject.marker))
		}
		html = strings.Replace(html, inject.marker, mustReadAdminAsset(inject.path), 1)
	}
	return html
}

func mustReadAdminAsset(path string) string {
	b, err := adminUIFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("adminui: %v", err))
	}
	return string(b)
}

// adminVendorAssets are the pinned third-party files the tool's pages load (task 395).
//
// Vendored and embedded rather than fetched from a CDN, for the reasons `adminui/vendor/README.md` gives at
// length — chiefly that the tool has to work on the Tuesday after the event, on a hotel connection, possibly
// behind something that blocks a CDN. A curator with three hundred photographs to hand in is not the person to
// discover that unpkg is unreachable.
//
// The content type is declared here rather than sniffed. `http.ServeContent` would guess from the extension,
// but an explicit map means a mis-served stylesheet is a wrong line in this table rather than a subtle
// behaviour of the standard library.
var adminVendorAssets = map[string]string{
	"htmx.min.js":   "application/javascript; charset=utf-8",
	"alpine.min.js": "application/javascript; charset=utf-8",
	"pico.min.css":  "text/css; charset=utf-8",
}

// serveAdminVendorHandler serves one pinned library by name.
//
// # Why these sit behind the admin credential
//
// They are public, unmodified libraries, so serving them openly would leak nothing — and they are behind
// `requireAdmin` anyway. `/admin/*` is the admin surface, and every response from it carries
// `Cache-Control: no-store` (task 371). Keeping that rule true without exceptions is worth more than saving a
// download: an exception here would be cited as precedent by the next thing that wanted one.
//
// The cost is ~180 KB re-fetched per full page load. For two or three curators, that is nothing.
//
// # Why the name is matched against a map rather than joined to a path
//
// The parameter comes from the URL. Joining it to a directory and reading the result is how a path traversal
// happens — `..%2f..%2fetc%2fpasswd` — and while `embed.FS` is not the host filesystem, it still holds this
// service's templates. A lookup in a fixed map cannot express anything that is not in the map.
func (app *application) serveAdminVendorHandler(w http.ResponseWriter, r *http.Request) {
	name := httprouter.ParamsFromContext(r.Context()).ByName("asset")
	contentType, known := adminVendorAssets[name]
	if !known {
		app.NotFoundResponse(w, r)
		return
	}

	body, err := adminUIFS.ReadFile("adminui/vendor/" + name)
	if err != nil {
		// Unreachable: the map and the embed directive are both compile-time, so a name in one is in the other.
		// Answered rather than asserted, because "unreachable" is a property of today's code.
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(body); err != nil {
		// A curator closing the tab mid-download. Nothing to answer with; the status is already sent.
		app.Logger.Debug("admin vendor asset write failed", "asset", name, "err", err)
	}
}
