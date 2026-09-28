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

// # Why this is a function taking a context rather than a file that runs on load
//
// page.js was 1,400 lines in one closure, and the eight features in it shared state by simply being in the same
// scope. Task 395 split it per feature; that split is only worth anything if what each feature *needs* from the
// others is written down, so every file here is `init<Feature>(ctx)` and `ctx` is the whole of the shared surface.
// main.js builds it and calls these in order.
//
// The uploader's dependency is one line: when a batch finishes, the contact sheet is stale.
//
// # Why the end of a batch waits for its own ids (tasks 437, 438, 439)
//
// An upload **publishes an event** and answers with the photograph's id; the grid reads a projection a consumer
// folds from that event (see `uploadAdminPhotoHandler`). The two are milliseconds apart in a healthy system and
// not ordered at all in principle, so reloading the grid the instant the last response lands is a read that may
// legitimately not see the photographs yet. That is what a curator reported: a list of uploaded files above a grid
// that does not have them, and no way to act on any of them without reloading the page by hand.
//
// So the batch's end hands the ids it was given to `ctx.settled`, which asks the library for them until it can see
// them all — and is bounded, so if they never arrive the sheet is reloaded anyway and the line says so. The wait
// is on the context rather than here because every write in the tool has this race and PRD 024 brings the next
// caller; the reasoning behind its shape is in main.js.
function initUpload(ctx) {
  const CONCURRENCY = 3;

  const drop = document.getElementById('drop');
  const input = document.getElementById('files');
  const rows = document.getElementById('rows');
  const prog = document.getElementById('prog');
  const bar = document.getElementById('bar');
  const progLabel = document.getElementById('proglabel');
  const note = document.getElementById('uploadnote');

  let queued = 0, done = 0, running = 0;
  const queue = [];
  // The batch in flight, or null between batches: what it has done so far, and the ids the server gave it.
  //
  // Counted here rather than derived from the rows afterwards, because the rows are presentation — and because the
  // outcome of a file is known exactly once, where the server said it.
  let batch = null;

  // --- the queue ------------------------------------------------------------

  function enqueue(files) {
    // A new batch is a drop onto an idle uploader. `queued`/`done` deliberately keep accumulating across a
    // session, so "idle" is the two being equal rather than a counter reset.
    if (done === queued) {
      batch = { stored: [], already: 0, gone: 0, failed: 0 };
      note.textContent = '';
    }
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

    // The batch just ended. Taken out of `batch` before finishing it, because `render` runs on every file and
    // this must happen once.
    if (!active && queued > 0 && done === queued && batch) {
      const b = batch;
      batch = null;
      finish(b);
    }
  }

  // --- the end of a batch ---------------------------------------------------

  async function finish(b) {
    // Nothing new in the library means nothing to wait for. The sheet is still reloaded: a re-upload of a card
    // whose photographs a colleague has since filed should show their albums.
    if (!b.stored.length) {
      ctx.reloadSheet();
      note.textContent = summary(b);
      return;
    }

    note.textContent = 'Opdaterer kontaktarket…';
    const caught = await ctx.settled(b.stored);
    ctx.reloadSheet();
    note.textContent = caught
      ? summary(b)
      : summary(b) + ' Kontaktarket kan være et øjeblik bagud — genindlæs siden, hvis nogle mangler.';
  }

  // summary is the sentence the curator reads when a batch is over.
  //
  // Every outcome that is not a plain success is named, because each one means something different to the person
  // holding the card: a duplicate is nothing to do, a previously-deleted photograph is a decision somebody made,
  // and a failure is a file to drag in again. Silence about any of them would read as "all of it went up".
  function summary(b) {
    const parts = [b.stored.length > 0 ? ctx.photoCount(b.stored.length) + ' tilføjet' : 'Ingen nye billeder'];
    if (b.already) parts.push(ctx.photoCount(b.already) + ' var lagt op i forvejen');
    if (b.gone) parts.push(ctx.photoCount(b.gone) + ' blev ikke lagt op igen, fordi de er slettet tidligere');
    if (b.failed) parts.push(ctx.photoCount(b.failed) + ' kunne ikke lægges op — se listen');
    return parts.join(' — ') + '.';
  }

  // --- one file -------------------------------------------------------------

  async function upload(job) {
    const body = new FormData();
    body.append('photo', job.file);

    try {
      const res = await ctx.fetch('/api/admin/photos', { method: 'POST', body });
      let payload = null;
      try { payload = await res.json(); } catch (_) { /* an error page rather than JSON */ }

      if (!res.ok) {
        // The server writes the reason, in Danish, because it is the side that knows why. The status is only
        // consulted for the cases that have no body — a proxy timing out, for instance.
        tally('failed');
        finishRow(job.li, 'err', (payload && (payload.error || payload.message)) || httpReason(res.status));
        return;
      }
      applyOutcome(job.li, payload);
    } catch (err) {
      // A dropped connection, a closed laptop, a tunnel. Said plainly, and the row invites the one recovery
      // that actually works: drag it again.
      tally('failed');
      finishRow(job.li, 'err', 'Forbindelsen blev afbrudt. Træk filen ind igen.');
    }
  }

  // tally records one file's outcome against the batch, for the line at the end and for the wait. `batch` can be
  // null if a file somehow settles after its batch was finished, which is why this is a function and not four
  // `batch.x++`.
  function tally(what, id) {
    if (!batch) return;
    if (what === 'stored') batch.stored.push(id);
    else batch[what]++;
  }

  function applyOutcome(li, out) {
    // A response we cannot read is not a success we should claim. The id is required on every outcome, not only
    // the stored one: it is the contract of this endpoint (`adminUploadResponse`) and it is what the wait at the
    // end of the batch is built on.
    if (!out || !out.photoId) { tally('failed'); finishRow(li, 'err', 'Uventet svar fra serveren.'); return; }

    // Three outcomes, three appearances. A duplicate reported as a plain success would make a duplicated card
    // impossible to notice, and a skipped deletion reported as success would be a lie (task 372).
    if (out.outcome === 'already') {
      tally('already');
      finishRow(li, 'skip', '', out, 'Allerede lagt op');
      return;
    }
    if (out.outcome === 'deleted') {
      tally('gone');
      finishRow(li, 'skip', out.message || '', out, 'Slettet tidligere');
      return;
    }
    // The id the server just generated from the bytes. Waited for below, because getting it back from a *read* is
    // what proves the event has been through the stream and into the projection.
    tally('stored', out.photoId);
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
}
