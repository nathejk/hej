// the photo credit (task 393)
//
// # The only field in this tool that records a person's name
//
// Everything else here is arranged so the library names nobody — no uploader, no curator, no person id
// (PRD 022 §6), held by a structural walk over the types rather than by review. This is the exception, and it
// is narrow on purpose: a **consenting adult volunteer, credited as the author of a photograph**, from text a
// curator typed. Nothing derives it, and nothing here can look a name up — there is no endpoint that would.
//
// The copy in the sheet says so, because a curator typing a colleague's name onto a public page should know
// that is what they are doing rather than filling in a field that looks like a caption.
//
// # Two routes, and only one of them can be misspelled (PRD 025, task 453)
//
// A curator either **picks** a crew member — the id is stored and the name is resolved at read time — or **types**
// a name. Both end up as one line under the photograph and the public page cannot tell them apart, which is the
// point: the difference is entirely in what happens afterwards.
//
//   - A picked credit is spelled the same way on every photograph, in every session, on every laptop. And when
//     the photographer asks to be removed, deleting their crew record removes the name from every photograph at
//     once — there is nothing to find and nothing to rewrite.
//   - A typed credit is a string. It is the only way to credit a guest, and it is nobody's to correct later.
//
// So the sheet offers the list first and says so. The typed field stays, because not every photographer is in the
// roster and a curator who cannot credit somebody at all is a curator who credits nobody.
//
// # Why the remembered default lives in the browser
//
// A card is one photographer, so the tedium is real: without something remembered, every batch means retyping
// the same line. It is in 'localStorage' and **not** on the server, deliberately — the credential is shared
// (§8.2), so a server-side "my credit" would be an attribution the tool cannot honestly make. The browser
// remembering what *this laptop* last typed claims nothing about who is using it.
//
// Registered under `credit` so the action bar can open it; see contactsheet.js.
function initCreditAction(ctx) {
  const creditPanel = document.getElementById('creditpanel');
  const creditNote = document.getElementById('creditnote');
  const creditText = document.getElementById('credittext');
  const CREDIT_KEY = 'hej.admin.lastCredit';

  function openCreditPanel() {
    ctx.openSheet(creditPanel);
    creditNote.textContent = ctx.photoCount(ctx.selected.size) + ' får fotokreditten.';
    // Prefilled from the last one typed on this machine, so a second card is one click. Not prefilled from the
    // selection: the photographs may carry different credits, and picking one of them to show would be a guess
    // that silently overwrites the others when the curator presses the button.
    try {
      if (!creditText.value) creditText.value = window.localStorage.getItem(CREDIT_KEY) || '';
    } catch (err) { /* storage disabled or full: the field is simply empty */ }
    creditText.focus();
    creditText.select();
    // After the focus, so fetching the list cannot steal the caret from the typed field: a curator who opened
    // this sheet to type a guest's name should be able to start typing immediately.
    loadCrew();
  }

  // --- the crew list -------------------------------------------------------
  //
  // Fetched when the sheet opens rather than on page load: most sessions never open it, and `/admin` is served
  // `no-store` (task 371), so a roster fetched on load would be a request nobody asked for on every page view.
  //
  // Held after the first fetch, because a curator sets credits on card after card and the roster does not change
  // between them. Re-fetched when they ask for the whole crew, which is a different list.
  const crewSelect = document.getElementById('creditcrew');
  const crewSearch = document.getElementById('crewsearch');
  const crewAll = document.getElementById('crewall');
  let crew = null; // [{ id, name }] as fetched
  let crewSection = null; // the section the held list is for

  async function loadCrew() {
    const section = crewAll.checked ? 'all' : 'pr';
    if (crew && crewSection === section) return;

    crewSelect.innerHTML = '';
    creditNote.textContent = 'Henter crewlisten…';
    try {
      const res = await ctx.fetch('/api/admin/crew?section=' + encodeURIComponent(section));
      if (!res.ok) throw new Error('fejl ' + res.status);
      const out = await res.json();
      crew = (out && out.crew) || [];
      crewSection = section;
      creditNote.textContent = ctx.photoCount(ctx.selected.size) + ' får fotokreditten.';
    } catch (err) {
      crew = null;
      crewSection = null;
      // Said plainly, and the typed field below still works — which is why this is not fatal to the sheet.
      creditNote.textContent = 'Kunne ikke hente crewlisten. Du kan stadig skrive et navn.';
    }
    renderCrew();
  }

  // renderCrew draws the list, filtered by the search box.
  //
  // Filtered in the browser rather than by asking the server again: the list is a few dozen names, and a
  // round trip per keystroke would make the search feel worse than scrolling.
  function renderCrew() {
    const q = crewSearch.value.trim().toLowerCase();
    crewSelect.innerHTML = '';
    if (!crew) return;

    let shown = 0;
    for (const member of crew) {
      if (q && member.name.toLowerCase().indexOf(q) < 0) continue;
      const option = document.createElement('option');
      option.value = member.id;
      option.textContent = member.name;
      crewSelect.appendChild(option);
      shown++;
    }
    if (!shown) {
      // An empty listbox with no explanation reads as a broken fetch. This says which of the two it is.
      const none = document.createElement('option');
      none.disabled = true;
      none.textContent = crew.length
        ? 'Ingen i listen matcher “' + crewSearch.value.trim() + '”'
        : 'Ingen i crewet her — prøv “Vis hele crewet”';
      crewSelect.appendChild(none);
    }
  }

  crewSearch.addEventListener('input', renderCrew);
  crewAll.addEventListener('change', loadCrew);

  document.getElementById('closecredit').addEventListener('click', () => { ctx.closeSheet(); });

  // sendCredit writes one of the two forms. `remember` is the typed value to keep on this machine, or undefined.
  //
  // `proof` is the value the library's `credit=` filter should agree on afterwards — a crew id, an exact line, or
  // `none` for a clearing (task 457). Using the filter rather than comparing the row's `credit` is deliberate: a
  // crew credit is stored as an id and rendered as a name, so a browser comparing strings would be guessing at the
  // server's formatting. `credit=<crew id>` asks the projection the question in its own terms, and the same filter
  // expression answers all three forms.
  //
  // Two credit lines cannot be proved this way, because the filter spends those words on something else: a typed
  // credit of exactly "none" or "any" asks "no credit" and "either kind" instead. The wait then ends in a
  // "may be behind" note on a write that in fact landed, which is the harmless direction, and neither word is a
  // plausible credit line.
  async function sendCredit(fields, remember, proof) {
    if (!ctx.selected.size) { creditNote.textContent = 'Vælg mindst ét billede.'; return; }
    creditNote.textContent = 'Gemmer…';
    // Captured before the write: this sheet leaves the selection alone, but the wait must not depend on that.
    const ids = Array.from(ctx.selected);
    try {
      const body = Object.assign({ photoIds: ids }, fields);
      const res = await ctx.fetch('/api/admin/photos', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        creditNote.textContent = (payload && payload.error) || 'Kunne ikke sætte fotokreditten.';
        return;
      }
      // Only a typed credit is remembered. A picked one needs no default — the list is the default — and
      // storing an id in the browser would be a person reference sitting in localStorage for no reason.
      if (remember) {
        try { window.localStorage.setItem(CREDIT_KEY, remember); } catch (err) { /* nothing to recover */ }
      }
      ctx.closeSheet();
      const caught = await ctx.settledFilter(ids, '&credit=' + encodeURIComponent(proof), true);
      // A credit is the one field on this page that a stranger reads, so a grid that still shows the old one is
      // worth a sentence rather than a silent reload.
      ctx.actionNote.textContent = 'Fotokredit gemt på ' + ctx.photoCount(ids.length) + '.' +
        (caught ? '' : ' ' + ctx.behindNote);
      // Reloaded so the sheet shows what was actually written, not what the browser hoped.
      ctx.reloadSheet();
    } catch (err) {
      creditNote.textContent = 'Kunne ikke sætte fotokreditten. Prøv igen.';
    }
  }

  document.getElementById('docreditcrew').addEventListener('click', () => {
    const id = crewSelect.value;
    if (!id) { creditNote.textContent = 'Vælg en fotograf i listen, eller skriv et navn nedenfor.'; return; }
    // The id, never the name. Sending the name would put it on the append-only log, where it could not be
    // erased — which is the whole reason this path exists (PRD 025 §8 D1).
    sendCredit({ creditCrewId: id }, undefined, id);
  });

  document.getElementById('docredit').addEventListener('click', () => {
    const credit = creditText.value.trim();
    if (!credit) { creditNote.textContent = 'Skriv en fotokredit, eller brug “Fjern fotokredit”.'; return; }
    sendCredit({ credit: credit }, credit, credit);
  });

  // Clearing is its own button rather than "save an empty field", so removing an attribution is a deliberate act
  // and not something a stray select-all-and-delete does on its way past.
  // Clearing sends an empty **typed** credit, which the fold writes over both fields — so one button removes
  // either kind of credit and a curator does not have to know which kind is on the photograph.
  // `none` rather than an empty `credit=`: the filter reads an empty value as "no filter", so asking with one
  // would wait for nothing at all and always succeed. `credit=none` is the projection's way of saying "neither
  // kind", which is exactly what clearing produces.
  document.getElementById('doclearcredit').addEventListener('click', () => sendCredit({ credit: '' }, '', 'none'));

  ctx.openers.credit = openCreditPanel;
}
