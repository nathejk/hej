// The caption sheet (task 396).
//
// It replaced the album editor's one caption field per photograph, which was fine at twelve items and is not at
// two hundred. The caption belongs to the **photograph**, not to the album, so setting it here changes it in every
// album the photograph is in; the sheet says so.
//
// With one photograph selected the field is prefilled with its caption, read from the cell's `alt` — the same text,
// rendered by the same fragment. With several it starts empty: they may carry different captions, and showing one
// of them would be a guess that silently overwrites the others.
//
// Registered under `caption` so the action bar can open it; see contactsheet.js.
function initCaptionAction(ctx) {
  const panel = document.getElementById('captionpanel');
  const note = document.getElementById('captionnote');
  const text = document.getElementById('captiontext');

  function open() {
    ctx.openSheet(panel);
    const n = ctx.selected.size;
    note.textContent = ctx.photoCount(n) + ' får billedteksten.';
    text.value = '';
    if (n === 1) {
      const id = Array.from(ctx.selected)[0];
      const img = document.querySelector('#sheet .cell[data-id="' + id + '"] img');
      if (img) text.value = img.alt;
    }
    text.focus();
    text.select();
  }

  document.getElementById('closecaption').addEventListener('click', () => { ctx.closeSheet(); });

  document.getElementById('docaption').addEventListener('click', async () => {
    if (!ctx.selected.size) { note.textContent = 'Vælg mindst ét billede.'; return; }
    note.textContent = 'Gemmer…';
    // Captured before the write, because the sentence below counts them and the wait needs the same list.
    const ids = Array.from(ctx.selected);
    const caption = text.value.trim();
    try {
      const res = await ctx.fetch('/api/admin/photos', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: ids, caption: caption }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        note.textContent = (payload && payload.error) || 'Kunne ikke gemme billedteksten.';
        return;
      }
      ctx.closeSheet();
      // **Waits for the caption itself, not for the photographs** (task 457). They already existed, so
      // `ctx.settled` would return on the first ask and prove nothing — the race this loses is the one where the
      // grid is re-read before the fold, and then the sheet prefills from the *old* caption the next time it is
      // opened. That is worse than a stale thumbnail: the curator retypes a caption they already wrote.
      //
      // The row is compared to the string we sent, which we know exactly. `|| ''` because the field is
      // `omitempty` — a cleared caption is an absent key, and clearing is the case that must settle too.
      const caught = await ctx.settledRows(ids, (row) => row && (row.caption || '') === caption);
      ctx.actionNote.textContent = 'Billedtekst gemt på ' + ctx.photoCount(ids.length) + '.' +
        (caught ? '' : ' ' + ctx.behindNote);
      // Reloaded so the cells' alt text — which is what this sheet prefills from — is what was actually written.
      ctx.reloadSheet();
    } catch (err) {
      note.textContent = 'Kunne ikke gemme billedteksten. Prøv igen.';
    }
  });

  ctx.openers.caption = open;
}
