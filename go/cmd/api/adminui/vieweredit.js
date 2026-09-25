// Editing a photograph's caption and its credit from inside the viewer (tasks 407 and 408, PRD 023 §7.7).
//
// # Why these live here and not in viewer.js
//
// PRD 023 §7.3 describes the viewer as two shared files, and task 402's header says the caption and credit
// editors would each "add one entry to `actions`". They do — but registered from **this** side of the boundary
// rather than from inside the shared file, and the difference is worth the paragraph:
//
//   - The write is `PATCH /api/admin/photos`, behind the admin credential, and it **must** carry the working
//     year or the API refuses it. That year lives in `ctx.fetch` (main.js), which exists precisely because a
//     default would be a silent wrong-year write, and `TestTheAdminScriptsFetchOnlyThroughTheYear` holds that
//     nothing in this tool calls `fetch` directly. Registering here means the new write path inherits that guard
//     instead of needing its own.
//   - `viewer.js` stays free of every admin concept. `TestTheViewerKnowsNothingAboutItsSurfaces` forbids `/api/`
//     and `/admin` in it, and the point of that test is not tidiness: the shared file is loaded by an
//     unauthenticated public page, and the strongest possible statement about a public editing control is that
//     the code for it is not in the file that page loads.
//
// The seam this uses is the one task 402 built for it: `window.hejViewer.register(name, action)`, plus the
// `hv:show` event so an open editor follows the arrow keys.
//
// # Two fields, two saves, and that is not an oversight
//
// A caption and a credit look like the same widget and are not the same kind of thing, so they are deliberately
// separate controls with separate requests:
//
//   - **One form writing both would let a curator fixing a typo in a caption blank a credit** by leaving it
//     alone, which is the kind of loss nobody notices until a photographer asks why their name is gone.
//   - **The credit is the one field in this tool that publishes a person's name** (task 393, PRD 011's single
//     documented exception). It gets its own warning, and clearing it is its own button rather than "save an
//     empty field" — the rule `creditaction.js` already applies, for the reason it records: removing an
//     attribution should be a deliberate act and not something a stray select-all-and-delete does on its way
//     past. The arrow keys make that more true here, not less.
function initViewerEdit(ctx) {
  // Lucide, added to the viewer's own icon table. Same icon set as everything else (.rules), and the viewer
  // resolves the name when it builds the button, so adding to the table is enough.
  if (window.hejViewer && window.hejViewer.icons) {
    window.hejViewer.icons.pencil =
      '<path d="M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 ' +
      '.623.622l4.353-1.32a2 2 0 0 0 .83-.497z"/><path d="m15 5 4 4"/>';
    window.hejViewer.icons.credit =
      '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>';
  }

  // One panel, reused by both fields.
  //
  // Named editPanel rather than the obvious thing, because the obvious thing is already the album sheet's
  // variable name in this tool, and TestTheCuratorsActionsOpenAsOverlays greps the assembled source for that name
  // being shown or hidden by hand — which is how it catches one of the tool's four sheets bypassing the shared
  // shell. The collision was accidental and the guard was right to be blunt about it.
  //
  // This panel is genuinely not one of those sheets: it lives inside the viewer's own modal dialog, so opening it
  // through `ctx.openSheet` would raise the tool's scrim over an overlay that is already modal. Built on first use so a page that never opens the viewer never builds it.
  let editPanel = null;
  let open = null;

  // Whether anything was saved while the viewer was open, so the sheet can be refreshed **on close** rather than
  // mid-viewing.
  //
  // Reloading the sheet immediately would swap 120 thumbnails out from under a curator who is still looking at
  // one — and it is not even necessary for correctness, because the cell's attributes are updated in place
  // below. The reload on close is what reconciles with the server, which also keeps every other mark on the cell
  // honest.
  let dirty = false;

  const FIELDS = {
    caption: {
      icon: 'pencil',
      label: 'Rediger billedtekst',
      heading: 'Billedtekst',
      // The caption sheet's own sentence, copied rather than re-worded: two phrasings of one fact is how a
      // curator learns to distrust both.
      hint: 'Billedteksten hører til billedet og vises i alle album, det ligger i.',
      read: (item) => item.caption,
      attribute: 'data-caption',
      // The caption is also the cell's alt text, which is where the sheet's own caption panel reads it from.
      alsoAlt: true,
      clear: null,
    },
    credit: {
      icon: 'credit',
      label: 'Rediger fotokredit',
      heading: 'Fotokredit',
      hint:
        'Fotokreditten vises offentligt sammen med billedet. Det er det ene felt her, der nævner et navn — ' +
        'skriv kun en fotograf, der har sagt ja til det.',
      read: (item) => item.credit,
      attribute: 'data-credit',
      alsoAlt: false,
      clear: 'Fjern fotokredit',
    },
  };

  function build(dialog) {
    if (editPanel) return editPanel;

    editPanel = document.createElement('div');
    editPanel.className = 'hv-edit';
    editPanel.hidden = true;

    const heading = document.createElement('p');
    heading.className = 'hv-edit-heading';

    const hint = document.createElement('p');
    hint.className = 'hv-edit-hint';

    const field = document.createElement('textarea');
    field.className = 'hv-edit-field';
    field.rows = 2;

    const preview = document.createElement('p');
    preview.className = 'hv-edit-preview';

    const row = document.createElement('p');
    row.className = 'hv-edit-row';

    const save = document.createElement('button');
    save.type = 'button';
    save.textContent = 'Gem';

    const clear = document.createElement('button');
    clear.type = 'button';
    clear.className = 'hv-edit-clear';

    const cancel = document.createElement('button');
    cancel.type = 'button';
    cancel.textContent = 'Luk';

    const note = document.createElement('p');
    note.className = 'hv-edit-note';
    note.setAttribute('aria-live', 'polite');

    row.appendChild(save);
    row.appendChild(clear);
    row.appendChild(cancel);
    editPanel.appendChild(heading);
    editPanel.appendChild(hint);
    editPanel.appendChild(field);
    editPanel.appendChild(preview);
    editPanel.appendChild(row);
    editPanel.appendChild(note);

    // Inside the dialog, above the filmstrip. Not appended at the end: the info panel now sits over the
    // photograph (task 416), so "after the info panel" no longer means anything, and an editor below the
    // filmstrip reads as belonging to the strip rather than to the photograph.
    const strip = dialog.querySelector('.hv-strip');
    if (strip) {
      dialog.insertBefore(editPanel, strip);
    } else {
      dialog.appendChild(editPanel);
    }

    editPanel.els = { heading, hint, field, preview, row, save, clear, cancel, note };

    save.addEventListener('click', () => send(field.value.trim()));
    clear.addEventListener('click', () => send(''));
    cancel.addEventListener('click', close);

    // An open editor follows the arrow keys: caption this one, press right, caption the next. That loop is the
    // reason PRD 023 §4 was reopened to allow editing here at all.
    dialog.addEventListener('hv:show', (e) => {
      if (!open) return;
      open.vctx.item = e.detail.item;
      fill();
    });
    dialog.addEventListener('hv:close', () => {
      close();
      if (dirty) {
        dirty = false;
        // The sheet's own refresh, not a second path to the same place.
        ctx.reloadSheet();
      }
    });

    return editPanel;
  }

  function fill() {
    const spec = open.spec;
    const els = editPanel.els;
    els.heading.textContent = spec.heading;
    els.hint.textContent = spec.hint;
    // **Prefilled from the photograph**, never from `hej.admin.lastCredit`. That key exists because a *batch* has
    // no single current value to show and retyping one line per card is real tedium; with one photograph in front
    // of you the honest prefill is its own text.
    els.field.value = spec.read(open.vctx.item) || '';
    els.clear.hidden = !spec.clear;
    if (spec.clear) els.clear.textContent = spec.clear;
    els.note.textContent = '';
    paintPreview();
  }

  function paintPreview() {
    const els = editPanel.els;
    if (open.name !== 'credit') {
      els.preview.hidden = true;
      return;
    }
    // What a family will read under the photograph, in the words the curator just typed. A curator putting a
    // colleague's name on a public page should be looking at the public form of it while they do.
    const value = els.field.value.trim();
    els.preview.hidden = false;
    els.preview.textContent = value ? 'Offentligt: ' + value : 'Offentligt: ingen fotokredit';
  }

  function close() {
    if (!editPanel) return;
    editPanel.hidden = true;
    open = null;
  }

  async function send(value) {
    if (!open) return;
    const spec = open.spec;
    const item = open.vctx.item;
    const els = editPanel.els;

    if (!item.id) {
      els.note.textContent = 'Kan ikke gemme: billedet har ingen id.';
      return;
    }

    const body = { photoIds: [item.id] };
    body[open.name] = value;

    els.note.textContent = 'Gemmer…';
    try {
      const res = await ctx.fetch('/api/admin/photos', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        // **The typed text stays.** A caption is a sentence somebody composed, and losing it to a dropped hotel
        // connection is the failure that makes a curator stop trusting the tool.
        els.note.textContent = (payload && payload.error) || 'Kunne ikke gemme. Prøv igen.';
        return;
      }

      // The cell under the overlay, updated in place: the viewer reads its items from the DOM, so this is what
      // makes the info panel show the new text the moment it is saved.
      if (item.node) {
        if (value) {
          item.node.setAttribute(spec.attribute, value);
        } else {
          item.node.removeAttribute(spec.attribute);
        }
        if (spec.alsoAlt) {
          const img = item.node.querySelector('img');
          if (img) img.alt = value;
        }
      }
      dirty = true;
      els.note.textContent = value ? 'Gemt.' : 'Fjernet.';
      // Re-read the item from the DOM so the info panel above shows what was just saved.
      open.vctx.refresh();
      paintPreview();
    } catch (err) {
      els.note.textContent = 'Kunne ikke gemme. Prøv igen.';
    }
  }

  function opener(name) {
    const spec = FIELDS[name];
    return (vctx) => {
      build(vctx.dialog);
      open = { name, spec, vctx };
      editPanel.hidden = false;
      fill();
      editPanel.els.field.focus();
      editPanel.els.field.select();
    };
  }

  if (window.hejViewer && typeof window.hejViewer.register === 'function') {
    for (const name of Object.keys(FIELDS)) {
      window.hejViewer.register(name, {
        icon: FIELDS[name].icon,
        label: FIELDS[name].label,
        activate: opener(name),
      });
    }
  }

  // The preview follows typing, so the public form of a credit is visible while it is being written rather than
  // after it is saved.
  document.addEventListener('input', (e) => {
    if (open && editPanel && e.target === editPanel.els.field) paintPreview();
  });
}
