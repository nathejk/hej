package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
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

// adminIndexHandler serves the tool.
//
// No OpenAPI annotations: this is an HTML page, and the annotation guard's scope is the JSON API (see
// glimtopenapi_test.go's isInScope, which task 380 widens to `/api/admin`).
func (app *application) adminIndexHandler(w http.ResponseWriter, r *http.Request) {
	data := adminPageData{
		Year:        app.config.eventYear,
		MaxUploadMB: maxAdminUpload >> 20,
	}

	// Nil is the normal degraded state, not an error: no database means no library, and the curator should be
	// told that rather than shown a zero that looks like "nobody has uploaded anything".
	if app.models.PhotoCurator == nil {
		data.Unavailable = true
	} else {
		counts, err := app.models.PhotoCurator.Counts(app.config.eventYear)
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

//go:embed adminui/page.html adminui/page.css adminui/page.js adminui/album.html adminui/album.css adminui/album.js
var adminUIFS embed.FS

var adminTemplates = template.Must(template.New("admin").Parse(
	mustInjectAdminAssets("adminui/page.html", "adminui/page.css", "adminui/page.js"),
))

// mustInjectAdminAssets splices a page's CSS and JS source into its HTML, ready to be parsed.
//
// The markers are `/* @inject page.css */` and `// @inject page.js`, each a comment in its own language so the
// HTML file stays valid on its own. Both must be found: a typo in a marker would otherwise produce a page that
// renders with no styling or no behaviour and no error anywhere, which is the failure mode this panics rather
// than tolerate. It runs once, at init, so a panic here is a binary that refuses to start — the right direction
// for an asset that cannot be assembled.
func mustInjectAdminAssets(htmlPath, cssPath, jsPath string) string {
	html := mustReadAdminAsset(htmlPath)
	for _, inject := range []struct{ marker, path string }{
		{"/* @inject " + filepath.Base(cssPath) + " */", cssPath},
		{"// @inject " + filepath.Base(jsPath), jsPath},
	} {
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
