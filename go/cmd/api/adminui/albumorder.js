// Drag-and-drop reordering in the album view (task 396).
//
// # What moves
//
// Drag a selected photograph and the **whole selection** moves with it, landing together in the order it already
// had in the album. Drag one that is not selected and only that one moves. The selection may include photographs
// not loaded yet ("Vælg alle der matcher filteret"); that is fine, because the server builds the order.
//
// # Why pointer events and not HTML5 drag-and-drop
//
// The cells are `<button>`s, for the listbox's keyboard handling, and Firefox does not reliably start a native drag
// on a button. Native DnD also drags one element's image, where this drags a selection. So: pointer events, a
// movement threshold that tells a drag from a click, and a click swallowed after a drag so it does not also toggle
// the cell it ended on.
//
// # Why the request names a neighbour, not an order
//
// The grid scrolls in pages, so it may hold 120 of 200. The request says "these, before (or after) that one" and
// `PATCH /api/admin/albums/{id}/move` builds the whole order — see moveAdminAlbumItemsHandler.
//
// Does nothing outside the album view.
function initAlbumOrder(ctx) {
  const editor = document.getElementById('albumeditor');
  if (!editor) return;
  const albumId = editor.dataset.album;
  const sheet = document.getElementById('sheet');

  const THRESHOLD = 6; // px of movement before a press becomes a drag
  let press = null; // { id, x, y, pointerId }
  let drag = null; // { moving: [ids], ghost, target, after }
  let swallowClick = false;

  function cellAt(x, y) {
    const el = document.elementFromPoint(x, y);
    const cell = el && el.closest ? el.closest('.cell') : null;
    return cell && sheet.contains(cell) ? cell : null;
  }

  function clearMarks() {
    for (const c of sheet.querySelectorAll('.drop-before, .drop-after')) {
      c.classList.remove('drop-before', 'drop-after');
    }
  }

  function start(e) {
    const moving = ctx.selected.has(press.id) ? Array.from(ctx.selected) : [press.id];
    const ghost = document.createElement('div');
    ghost.className = 'dragghost';
    ghost.textContent = 'Flytter ' + ctx.photoCount(moving.length);
    document.body.appendChild(ghost);
    drag = { moving: moving, ghost: ghost, target: null, after: false };
    const set = new Set(moving);
    for (const c of sheet.querySelectorAll('.cell')) c.classList.toggle('dragging', set.has(c.dataset.id));
    document.body.classList.add('draggingcells');
    move(e);
  }

  function move(e) {
    drag.ghost.style.left = e.clientX + 12 + 'px';
    drag.ghost.style.top = e.clientY + 12 + 'px';

    // Scroll when the pointer nears the top or bottom edge, so a drop target further down can be reached. The
    // infinite scroll loads the next page as the grid's end comes into view.
    const edge = 60;
    if (e.clientY < edge) window.scrollBy(0, -20);
    else if (e.clientY > window.innerHeight - edge) window.scrollBy(0, 20);

    clearMarks();
    drag.target = null;
    const cell = cellAt(e.clientX, e.clientY);
    if (!cell || cell.classList.contains('dragging')) return;
    const r = cell.getBoundingClientRect();
    drag.after = e.clientX > r.left + r.width / 2;
    drag.target = cell.dataset.id;
    cell.classList.add(drag.after ? 'drop-after' : 'drop-before');
  }

  function end() {
    const d = drag;
    drag = null;
    press = null;
    clearMarks();
    document.body.classList.remove('draggingcells');
    for (const c of sheet.querySelectorAll('.dragging')) c.classList.remove('dragging');
    if (d) d.ghost.remove();
    return d;
  }

  async function send(d) {
    const body = { photoIds: d.moving };
    body[d.after ? 'afterPhotoId' : 'beforePhotoId'] = d.target;
    ctx.actionNote.textContent = 'Flytter…';
    try {
      const res = await ctx.fetch('/api/admin/albums/' + encodeURIComponent(albumId) + '/move', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const out = await res.json().catch(() => null);
        ctx.actionNote.textContent = (out && out.error) || 'Kunne ikke flytte billederne (fejl ' + res.status + ').';
        return;
      }
      ctx.actionNote.textContent = ctx.photoCount(d.moving.length) + ' flyttet.';
      // Reloaded so the grid shows the order the server now holds. The selection is kept: moving the same photographs
      // again is common, and it survives the reload by design (contactsheet.js).
      ctx.reloadSheet();
    } catch (err) {
      ctx.actionNote.textContent = 'Kunne ikke flytte billederne. Prøv igen.';
    }
  }

  sheet.addEventListener('pointerdown', (e) => {
    // A modified click is a selection gesture (shift for a range), never a drag.
    if (e.button !== 0 || e.shiftKey || e.metaKey || e.ctrlKey) return;
    const cell = e.target.closest('.cell');
    if (!cell || !sheet.contains(cell)) return;
    press = { id: cell.dataset.id, x: e.clientX, y: e.clientY, pointerId: e.pointerId };
  });

  window.addEventListener('pointermove', (e) => {
    if (!press || e.pointerId !== press.pointerId) return;
    if (!drag) {
      if (Math.abs(e.clientX - press.x) + Math.abs(e.clientY - press.y) < THRESHOLD) return;
      start(e);
    } else {
      move(e);
    }
    e.preventDefault();
  });

  window.addEventListener('pointerup', (e) => {
    if (!press || e.pointerId !== press.pointerId) return;
    const d = end();
    if (!d) return; // a click, not a drag: contactsheet.js toggles the cell
    swallowClick = true;
    // Cleared once this pointerup's click has had its chance to fire: a drop outside the grid produces no click
    // there, and a flag left set would swallow the curator's next real one.
    setTimeout(() => { swallowClick = false; }, 0);
    if (d.target) send(d);
  });

  window.addEventListener('pointercancel', () => { end(); });
  window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && drag) end();
  });

  // The click that ends a drag must not also toggle the cell it lands on. Capturing, so it runs before the
  // contact sheet's delegated handler.
  sheet.addEventListener('click', (e) => {
    if (swallowClick) { swallowClick = false; e.stopPropagation(); e.preventDefault(); }
  }, true);

  // The thumbnails' own native drag would otherwise start and fight this one.
  sheet.addEventListener('dragstart', (e) => { e.preventDefault(); });
}
