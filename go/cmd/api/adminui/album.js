// The album editor (task 378).
//
// Ordinals are moved with buttons rather than dragged: PRD 022 §4 puts drag-to-reorder explicitly outside the
// launch requirements, and two buttons are keyboard-operable for free, which a drag handle is not.
(() => {
  'use strict';

  const main = document.querySelector('main');
  const albumId = main.dataset.album;
  const note = document.getElementById('note');
  const items = document.getElementById('items');

  function say(text, bad) {
    note.textContent = text;
    note.style.color = bad ? '#b91c1c' : '#166534';
  }

  async function send(path, method, body) {
    const res = await fetch(path, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (res.status === 204) return {};
    const out = await res.json().catch(() => null);
    if (!res.ok) throw new Error((out && out.error) || 'Fejl ' + res.status);
    return out || {};
  }

  // --- the album's own fields ----------------------------------------------

  document.getElementById('save').addEventListener('click', async () => {
    say('Gemmer…');
    try {
      // Only the fields on this form are sent. The request's pointer semantics mean anything omitted is left
      // alone, so this cannot disturb the publication state — which has its own buttons for exactly that reason.
      await send('/api/admin/albums/' + encodeURIComponent(albumId), 'PATCH', {
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
        await send('/api/admin/albums/' + encodeURIComponent(albumId), 'PATCH', { published });
        // Reloaded rather than patched in place: the header's state, the buttons and the public link all change
        // together, and re-rendering them by hand is three chances to show a stale one.
        location.reload();
      } catch (err) {
        say(err.message, true);
      }
    });
  }

  // --- captions -------------------------------------------------------------
  //
  // Saved on blur rather than with a button, because a caption per photograph would otherwise mean a button per
  // photograph. The caption belongs to the **photograph**, so this changes it in every album it appears in —
  // which the page says above the list, because it would otherwise be a surprise.

  if (items) {
    for (const input of items.querySelectorAll('.cap')) {
      let original = input.value;
      input.addEventListener('blur', async () => {
        if (input.value === original) return;
        try {
          await send('/api/admin/photos', 'PATCH', {
            photoIds: [input.dataset.photo],
            caption: input.value,
          });
          original = input.value;
          say('Billedtekst gemt.');
        } catch (err) {
          say(err.message, true);
        }
      });
    }

    // --- the order ----------------------------------------------------------

    items.addEventListener('click', (e) => {
      const up = e.target.closest('.up');
      const down = e.target.closest('.down');
      if (!up && !down) return;

      const li = e.target.closest('li');
      // Only live items move. A removed one has no position in the order being sent, and swapping past it would
      // silently change what "next" means.
      const movable = Array.from(items.querySelectorAll('li:not(.removed)'));
      const i = movable.indexOf(li);
      if (i < 0) return;

      const j = up ? i - 1 : i + 1;
      if (j < 0 || j >= movable.length) return;

      // Moved in the DOM only. Nothing is saved until "Gem rækkefølge", so a curator can shuffle freely and
      // change their mind without a dozen events on the log.
      if (up) items.insertBefore(li, movable[j]);
      else items.insertBefore(movable[j], li);
      say('Rækkefølgen er ikke gemt endnu.', true);
    });

    document.getElementById('saveorder').addEventListener('click', async () => {
      const order = Array.from(items.querySelectorAll('li:not(.removed)')).map((li) => li.dataset.photo);
      if (!order.length) { say('Der er ingen billeder at sortere.', true); return; }

      say('Gemmer rækkefølge…');
      try {
        // The whole live order in one request. The server publishes one event for it, because moving one item
        // into a position another holds is not expressible as independent writes — see album's ItemsReordered.
        await send('/api/admin/albums/' + encodeURIComponent(albumId) + '/items', 'PATCH', { photoIds: order });
        location.reload();
      } catch (err) {
        say(err.message, true);
      }
    });
  }
})();
