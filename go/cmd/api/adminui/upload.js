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

  // The album these uploads go into, or "" on the all-photos view (task 474).
  //
  // Read from the editor card, which is the one element that states which album this page is — rather than a second
  // attribute on the uploader saying the same thing, which is how the two come to disagree.
  const editor = document.getElementById('albumeditor');
  const intoAlbum = editor ? (editor.dataset.album || '') : '';

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
      // `file` is what goes into the album: the ids stored **and** the ones that were already in the library.
      //
      // Including `already` is the point of the feature rather than an edge case. Half a card is routinely already
      // uploaded — a colleague's dump, a retry, a second pass — and if those were skipped, dragging a card into an
      // album would file some of it and silently leave the rest out. `gone` is deliberately absent: a photograph a
      // curator deleted must not be filed into an album, which would resurrect it into public view.
      batch = { stored: [], file: [], already: 0, gone: 0, failed: 0 };
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
    // Nothing new and nothing to file means nothing to wait for. The sheet is still reloaded: a re-upload of a card
    // whose photographs a colleague has since filed should show their albums.
    if (!b.stored.length && !(intoAlbum && b.file.length)) {
      ctx.reloadSheet();
      note.textContent = summary(b);
      return;
    }

    note.textContent = 'Opdaterer kontaktarket…';
    // **The wait comes first, and the filing second.** `/api/admin/albums/items` validates the photographs against
    // the library projection, so filing an id the fold has not reached yet would be refused for a photograph that is
    // perfectly fine — the race task 437 was reported for, in a new place.
    const caught = await ctx.settled(b.stored);

    let filed = null;
    if (intoAlbum && b.file.length) {
      note.textContent = 'Lægger billederne i albummet…';
      filed = await fileIntoAlbum(b.file);
    }

    ctx.reloadSheet();
    let line = summary(b);
    if (filed) line += ' ' + filed;
    if (!caught) line += ' Kontaktarket kan være et øjeblik bagud — genindlæs siden, hvis nogle mangler.';
    note.textContent = line;
  }

  // fileIntoAlbum adds the batch to this page's album, and returns the sentence to append.
  //
  // # One request for the whole batch, not one per photograph
  //
  // Because adding to an album **re-sorts it** when its sort mode is not manual (PRD 024): filing three hundred
  // photographs one at a time would publish three hundred reorder events, each rewriting every ordinal in the album,
  // for one arrangement nobody saw the intermediate states of. The existing endpoint takes a list precisely so the
  // sort is applied once, and it is the same endpoint the action bar's "Tilføj til album" uses — so a card dragged
  // here and a selection filed there cannot end up ordered differently.
  //
  // The server writes the sentence, as it does for the sheet: it is the side that knows how many were already in the
  // album and whether the addition re-sorted it.
  async function fileIntoAlbum(ids) {
    try {
      const res = await ctx.fetch('/api/admin/albums/items', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: ids, albumIds: [intoAlbum] }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        // Said plainly, and it names the recovery that works: the photographs *are* in the library, so filing them
        // is the action bar's job now. Anything else would imply the upload failed, which it did not.
        return (payload && payload.error)
          ? 'Billederne blev lagt op, men kunne ikke lægges i albummet: ' + payload.error +
            ' Vælg dem og brug Tilføj til album.'
          : 'Billederne blev lagt op, men kunne ikke lægges i albummet. Vælg dem og brug Tilføj til album.';
      }
      // Waited for, so the grid below is the album including them rather than the album without them (task 457).
      await ctx.settledFilter(ids, '&album=' + encodeURIComponent(intoAlbum), true);
      return payload && payload.message ? payload.message : 'Lagt i albummet.';
    } catch (err) {
      return 'Billederne blev lagt op, men kunne ikke lægges i albummet. Vælg dem og brug Tilføj til album.';
    }
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
    if (isVideo(job.file)) return uploadVideo(job);
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
    // Both outcomes that leave a live photograph in the library are filed into the album. See `file` above for why
    // `already` belongs here and `gone` does not.
    if (id && (what === 'stored' || what === 'already')) batch.file.push(id);
  }

  function applyOutcome(li, out, storedLabel) {
    // A response we cannot read is not a success we should claim. The id is required on every outcome, not only
    // the stored one: it is the contract of this endpoint (`adminUploadResponse`) and it is what the wait at the
    // end of the batch is built on.
    if (!out || !out.photoId) { tally('failed'); finishRow(li, 'err', 'Uventet svar fra serveren.'); return; }

    // Three outcomes, three appearances. A duplicate reported as a plain success would make a duplicated card
    // impossible to notice, and a skipped deletion reported as success would be a lie (task 372).
    if (out.outcome === 'already') {
      tally('already', out.photoId);
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
    finishRow(li, 'ok', '', out, storedLabel || 'Lagt op');
  }

  // --- one video (PRD 029, task 497) -----------------------------------------
  //
  // # Chunks, not one request
  //
  // A clip is up to 4 GB and a camp's uplink drops. So a video goes up through a server-side session in 8 MiB
  // chunks (`adminvideoupload.go`): a dropped chunk is retried from the offset the server reports, with back-off,
  // and the row shows a percentage rather than a spinner for twenty minutes.
  //
  // The session id is remembered in localStorage against the file's name, size and date, so dragging the same clip
  // in again after a closed laptop continues where it stopped instead of starting over. The server forgets a session
  // after 24 hours; a stale id answers 404 and the upload simply starts fresh.

  const VIDEO_NAME = /\.(mov|mp4|m4v|webm|avi|mts|m2ts|3gp|mkv)$/i;
  function isVideo(file) {
    return (file.type && file.type.startsWith('video/')) || VIDEO_NAME.test(file.name || '');
  }

  function sessionKey(file) {
    return 'hej-video-upload:' + file.name + ':' + file.size + ':' + (file.lastModified || 0);
  }

  const sleep = (ms) => new Promise((res) => setTimeout(res, ms));

  async function videoSession(file) {
    const key = sessionKey(file);
    const remembered = localStorage.getItem(key);
    if (remembered) {
      try {
        const res = await ctx.fetch('/api/admin/videos/uploads/' + encodeURIComponent(remembered));
        if (res.ok) {
          const s = await res.json();
          return { id: s.uploadId, offset: s.offset, chunk: s.chunkSize };
        }
      } catch (_) { /* start a new one below */ }
      localStorage.removeItem(key);
    }
    const res = await ctx.fetch('/api/admin/videos/uploads', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ fileName: file.name, size: file.size }),
    });
    const s = await res.json().catch(() => null);
    if (!res.ok) return { error: (s && (s.error || s.message)) || videoReason(res.status) };
    localStorage.setItem(key, s.uploadId);
    return { id: s.uploadId, offset: 0, chunk: s.chunkSize };
  }

  async function uploadVideo(job) {
    const file = job.file;
    const state = job.li.querySelector('.state');
    try {
      const session = await videoSession(file);
      if (session.error) { tally('failed'); finishRow(job.li, 'err', session.error); return; }
      const url = '/api/admin/videos/uploads/' + encodeURIComponent(session.id);
      const chunk = session.chunk || (8 << 20);
      let offset = session.offset;
      let failures = 0;

      while (offset < file.size) {
        state.textContent = 'Uploader video… ' + Math.floor((offset * 100) / file.size) + ' %';
        const end = Math.min(offset + chunk, file.size);
        try {
          const res = await ctx.fetch(url, {
            method: 'PUT',
            headers: { 'Content-Range': 'bytes ' + offset + '-' + (end - 1) + '/' + file.size,
                       'Content-Type': 'application/octet-stream' },
            body: file.slice(offset, end),
          });
          const s = await res.json().catch(() => null);
          // 409 is the server saying where it actually is — after a chunk whose response was lost, say.
          if ((res.ok || res.status === 409) && s) { offset = s.offset; failures = 0; continue; }
          if (res.status >= 400 && res.status < 500 && res.status !== 429) {
            localStorage.removeItem(sessionKey(file));
            tally('failed');
            finishRow(job.li, 'err', (s && (s.error || s.message)) || videoReason(res.status));
            return;
          }
          throw new Error('HTTP ' + res.status);
        } catch (err) {
          if (++failures > 8) {
            tally('failed');
            finishRow(job.li, 'err', 'Forbindelsen svigtede for mange gange. Træk videoen ind igen — den fortsætter, ' +
              'hvor den slap.');
            return;
          }
          state.textContent = 'Forbindelsen svigtede — prøver igen…';
          await sleep(Math.min(30000, 1000 * 2 ** failures));
          try {
            const r = await ctx.fetch(url);
            if (r.ok) offset = (await r.json()).offset;
          } catch (_) { /* the next PUT will find out */ }
        }
      }

      state.textContent = 'Kontrollerer videoen…';
      const res = await ctx.fetch(url + '/complete', { method: 'POST' });
      let payload = null;
      try { payload = await res.json(); } catch (_) { /* an error page rather than JSON */ }
      localStorage.removeItem(sessionKey(file));
      if (!res.ok) {
        tally('failed');
        finishRow(job.li, 'err', (payload && (payload.error || payload.message)) || videoReason(res.status));
        return;
      }
      applyOutcome(job.li, payload, 'Lagt op — behandles');
    } catch (err) {
      tally('failed');
      finishRow(job.li, 'err', 'Forbindelsen blev afbrudt. Træk videoen ind igen — den fortsætter, hvor den slap.');
    }
  }

  function videoReason(status) {
    if (status === 413) return 'Videoen er for stor eller for lang (højst 4 GB og 30 minutter).';
    if (status === 400) return 'Filen er ikke en video vi kan læse.';
    return httpReason(status);
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
    if (isVideo(file)) {
      // No local preview for a video: decoding one to draw a frame costs more than the row is worth.
      thumb = document.createElement('div');
      thumb.className = 'noimg';
      thumb.textContent = '▶';
    } else if (file.type && file.type.startsWith('image/')) {
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
