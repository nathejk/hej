package main

import (
	"html/template"
	"net/http"
)

// The admin tool's page shell (PRD 022 §7, tasks 369–371).
//
// # What this is in Phase 2, and what it becomes
//
// The credential, the gating and the hardening (tasks 369–371) need something to protect in order to be
// testable at all: a door with no room behind it can be asserted about only in the abstract. So this is the
// shell — the year, the library's counts, and nothing else. The drop zone (task 373), the contact sheet
// (task 374) and the album editor (task 378) fill it in.
//
// It is deliberately not a stub that says "coming soon". The counts are read through the curator interface,
// so this page already exercises the whole stack end to end: credential → draft-visible read → rendered
// page. That is what makes the Phase 2 tests worth anything.
//
// # Why this does not use the public site's template or layout
//
// `publicsite.go` renders the public pages with a shared layout, an inline stylesheet and a deliberately low
// browser floor — no `oklch()`, no nesting, no custom properties — because those pages are served to
// whatever device a family owns, and they must work without JavaScript.
//
// None of that applies here, and inheriting it would be the wrong kind of consistency. This page is served
// to two or three organizers on laptops, behind a password, and the work it does (selecting hundreds of
// files, tracking uploads, multi-select) **requires** JavaScript. PRD 022 §7 records the trade: the admin
// page may use modern JavaScript freely; the public pages may not gain any.
//
// What it must not do is drag the public pages up or down with it, which is why the two do not share a
// template.
//
// # Danish, like the rest of the surface
//
// The people using this are Danish volunteers. Two sentences in this feature carry real weight and are worth
// writing carefully rather than generating: the difference between removing a photograph from an album and
// deleting it from the library (task 379), and what an out-of-bounds or unjudgeable position means
// (task 376). Neither is on this page yet.

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
	// curator that they are logged in and that the problem is downstream. A bare 503 would be
	// indistinguishable from a wrong password.
	Unavailable bool
}

// adminCountsView is the header's numbers, as strings the template can print without arithmetic.
type adminCountsView struct {
	Total        int
	InNoAlbum    int
	WithLocation int
	Plottable    int
	OutOfBounds  int
	Unknown      int
	Tagged       int
	Deleted      int
}

// adminIndexHandler serves the tool.
//
// No OpenAPI annotations: this is an HTML page, and the annotation guard's scope is the JSON API (see
// glimtopenapi_test.go's isInScope, which task 380 widens to `/api/admin`). The JSON endpoints that arrive in
// Phase 3 carry them.
func (app *application) adminIndexHandler(w http.ResponseWriter, r *http.Request) {
	data := adminPageData{Year: app.config.eventYear}

	// Nil is the normal degraded state, not an error: no database means no library, and the curator should
	// be told that rather than shown a zero that looks like "nobody has uploaded anything".
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

	// The headers are already set by requireAdmin, which is where they belong: every response from this
	// surface carries them, including the ones that never reach a handler.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminTemplates.ExecuteTemplate(w, "admin", data); err != nil {
		// Logged rather than answered: ExecuteTemplate may have written a partial body already, so there is
		// no status left to set. Same reasoning as renderPublicPage's.
		app.Logger.Error("rendering the admin page", "err", err)
	}
}

// adminTemplates holds the tool's markup.
//
// Inline, like the public site's, and for the same two reasons: there is no template directory or embed set
// in this service to join, and a page whose markup lives next to its handler is a page whose data type and
// its rendering cannot drift apart unnoticed.
var adminTemplates = template.Must(template.New("admin").Parse(`
{{define "admin"}}<!doctype html>
<html lang="da">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<!-- Belt and braces with the X-Robots-Tag header requireAdmin sets: a password-protected page in a search
     index is an invitation, and the cost of saying it twice is one line. -->
<meta name="robots" content="noindex, nofollow">
<title>Billedarkiv {{.Year}} — Nathejk</title>
<style>
:root { color-scheme: light dark; }
* { box-sizing: border-box; }
body {
  margin: 0;
  font: 16px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: #f4f4f5;
  color: #18181b;
}
header {
  display: flex; align-items: baseline; gap: 1rem; flex-wrap: wrap;
  padding: 1rem 1.5rem;
  background: #18181b; color: #fafafa;
}
/* Impact, as the app and the public site do for a major headline. Not a webfont: the admin tool has no
   build step and a headline face is not worth a network request on a hotel connection. */
h1 {
  margin: 0;
  font-family: Impact, Haettenschweiler, "Arial Narrow Bold", sans-serif;
  font-size: 1.75rem; font-weight: normal; letter-spacing: 0.02em;
}
.year {
  font-family: Impact, Haettenschweiler, "Arial Narrow Bold", sans-serif;
  font-size: 1.75rem;
  /* The year is the one thing on this page a curator cannot fix by editing, so it is the one thing given
     an accent colour. PRD 022 §5. */
  color: #fbbf24;
}
main { padding: 1.5rem; max-width: 60rem; }
.counts { display: flex; flex-wrap: wrap; gap: 0.5rem 1.5rem; padding: 0; margin: 0 0 1.5rem; list-style: none; }
.counts li { min-width: 7rem; }
.counts .n { display: block; font-size: 1.5rem; font-weight: 600; }
.counts .k { color: #52525b; font-size: 0.875rem; }
.warn { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 0.75rem 1rem; margin-bottom: 1.5rem; }
.todo { color: #52525b; }
</style>
</head>
<body>
<header>
  <h1>Billedarkiv</h1>
  <span class="year">{{.Year}}</span>
</header>
<main>
{{if .Unavailable}}
  <p class="warn">Billedarkivet kan ikke læses lige nu. Du er logget ind, men databasen svarer ikke —
  prøv igen om et øjeblik.</p>
{{else}}
  <ul class="counts">
    <li><span class="n">{{.Counts.Total}}</span><span class="k">billeder</span></li>
    <li><span class="n">{{.Counts.InNoAlbum}}</span><span class="k">uden album</span></li>
    <li><span class="n">{{.Counts.WithLocation}}</span><span class="k">med position</span></li>
    <li><span class="n">{{.Counts.Plottable}}</span><span class="k">på kortet</span></li>
    <li><span class="n">{{.Counts.OutOfBounds}}</span><span class="k">uden for området</span></li>
    <li><span class="n">{{.Counts.Unknown}}</span><span class="k">ikke vurderet</span></li>
    <li><span class="n">{{.Counts.Tagged}}</span><span class="k">med patrulje</span></li>
    <li><span class="n">{{.Counts.Deleted}}</span><span class="k">slettede</span></li>
  </ul>
{{end}}
  <p class="todo">Upload og redigering kommer her.</p>
</main>
</body>
</html>{{end}}
`))
