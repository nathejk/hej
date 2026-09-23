package main

import (
	"html/template"
	"net/http"
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
// Inline, like the public site's, for the same two reasons: there is no template directory or embed set in
// this service to join, and a page whose markup lives beside its handler cannot drift from its data type
// unnoticed.
//
// # The one Go-template hazard in here
//
// `html/template` contextually escapes actions inside `<script>`, so interpolating template values into
// JavaScript is a way to get subtly mangled output. Nothing does: every value the script needs is read from a
// `data-` attribute on an element. That is also why the year appears in the markup twice — once for a human
// and once for the uploader — rather than being passed through a variable.
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
<!-- The map island's Leaflet, **vendored and self-hosted** like the public patrol page's (task 342). Not a CDN:
     see scripts/vendor-leaflet.sh for the reasoning, which applies here too — and the same files, so this page
     adds no new dependency and no second mapping library. This is the one exception to the page's
     no-build-step rule (PRD 022 §7), and it is an exception because reusing the island is cheaper than
     introducing a second way to draw a map. -->
<link rel="stylesheet" href="/vendor/leaflet.css">
<script src="/vendor/leaflet.js" defer></script>
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
  position: sticky; top: 0; z-index: 10;
}
/* Impact, as the app and the public site do for a major headline. Not a webfont: this page has no build step
   and a headline face is not worth a network request. */
h1 {
  margin: 0;
  font-family: Impact, Haettenschweiler, "Arial Narrow Bold", sans-serif;
  font-size: 1.75rem; font-weight: normal; letter-spacing: 0.02em;
}
/* The year is the one thing on this page a curator cannot fix by editing, so it is the one thing given an
   accent colour and its own size. PRD 022 §5. */
.year {
  font-family: Impact, Haettenschweiler, "Arial Narrow Bold", sans-serif;
  font-size: 1.75rem; color: #fbbf24;
}
main { padding: 1.5rem; max-width: 72rem; }
h2 { font-family: Impact, Haettenschweiler, "Arial Narrow Bold", sans-serif;
     font-weight: normal; font-size: 1.25rem; letter-spacing: 0.02em; margin: 2rem 0 0.75rem; }

.counts { display: flex; flex-wrap: wrap; gap: 0.5rem 1.5rem; padding: 0; margin: 0 0 1rem; list-style: none; }
.counts li { min-width: 7rem; }
.counts .n { display: block; font-size: 1.5rem; font-weight: 600; }
.counts .k { color: #52525b; font-size: 0.875rem; }
.warn { background: #fef3c7; border-left: 4px solid #f59e0b; padding: 0.75rem 1rem; margin-bottom: 1.5rem; }

/* The drop zone is a thin bar when idle and grows during a batch (PRD 022 §7). Driven by a class on the
   section rather than by inline styles, so the two states are readable in one place. */
#drop {
  border: 2px dashed #a1a1aa; border-radius: 0.5rem; background: #fff;
  padding: 0.75rem 1rem; transition: padding 0.15s ease, background 0.15s ease;
}
#drop.idle { padding: 0.75rem 1rem; }
#drop.busy, #drop.hover { padding: 1.5rem 1rem; }
#drop.hover { background: #eff6ff; border-color: #2563eb; }
#drop .hint { color: #52525b; font-size: 0.9375rem; margin: 0; }
#drop .row1 { display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
/* A real label pointing at a real input: the whole control is keyboard reachable and screen-reader
   labelled without any JavaScript, which is the accessibility requirement in PRD 022 §6. The input is
   visually hidden rather than display:none, because display:none takes it out of the tab order. */
.filelabel {
  display: inline-flex; align-items: center; gap: 0.5rem;
  background: #18181b; color: #fafafa; padding: 0.5rem 0.875rem; border-radius: 0.375rem;
  cursor: pointer; font-weight: 500;
}
.filelabel:focus-within { outline: 3px solid #2563eb; outline-offset: 2px; }
#files { position: absolute; width: 1px; height: 1px; opacity: 0; overflow: hidden; }
.progress { flex: 1 1 12rem; min-width: 10rem; }
.progress .bar { height: 0.5rem; background: #e4e4e7; border-radius: 999px; overflow: hidden; }
.progress .bar > i { display: block; height: 100%; width: 0; background: #16a34a; transition: width 0.2s ease; }
.progress .label { font-size: 0.875rem; color: #52525b; }

#rows { list-style: none; padding: 0; margin: 1rem 0 0; display: grid; gap: 0.25rem; }
#rows li {
  display: grid; grid-template-columns: 3rem 1fr auto; gap: 0.75rem; align-items: center;
  padding: 0.375rem 0.5rem; background: #fff; border-radius: 0.375rem; font-size: 0.9375rem;
}
#rows li.err { background: #fef2f2; }
#rows li.skip { background: #f4f4f5; color: #52525b; }
#rows img, #rows .noimg {
  width: 3rem; height: 3rem; object-fit: cover; border-radius: 0.25rem; background: #e4e4e7;
}
#rows .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
#rows .why { color: #b91c1c; font-size: 0.875rem; }
#rows li.pending img { opacity: 0.45; }
.state { font-size: 0.875rem; white-space: nowrap; display: flex; align-items: center; gap: 0.5rem; }
/* The four position verdicts read differently on purpose. PRD 011 §6 requires a rejected coordinate to be
   visible *as rejected* rather than merely absent from the map, and 'unknown' is a statement about us
   rather than about the photograph — so it must not look like a rejection either. */
.pos { font-size: 0.8125rem; padding: 0.0625rem 0.375rem; border-radius: 999px; }
.pos.inside { background: #dcfce7; color: #166534; }
.pos.outside { background: #fee2e2; color: #991b1b; }
.pos.unknown { background: #fef3c7; color: #92400e; }
.todo { color: #52525b; }

/* --- the contact sheet ---------------------------------------------------- */
#filters { display: flex; flex-wrap: wrap; gap: 0.375rem; margin-bottom: 0.75rem; }
.f {
  font: inherit; font-size: 0.875rem; cursor: pointer;
  background: #fff; color: #18181b; border: 1px solid #d4d4d8;
  padding: 0.3125rem 0.75rem; border-radius: 999px;
}
.f:hover { border-color: #71717a; }
.f.on { background: #18181b; color: #fafafa; border-color: #18181b; }
.f:focus-visible, #sheet:focus-visible, .cell:focus-visible, button:focus-visible {
  outline: 3px solid #2563eb; outline-offset: 2px;
}

#sheet {
  display: grid; gap: 0.375rem;
  grid-template-columns: repeat(auto-fill, minmax(8rem, 1fr));
}
.cell {
  position: relative; cursor: pointer; background: #fff; border-radius: 0.375rem;
  overflow: hidden; border: 3px solid transparent; padding: 0;
}
.cell img { display: block; width: 100%; aspect-ratio: 4 / 3; object-fit: cover; background: #e4e4e7; }
/* Selection is a border plus a tick, never colour alone: a curator working through a card for an hour should
   not have to compare shades to know what a bulk action is about to apply to. */
.cell[aria-selected="true"] { border-color: #2563eb; }
.cell[aria-selected="true"] .tick { display: grid; }
.tick {
  display: none; position: absolute; top: 0.25rem; left: 0.25rem;
  width: 1.25rem; height: 1.25rem; border-radius: 999px;
  background: #2563eb; color: #fff; place-items: center; font-size: 0.75rem;
}
.marks { position: absolute; bottom: 0.25rem; left: 0.25rem; right: 0.25rem;
         display: flex; gap: 0.25rem; flex-wrap: wrap; }
.mark { font-size: 0.6875rem; padding: 0.0625rem 0.3125rem; border-radius: 999px; background: #27272aee; color: #fafafa; }
.mark.outside { background: #b91c1ccc; }
.mark.unknown { background: #b45309cc; }
.mark.inside { background: #15803dcc; }
.cell.gone { opacity: 0.5; }
.cell.gone img { filter: grayscale(1); }

#actions {
  position: sticky; bottom: 0; z-index: 9;
  display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap;
  margin-top: 1rem; padding: 0.75rem 1rem;
  background: #18181b; color: #fafafa; border-radius: 0.5rem;
}
#actions button {
  font: inherit; font-size: 0.875rem; cursor: pointer;
  background: #3f3f46; color: #fafafa; border: 1px solid #52525b;
  padding: 0.375rem 0.75rem; border-radius: 0.375rem;
}
#actions button:disabled { opacity: 0.5; cursor: not-allowed; }
#selcount { font-weight: 600; margin-right: auto; }

/* The action sheets (task 390).

   # Overlays, not cards stacked under the contact sheet

   They used to reveal inline below the grid. With a selection made near the bottom of a 120-thumbnail page
   that opened the panel off-screen: the curator pressed a button and nothing appeared to happen.

   Still **one page**, which is the hard constraint PRD 022 §7 imposes — navigating away would lose the
   selection, so none of these may become a route. An overlay satisfies that; a card underneath was never the
   requirement, only the first implementation of it.

   One '.sheet' rule rather than four near-identical id blocks. The previous version repeated the same chrome
   for #panel, #pospanel, #tagpanel and #delpanel, which is four places for a fifth action to be styled
   slightly differently. */
#scrim {
  position: fixed; inset: 0; background: rgba(9, 9, 11, 0.55);
  z-index: 40;
}
.sheet {
  position: fixed; z-index: 50;
  top: 50%; left: 50%; transform: translate(-50%, -50%);
  width: calc(100vw - 2rem); max-width: 34rem;
  /* Tall sheets scroll inside themselves rather than growing past the viewport — the position sheet holds an
     18rem map, and on a laptop in landscape that used to push its own buttons off the bottom. */
  max-height: calc(100vh - 2rem); overflow-y: auto;
  padding: 1rem; background: #fff;
  border: 1px solid #d4d4d8; border-radius: 0.5rem;
  box-shadow: 0 10px 40px rgba(9, 9, 11, 0.25);
}
.sheet h3 { margin: 0 0 0.5rem; font-size: 1rem; }
.sheet h4 { margin: 0 0 0.25rem; font-size: 0.9375rem; }
.sheet p { margin: 0.5rem 0; }
.sheet .hint { color: #52525b; font-size: 0.875rem; }
.sheet button {
  font: inherit; font-size: 0.875rem; cursor: pointer;
  background: #18181b; color: #fafafa; border: 1px solid #18181b;
  padding: 0.375rem 0.75rem; border-radius: 0.375rem;
}
.sheet button:disabled { opacity: 0.5; cursor: not-allowed; }
/* The secondary buttons: closing, and the actions that only look something up. */
.sheet button.ghost { background: #fff; color: #18181b; border-color: #d4d4d8; }
.sheet input[type=text], .sheet select {
  font: inherit; padding: 0.375rem 0.5rem; border: 1px solid #d4d4d8; border-radius: 0.375rem;
}
.sheet input[type=text] { min-width: 14rem; }
/* Body scroll is locked while a sheet is open, or the backdrop scrolls the grid underneath it. */
body.sheetopen { overflow: hidden; }

#albumlist { display: grid; gap: 0.25rem; max-height: 14rem; overflow: auto; }
#albumlist label { display: flex; gap: 0.5rem; align-items: baseline; }
#albumlist .draft { color: #92400e; font-size: 0.8125rem; }
#albumlist .count { color: #52525b; font-size: 0.8125rem; }

/* The album list (task 391). A grid of cards rather than a table: the cover is how a curator recognises an
   album, and a table would put it in a column. */
#albums { display: grid; gap: 0.75rem; grid-template-columns: repeat(auto-fill, minmax(15rem, 1fr)); }
#albums .card {
  display: flex; gap: 0.625rem; align-items: flex-start;
  padding: 0.625rem; background: #fff;
  border: 1px solid #e4e4e7; border-radius: 0.5rem;
}
/* A draft is marked on the card itself, not only by a badge, so "what is not published" is answerable by
   glancing rather than by reading every row. */
#albums .card.draft { border-color: #fcd34d; background: #fffbeb; }
#albums .card.gone { border-color: #e4e4e7; background: #fafafa; opacity: 0.65; }
#albums .cover {
  width: 4rem; height: 4rem; flex: none; object-fit: cover;
  border-radius: 0.375rem; background: #f4f4f5;
}
/* An album with no live items has no cover. A dashed placeholder rather than a blank gap, so the row reads as
   "empty album" instead of "image failed to load". */
#albums .cover.none { border: 1px dashed #d4d4d8; }
#albums .meta { min-width: 0; flex: 1; }
#albums .t { font-weight: 600; display: block; overflow-wrap: anywhere; }
#albums .s { color: #52525b; font-size: 0.8125rem; display: block; }
#albums .badge {
  display: inline-block; font-size: 0.75rem; padding: 0.0625rem 0.375rem;
  border-radius: 0.25rem; margin-right: 0.25rem;
}
#albums .badge.pub { background: #dcfce7; color: #166534; }
#albums .badge.drafty { background: #fef3c7; color: #92400e; }
#albums .badge.del { background: #f4f4f5; color: #52525b; }
#albums .acts { display: flex; gap: 0.375rem; margin-top: 0.375rem; flex-wrap: wrap; }
#albums .acts a, #albums .acts button {
  font: inherit; font-size: 0.8125rem; cursor: pointer; text-decoration: none;
  background: #fff; color: #18181b; border: 1px solid #d4d4d8;
  padding: 0.1875rem 0.5rem; border-radius: 0.375rem;
}
#albums .acts button:disabled { opacity: 0.5; cursor: not-allowed; }
#newalbumbtn {
  font: inherit; font-size: 0.875rem; cursor: pointer;
  background: #18181b; color: #fafafa; border: 1px solid #18181b;
  padding: 0.375rem 0.75rem; border-radius: 0.375rem;
}

#posmap { height: 18rem; border-radius: 0.375rem; margin: 0.5rem 0; }
#tagpanel input[type=text] { width: 8rem; min-width: 0; }
#tagfound.ok { color: #166534; font-weight: 500; }

/* The delete sheet. The two choices are visually separated and the destructive one is marked, because the
   whole point of it is that they are different acts (PRD 022 §5). */
#delpanel p { font-size: 0.9375rem; }
#delpanel .choice { border: 1px solid #e4e4e7; border-radius: 0.375rem; padding: 0.75rem; margin: 0.75rem 0; }
#delpanel .choice.danger { border-color: #fca5a5; background: #fef2f2; }
#delpanel label { display: block; font-size: 0.875rem; color: #52525b; }
#delpanel input[type=text] { width: 100%; }
/* Its buttons default to secondary, so the one red button is the only thing that looks like an action. */
#delpanel button { background: #fff; color: #18181b; border-color: #d4d4d8; }
/* The destructive action is the only red button in the tool. */
#delpanel button.danger { background: #b91c1c; color: #fff; border-color: #b91c1c; font-weight: 500; }
svg { width: 1.125em; height: 1.125em; stroke: currentColor; fill: none;
      stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }
</style>
</head>
<body>
<header>
  <h1>Billedarkiv</h1>
  <span class="year" data-year="{{.Year}}">{{.Year}}</span>
</header>
<main>
{{if .Unavailable}}
  <p class="warn">Billedarkivet kan ikke læses lige nu. Du er logget ind, men databasen svarer ikke —
  prøv igen om et øjeblik.</p>
{{else}}
  <ul class="counts" id="counts">
    <li><span class="n" data-count="total">{{.Counts.Total}}</span><span class="k">billeder</span></li>
    <li><span class="n" data-count="inNoAlbum">{{.Counts.InNoAlbum}}</span><span class="k">uden album</span></li>
    <li><span class="n" data-count="withLocation">{{.Counts.WithLocation}}</span><span class="k">med position</span></li>
    <li><span class="n" data-count="plottable">{{.Counts.Plottable}}</span><span class="k">på kortet</span></li>
    <li><span class="n" data-count="outOfBounds">{{.Counts.OutOfBounds}}</span><span class="k">uden for området</span></li>
    <li><span class="n" data-count="unknown">{{.Counts.Unknown}}</span><span class="k">ikke vurderet</span></li>
    <li><span class="n" data-count="tagged">{{.Counts.Tagged}}</span><span class="k">med patrulje</span></li>
    <li><span class="n" data-count="deleted">{{.Counts.Deleted}}</span><span class="k">slettede</span></li>
  </ul>
{{end}}

  <h2>Læg billeder op</h2>
  <section id="drop" class="idle" data-max-mb="{{.MaxUploadMB}}" aria-describedby="drophint">
    <div class="row1">
      <label class="filelabel" for="files">
        <!-- Lucide: image-up -->
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10.3 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v7"/><path d="m21 15-3-3-3 3"/><path d="M18 12v9"/><circle cx="9" cy="9" r="2"/></svg>
        Vælg billeder
        <input type="file" id="files" multiple accept="image/*">
      </label>
      <div class="progress" hidden id="prog">
        <div class="bar"><i id="bar"></i></div>
        <div class="label" id="proglabel"></div>
      </div>
    </div>
    <p class="hint" id="drophint">Træk en mappe eller en bunke filer herind. Op til {{.MaxUploadMB}} MB pr. billede.
    Uploader du de samme filer igen, bliver de ikke lagt op to gange.</p>
  </section>

  <ul id="rows"></ul>

  <!-- The album list (task 391).

       It sits above the contact sheet because it answers the question a curator opens this page with — "what
       have I not published yet?" — whereas the sheet answers "what have I not sorted yet?".

       Before this, the only album markup on the page was the assignment checkboxes inside the add-to-album
       sheet, and the editor at /admin/album/{slug} was reachable only by typing a slug. So publishing was
       built and unusable. -->
  <h2>Album</h2>
  <p id="albumsnote" class="hint" aria-live="polite"></p>
  <div id="albums" data-year="{{.Year}}"></div>
  <p>
    <button type="button" id="newalbumbtn">Opret et album</button>
  </p>

  <h2>Kontaktark</h2>
  <div id="filters" role="group" aria-label="Filtre">
    <button type="button" class="f on" data-q="">Alle</button>
    <button type="button" class="f" data-q="album=none">Uden album</button>
    <button type="button" class="f" data-q="location=no">Uden position</button>
    <button type="button" class="f" data-q="location=yes">Med position</button>
    <button type="button" class="f" data-q="verdict=outside">Uden for området</button>
    <button type="button" class="f" data-q="verdict=unknown">Ikke vurderet</button>
    <button type="button" class="f" data-q="tagged=yes">Med patrulje</button>
    <button type="button" class="f" data-q="tagged=no">Uden patrulje</button>
    <button type="button" class="f" data-q="deleted=1">Inkl. slettede</button>
  </div>

  <p id="sheetnote" class="todo" aria-live="polite"></p>
  <p><button type="button" id="selall">Vælg alle der matcher filteret</button></p>
  <div id="sheet" role="listbox" aria-multiselectable="true" aria-label="Billeder" tabindex="0"></div>
  <p><button type="button" id="more" hidden>Hent flere</button></p>

  <!-- The action bar is pinned rather than a page of its own, because navigating away loses the selection
       (PRD 022 §7). Its four actions arrive with tasks 375–379; the bar and the selection they act on are
       this task's. -->
  <div id="actions" hidden aria-live="polite">
    <span id="selcount"></span>
    <button type="button" data-act="album">Tilføj til album</button>
    <button type="button" data-act="position">Sæt position</button>
    <button type="button" data-act="patrol">Tag patrulje</button>
    <button type="button" data-act="delete">Fjern eller slet</button>
    <button type="button" id="clearsel">Ryd valg</button>
  </div>

  <!-- The sheets open **over** the page rather than on their own route, because navigating away loses the
       selection (PRD 022 §7). That is the one hard constraint on this layout, and it is why these are
       overlays rather than pages — not why they were once cards underneath (task 390).

       The scrim is a sibling rather than each sheet's own backdrop, so exactly one can be open. -->
  <div id="scrim" hidden></div>

  <div id="panel" class="sheet" hidden role="dialog" aria-modal="true" aria-label="Tilføj til album">
    <h3>Tilføj til album</h3>
    <p id="panelnote"></p>
    <div id="albumlist"></div>
    <details id="newalbum">
      <summary>Opret et nyt album</summary>
      <p>
        <label for="newtitle">Titel</label>
        <input type="text" id="newtitle" maxlength="120" placeholder="Lørdag morgen">
        <button type="button" class="ghost" id="createalbum">Opret</button>
      </p>
      <p class="hint">Nye album er <strong>ikke</strong> udgivet. Du udgiver dem, når de er færdige.</p>
    </details>
    <p>
      <button type="button" id="doadd">Tilføj</button>
      <button type="button" class="ghost" id="closepanel">Annuller</button>
    </p>
  </div>

  <!-- The position panel (task 376). Inline like the album one, for the same reason: navigating away loses the
       selection, and the selection is the input to the action. -->
  <div id="pospanel" class="sheet" hidden role="dialog" aria-modal="true" aria-label="Sæt position">
    <h3>Sæt position</h3>
    <p id="posnote"></p>
    <p>
      <label for="cppick">Vælg en post</label>
      <select id="cppick">
        <option value="">— eller klik på kortet —</option>
      </select>
    </p>
    <!-- The map island's container, hidden until Leaflet actually draws — the same treatment the public patrol
         page gives it. Where the island cannot run, the post picker above is still a complete way to do the
         job, which is what keeps the map an enhancement rather than a requirement. -->
    <div id="posmap" hidden></div>
    <p id="pospicked" class="hint"></p>
    <p>
      <button type="button" id="doposition" disabled>Sæt position</button>
      <button type="button" class="ghost" id="doclear">Fjern position</button>
      <button type="button" class="ghost" id="closepos">Annuller</button>
    </p>
  </div>
  <!-- The patrol tag panel (task 377). No picker and no roster: the curator types the number from the sign and
       the tool confirms which patrol it is. There is deliberately no endpoint that lists patrols. -->
  <div id="tagpanel" class="sheet" hidden role="dialog" aria-modal="true" aria-label="Tag patrulje">
    <h3>Tag patrulje</h3>
    <p id="tagnote"></p>
    <p>
      <label for="tagnum">Patruljens nummer</label>
      <input type="text" id="tagnum" inputmode="numeric" maxlength="8" placeholder="42" autocomplete="off">
      <button type="button" class="ghost" id="lookuppatrol">Find</button>
    </p>
    <p id="tagfound" class="hint"></p>
    <p>
      <button type="button" id="dotag" disabled>Tag billederne</button>
      <button type="button" class="ghost" id="closetag">Annuller</button>
    </p>
  </div>
  <!-- The delete panel (task 379). **The copy here is the substance, not decoration.**

       PRD 022 §5 and §7 both single it out: removing a photograph from an album leaves it in the library and in
       every other album, while deleting it from the library removes it from all of them — and one of those two is
       what an organizer means when they say "take it down". If the two read alike, the wrong one gets pressed
       under exactly the pressure that makes it matter.

       So this panel offers **both**, states plainly what each does, and makes the destructive one visually
       distinct. The safe one is offered first, because it is the one a curator usually wants. -->
  <div id="delpanel" class="sheet" hidden role="dialog" aria-modal="true" aria-label="Fjern eller slet">
    <h3>Fjern eller slet</h3>
    <p id="delnote"></p>

    <div class="choice">
      <h4>Fjern fra et album</h4>
      <p>Billederne bliver taget ud af det album du vælger. <strong>De bliver liggende i arkivet</strong> og i de
      andre album de ligger i, og du kan lægge dem tilbage når som helst.</p>
      <p>
        <label for="delalbum">Album</label>
        <select id="delalbum"><option value="">— vælg album —</option></select>
        <button type="button" id="doremove" disabled>Fjern fra albummet</button>
      </p>
    </div>

    <div class="choice danger">
      <h4>Slet fra arkivet</h4>
      <p><strong>Billederne forsvinder helt.</strong> De bliver fjernet fra alle album, de kan ikke ses
      offentligt, og filerne bliver slettet. Det er det du skal bruge, hvis nogen har bedt om at få et billede
      taget ned.</p>
      <p>Det kan <strong>ikke</strong> fortrydes herfra — kun en udvikler kan hente et slettet billede frem igen.</p>
      <p>
        <label for="delreason">Hvorfor? (valgfrit, gemmes i loggen)</label>
        <input type="text" id="delreason" maxlength="500" placeholder="En forælder har bedt om det">
      </p>
      <p>
        <button type="button" id="dodelete" class="danger">Slet <span id="delcount"></span> fra arkivet</button>
      </p>
    </div>

    <p><button type="button" class="ghost" id="closedel">Annuller</button></p>
  </div>
</main>
<script>
// The uploader (task 373).
//
// # One request per file, three at a time
//
// Not an implementation detail — it is the design (PRD 022 §6). One bad file fails alone instead of taking 299
// with it; a dropped connection costs the file in flight rather than the afternoon; and because the server
// derives a photograph's id from its bytes (task 372), re-dragging the same folder is a correct recovery
// procedure rather than a way to duplicate a card.
//
// Three is a deliberate number. One is needlessly slow over a decent connection; ten saturates an uplink so
// that every file slows down together and the browser's own progress becomes meaningless. Three keeps the
// pipe busy while leaving each request's progress legible.
(() => {
  'use strict';

  const CONCURRENCY = 3;

  const drop = document.getElementById('drop');
  const input = document.getElementById('files');
  const rows = document.getElementById('rows');
  const prog = document.getElementById('prog');
  const bar = document.getElementById('bar');
  const progLabel = document.getElementById('proglabel');

  let queued = 0, done = 0, running = 0;
  const queue = [];

  // --- the queue ------------------------------------------------------------

  function enqueue(files) {
    for (const file of files) {
      queued++;
      queue.push({ file, li: addRow(file) });
    }
    render();
    pump();
  }

  function pump() {
    while (running < CONCURRENCY && queue.length > 0) {
      running++;
      const job = queue.shift();
      upload(job).finally(() => {
        running--;
        done++;
        render();
        // Recurse rather than loop, so a finished job immediately starts the next one instead of waiting for
        // the whole wave to drain.
        pump();
      });
    }
  }

  function render() {
    const active = queued > 0 && done < queued;
    drop.classList.toggle('busy', active);
    drop.classList.toggle('idle', !active);
    prog.hidden = queued === 0;
    const pct = queued === 0 ? 0 : Math.round((done / queued) * 100);
    bar.style.width = pct + '%';
    progLabel.textContent = active
      ? done + ' af ' + queued + ' — ' + (queued - done) + ' tilbage'
      : (queued === 0 ? '' : 'Færdig: ' + done + ' af ' + queued);

    // When a batch finishes, refresh the sheet so the new photographs are there to sort. Without this the
    // curator uploads a card and then has to work out that the grid needs reloading, which is the kind of
    // small friction that makes a tool feel broken.
    if (!active && queued > 0 && done === queued && typeof load === 'function') load(true);
  }

  // --- one file -------------------------------------------------------------

  async function upload(job) {
    const body = new FormData();
    body.append('photo', job.file);

    try {
      const res = await fetch('/api/admin/photos', { method: 'POST', body });
      let payload = null;
      try { payload = await res.json(); } catch (_) { /* an error page rather than JSON */ }

      if (!res.ok) {
        // The server writes the reason, in Danish, because it is the side that knows why. The status is only
        // consulted for the cases that have no body — a proxy timing out, for instance.
        finishRow(job.li, 'err', (payload && (payload.error || payload.message)) || httpReason(res.status));
        return;
      }
      applyOutcome(job.li, payload);
    } catch (err) {
      // A dropped connection, a closed laptop, a tunnel. Said plainly, and the row invites the one recovery
      // that actually works: drag it again.
      finishRow(job.li, 'err', 'Forbindelsen blev afbrudt. Træk filen ind igen.');
    }
  }

  function applyOutcome(li, out) {
    if (!out) { finishRow(li, 'err', 'Uventet svar fra serveren.'); return; }

    // Three outcomes, three appearances. A duplicate reported as a plain success would make a duplicated card
    // impossible to notice, and a skipped deletion reported as success would be a lie (task 372).
    if (out.outcome === 'already') {
      finishRow(li, 'skip', '', out, 'Allerede lagt op');
      return;
    }
    if (out.outcome === 'deleted') {
      finishRow(li, 'skip', out.message || '', out, 'Slettet tidligere');
      return;
    }
    finishRow(li, 'ok', '', out, 'Lagt op');
  }

  function httpReason(status) {
    if (status === 413) return 'Filen er for stor (over ' + drop.dataset.maxMb + ' MB).';
    if (status === 400) return 'Filen er ikke et billede vi kan læse.';
    if (status === 401) return 'Du er blevet logget ud. Genindlæs siden.';
    if (status === 507) return 'Der er ikke plads på serveren. BEHOLD KORTET og sig det til en udvikler.';
    if (status === 429) return 'For mange forsøg. Vent et øjeblik og træk filerne ind igen.';
    if (status === 503) return 'Arkivet er ikke tilgængeligt lige nu. Prøv igen om et øjeblik.';
    return 'Fejl ' + status + '.';
  }

  // --- rows -----------------------------------------------------------------

  function addRow(file) {
    const li = document.createElement('li');
    li.className = 'pending';

    // The preview is the **local** file, not a fetch of the stored rendition. No round trip, instant, and it
    // shows the photographer what they actually selected. The server's own rendition is the contact sheet's
    // job (task 374). Revoked on load so a 300-file batch does not hold 300 decoded bitmaps.
    let thumb;
    if (file.type && file.type.startsWith('image/')) {
      thumb = document.createElement('img');
      thumb.alt = '';
      const url = URL.createObjectURL(file);
      thumb.src = url;
      thumb.addEventListener('load', () => URL.revokeObjectURL(url), { once: true });
      thumb.addEventListener('error', () => URL.revokeObjectURL(url), { once: true });
    } else {
      thumb = document.createElement('div');
      thumb.className = 'noimg';
    }

    const name = document.createElement('span');
    name.className = 'name';
    name.textContent = file.name || '(uden navn)';

    const state = document.createElement('span');
    state.className = 'state';
    state.textContent = 'Uploader…';

    li.append(thumb, name, state);
    rows.prepend(li);
    return li;
  }

  function finishRow(li, kind, why, out, label) {
    li.classList.remove('pending');
    li.classList.toggle('err', kind === 'err');
    li.classList.toggle('skip', kind === 'skip');

    const state = li.querySelector('.state');
    state.textContent = '';

    if (why) {
      const w = document.createElement('span');
      w.className = 'why';
      w.textContent = why;
      state.append(w);
    }
    if (label) {
      const l = document.createElement('span');
      l.textContent = label;
      state.append(l);
    }
    // The position badge, when the file carried a coordinate. Four states that must read differently:
    // 'inside' is plottable, 'outside' was judged not at the event, and 'unknown' means we had no race area
    // to judge it against — a statement about us, so it must not look like a rejection of the photograph.
    if (out && out.location) {
      const v = out.location.boundsVerdict;
      const p = document.createElement('span');
      p.className = 'pos ' + (v === 'inside' ? 'inside' : v === 'outside' ? 'outside' : 'unknown');
      p.textContent = v === 'inside' ? 'har position'
        : v === 'outside' ? 'uden for området'
        : 'position kunne ikke vurderes';
      state.append(p);
    }
  }

  // --- input and drag-and-drop ---------------------------------------------

  input.addEventListener('change', () => {
    if (input.files && input.files.length) enqueue(Array.from(input.files));
    // Cleared so selecting the same folder twice fires 'change' again — which a photographer retrying after a
    // dropped connection will do, and which would otherwise silently do nothing.
    input.value = '';
  });

  for (const type of ['dragenter', 'dragover']) {
    drop.addEventListener(type, (e) => { e.preventDefault(); drop.classList.add('hover'); });
  }
  for (const type of ['dragleave', 'drop']) {
    drop.addEventListener(type, () => drop.classList.remove('hover'));
  }

  drop.addEventListener('drop', async (e) => {
    e.preventDefault();
    const items = e.dataTransfer && e.dataTransfer.items;

    // A dropped *folder* only yields its contents through the entries API; 'dataTransfer.files' is empty for
    // a directory. Since the stated use is "drag a folder onto the page", the entries path is the primary one
    // and 'files' is the fallback for browsers or drops that do not offer entries.
    if (items && items.length && items[0].webkitGetAsEntry) {
      const entries = [];
      for (const item of items) {
        const entry = item.webkitGetAsEntry();
        if (entry) entries.push(entry);
      }
      const files = [];
      for (const entry of entries) await walk(entry, files);
      if (files.length) enqueue(files);
      return;
    }
    if (e.dataTransfer && e.dataTransfer.files.length) enqueue(Array.from(e.dataTransfer.files));
  });

  // walk collects every file under an entry, recursively.
  //
  // 'readEntries' returns at most a hundred entries per call and signals the end with an empty batch, which is
  // why this loops rather than reading once — a card's worth of photographs in one folder is exactly the case
  // a single read gets wrong, and it would silently upload the first hundred.
  async function walk(entry, out) {
    if (entry.isFile) {
      const file = await new Promise((res, rej) => entry.file(res, rej)).catch(() => null);
      // Skip the hidden files every card and every operating system leaves behind: .DS_Store, ._originals,
      // Thumbs.db. The server would refuse them correctly, but as three hundred red rows nobody can read past.
      if (file && !file.name.startsWith('.') && file.name !== 'Thumbs.db') out.push(file);
      return;
    }
    if (!entry.isDirectory) return;

    const reader = entry.createReader();
    for (;;) {
      const batch = await new Promise((res) => reader.readEntries(res, () => res([])));
      if (!batch.length) break;
      for (const child of batch) await walk(child, out);
    }
  }

  // --- the contact sheet (task 374) ----------------------------------------
  //
  // # The selection is the tool's real state
  //
  // Everything in the action bar acts on it, so it is held as a Set of ids rather than read off the DOM. Two
  // reasons, and the second is the one that matters: a DOM-derived selection would be lost by any re-render,
  // and it would silently shrink to "what is currently loaded" — which would make "select all matching this
  // filter" a lie the moment the grid was paged.

  const sheet = document.getElementById('sheet');
  const filters = document.getElementById('filters');
  const note = document.getElementById('sheetnote');
  const more = document.getElementById('more');
  const actions = document.getElementById('actions');
  const selCount = document.getElementById('selcount');
  const clearSel = document.getElementById('clearsel');

  const selected = new Set();
  // The order cells are shown in, so shift-click can resolve a range. Kept separately from the DOM for the
  // reason above.
  let order = [];
  let lastClicked = null;
  let query = '';
  let offset = 0;
  let loading = false;

  // --- the sheet shell (task 390) ---------------------------------------------
  //
  // One place that opens and closes an overlay, rather than each action hiding the other three by hand — which
  // is what this replaced, four assignments per opener, and four more to forget when a fifth action lands.
  //
  // The keyboard handling is not decoration. A curator tagging three hundred photographs works fast and from
  // the keyboard; a dialog that traps nothing and returns focus nowhere makes that impossible.
  const scrim = document.getElementById('scrim');
  // The element focus returns to when the sheet closes. Without it, closing a sheet drops focus to the top of
  // the document and the next Tab starts from the page header.
  let sheetOpener = null;
  let openSheetEl = null;

  function openSheet(el, opener) {
    if (openSheetEl && openSheetEl !== el) closeSheet({ restore: false });
    sheetOpener = opener || document.activeElement;
    openSheetEl = el;
    scrim.hidden = false;
    el.hidden = false;
    document.body.classList.add('sheetopen');
    focusFirst(el);
  }

  function closeSheet(opts) {
    if (!openSheetEl) return;
    openSheetEl.hidden = true;
    openSheetEl = null;
    scrim.hidden = true;
    document.body.classList.remove('sheetopen');
    // Restored unless one sheet is handing over to another, where the opener is about to be replaced anyway.
    if (!opts || opts.restore !== false) {
      if (sheetOpener && sheetOpener.isConnected) sheetOpener.focus();
      sheetOpener = null;
    }
  }

  function focusables(el) {
    return Array.prototype.filter.call(
      el.querySelectorAll('button, [href], input, select, textarea, summary, [tabindex]:not([tabindex="-1"])'),
      (n) => !n.disabled && n.offsetParent !== null
    );
  }

  function focusFirst(el) {
    const f = focusables(el);
    if (f.length) f[0].focus();
  }

  // Escape closes, and Tab cycles inside the open sheet instead of walking off into the contact sheet behind
  // it. Bound once on the document rather than per sheet.
  document.addEventListener('keydown', (e) => {
    if (!openSheetEl) return;
    if (e.key === 'Escape') { e.preventDefault(); closeSheet(); return; }
    if (e.key !== 'Tab') return;
    const f = focusables(openSheetEl);
    if (!f.length) return;
    const first = f[0], last = f[f.length - 1];
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  });

  // Clicking the backdrop closes. Deliberately **not** on the delete sheet: a stray click next to a
  // confirmation that says how many photographs it will delete should not be how that dialog goes away.
  scrim.addEventListener('click', () => {
    if (openSheetEl && openSheetEl.id === 'delpanel') return;
    closeSheet();
  });

  function syncActions() {
    const n = selected.size;
    actions.hidden = n === 0;
    selCount.textContent = n === 1 ? '1 valgt' : n + ' valgte';
    // Every action is wired now (tasks 375–379).
    //
    // Closing the bar closes the sheet with it: an action on an empty selection is a button that cannot do
    // anything.
    if (n === 0) closeSheet();
  }

  function paint(cell) {
    cell.setAttribute('aria-selected', selected.has(cell.dataset.id) ? 'true' : 'false');
  }

  function toggle(id, on) {
    if (on === undefined) on = !selected.has(id);
    if (on) selected.add(id); else selected.delete(id);
    const cell = sheet.querySelector('[data-id="' + id + '"]');
    if (cell) paint(cell);
    syncActions();
  }

  // Shift-click selects the range between the last click and this one, which is how a curator picks "these
  // forty from Post 3" — PRD 022 §3 calls that the true shape of the work.
  function selectRange(toId) {
    const a = order.indexOf(lastClicked);
    const b = order.indexOf(toId);
    if (a < 0 || b < 0) { toggle(toId, true); return; }
    const [lo, hi] = a < b ? [a, b] : [b, a];
    for (let i = lo; i <= hi; i++) selected.add(order[i]);
    for (const cell of sheet.querySelectorAll('.cell')) paint(cell);
    syncActions();
  }

  function cellFor(p) {
    const cell = document.createElement('button');
    cell.type = 'button';
    cell.className = 'cell' + (p.deleted ? ' gone' : '');
    cell.dataset.id = p.id;
    cell.setAttribute('role', 'option');
    cell.setAttribute('aria-selected', 'false');

    const img = document.createElement('img');
    img.loading = 'lazy';
    img.decoding = 'async';
    // Addressed by id and variant, never by ref. The server resolves it through the projection, which is what
    // stops this route being a file server for the whole blob store (see adminlibrary.go).
    img.src = '/api/admin/photos/' + encodeURIComponent(p.id) + '/media?variant=thumb';
    img.alt = p.caption || '';

    const tick = document.createElement('span');
    tick.className = 'tick';
    tick.setAttribute('aria-hidden', 'true');
    tick.textContent = '✓';

    const marks = document.createElement('span');
    marks.className = 'marks';
    // The position marks. Three states that must read differently, and 'unknown' must not read as a rejection:
    // it is a statement about us — there was no race area to judge against — so blaming the photograph would
    // be wrong. 'none' gets no mark at all, because having no coordinate is the ordinary case and a badge for
    // it would be noise on most of the card.
    if (p.boundsVerdict === 'inside') marks.append(mark('inside', 'position'));
    else if (p.boundsVerdict === 'outside') marks.append(mark('outside', 'uden for området'));
    else if (p.boundsVerdict === 'unknown') marks.append(mark('unknown', 'ikke vurderet'));

    if (p.albumCount > 0) marks.append(mark('', p.albumCount === 1 ? '1 album' : p.albumCount + ' album'));
    if (p.tagCount > 0) marks.append(mark('', 'patrulje'));
    if (p.deleted) marks.append(mark('outside', 'slettet'));

    cell.append(img, tick, marks);

    cell.addEventListener('click', (e) => {
      if (e.shiftKey && lastClicked) selectRange(p.id);
      else { toggle(p.id); lastClicked = p.id; }
    });
    return cell;
  }

  function mark(kind, text) {
    const s = document.createElement('span');
    s.className = 'mark' + (kind ? ' ' + kind : '');
    s.textContent = text;
    return s;
  }

  async function load(reset) {
    if (loading) return;
    loading = true;
    if (reset) { offset = 0; order = []; sheet.textContent = ''; lastClicked = null; }

    const url = '/api/admin/photos?limit=120&offset=' + offset + (query ? '&' + query : '');
    try {
      const res = await fetch(url);
      if (!res.ok) {
        note.textContent = res.status === 401
          ? 'Du er blevet logget ud. Genindlæs siden.'
          : 'Kunne ikke hente billederne (fejl ' + res.status + ').';
        return;
      }
      const data = await res.json();

      for (const p of data.photos) {
        order.push(p.id);
        sheet.append(cellFor(p));
      }
      offset += data.photos.length;
      more.hidden = !data.hasMore;
      updateCounts(data.counts);

      note.textContent = order.length === 0
        ? 'Ingen billeder matcher.'
        : order.length + ' vist' + (data.hasMore ? ' — der er flere' : '');

      // Re-paint, because a filter change may bring back a photograph that is still selected. The selection
      // deliberately survives filtering: a curator narrows to "uden position", picks forty, then widens to
      // check something — losing the forty would make the filters unusable as a working tool.
      for (const cell of sheet.querySelectorAll('.cell')) paint(cell);
      syncActions();
    } catch (err) {
      note.textContent = 'Kunne ikke hente billederne. Prøv igen.';
    } finally {
      loading = false;
    }
  }

  // The header is refreshed wholesale from the payload, keyed by the JSON field name in a data attribute.
  //
  // Every number rather than just the total, because they are read together: "312 billeder / 47 uden album"
  // with a stale second figure is worse than no figure, since the curator uses it to decide what to sort next.
  // Keyed by attribute rather than by id so adding a count is a template change and nothing else.
  function updateCounts(c) {
    if (!c) return;
    for (const el of document.querySelectorAll('[data-count]')) {
      const v = c[el.dataset.count];
      if (typeof v === 'number') el.textContent = String(v);
    }
  }

  filters.addEventListener('click', (e) => {
    const b = e.target.closest('.f');
    if (!b) return;
    for (const other of filters.querySelectorAll('.f')) other.classList.toggle('on', other === b);
    query = b.dataset.q;
    load(true);
  });

  more.addEventListener('click', () => load(false));

  // "Select everything matching this filter", beyond what is on screen.
  //
  // PRD 022 §6 requires it, and the reasoning in §3 is the point: "these forty are from Post 3" is the true
  // shape of the work, and a selection model that topped out at the loaded page would push the curator back to
  // doing it forty times — which is how it does not get done.
  //
  // It pages the ids rather than adding a bespoke endpoint: the same read, the same filter, one field used. A
  // dedicated "all ids" route would be a second place for the filter to be interpreted, and the two
  // disagreeing is precisely how a bulk action lands on the wrong photographs.
  const selAll = document.getElementById('selall');
  selAll.addEventListener('click', async () => {
    selAll.disabled = true;
    const before = selected.size;
    try {
      let off = 0;
      for (;;) {
        const url = '/api/admin/photos?limit=500&offset=' + off + (query ? '&' + query : '');
        const res = await fetch(url);
        if (!res.ok) { note.textContent = 'Kunne ikke hente alle billeder (fejl ' + res.status + ').'; return; }
        const data = await res.json();
        for (const p of data.photos) selected.add(p.id);
        off += data.photos.length;
        // Guard against a server that keeps saying "more" — an empty page ends the loop regardless, so a bug
        // upstream cannot spin here forever.
        if (!data.hasMore || data.photos.length === 0) break;
      }
      for (const cell of sheet.querySelectorAll('.cell')) paint(cell);
      syncActions();
      note.textContent = (selected.size - before) + ' billeder lagt til valget — ' + selected.size + ' valgte i alt';
    } catch (err) {
      note.textContent = 'Kunne ikke hente alle billeder. Prøv igen.';
    } finally {
      selAll.disabled = false;
    }
  });
  clearSel.addEventListener('click', () => {
    selected.clear();
    for (const cell of sheet.querySelectorAll('.cell')) paint(cell);
    syncActions();
    sheet.focus();
  });

  // Keyboard: the grid is a listbox, arrows move focus between cells and space toggles. Required by PRD 022
  // §6 — two or three people use this for hours, and a selection model reachable only by mouse is the usual
  // way that becomes painful.
  sheet.addEventListener('keydown', (e) => {
    const cells = Array.from(sheet.querySelectorAll('.cell'));
    if (!cells.length) return;
    const current = document.activeElement.closest ? document.activeElement.closest('.cell') : null;
    let i = current ? cells.indexOf(current) : -1;

    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') { i = Math.min(cells.length - 1, i + 1); }
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') { i = Math.max(0, i - 1); }
    else if (e.key === ' ' || e.key === 'Enter') {
      if (current) { e.preventDefault(); toggle(current.dataset.id); lastClicked = current.dataset.id; }
      return;
    } else if ((e.key === 'a' || e.key === 'A') && (e.metaKey || e.ctrlKey)) {
      // Select everything currently loaded. "Everything matching the filter" beyond the loaded page is the
      // explicit button below, because a keystroke that silently selects rows the curator has not seen is a
      // bulk action waiting to surprise them.
      e.preventDefault();
      for (const id of order) selected.add(id);
      for (const cell of cells) paint(cell);
      syncActions();
      return;
    } else {
      return;
    }

    e.preventDefault();
    cells[i].focus();
  });

  // --- the album list (task 391) -------------------------------------------
  //
  // # Why this exists at all
  //
  // The album editor — /admin/album/{slug}, where publishing, the title, the description, the order and the
  // captions live — had **no link from anywhere**. It was reachable only by typing a slug into the address bar,
  // so the whole publish half of PRD 022 §5 was built and unusable. Reported that way.
  //
  // # Why publishing is here and not only in the editor
  //
  // "Publish this" is the common case and needs no editing. Making a curator open a page to press one button,
  // when they are looking at a list that already says which albums are drafts, is the friction that makes a
  // tool feel like a form.
  //
  // Drafts and **deleted** albums are shown, not filtered. This is the curator's read (task 366): the public
  // one hides them, and the difference between the two is the whole reason there are two.

  const albumsEl = document.getElementById('albums');
  const albumsNote = document.getElementById('albumsnote');

  async function loadAlbums() {
    albumsNote.textContent = 'Henter album…';
    try {
      const res = await fetch('/api/admin/albums');
      if (!res.ok) {
        albumsNote.textContent = 'Kunne ikke hente album (fejl ' + res.status + ').';
        return;
      }
      const data = await res.json();
      renderAlbums(data.albums || []);
    } catch (err) {
      albumsNote.textContent = 'Kunne ikke hente album. Prøv igen.';
    }
  }

  function renderAlbums(albums) {
    albumsEl.textContent = '';
    if (!albums.length) {
      albumsNote.textContent = 'Der er ingen album endnu.';
      return;
    }
    const drafts = albums.filter((a) => !a.published && !a.deleted).length;
    // The count a curator actually wants: not "5 albums" but "2 of them are not published".
    albumsNote.textContent = drafts === 0
      ? 'Alle album er udgivet.'
      : (drafts === 1 ? '1 album er ikke udgivet endnu.' : drafts + ' album er ikke udgivet endnu.');

    for (const a of albums) albumsEl.append(albumCard(a));
  }

  function albumCard(a) {
    const card = document.createElement('div');
    card.className = 'card' + (a.deleted ? ' gone' : (a.published ? '' : ' draft'));
    card.dataset.album = a.albumId;

    // The cover, through the admin media route rather than the public one: this list shows unpublished albums,
    // and the public route would — correctly — refuse their photographs.
    if (a.coverPhotoId) {
      const img = document.createElement('img');
      img.className = 'cover';
      img.src = '/api/admin/photos/' + encodeURIComponent(a.coverPhotoId) + '/media?variant=thumb';
      img.alt = '';
      img.loading = 'lazy';
      card.append(img);
    } else {
      const ph = document.createElement('div');
      ph.className = 'cover none';
      card.append(ph);
    }

    const meta = document.createElement('div');
    meta.className = 'meta';

    const t = document.createElement('span');
    t.className = 't';
    t.textContent = a.title;
    meta.append(t);

    const badges = document.createElement('span');
    badges.className = 's';
    badges.append(badge(a.deleted ? 'del' : (a.published ? 'pub' : 'drafty'),
      a.deleted ? 'Slettet' : (a.published ? 'Udgivet' : 'Kladde')));
    // The count is live items only, so an album whose photographs were all deleted reads 0 — rendered rather
    // than hidden, because "why is this album empty" is a question with an answer the curator needs.
    const n = document.createElement('span');
    n.textContent = a.itemCount === 1 ? '1 billede' : a.itemCount + ' billeder';
    badges.append(n);
    meta.append(badges);

    const acts = document.createElement('div');
    acts.className = 'acts';

    const edit = document.createElement('a');
    edit.href = '/admin/album/' + encodeURIComponent(a.slug);
    edit.textContent = 'Redigér';
    acts.append(edit);

    // A deleted album gets no publish button. Restoring one is not built (PRD 022 §11 Q6's neighbour), and a
    // button that would publish something taken down is the wrong thing to offer.
    if (!a.deleted) {
      const pub = document.createElement('button');
      pub.type = 'button';
      pub.textContent = a.published ? 'Fjern fra forsiden' : 'Udgiv på forsiden';
      pub.addEventListener('click', () => setPublished(a, pub));
      acts.append(pub);
    }

    if (a.published && !a.deleted) {
      const view = document.createElement('a');
      view.href = '/' + albumsEl.dataset.year + '/album/' + encodeURIComponent(a.slug);
      view.target = '_blank';
      view.rel = 'noopener';
      view.textContent = 'Se den';
      acts.append(view);
    }

    meta.append(acts);
    card.append(meta);
    return card;
  }

  function badge(kind, text) {
    const b = document.createElement('span');
    b.className = 'badge ' + kind;
    b.textContent = text;
    return b;
  }

  async function setPublished(a, btn) {
    btn.disabled = true;
    const want = !a.published;
    try {
      // Publication is sent **alone** (task 378): a curator pressing this has said one thing, and bundling it
      // with the album's other fields would let a half-typed title ride along with it.
      const res = await fetch('/api/admin/albums/' + encodeURIComponent(a.albumId), {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ published: want }),
      });
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        albumsNote.textContent = (payload && payload.error) || 'Kunne ikke ændre albummet.';
        btn.disabled = false;
        return;
      }
      // The fold is asynchronous, so a reload now would show the old state. Said rather than papered over — the
      // editor page carries the same sentence, and without it a curator presses the button twice.
      albumsNote.textContent = want
        ? 'Albummet er udgivet. Det slår igennem på forsiden inden for et minut.'
        : 'Albummet er taget af forsiden. Det slår igennem inden for et minut.';
      a.published = want;
      btn.textContent = want ? 'Fjern fra forsiden' : 'Udgiv på forsiden';
      btn.disabled = false;
      const card = albumsEl.querySelector('[data-album="' + a.albumId + '"]');
      if (card) card.className = 'card' + (want ? '' : ' draft');
    } catch (err) {
      albumsNote.textContent = 'Kunne ikke ændre albummet. Prøv igen.';
      btn.disabled = false;
    }
  }

  // Creating an album from the list, rather than only from inside the add-to-album sheet. The sheet's version
  // exists because a curator creates an album *in order to* file a selection into it; this one exists because
  // sometimes you just want the album.
  document.getElementById('newalbumbtn').addEventListener('click', async () => {
    const title = (window.prompt('Titel på det nye album') || '').trim();
    if (!title) return;
    albumsNote.textContent = 'Opretter…';
    try {
      const res = await fetch('/api/admin/albums', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        albumsNote.textContent = (payload && payload.error) || 'Kunne ikke oprette albummet.';
        return;
      }
      await loadAlbums();
      albumsNote.textContent = 'Albummet “' + payload.title + '” er oprettet som kladde.';
    } catch (err) {
      albumsNote.textContent = 'Kunne ikke oprette albummet. Prøv igen.';
    }
  });

  loadAlbums();

  // --- the add-to-album action (task 375) ----------------------------------
  //
  // The sheet opens over the page and the selection survives it, which is the constraint PRD 022 §7 puts on
  // this layout: navigating away would lose the selection, and the selection is the input to every action.

  const panel = document.getElementById('panel');
  const panelNote = document.getElementById('panelnote');
  const albumList = document.getElementById('albumlist');
  const newTitle = document.getElementById('newtitle');

  async function openAlbumPanel() {
    openSheet(panel);
    panelNote.textContent = 'Henter album…';
    albumList.textContent = '';

    try {
      const res = await fetch('/api/admin/albums');
      if (!res.ok) { panelNote.textContent = 'Kunne ikke hente album (fejl ' + res.status + ').'; return; }
      const data = await res.json();

      const live = data.albums.filter((a) => !a.deleted);
      if (!live.length) {
        panelNote.textContent = 'Der er ingen album endnu. Opret et nedenfor.';
      } else {
        panelNote.textContent = 'Vælg et eller flere album. ' + selected.size +
          (selected.size === 1 ? ' billede bliver lagt i dem.' : ' billeder bliver lagt i dem.');
      }

      for (const a of live) {
        const label = document.createElement('label');
        const cb = document.createElement('input');
        cb.type = 'checkbox';
        cb.value = a.albumId;
        const name = document.createElement('span');
        name.textContent = a.title;
        label.append(cb, name);
        // A draft is marked, because "why is it not on the frontpage" is the question a curator asks after
        // filing forty photographs into an album they never published.
        if (!a.published) {
          const d = document.createElement('span');
          d.className = 'draft';
          d.textContent = 'kladde';
          label.append(d);
        }
        const c = document.createElement('span');
        c.className = 'count';
        c.textContent = a.itemCount + ' billeder';
        label.append(c);
        albumList.append(label);
      }
    } catch (err) {
      panelNote.textContent = 'Kunne ikke hente album. Prøv igen.';
    }
  }

  document.getElementById('closepanel').addEventListener('click', () => {
    closeSheet();
  });

  document.getElementById('createalbum').addEventListener('click', async () => {
    const title = newTitle.value.trim();
    if (!title) { panelNote.textContent = 'Albummet skal have en titel.'; return; }

    try {
      const res = await fetch('/api/admin/albums', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        panelNote.textContent = (payload && payload.error) || 'Kunne ikke oprette albummet.';
        return;
      }
      newTitle.value = '';
      await openAlbumPanel();
      // The list above is now stale by one album. Refreshed rather than left — a curator who creates an album
      // here and then looks for it in the list should find it.
      loadAlbums();
      // Tick the album just created, since creating one inline is something a curator does *in order to* file
      // the current selection into it.
      for (const cb of albumList.querySelectorAll('input[type=checkbox]')) {
        if (cb.value === payload.albumId) cb.checked = true;
      }
      panelNote.textContent = 'Albummet “' + payload.title + '” er oprettet som kladde.';
    } catch (err) {
      panelNote.textContent = 'Kunne ikke oprette albummet. Prøv igen.';
    }
  });

  document.getElementById('doadd').addEventListener('click', async () => {
    const albumIds = Array.from(albumList.querySelectorAll('input:checked')).map((cb) => cb.value);
    if (!albumIds.length) { panelNote.textContent = 'Vælg mindst ét album.'; return; }
    if (!selected.size) { panelNote.textContent = 'Vælg mindst ét billede.'; return; }

    panelNote.textContent = 'Tilføjer…';
    try {
      const res = await fetch('/api/admin/albums/items', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: Array.from(selected), albumIds }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        // A 503 means nothing was filed, and the selection is deliberately kept so the curator can retry
        // without picking forty photographs again.
        panelNote.textContent = (payload && payload.error) || 'Kunne ikke tilføje billederne.';
        return;
      }

      // The server writes the sentence, because it is the side that knows how many were already there.
      note.textContent = payload.message || 'Tilføjet.';
      closeSheet();
      selected.clear();
      // Reloaded so the album marks on the thumbnails are right, which is how the curator sees what is left
      // to sort.
      load(true);
    } catch (err) {
      panelNote.textContent = 'Kunne ikke tilføje billederne. Prøv igen.';
    }
  });

  actions.addEventListener('click', (e) => {
    const b = e.target.closest('button[data-act]');
    if (!b || b.disabled) return;
    if (b.dataset.act === 'album') openAlbumPanel();
    if (b.dataset.act === 'position') openPositionPanel();
    if (b.dataset.act === 'patrol') openTagPanel();
    if (b.dataset.act === 'delete') openDeletePanel();
  });

  // --- the bulk position (task 376) -----------------------------------------
  //
  // Two ways to give the point, and the post picker is the one a curator actually uses: they know "Post 3", not
  // a coordinate. The id is sent rather than the coordinate, so the server resolves it — a stale coordinate in
  // this browser must not become a pin on a public map.

  const posPanel = document.getElementById('pospanel');
  const posNote = document.getElementById('posnote');
  const cpPick = document.getElementById('cppick');
  const posMapEl = document.getElementById('posmap');
  const posPicked = document.getElementById('pospicked');
  const doPosition = document.getElementById('doposition');

  // The point chosen by clicking the map, if any. A post chosen in the select wins, because it is the more
  // precise statement of intent — and the server is told which of the two, never both.
  let clicked = null;
  let posMap = null;
  let posMarker = null;

  function describeChoice() {
    const cp = cpPick.value;
    if (cp) {
      const name = cpPick.options[cpPick.selectedIndex].textContent;
      posPicked.textContent = 'Valgt: ' + name;
      doPosition.disabled = false;
      return;
    }
    if (clicked) {
      posPicked.textContent = 'Valgt på kortet: ' + clicked.lat.toFixed(5) + ', ' + clicked.lng.toFixed(5);
      doPosition.disabled = false;
      return;
    }
    posPicked.textContent = '';
    doPosition.disabled = true;
  }

  // # Why the map is drawn last, and why it is measured again after (task 390)
  //
  // Leaflet computes its pixel size when the map is created and caches it. Creating one inside a container
  // that is hidden — or that the browser has not laid out yet — gives it a size of zero, and the symptom is a
  // map that loads one tile in the corner and ignores every drag. So 'drawPositionMap' runs after 'openSheet'
  // has made the sheet visible, and 'invalidateSize' is called on the frame after that, once layout has
  // settled. It was safe while the panel was an inline card that was already in flow; it is not safe now.
  async function openPositionPanel() {
    openSheet(posPanel);
    clicked = null;
    cpPick.value = '';
    describeChoice();
    posNote.textContent = selected.size === 1
      ? '1 billede får positionen.'
      : selected.size + ' billeder får positionen.';

    await loadCheckpoints();
    await drawPositionMap();
    if (posMap) requestAnimationFrame(() => posMap.invalidateSize());
  }

  async function loadCheckpoints() {
    try {
      const res = await fetch('/api/admin/checkpoints');
      if (!res.ok) return;
      const data = await res.json();

      // Rebuilt each time the panel opens, because posts get sited during the season and a stale list would
      // offer a post that no longer resolves.
      while (cpPick.options.length > 1) cpPick.remove(1);
      for (const c of data.checkpoints) {
        const o = document.createElement('option');
        o.value = c.id;
        o.textContent = c.name;
        o.dataset.lat = c.lat;
        o.dataset.lng = c.lng;
        cpPick.append(o);
      }
      // An empty list is the ordinary early-season state, and it is the same condition that makes a
      // curator-placed point unjudgeable — so the two are explained together rather than leaving the curator to
      // connect them.
      if (!data.checkpoints.length) {
        posNote.textContent += ' Ingen poster har en placering endnu, så positionen kan ikke vurderes' +
          ' — billederne kommer ikke på kortet før posterne er sat.';
      }
    } catch (err) {
      // The picker is an aid, not the mechanism: clicking the map still works.
    }
  }

  // The map island, reusing the vendored Leaflet and the shared layer config the app and the public map use
  // (task 353), so the curator places points on the same base map everybody else sees.
  //
  // # Why this is more than three lines (task 389)
  //
  // The first version read 'layers.layers[0]' and drew nothing at all, because 'maplayers.json' keys 'layers'
  // by **layer id** ('dtk25', 'dtk50', 'orto') rather than as an array — so 'base' was always 'undefined' and
  // the container sat empty. It also skipped three things the layer needs to actually render:
  //
  //   - the **token**. These are Dataforsyningen WMS endpoints and refuse an unauthenticated request; the
  //     token comes from '/api/config', the same place the Vue app reads it.
  //   - the 'layers' and 'format' **parameters**. A WMS URL without them is not a tile request.
  //   - **retry**. Leaflet has none: one failed image leaves that tile grey until something recreates it, and
  //     the file's own comment records that patchy rural data makes that the normal case rather than the
  //     exception. Ported from 'EventMap.vue''s 'attachTileRetry' rather than reinvented — same backoff, same
  //     jitter, same cache-buster, and the numbers come from the shared file so the two cannot drift.
  //
  // This page has no build step, so the app's TypeScript cannot be imported. The mitigation is that every
  // value is read from 'maplayers.json' at runtime: if the layer definitions or the retry policy change, this
  // map follows without an edit here.
  async function drawPositionMap() {
    if (typeof L === 'undefined') return; // Leaflet blocked or still loading: the picker is enough
    if (posMap) { posMap.invalidateSize(); return; }

    let cfg = null, token = '';
    try {
      const [layersRes, confRes] = await Promise.all([
        fetch('/maplayers.json'),
        fetch('/api/config'),
      ]);
      if (layersRes.ok) cfg = await layersRes.json();
      if (confRes.ok) token = (await confRes.json()).dataforsyningen_token || '';
    } catch (err) { /* fall through: an empty map is still clickable */ }

    posMapEl.hidden = false;
    const minZoom = (cfg && cfg.minZoom) || 7;
    const maxZoom = (cfg && cfg.maxZoom) || 18;
    posMap = L.map(posMapEl, { minZoom: minZoom, maxZoom: maxZoom }).setView([55.7332, 12.2648], 11);

    // The file's own default, by key. Falling back to the first entry rather than to a hard-coded id, so a
    // renamed default does not empty the map again — which is the bug this function had.
    const defs = (cfg && cfg.layers) || {};
    const key = (cfg && cfg.default && defs[cfg.default]) ? cfg.default : Object.keys(defs)[0];
    const base = key ? defs[key] : null;

    if (base && base.url) {
      const layer = L.tileLayer.wms(base.url, {
        layers: base.layer,
        format: base.format,
        transparent: false,
        crossOrigin: 'anonymous',
        attribution: base.attribution || (cfg && cfg.attribution) || '',
        token: token,
        maxZoom: maxZoom,
      });
      attachTileRetry(layer, (cfg && cfg.retry) || {});
      layer.addTo(posMap);
    } else {
      // Said out loud rather than left as a grey rectangle. The picker below still works, so this is a
      // degraded map and not a broken panel — but a curator staring at an empty square should be told which.
      posNote.textContent = 'Kortet kan ikke hentes lige nu. Du kan stadig vælge en post i listen.';
    }

    posMap.on('click', (e) => {
      clicked = { lat: e.latlng.lat, lng: e.latlng.lng };
      // Choosing on the map clears the post, so the two cannot both be sent — the server refuses that, and it
      // should never have to.
      cpPick.value = '';
      if (posMarker) posMarker.remove();
      posMarker = L.marker(e.latlng).addTo(posMap);
      describeChoice();
    });
  }

  // Retry a failed tile a few times before giving up, with exponential backoff and jitter.
  //
  // A port of EventMap.vue's attachTileRetry, deliberately faithful rather than simplified:
  //
  //   - re-assigning 'src' on the **same** <img> keeps Leaflet's own load/error handlers attached, so a late
  //     success still fades the tile in normally. Creating a new image would lose that.
  //   - the '_retry=' suffix defeats negative caching of the failed response by the browser or a proxy.
  //   - jitter stops a whole screen of failed tiles retrying in lockstep and hammering the service.
  //   - 'isConnected' guards a tile Leaflet has since discarded by panning or a layer swap.
  function attachTileRetry(layer, retry) {
    const limit = retry.limit || 3;
    const baseDelay = retry.baseDelayMs || 400;
    const jitter = retry.jitterMs || 250;

    layer.on('tileerror', (event) => {
      const tile = event.tile;
      if (!tile) return;
      const attempt = (tile._hejRetries || 0) + 1;
      if (attempt > limit) return;
      tile._hejRetries = attempt;

      const delay = baseDelay * Math.pow(2, attempt - 1) + Math.random() * jitter;
      const original = tile.src.replace(/&_retry=\d+$/, '');
      window.setTimeout(() => {
        if (!tile.isConnected) return;
        tile.src = original + '&_retry=' + attempt;
      }, delay);
    });
  }

  cpPick.addEventListener('change', () => {
    if (cpPick.value) {
      clicked = null;
      if (posMarker) { posMarker.remove(); posMarker = null; }
      const o = cpPick.options[cpPick.selectedIndex];
      if (posMap && o.dataset.lat) {
        const ll = [parseFloat(o.dataset.lat), parseFloat(o.dataset.lng)];
        posMarker = L.marker(ll).addTo(posMap);
        posMap.setView(ll, 14);
      }
    }
    describeChoice();
  });

  document.getElementById('closepos').addEventListener('click', () => { closeSheet(); });

  async function sendPosition(payload) {
    posNote.textContent = 'Gemmer…';
    try {
      const res = await fetch('/api/admin/photos', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(Object.assign({ photoIds: Array.from(selected) }, payload)),
      });
      const out = await res.json().catch(() => null);
      if (!res.ok) {
        posNote.textContent = (out && out.error) || 'Kunne ikke gemme positionen.';
        return;
      }
      // The server writes the sentence, because it is the side that ran the bounds check and knows which of the
      // three verdicts happened — and two of them are refusals the curator must not mistake for a fault at
      // their end.
      note.textContent = out.message || 'Gemt.';
      closeSheet();
      selected.clear();
      load(true);
    } catch (err) {
      posNote.textContent = 'Kunne ikke gemme positionen. Prøv igen.';
    }
  }

  doPosition.addEventListener('click', () => {
    if (cpPick.value) { sendPosition({ checkpointId: cpPick.value }); return; }
    if (clicked) { sendPosition({ location: { lat: clicked.lat, lng: clicked.lng } }); return; }
    posNote.textContent = 'Vælg en post eller klik på kortet.';
  });

  document.getElementById('doclear').addEventListener('click', () => {
    sendPosition({ clearLocation: true });
  });

  // --- the patrol tag (task 377) --------------------------------------------
  //
  // No picker and no roster. 'publicpatrol.Queries' has one read, by number, and its doc says the absence of a
  // list is deliberate — a list read is what a scraper would ask for. So the curator types the number from the
  // patrol's sign, and the tool shows the name back before anything is saved.
  //
  // The confirmation is not decoration: patrol numbers are not unique per year and the resolve does
  // 'ORDER BY teamId LIMIT 1', so a human seeing *which* patrol they got is what makes that defensible.

  const tagPanel = document.getElementById('tagpanel');
  const tagNote = document.getElementById('tagnote');
  const tagNum = document.getElementById('tagnum');
  const tagFound = document.getElementById('tagfound');
  const doTag = document.getElementById('dotag');

  // The patrol the curator has confirmed, if any. Cleared whenever the number changes, so the confirmed patrol
  // and the number in the box can never disagree — which is the one way this could tag the wrong patrol.
  let confirmedPatrol = null;

  function openTagPanel() {
    openSheet(tagPanel);
    confirmedPatrol = null;
    tagFound.textContent = '';
    tagFound.classList.remove('ok');
    doTag.disabled = true;
    tagNote.textContent = selected.size === 1
      ? '1 billede bliver tagget.'
      : selected.size + ' billeder bliver tagget.';
    tagNum.focus();
  }

  async function lookupPatrol() {
    const number = tagNum.value.trim();
    confirmedPatrol = null;
    doTag.disabled = true;
    tagFound.classList.remove('ok');

    if (!number) { tagFound.textContent = 'Skriv patruljens nummer.'; return; }

    tagFound.textContent = 'Søger…';
    try {
      const res = await fetch('/api/admin/patrols/' + encodeURIComponent(number));
      if (res.status === 404) {
        tagFound.textContent = 'Der er ingen patrulje med nummer ' + number + ' i år.';
        return;
      }
      if (!res.ok) {
        const payload = await res.json().catch(() => null);
        tagFound.textContent = (payload && payload.error) || 'Kunne ikke søge (fejl ' + res.status + ').';
        return;
      }
      const p = await res.json();
      confirmedPatrol = p;

      // What the curator confirms against: the patrol's own name, its group and its korps. Never a person — the
      // endpoint has no field for one.
      const bits = [p.name, p.group, p.korps].filter(Boolean);
      tagFound.textContent = 'Patrulje ' + p.number + (bits.length ? ': ' + bits.join(' · ') : '');
      tagFound.classList.add('ok');
      doTag.disabled = false;
    } catch (err) {
      tagFound.textContent = 'Kunne ikke søge. Prøv igen.';
    }
  }

  document.getElementById('lookuppatrol').addEventListener('click', lookupPatrol);
  tagNum.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); lookupPatrol(); }
  });
  // Any edit invalidates the confirmation, so the button cannot act on a patrol the curator is no longer looking
  // at.
  tagNum.addEventListener('input', () => {
    confirmedPatrol = null;
    doTag.disabled = true;
    tagFound.textContent = '';
    tagFound.classList.remove('ok');
  });

  document.getElementById('closetag').addEventListener('click', () => { closeSheet(); });

  doTag.addEventListener('click', async () => {
    if (!confirmedPatrol) { tagFound.textContent = 'Find patruljen først.'; return; }

    tagNote.textContent = 'Tagger…';
    try {
      // The **number** is sent, not the team id: the server re-resolves it, so a client cannot tag a patrol other
      // than the one the curator confirmed.
      const res = await fetch('/api/admin/photos/tags', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: Array.from(selected), number: confirmedPatrol.number }),
      });
      const out = await res.json().catch(() => null);
      if (!res.ok) {
        tagNote.textContent = (out && out.error) || 'Kunne ikke tagge billederne.';
        return;
      }
      note.textContent = out.message || 'Tagget.';
      closeSheet();
      selected.clear();
      load(true);
    } catch (err) {
      tagNote.textContent = 'Kunne ikke tagge billederne. Prøv igen.';
    }
  });

  // --- removing and deleting (task 379) -------------------------------------
  //
  // Two different acts behind one button, and the panel's job is to make the difference unmissable. Removing from
  // an album leaves the photograph in the archive; deleting takes it out of everything and frees its bytes. One of
  // the two is what somebody means by "take it down", and if they read alike the wrong one gets pressed.
  //
  // Both loop the selection with the uploader's bounded concurrency rather than a bulk endpoint: one event per
  // photograph keeps the audit log one line per act, which with a shared credential is the only record there is.

  const delPanel = document.getElementById('delpanel');
  const delNote = document.getElementById('delnote');
  const delAlbum = document.getElementById('delalbum');
  const delReason = document.getElementById('delreason');
  const delCount = document.getElementById('delcount');
  const doRemove = document.getElementById('doremove');

  async function openDeletePanel() {
    openSheet(delPanel);
    delReason.value = '';
    delAlbum.value = '';
    doRemove.disabled = true;

    const n = selected.size;
    delNote.textContent = n === 1 ? '1 billede er valgt.' : n + ' billeder er valgt.';
    delCount.textContent = n === 1 ? '1 billede' : n + ' billeder';

    // Only albums the selection could plausibly be in are worth offering, but filtering that would need a read per
    // photograph — so every live album is listed and the server answers 404 for a photograph that is not in the
    // one chosen, which the loop below reports as "lå ikke i albummet".
    try {
      const res = await fetch('/api/admin/albums');
      if (!res.ok) return;
      const data = await res.json();
      while (delAlbum.options.length > 1) delAlbum.remove(1);
      for (const a of data.albums.filter((x) => !x.deleted)) {
        const o = document.createElement('option');
        o.value = a.albumId;
        o.textContent = a.title + (a.published ? '' : ' (kladde)');
        delAlbum.append(o);
      }
    } catch (err) { /* the album list is an aid; deleting does not need it */ }
  }

  delAlbum.addEventListener('change', () => { doRemove.disabled = !delAlbum.value; });
  document.getElementById('closedel').addEventListener('click', () => { closeSheet(); });

  // runOverSelection issues one request per selected photograph, three at a time.
  //
  // The uploader's pattern, for the uploader's reasons: one failure does not take the batch with it, and progress
  // is real rather than a bar that finishes and then waits.
  async function runOverSelection(makeRequest) {
    const ids = Array.from(selected);
    let ok = 0, missing = 0, failed = 0;
    let i = 0;

    async function worker() {
      for (;;) {
        const id = ids[i++];
        if (id === undefined) return;
        try {
          const res = await makeRequest(id);
          if (res.status === 404) missing++;
          else if (res.ok || res.status === 204) ok++;
          else failed++;
        } catch (err) {
          failed++;
        }
      }
    }

    await Promise.all([worker(), worker(), worker()]);
    return { ok, missing, failed };
  }

  doRemove.addEventListener('click', async () => {
    const albumId = delAlbum.value;
    if (!albumId) return;

    delNote.textContent = 'Fjerner…';
    const r = await runOverSelection((id) =>
      fetch('/api/admin/albums/' + encodeURIComponent(albumId) + '/items/' + encodeURIComponent(id),
        { method: 'DELETE' }));

    // Said plainly, including the ones that were not in the album: a curator who selected across albums needs to
    // know why the number is smaller than their selection.
    const bits = [r.ok + (r.ok === 1 ? ' billede fjernet' : ' billeder fjernet')];
    if (r.missing) bits.push(r.missing + ' lå ikke i albummet');
    if (r.failed) bits.push(r.failed + ' fejlede');
    note.textContent = bits.join(' · ') + '.';

    closeSheet();
    selected.clear();
    load(true);
  });

  document.getElementById('dodelete').addEventListener('click', async () => {
    const n = selected.size;
    // A native confirm, deliberately: this is the one irreversible action in the tool, and the browser's own dialog
    // is the one thing a curator cannot dismiss by muscle memory. The count is in the question because "select all
    // in filter" makes a mis-aimed delete plausible (PRD 022 §11 Q6).
    const word = n === 1 ? 'dette billede' : 'disse ' + n + ' billeder';
    if (!window.confirm('Slet ' + word + ' fra arkivet?\n\n' +
      'De forsvinder fra alle album og kan ikke ses offentligt. ' +
      'Det kan ikke fortrydes herfra.')) return;

    delNote.textContent = 'Sletter…';
    const body = JSON.stringify({ reason: delReason.value });
    const r = await runOverSelection((id) =>
      fetch('/api/admin/photos/' + encodeURIComponent(id),
        { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body }));

    const bits = [r.ok + (r.ok === 1 ? ' billede slettet' : ' billeder slettet')];
    if (r.failed) bits.push(r.failed + ' fejlede');
    note.textContent = bits.join(' · ') + '.';

    closeSheet();
    selected.clear();
    load(true);
  });

  // The initial load runs last, after every declaration it can reach.
  //
  // 'syncActions' touches 'panel', which is a 'const' declared further up this block — and a 'const' read before
  // its initialiser throws a ReferenceError rather than giving undefined. The first load is asynchronous, so in
  // practice it would resolve after the block finished either way; putting it here means that does not have to
  // be reasoned about every time somebody adds a declaration.
  load(true);
})();
</script>
</body>
</html>{{end}}
`))
