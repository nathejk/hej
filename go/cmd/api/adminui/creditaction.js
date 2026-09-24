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
  }

  document.getElementById('closecredit').addEventListener('click', () => { ctx.closeSheet(); });

  async function sendCredit(credit) {
    if (!ctx.selected.size) { creditNote.textContent = 'Vælg mindst ét billede.'; return; }
    creditNote.textContent = 'Gemmer…';
    try {
      const res = await ctx.fetch('/api/admin/photos', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: Array.from(ctx.selected), credit: credit }),
      });
      const payload = await res.json().catch(() => null);
      if (!res.ok) {
        creditNote.textContent = (payload && payload.error) || 'Kunne ikke sætte fotokreditten.';
        return;
      }
      if (credit) {
        try { window.localStorage.setItem(CREDIT_KEY, credit); } catch (err) { /* nothing to recover */ }
      }
      ctx.closeSheet();
      // Reloaded so the sheet shows what was actually written, not what the browser hoped.
      ctx.reloadSheet();
    } catch (err) {
      creditNote.textContent = 'Kunne ikke sætte fotokreditten. Prøv igen.';
    }
  }

  document.getElementById('docredit').addEventListener('click', () => {
    const credit = creditText.value.trim();
    if (!credit) { creditNote.textContent = 'Skriv en fotokredit, eller brug “Fjern fotokredit”.'; return; }
    sendCredit(credit);
  });

  // Clearing is its own button rather than "save an empty field", so removing an attribution is a deliberate act
  // and not something a stray select-all-and-delete does on its way past.
  document.getElementById('doclearcredit').addEventListener('click', () => sendCredit(''));

  ctx.openers.credit = openCreditPanel;
}
