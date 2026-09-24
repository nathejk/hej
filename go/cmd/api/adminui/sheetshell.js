// The sheet shell (task 390).
//
// One place that opens and closes an overlay, rather than each action hiding the other three by hand — which is
// what this replaced, four assignments per opener, and four more to forget when a fifth action lands.
//
// The keyboard handling is not decoration. A curator tagging three hundred photographs works fast and from the
// keyboard; a dialog that traps nothing and returns focus nowhere makes that impossible.
//
// Publishes `openSheet`, `closeSheet` and `fragment` onto the context. It needs nothing back, which is why it is
// initialised first: every other feature opens a sheet and none of them can open one before this runs.
function initSheetShell(ctx) {
  // --- the sheet shell (task 390) ---------------------------------------------
  //
  // One place that opens and closes an overlay, rather than each action hiding the other three by hand — which
  // is what this replaced, four assignments per opener, and four more to forget when a fifth action lands.
  //
  // The keyboard handling is not decoration. A curator tagging three hundred photographs works fast and from
  // the keyboard; a dialog that traps nothing and returns focus nowhere makes that impossible.
  const scrim = document.getElementById('scrim');
  // The element focus returns to when the sheet closes. Without it, closing a sheet drops focus to the top of
  // the document and the next Tab starts from the page header.
  let sheetOpener = null;
  let openSheetEl = null;

  function openSheet(el, opener) {
    if (openSheetEl && openSheetEl !== el) closeSheet({ restore: false });
    sheetOpener = opener || document.activeElement;
    openSheetEl = el;
    scrim.hidden = false;
    el.hidden = false;
    document.body.classList.add('sheetopen');
    focusFirst(el);
  }

  function closeSheet(opts) {
    if (!openSheetEl) return;
    openSheetEl.hidden = true;
    openSheetEl = null;
    scrim.hidden = true;
    document.body.classList.remove('sheetopen');
    // Restored unless one sheet is handing over to another, where the opener is about to be replaced anyway.
    if (!opts || opts.restore !== false) {
      if (sheetOpener && sheetOpener.isConnected) sheetOpener.focus();
      sheetOpener = null;
    }
  }

  function focusables(el) {
    return Array.prototype.filter.call(
      el.querySelectorAll('button, [href], input, select, textarea, summary, [tabindex]:not([tabindex="-1"])'),
      (n) => !n.disabled && n.offsetParent !== null
    );
  }

  function focusFirst(el) {
    const f = focusables(el);
    if (f.length) f[0].focus();
  }

  // Escape closes, and Tab cycles inside the open sheet instead of walking off into the contact sheet behind
  // it. Bound once on the document rather than per sheet.
  document.addEventListener('keydown', (e) => {
    if (!openSheetEl) return;
    if (e.key === 'Escape') { e.preventDefault(); closeSheet(); return; }
    if (e.key !== 'Tab') return;
    const f = focusables(openSheetEl);
    if (!f.length) return;
    const first = f[0], last = f[f.length - 1];
    if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  });

  // Clicking the backdrop closes. Deliberately **not** on the delete sheet: a stray click next to a
  // confirmation that says how many photographs it will delete should not be how that dialog goes away.
  scrim.addEventListener('click', () => {
    if (openSheetEl && openSheetEl.id === 'delpanel') return;
    closeSheet();
  });

  // fragment asks the server to render a piece of a sheet into it (task 395).
  //
  // # Why a sheet's fragment is fetched from here rather than declared in the markup
  //
  // The album list on the page can declare `hx-trigger="load"` because it is always wanted. A sheet's is wanted
  // *when the sheet opens*, and nothing htmx can put on a hidden element expresses that — `load` would fetch five
  // sheets' contents on every page load, and `revealed` does not fire for something unhidden by script.
  //
  // # Why htmx is read lazily
  //
  // htmx is a deferred script and this file is inline, so htmx does **not** exist while this block runs — only by
  // the time a curator can click. Reading `window.htmx` here rather than capturing it at the top is what makes
  // that ordering irrelevant. If it is missing entirely the sheet still opens, with an empty control: degraded,
  // which is the same choice the Leaflet island makes.
  function fragment(method, url, target) {
    if (!window.htmx) return;
    window.htmx.ajax(method, url, { target: target, swap: 'outerHTML' });
  }

  ctx.openSheet = openSheet;
  ctx.closeSheet = closeSheet;
  ctx.fragment = fragment;
}
