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
    const res = await fetch('/api/admin/albums/' + encodeURIComponent(albumId), {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (res.status === 204) return;
    const out = await res.json().catch(() => null);
    if (!res.ok) throw new Error((out && out.error) || 'Fejl ' + res.status);
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
}
