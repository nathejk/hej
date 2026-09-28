// the add-to-album action (task 375)
//
// The sheet opens over the page and the selection survives it, which is the constraint PRD 022 §7 puts on
// this layout: navigating away would lose the selection, and the selection is the input to every action.
//
// The album checkboxes and the inline create are htmx fragments (task 395). What is left here is the part that
// needs the selection: the note that counts it, and the add that sends it.
//
// Registered under `album` so the action bar can open it; see contactsheet.js.
function initAlbumAction(ctx) {
  const panel = document.getElementById('panel');
  const panelNote = document.getElementById('panelnote');

  function openAlbumPanel() {
    ctx.openSheet(panel);
    // The count is the browser's sentence to write — no fragment knows the selection. The fragment writes the
    // one only it can: "der er ingen album endnu".
    panelNote.textContent = 'Vælg et eller flere album. ' + ctx.photoCount(ctx.selected.size) +
      ' bliver lagt i dem.';
    // Fetched on open rather than once at page load, because albums get created while this page is open — by the
    // list above, or by the form inside this very sheet.
    //
    // No ticks are carried over from a previous open: the selection has changed, so the albums the curator chose
    // for the last one are not a statement about this one.
    ctx.fragment('POST', '/admin/fragments/albumpicker', '#albumlist');
  }

  document.getElementById('closepanel').addEventListener('click', () => {
    ctx.closeSheet();
  });

  document.getElementById('doadd').addEventListener('click', async () => {
    // Re-queried rather than held in a const: the fragment swap replaces this element, so a reference taken at
    // load would go stale the first time the picker rendered. Same reason its handlers are delegated.
    const albumIds = Array.from(document.querySelectorAll('#albumlist input:checked')).map((cb) => cb.value);
    if (!albumIds.length) { panelNote.textContent = 'Vælg mindst ét album.'; return; }
    if (!ctx.selected.size) { panelNote.textContent = 'Vælg mindst ét billede.'; return; }

    panelNote.textContent = 'Tilføjer…';
    // Captured before the selection is cleared below; the wait needs the same list the write was given.
    const ids = Array.from(ctx.selected);
    try {
      const res = await ctx.fetch('/api/admin/albums/items', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: ids, albumIds }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        // A 503 means nothing was filed, and the selection is deliberately kept so the curator can retry
        // without picking forty photographs again.
        panelNote.textContent = (payload && payload.error) || 'Kunne ikke tilføje billederne.';
        return;
      }

      // **Membership, per album** (task 457). `ctx.settled` is the wrong question again: these photographs
      // existed before the write. `albumCount` would be the wrong one too — a photograph already in another album
      // has a non-zero count either way — so what is waited for is the album read itself, which is also the read
      // whose marks the grid draws from.
      //
      // Each chosen album is asked separately and in order. Two or three is the normal case, they are independent
      // writes, and one that has not folded is the one the curator needs told about.
      let caught = true;
      for (const albumId of albumIds) {
        if (!await ctx.settledFilter(ids, '&album=' + encodeURIComponent(albumId), true)) caught = false;
      }
      // The server writes the sentence, because it is the side that knows how many were already there.
      ctx.actionNote.textContent = (payload.message || 'Tilføjet.') + (caught ? '' : ' ' + ctx.behindNote);
      ctx.closeSheet();
      ctx.selected.clear();
      // Reloaded so the album marks on the thumbnails are right, which is how the curator sees what is left
      // to sort. The counts in the page header follow from the same read.
      ctx.reloadSheet();
      // The album list's item counts just changed, so it is told to re-fetch.
      ctx.albumsChanged();
    } catch (err) {
      panelNote.textContent = 'Kunne ikke tilføje billederne. Prøv igen.';
    }
  });


  ctx.openers.album = openAlbumPanel;
}
