// removing and deleting (task 379)
//
// Two different acts behind one button, and the panel's job is to make the difference unmissable. Removing from
// an album leaves the photograph in the archive; deleting takes it out of everything and frees its bytes. One of
// the two is what somebody means by "take it down", and if they read alike the wrong one gets pressed.
//
// Both loop the selection with the uploader's bounded concurrency rather than a bulk endpoint: one event per
// photograph keeps the audit log one line per act, which with a shared credential is the only record there is.
//
// Registered under `delete` so the action bar can open it; see contactsheet.js.
function initDeleteAction(ctx) {
  const delPanel = document.getElementById('delpanel');
  const delNote = document.getElementById('delnote');
  const delReason = document.getElementById('delreason');
  const delCount = document.getElementById('delcount');
  const doRemove = document.getElementById('doremove');

  function openDeletePanel() {
    ctx.openSheet(delPanel);
    delReason.value = '';
    doRemove.disabled = true;

    const n = ctx.selected.size;
    delNote.textContent = n === 1 ? '1 billede er valgt.' : n + ' billeder er valgt.';
    delCount.textContent = n === 1 ? '1 billede' : n + ' billeder';

    // The album select is a fragment. Fetched on open rather than at page load, because albums are created while
    // this page is open.
    ctx.fragment('GET', '/admin/fragments/delalbumpicker', '#delalbum');
  }

  // Delegated from the sheet rather than bound to the select, because the fragment swap replaces that element.
  delPanel.addEventListener('change', (e) => {
    if (e.target.id === 'delalbum') doRemove.disabled = !e.target.value;
  });
  document.getElementById('closedel').addEventListener('click', () => { ctx.closeSheet(); });

  // runOverSelection issues one request per ctx.selected photograph, three at a time.
  //
  // The uploader's pattern, for the uploader's reasons: one failure does not take the batch with it, and progress
  // is real rather than a bar that finishes and then waits.
  async function runOverSelection(makeRequest) {
    const ids = Array.from(ctx.selected);
    let ok = 0, missing = 0, failed = 0;
    let i = 0;

    async function worker() {
      for (;;) {
        const id = ids[i++];
        if (id === undefined) return;
        try {
          const res = await makeRequest(id);
          if (res.status === 404) missing++;
          else if (res.ok || res.status === 204) ok++;
          else failed++;
        } catch (err) {
          failed++;
        }
      }
    }

    await Promise.all([worker(), worker(), worker()]);
    return { ok, missing, failed };
  }

  doRemove.addEventListener('click', async () => {
    const sel = document.getElementById('delalbum');
    const albumId = sel ? sel.value : '';
    if (!albumId) return;

    delNote.textContent = 'Fjerner…';
    const r = await runOverSelection((id) =>
      fetch('/api/admin/albums/' + encodeURIComponent(albumId) + '/items/' + encodeURIComponent(id),
        { method: 'DELETE' }));

    // Said plainly, including the ones that were not in the album: a curator who ctx.selected across albums needs to
    // know why the number is smaller than their selection.
    const bits = [r.ok + (r.ok === 1 ? ' billede fjernet' : ' billeder fjernet')];
    if (r.missing) bits.push(r.missing + ' lå ikke i albummet');
    if (r.failed) bits.push(r.failed + ' fejlede');
    ctx.actionNote.textContent = bits.join(' · ') + '.';

    ctx.closeSheet();
    ctx.selected.clear();
    ctx.reloadSheet();
    // An album's item count just changed, so the list is told to re-fetch.
    ctx.albumsChanged();
  });

  document.getElementById('dodelete').addEventListener('click', async () => {
    const n = ctx.selected.size;
    // A native confirm, deliberately: this is the one irreversible action in the tool, and the browser's own dialog
    // is the one thing a curator cannot dismiss by muscle memory. The count is in the question because "select all
    // in filter" makes a mis-aimed delete plausible (PRD 022 §11 Q6).
    const word = n === 1 ? 'dette billede' : 'disse ' + n + ' billeder';
    if (!window.confirm('Slet ' + word + ' fra arkivet?\n\n' +
      'De forsvinder fra alle album og kan ikke ses offentligt. ' +
      'Det kan ikke fortrydes herfra.')) return;

    delNote.textContent = 'Sletter…';
    const body = JSON.stringify({ reason: delReason.value });
    const r = await runOverSelection((id) =>
      fetch('/api/admin/photos/' + encodeURIComponent(id),
        { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body }));

    const bits = [r.ok + (r.ok === 1 ? ' billede slettet' : ' billeder slettet')];
    if (r.failed) bits.push(r.failed + ' fejlede');
    ctx.actionNote.textContent = bits.join(' · ') + '.';

    ctx.closeSheet();
    ctx.selected.clear();
    ctx.reloadSheet();
    // A deleted photograph leaves every album it was in, so their counts and covers changed.
    ctx.albumsChanged();
  });

  ctx.openers.delete = openDeletePanel;
}
