// The album editor's card (task 378; part of the tool's page since task 396).
//
// Title, description, sort order and publication. The photographs are the shared contact sheet below the card, so
// their captions and order are not handled here any more: captions are the "Billedtekst" action on a selection
// (captionaction.js), and the one-field-per-photograph list and its ↑/↓ buttons went with the move, because
// neither scales to an album of 200.
//
// Does nothing on the all-photos view, which has no card.
function initAlbumEditor(ctx) {
  const editor = document.getElementById('albumeditor');
  if (!editor) return;
  const albumId = editor.dataset.album;
  const note = document.getElementById('editnote');

  function say(text, bad) {
    note.textContent = text;
    note.style.color = bad ? '#b91c1c' : '#166534';
  }

  async function send(body) {
    const res = await ctx.fetch('/api/admin/albums/' + encodeURIComponent(albumId), {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (res.status === 204) return null;
    const out = await res.json().catch(() => null);
    if (!res.ok) throw new Error((out && out.error) || 'Fejl ' + res.status);
    // Answered, because the sort control needs what came back: whether the album was re-sorted, and the order it
    // was re-sorted into. Every other caller ignores it.
    return out;
  }

  // The photographs' own order (PRD 024 §6 R10, task 446).
  //
  // Saved on `change`, not by the card's save button: it takes effect immediately — the server re-sorts the album
  // and the grid below reorders — and a control whose effect is visible while its value is still unsaved is the
  // combination nobody can reason about.
  //
  // `sortModeName` is shared with albumorder.js through the context, because the confirmation before a hand move
  // has to name the mode the curator chose here, in the same words they chose it in. Two lists of five labels
  // would eventually disagree, and the one that disagreed would be the one in the warning.
  const sortMode = document.getElementById('sortmode');
  if (sortMode) {
    sortMode.addEventListener('change', async () => {
      const mode = sortMode.value;
      say('Gemmer rækkefølgen…');
      sortMode.disabled = true;
      try {
        const out = await send({ sortMode: mode });
        // The card is the one place that knows the mode now, and albumorder.js reads it off the element before a
        // drag. Updated here rather than on the next page load, or the first drag after a change would ask the
        // wrong question — or none.
        editor.dataset.sortMode = mode;
        if (out && out.resorted) {
          say('Albummet er lagt om: ' + ctx.sortModeName(mode).toLowerCase() + '. Opdaterer…');
          // Waits for the **positions**, not for the photographs: they were already there. See ctx.settledOrder.
          const settled = await ctx.settledOrder(albumId, out.resortedOrder);
          ctx.reloadSheet();
          say(settled
            ? 'Albummet er nu sorteret efter ' + ctx.sortModeName(mode).toLowerCase() + '.'
            : 'Rækkefølgen er gemt. ' + ctx.behindNote);
        } else if (mode === 'manual') {
          // Nothing was reordered and nothing should have been: manual keeps the arrangement it was given, which
          // is what makes switching to it safe.
          say('Albummet er nu i manuel rækkefølge. Du bestemmer selv, og nye billeder lægges til sidst.');
        } else {
          say('Albummet lå allerede i den rækkefølge.');
        }
      } catch (err) {
        say('Kunne ikke gemme rækkefølgen: ' + err.message, true);
      } finally {
        sortMode.disabled = false;
      }
    });
  }

  document.getElementById('save').addEventListener('click', async () => {
    say('Gemmer…');
    try {
      // Only the fields on this form are sent. The request's pointer semantics mean anything omitted is left
      // alone, so this cannot disturb the publication state — which has its own buttons for exactly that reason.
      await send({
        title: document.getElementById('title').value,
        description: document.getElementById('desc').value,
        sortOrder: parseInt(document.getElementById('sort').value, 10) || 0,
      });
      say('Gemt.');
    } catch (err) {
      say(err.message, true);
    }
  });

  for (const [id, published] of [['publish', true], ['unpublish', false]]) {
    const b = document.getElementById(id);
    if (!b) continue;
    b.addEventListener('click', async () => {
      say(published ? 'Udgiver…' : 'Fjerner…');
      try {
        // Publication is sent **alone**, so an unsaved edit in the form above cannot ride along with it. A
        // curator pressing "udgiv" has said one thing and should not accidentally publish a half-typed title.
        await send({ published });
        // Reloaded rather than patched in place: the badge, the buttons and the public link all change together,
        // and re-rendering them by hand is three chances to show a stale one.
        location.reload();
      } catch (err) {
        say(err.message, true);
      }
    });
  }

  // "Gør til forsidebillede" (task 396), on the action bar because it acts on a selection — of exactly one. A cover
  // can be any photograph in the album, not only the first; the grid marks the current one.
  //
  // No sheet: there is nothing to choose once the photograph is picked, and a dialog asking "are you sure" about a
  // choice that is one more click to change would be noise.
  ctx.openers.cover = async () => {
    if (ctx.selected.size !== 1) {
      ctx.actionNote.textContent = 'Vælg ét billede for at gøre det til forsidebillede.';
      return;
    }
    const photoId = Array.from(ctx.selected)[0];
    ctx.actionNote.textContent = 'Gemmer forsidebillede…';
    try {
      await send({ coverPhotoId: photoId });
      ctx.actionNote.textContent = 'Forsidebilledet er skiftet.';
      // Reloaded so the mark moves to the cell the server now calls the cover.
      ctx.reloadSheet();
    } catch (err) {
      ctx.actionNote.textContent = err.message;
    }
  };
}
