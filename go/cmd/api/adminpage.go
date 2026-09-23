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

/* The action panel. Inline, because navigating away would lose the selection (PRD 022 §7). */
#panel {
  margin-top: 0.5rem; padding: 1rem; background: #fff;
  border: 1px solid #d4d4d8; border-radius: 0.5rem; max-width: 34rem;
}
#panel h3 { margin: 0 0 0.5rem; font-size: 1rem; }
#panel p { margin: 0.5rem 0; }
#panel .hint { color: #52525b; font-size: 0.875rem; }
#panel button {
  font: inherit; font-size: 0.875rem; cursor: pointer;
  background: #18181b; color: #fafafa; border: 1px solid #18181b;
  padding: 0.375rem 0.75rem; border-radius: 0.375rem;
}
#panel #closepanel, #panel #createalbum { background: #fff; color: #18181b; border-color: #d4d4d8; }
#panel input[type=text] {
  font: inherit; padding: 0.375rem 0.5rem; border: 1px solid #d4d4d8; border-radius: 0.375rem;
  min-width: 14rem;
}
#albumlist { display: grid; gap: 0.25rem; max-height: 14rem; overflow: auto; }
#albumlist label { display: flex; gap: 0.5rem; align-items: baseline; }
#albumlist .draft { color: #92400e; font-size: 0.8125rem; }
#albumlist .count { color: #52525b; font-size: 0.8125rem; }
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
    <button type="button" data-act="position" disabled>Sæt position</button>
    <button type="button" data-act="patrol" disabled>Tag patrulje</button>
    <button type="button" data-act="delete" disabled>Slet</button>
    <button type="button" id="clearsel">Ryd valg</button>
  </div>

  <!-- The panel opens inline rather than on its own page, because navigating away loses the selection
       (PRD 022 §7). That is the one hard constraint on this layout. -->
  <div id="panel" hidden role="dialog" aria-label="Tilføj til album">
    <h3>Tilføj til album</h3>
    <p id="panelnote"></p>
    <div id="albumlist"></div>
    <details id="newalbum">
      <summary>Opret et nyt album</summary>
      <p>
        <label for="newtitle">Titel</label>
        <input type="text" id="newtitle" maxlength="120" placeholder="Lørdag morgen">
        <button type="button" id="createalbum">Opret</button>
      </p>
      <p class="hint">Nye album er <strong>ikke</strong> udgivet. Du udgiver dem, når de er færdige.</p>
    </details>
    <p>
      <button type="button" id="doadd">Tilføj</button>
      <button type="button" id="closepanel">Annuller</button>
    </p>
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

  function syncActions() {
    const n = selected.size;
    actions.hidden = n === 0;
    selCount.textContent = n === 1 ? '1 valgt' : n + ' valgte';
    // The remaining three actions arrive with tasks 376-379. They stay disabled rather than absent so the shape
    // of the tool is visible, and so the selection can be exercised against the bar it will drive.
    for (const b of actions.querySelectorAll('button[data-act]')) {
      if (b.dataset.act !== 'album') b.disabled = true;
    }
    // Closing the bar closes the panel with it: a panel acting on an empty selection is a button that cannot
    // do anything.
    if (n === 0 && panel) panel.hidden = true;
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

  // --- the add-to-album action (task 375) ----------------------------------
  //
  // The panel opens inline and the selection survives it, which is the constraint PRD 022 §7 puts on this
  // layout: navigating away would lose the selection, and the selection is the input to every action.

  const panel = document.getElementById('panel');
  const panelNote = document.getElementById('panelnote');
  const albumList = document.getElementById('albumlist');
  const newTitle = document.getElementById('newtitle');

  async function openAlbumPanel() {
    panel.hidden = false;
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
    panel.hidden = true;
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
      panel.hidden = true;
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
