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

  // --- the album list ------------------------------------------------------
  //
  // Not here. It lives in `adminui/fragments.html` and `adminfragments.go` as an htmx fragment (task 395):
  // around 190 lines of fetch-then-build-DOM replaced by a template the server renders and htmx swaps in.
  //
  // The reason is testability. The list's rules — drafts and deleted albums shown rather than filtered (task
  // 366), covers fetched through the **admin** media route because the public one correctly refuses an
  // unpublished album's photographs (task 382), publication posted alone (task 378) — were only ever checkable
  // by grepping this file's own source text, and that produced three false positives in one session. As Go
  // handlers returning HTML they are checkable by asking for the page.
  //
  // What is left here is the one line of coupling: code in this file that changes the set of albums fires
  // `albums-changed` on `<body>`, and the fragment listens for it.

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
      // The list above is now stale by one album. It is an htmx fragment, so it is told rather than redrawn: it
      // listens for this event on `body` and re-fetches itself (task 395).
      document.body.dispatchEvent(new Event('albums-changed'));
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
    if (b.dataset.act === 'credit') openCreditPanel();
    if (b.dataset.act === 'delete') openDeletePanel();
  });

  // --- the photo credit (task 393) ------------------------------------------
  //
  // # The only field in this tool that records a person's name
  //
  // Everything else here is arranged so the library names nobody — no uploader, no curator, no person id
  // (PRD 022 §6), held by a structural walk over the types rather than by review. This is the exception, and it
  // is narrow on purpose: a **consenting adult volunteer, credited as the author of a photograph**, from text a
  // curator typed. Nothing derives it, and nothing here can look a name up — there is no endpoint that would.
  //
  // The copy in the sheet says so, because a curator typing a colleague's name onto a public page should know
  // that is what they are doing rather than filling in a field that looks like a caption.
  //
  // # Why the remembered default lives in the browser
  //
  // A card is one photographer, so the tedium is real: without something remembered, every batch means retyping
  // the same line. It is in 'localStorage' and **not** on the server, deliberately — the credential is shared
  // (§8.2), so a server-side "my credit" would be an attribution the tool cannot honestly make. The browser
  // remembering what *this laptop* last typed claims nothing about who is using it.

  const creditPanel = document.getElementById('creditpanel');
  const creditNote = document.getElementById('creditnote');
  const creditText = document.getElementById('credittext');
  const CREDIT_KEY = 'hej.admin.lastCredit';

  function openCreditPanel() {
    openSheet(creditPanel);
    creditNote.textContent = selected.size === 1
      ? '1 billede får fotokreditten.'
      : selected.size + ' billeder får fotokreditten.';
    // Prefilled from the last one typed on this machine, so a second card is one click. Not prefilled from the
    // selection: the photographs may carry different credits, and picking one of them to show would be a guess
    // that silently overwrites the others when the curator presses the button.
    try {
      if (!creditText.value) creditText.value = window.localStorage.getItem(CREDIT_KEY) || '';
    } catch (err) { /* storage disabled or full: the field is simply empty */ }
    creditText.focus();
    creditText.select();
  }

  document.getElementById('closecredit').addEventListener('click', () => { closeSheet(); });

  async function sendCredit(credit) {
    if (!selected.size) { creditNote.textContent = 'Vælg mindst ét billede.'; return; }
    creditNote.textContent = 'Gemmer…';
    try {
      const res = await fetch('/api/admin/photos', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: Array.from(selected), credit: credit }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        creditNote.textContent = (payload && payload.error) || 'Kunne ikke sætte fotokreditten.';
        return;
      }
      if (credit) {
        try { window.localStorage.setItem(CREDIT_KEY, credit); } catch (err) { /* nothing to recover */ }
      }
      closeSheet();
      // Reloaded so the sheet shows what was actually written, not what the browser hoped.
      load(true);
    } catch (err) {
      creditNote.textContent = 'Kunne ikke sætte fotokreditten. Prøv igen.';
    }
  }

  document.getElementById('docredit').addEventListener('click', () => {
    const credit = creditText.value.trim();
    if (!credit) { creditNote.textContent = 'Skriv en fotokredit, eller brug “Fjern fotokredit”.'; return; }
    sendCredit(credit);
  });

  // Clearing is its own button rather than "save an empty field", so removing an attribution is a deliberate act
  // and not something a stray select-all-and-delete does on its way past.
  document.getElementById('doclearcredit').addEventListener('click', () => sendCredit(''));

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
