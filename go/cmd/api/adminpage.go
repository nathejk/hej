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
    <li><span class="n" id="c-total">{{.Counts.Total}}</span><span class="k">billeder</span></li>
    <li><span class="n">{{.Counts.InNoAlbum}}</span><span class="k">uden album</span></li>
    <li><span class="n">{{.Counts.WithLocation}}</span><span class="k">med position</span></li>
    <li><span class="n">{{.Counts.Plottable}}</span><span class="k">på kortet</span></li>
    <li><span class="n">{{.Counts.OutOfBounds}}</span><span class="k">uden for området</span></li>
    <li><span class="n">{{.Counts.Unknown}}</span><span class="k">ikke vurderet</span></li>
    <li><span class="n">{{.Counts.Tagged}}</span><span class="k">med patrulje</span></li>
    <li><span class="n">{{.Counts.Deleted}}</span><span class="k">slettede</span></li>
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

  <p class="todo">Kontaktark, album og positioner kommer her.</p>
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
  const total = document.getElementById('c-total');

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
    if (total) total.textContent = String((parseInt(total.textContent, 10) || 0) + 1);
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
})();
</script>
</body>
</html>{{end}}
`))
