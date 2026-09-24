// The contact sheet (task 374).
//
// # The selection is the tool's real state
//
// Everything in the action bar acts on it, so it is held as a Set of ids rather than read off the DOM. Two
// reasons, and the second is the one that matters: a DOM-derived selection would be lost by any re-render, and it
// would silently shrink to "what is currently loaded" — which would make "select all matching this filter" a lie
// the moment the grid was paged.
//
// That is also what made the cells safe to render server-side (task 395): a swap replaces cells, and the Set does
// not notice. See adminui/fragments.html.
//
// Publishes the selection and everything that reads or writes it onto the context, because every action needs it.
function initContactSheet(ctx) {
  const sheet = document.getElementById('sheet');
  const filters = document.getElementById('filters');
  const note = document.getElementById('sheetnote');
  const actionNote = document.getElementById('actionnote');
  const actions = document.getElementById('actions');
  const selCount = document.getElementById('selcount');
  const clearSel = document.getElementById('clearsel');

  const selected = new Set();
  // The order cells are shown in, so shift-click can resolve a range.
  //
  // **Derived from the DOM, unlike the selection.** That is not a contradiction of the paragraph above: `order` is
  // not state, it is the definition of "the cells currently shown", so reading it off the cells currently shown
  // cannot be wrong. The selection is the thing that must outlive a re-render, and it is the thing kept out of the
  // DOM. Getting that distinction right is what made the grid safe to render server-side (task 395).
  let order = [];
  let lastClicked = null;
  // The query the page was served with (task 396): the filter its URL names on the all-photos view, or
  // `album={id}` on the album view. The first page of thumbnails was already requested with it, and "select all
  // matching" pages with it, so on the album view that means the album's photographs and nothing else.
  let query = sheet.dataset.query || '';

  // chosen renders "1 valgt" or "12 valgte" (task 387).
  //
  // Local rather than on the context, because both places that count a *selection* are in this file. `valgt` is an
  // adjective rather than a noun, which is why it does not go through `ctx.photoCount`: "1 billede valgt" and "1
  // valgt" inflect on different words.
  function chosen(n) {
    return n === 1 ? '1 valgt' : n + ' valgte';
  }
  function syncActions() {
    const n = selected.size;
    actions.hidden = n === 0;
    selCount.textContent = chosen(n);
    // Every action is wired now (tasks 375–379).
    //
    // Closing the bar closes the sheet with it: an action on an empty selection is a button that cannot do
    // anything.
    if (n === 0) ctx.closeSheet();
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

  // The cells themselves are an htmx fragment (task 395) — see adminui/fragments.html. What is left here is
  // everything that needs the selection, which no fragment can see.

  // load asks for a page of thumbnails.
  //
  // `reset` replaces the grid's contents; otherwise the page is appended. One fragment serves both, and the
  // response also carries the shown-count, the "more" button and the header's counts out of band — so one request
  // keeps all four in step rather than four places computing them.
  function load(reset, offset) {
    if (!window.htmx) return;
    if (reset) { order = []; lastClicked = null; pending = -1; }
    const url = '/admin/fragments/photos?limit=120&offset=' + (offset || 0) + (query ? '&' + query : '');
    window.htmx.ajax('GET', url, { target: '#sheet', swap: reset ? 'innerHTML' : 'beforeend' });
  }

  // After every swap: re-derive the display order, and re-paint from the selection.
  //
  // **The re-paint is the load-bearing line.** The selection deliberately survives filtering — a curator narrows
  // to "uden position", picks forty, then widens to check something, and losing the forty would make the filters
  // unusable as a working tool. The cells that come back carry `aria-selected="false"`, because the server does
  // not know the selection; this is where they learn.
  sheet.addEventListener('htmx:afterSwap', () => {
    order = Array.from(sheet.querySelectorAll('.cell')).map((c) => c.dataset.id);
    for (const cell of sheet.querySelectorAll('.cell')) paint(cell);
    syncActions();
  });

  // A failed load is a sentence rather than an empty grid. htmx does not swap an error response, so without this
  // the previous page's thumbnails would sit there looking current.
  sheet.addEventListener('htmx:responseError', (e) => {
    const status = e.detail.xhr.status;
    note.textContent = status === 401
      ? 'Du er blevet logget ud. Genindlæs siden.'
      : 'Kunne ikke hente billederne (fejl ' + status + ').';
  });
  sheet.addEventListener('htmx:sendError', () => {
    note.textContent = 'Kunne ikke hente billederne. Prøv igen.';
  });

  // Delegated, rather than bound per cell as it was before the cells became server-rendered. This is what makes a
  // swap free: there is nothing to re-bind.
  sheet.addEventListener('click', (e) => {
    const cell = e.target.closest('.cell');
    if (!cell || !sheet.contains(cell)) return;
    const id = cell.dataset.id;
    if (e.shiftKey && lastClicked) selectRange(id);
    else { toggle(id); lastClicked = id; }
  });

  // The album view has no filters: its query is the album.
  if (filters) filters.addEventListener('click', (e) => {
    const b = e.target.closest('.f');
    if (!b) return;
    for (const other of filters.querySelectorAll('.f')) other.classList.toggle('on', other === b);
    query = b.dataset.q;
    // Into the address bar, so a reload keeps the filter and the URL can be sent to a colleague. `replaceState`
    // rather than `pushState`: Back leaving the page is what a curator expects, not stepping through filters.
    history.replaceState(null, '', filters.dataset.root + '/photos' + (query ? '?' + query : ''));
    load(true);
  });

  // Delegated from the wrapper, because the button arrives as an out-of-band swap and is replaced on every load.
  // Its `data-offset` is where the next page starts, computed by the server — the browser adding up page sizes
  // would drift the first time a clamped limit differed from the one it asked for.
  //
  // From the document rather than from `#morewrap` itself, because `hx-swap-oob="true"` replaces the wrapper
  // element outright: a listener bound to the first one would be gone after the first page.
  //
  // `pending` is the offset already asked for, so the button and the scroll observer below cannot both fetch the
  // same page.
  let pending = -1;
  function more(button) {
    const offset = parseInt(button.dataset.offset, 10) || 0;
    if (offset === pending) return;
    pending = offset;
    load(false, offset);
  }
  document.addEventListener('click', (e) => {
    if (e.target.id === 'more') more(e.target);
  });

  // Infinite scroll (task 396). The button stays — it is the keyboard's way to the next page, and what shows if
  // the observer never fires — and the observer presses it when it comes within reach. A generous margin, so the
  // next page is usually in before the curator reaches the end of this one.
  //
  // Re-observed after every swap, because each page brings a new button (see above). The page the button points
  // at is still the server's offset, not a count kept here.
  const reach = 'IntersectionObserver' in window
    ? new IntersectionObserver((entries) => {
      for (const entry of entries) if (entry.isIntersecting) more(entry.target);
    }, { rootMargin: '600px 0px' })
    : null;
  function observeMore() {
    if (!reach) return;
    reach.disconnect();
    const button = document.getElementById('more');
    if (button) reach.observe(button);
  }
  document.body.addEventListener('htmx:afterSettle', observeMore);
  // A page that failed can be asked for again, by the button or by scrolling back to it.
  for (const ev of ['htmx:responseError', 'htmx:sendError']) sheet.addEventListener(ev, () => { pending = -1; });

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
        const res = await ctx.fetch(url);
        if (!res.ok) { actionNote.textContent = 'Kunne ikke hente alle billeder (fejl ' + res.status + ').'; return; }
        const data = await res.json();
        for (const p of data.photos) selected.add(p.id);
        off += data.photos.length;
        // Guard against a server that keeps saying "more" — an empty page ends the loop regardless, so a bug
        // upstream cannot spin here forever.
        if (!data.hasMore || data.photos.length === 0) break;
      }
      for (const cell of sheet.querySelectorAll('.cell')) paint(cell);
      syncActions();
      actionNote.textContent = ctx.photoCount(selected.size - before) + ' lagt til valget — ' +
        chosen(selected.size) + ' i alt';
    } catch (err) {
      actionNote.textContent = 'Kunne ikke hente alle billeder. Prøv igen.';
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

  ctx.selected = selected;
  ctx.reloadSheet = () => load(true);
  ctx.actionNote = actionNote;
  // The action bar dispatches to whichever opener a feature registered under its `data-act` name. Registered
  // rather than listed here, so adding a sixth action is one file and not two.
  actions.addEventListener('click', (e) => {
    const b = e.target.closest('button[data-act]');
    if (!b || b.disabled) return;
    const open = ctx.openers[b.dataset.act];
    if (open) open();
  });
}
