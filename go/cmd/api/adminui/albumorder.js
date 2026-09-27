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
// # Why the landing place is an empty frame and not a line
//
// The indicator used to be a bar drawn down one edge of the cell the photographs would land beside. It told the
// curator *which side*, but not that anything was going to move — so a fifty-photograph move looked the same as a
// one-photograph one until it had happened. Instead the grid opens a gap: dashed empty frames stand where the
// photographs will land, and the cells after them shift along. What the drop will do is then visible before the
// pointer is released.
//
// The photographs being moved **leave the grid** while they are in flight — `display: none`, not dimmed in place —
// and ride the pointer as a small stack of themselves. Two reasons, and the second is the point:
//
//   - keeping them in the flow meant the grid grew by a gap while still holding everything, so the arrangement under
//     the pointer was one that would never exist;
//   - with them gone, what is left on screen *is* the album minus the selection, and the gap is the only space the
//     selection occupies. The curator is looking at the result rather than at a diagram of it.
//
// The stack carries at most three thumbnails. It is a handful, not an inventory: the count underneath it is the
// real number, and a selection may include photographs that are not loaded and have no thumbnail to carry.
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
  // How many frames a gap is ever opened with. A selection can be two hundred photographs — "vælg alle der matcher
  // filteret" makes that one click — and two hundred frames would push the grid around so violently that the cell
  // the curator is aiming at would leave the screen. Four is enough to read as "several", and the ghost under the
  // pointer carries the true count.
  const MAXSLOTS = 4;
  // How many thumbnails the stack carries. See the header: a handful, and the label holds the truth.
  const MAXCARRIED = 3;
  let press = null; // { id, x, y, pointerId }
  let drag = null; // { moving: [ids], ghost, target, after }
  let slots = []; // the dashed frames currently standing in the grid
  let swallowClick = false;

  function cellAt(x, y) {
    const el = document.elementFromPoint(x, y);
    const cell = el && el.closest ? el.closest('.cell') : null;
    return cell && sheet.contains(cell) ? cell : null;
  }

  function onSlot(x, y) {
    const el = document.elementFromPoint(x, y);
    return !!(el && el.classList && el.classList.contains('dropslot'));
  }

  function clearMarks() {
    for (const s of slots) s.remove();
    slots = [];
  }

  // Whether the gap already open is the one this cell and side ask for. Without this the indicator oscillates: the
  // frames take up room, which moves the cell under the pointer, which asks for a gap one place over, and so on
  // every pointermove. The two ways of naming the same gap are "before the cell after the frames" and "after the
  // cell before them".
  function gapIsOpen(cell, after) {
    if (!slots.length) return false;
    if (!after && cell === slots[slots.length - 1].nextElementSibling) return true;
    if (after && cell === slots[0].previousElementSibling) return true;
    return false;
  }

  function openGap(cell, after, count) {
    const frames = document.createDocumentFragment();
    for (let i = 0; i < Math.min(count, MAXSLOTS); i++) {
      const slot = document.createElement('div');
      slot.className = 'dropslot';
      // Not a cell, and not in the listbox: the grid's keyboard handling and the selection both walk `.cell`, and a
      // hole in the order is not something a curator can select or focus.
      slot.setAttribute('aria-hidden', 'true');
      slots.push(slot);
      frames.appendChild(slot);
    }
    if (after) cell.after(frames); else cell.before(frames);
  }

  // carry builds what follows the pointer: a stack of the photographs themselves, with the count under it.
  //
  // The grabbed one is carried first so the photograph under the pointer is the one the curator took hold of. The
  // thumbnails are clones, which costs nothing to decode — the browser already has these images — and cannot
  // disturb the cells they came from.
  function carry(moving) {
    const ghost = document.createElement('div');
    ghost.className = 'dragghost';
    ghost.setAttribute('aria-hidden', 'true');

    const stack = document.createElement('div');
    stack.className = 'dragstack';
    const first = [press.id].concat(moving.filter((id) => id !== press.id));
    for (const id of first) {
      if (stack.childElementCount >= MAXCARRIED) break;
      // A photograph in the selection but not on the page has no thumbnail here. It still moves; it is just not
      // one of the three being shown.
      const img = sheet.querySelector('[data-id="' + id + '"] img');
      if (!img) continue;
      const copy = img.cloneNode(false);
      copy.removeAttribute('class');
      stack.appendChild(copy);
    }
    ghost.appendChild(stack);

    const label = document.createElement('span');
    label.className = 'draglabel';
    label.textContent = 'Flytter ' + ctx.photoCount(moving.length);
    ghost.appendChild(label);

    document.body.appendChild(ghost);
    return ghost;
  }

  function start(e) {
    const moving = ctx.selected.has(press.id) ? Array.from(ctx.selected) : [press.id];
    const ghost = carry(moving);
    drag = { moving: moving, ghost: ghost, target: null, after: false };
    const set = new Set(moving);
    // `dragging` takes them out of the grid entirely (page.css). Only a class, so the cells themselves — and the
    // selection painted on them — are untouched and an Escape puts them straight back.
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

    // The pointer resting on the gap it just opened is not a reason to close it again — and it is where the pointer
    // spends most of a drag, because the gap opens under it.
    if (onSlot(e.clientX, e.clientY)) return;

    const cell = cellAt(e.clientX, e.clientY);
    // The `dragging` test is belt and braces: those cells are out of the layout, so `elementFromPoint` cannot
    // return one. It stays because the server refuses a move whose target is one of the photographs moving, and
    // this is the client side of that same rule.
    if (!cell || cell.classList.contains('dragging')) {
      clearMarks();
      drag.target = null;
      return;
    }
    const r = cell.getBoundingClientRect();
    const after = e.clientX > r.left + r.width / 2;
    drag.target = cell.dataset.id;
    drag.after = after;
    if (gapIsOpen(cell, after)) return;
    clearMarks();
    openGap(cell, after, drag.moving.length);
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
