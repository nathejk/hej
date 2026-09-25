// "Vis stort" — open the shared photo viewer on the selection (task 406, PRD 023).
//
// # Why this is an action rather than a control on the cell
//
// The gap it closes is real: the contact sheet exists to *sort*, so a click selects, and that left no gesture
// meaning "show me this one big enough to judge". A curator deciding whether a photograph is worth publishing
// was doing it from a 150px tile.
//
// The obvious fix — a small expand control in the cell's corner — cannot be built. The cell **is** a
// `<button>` carrying `role="option"`, inside `<div id="sheet" role="listbox">`, so a control in its corner is a
// button inside a button: invalid markup, unreliably clickable, and undescribable to a screen reader. An
// `option` may not contain interactive descendants either, so changing the element alone would not fix it. The
// four ways out and what each costs are written up in task 406; this is the one that leaves a click meaning
// exactly what it means today.
//
// **And the two-click cost is paid once, not per photograph**, because the viewer's arrow keys then walk the
// whole sheet: a curator enters it once and judges three hundred photographs from inside it. That is what makes
// the cheapest entry gesture the right one rather than a compromise.
//
// # Where it opens
//
// On the **first selected** photograph, in the sheet's own order. Not on "the last one clicked": with a
// shift-click range the last click is the end of the range, and opening at the end of a selection a curator
// just swept forwards reads as the viewer having lost its place.
//
// Registered under `view` so the action bar can dispatch to it; see contactsheet.js.
function initViewAction(ctx) {
  const sheet = document.getElementById('sheet');

  ctx.openers.view = () => {
    // No viewer, no overlay. The asset is an enhancement (PRD 023 §7.3): if it failed to load, the sheet is the
    // sheet a curator had before, and saying so is better than a button that appears to do nothing.
    if (!window.hejViewer || typeof window.hejViewer.open !== 'function') {
      ctx.actionNote.textContent = 'Billedviseren kunne ikke indlæses. Prøv at genindlæse siden.';
      return;
    }
    if (!ctx.selected.size) {
      ctx.actionNote.textContent = 'Vælg mindst ét billede.';
      return;
    }

    // The sheet's order, which in the album view is the album's order — the fragment renders it, so the DOM is
    // already right and there is nothing to sort here. That matters: the album view is where order is the
    // subject, and a viewer walking the library's order there would show a different sequence from the screen.
    const cells = sheet.querySelectorAll('[data-viewer-item]');
    for (let i = 0; i < cells.length; i++) {
      if (ctx.selected.has(cells[i].dataset.id)) {
        window.hejViewer.open(sheet, i);
        return;
      }
    }

    // Selected, but not on screen: the selection survives a filter change (PRD 022 §7) and the photographs it
    // names may have been filtered away. Say which, rather than opening something arbitrary.
    ctx.actionNote.textContent = 'De valgte billeder vises ikke med det aktuelle filter.';
  };
}
