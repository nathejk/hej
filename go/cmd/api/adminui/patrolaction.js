// the patrol tag (task 377)
//
// No picker and no roster. 'publicpatrol.Queries' has one read, by number, and its doc says the absence of a
// list is deliberate — a list read is what a scraper would ask for. So the curator types the number from the
// patrol's sign, and the tool shows the name back before anything is saved.
//
// The confirmation is not decoration: patrol numbers are not unique per year and the resolve does
// 'ORDER BY teamId LIMIT 1', so a human seeing *which* patrol they got is what makes that defensible.
//
// Registered under `patrol` so the action bar can open it; see contactsheet.js.
function initPatrolAction(ctx) {
  const tagPanel = document.getElementById('tagpanel');
  const tagNote = document.getElementById('tagnote');
  const tagNum = document.getElementById('tagnum');
  const doTag = document.getElementById('dotag');

  // The number of the patrol the curator has confirmed, or ''.
  //
  // Read back out of the rendered confirmation rather than kept as a parallel copy of it, so the patrol this will
  // tag and the patrol on screen cannot disagree — which is the one way this could tag the wrong one.
  function confirmedNumber() {
    const found = document.getElementById('tagfound');
    return (found && found.dataset.number) || '';
  }

  function openTagPanel() {
    ctx.openSheet(tagPanel);
    clearPatrolConfirmation();
    tagNote.textContent = ctx.photoCount(ctx.selected.size) + ' bliver tagget.';
    tagNum.focus();
  }

  // Cleared in the browser rather than by asking the server for an empty line: there is nothing to render, and a
  // request to say "nothing yet" would be a round trip the curator waits for.
  function clearPatrolConfirmation() {
    const found = document.getElementById('tagfound');
    if (!found) return;
    found.textContent = '';
    found.classList.remove('ok');
    delete found.dataset.number;
    doTag.disabled = true;
  }

  // The confirmation itself is an htmx fragment: the lookup, the "no such patrol" line and the found patrol's
  // name, group and korps are all rendered server-side (see fragments.html). What stays here is the one thing the
  // fragment cannot reach — the tag button, which lives outside the swapped region because its click acts on the
  // selection.
  tagPanel.addEventListener('htmx:afterSwap', () => {
    doTag.disabled = confirmedNumber() === '';
  });

  // Any edit invalidates the confirmation, so the button cannot act on a patrol the curator is no longer looking
  // at.
  tagNum.addEventListener('input', clearPatrolConfirmation);

  document.getElementById('closetag').addEventListener('click', () => { ctx.closeSheet(); });

  doTag.addEventListener('click', async () => {
    const number = confirmedNumber();
    if (!number) { tagNote.textContent = 'Find patruljen først.'; return; }

    tagNote.textContent = 'Tagger…';
    // Captured before the selection is cleared below.
    const ids = Array.from(ctx.selected);
    try {
      // The **number** is sent, not a team id: the server re-resolves it, so a client cannot tag a patrol other
      // than the one the curator confirmed.
      const res = await ctx.fetch('/api/admin/photos/tags', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIds: ids, number: number }),
      });
      const out = await res.json().catch(() => null);
      if (!res.ok) {
        tagNote.textContent = (out && out.error) || 'Kunne ikke tagge billederne.';
        return;
      }
      // **Tagged-ness, through the library's own filter** (task 457). `tagCount` is a count and a photograph may
      // already carry a tag, so a count cannot say whether *this* tag landed; `tagged=yes` at least says the
      // projection has a tag for it, which is what the grid's mark draws from and what this write produced.
      //
      // It cannot prove the tag names *this* patrol: the library exposes no tag ids, deliberately — there is no
      // patrol list anywhere in the service (PRD 022 §8.6) and a tag-id read would be one. A curator tagging an
      // already-tagged photograph therefore gets a wait that can end early. That is a weaker proof than the
      // others here and it is the honest ceiling of the reads that exist.
      const caught = await ctx.settledFilter(ids, '&tagged=yes', true);
      ctx.actionNote.textContent = (out.message || 'Tagget.') + (caught ? '' : ' ' + ctx.behindNote);
      ctx.closeSheet();
      ctx.selected.clear();
      ctx.reloadSheet();
    } catch (err) {
      tagNote.textContent = 'Kunne ikke tagge billederne. Prøv igen.';
    }
  });

  ctx.openers.patrol = openTagPanel;
}
