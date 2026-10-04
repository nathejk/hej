// "Behandl video igen" — put failed videos back in the transcode queue (PRD 029, task 497).
//
// An action rather than a control on the tile for viewaction.js's reason: the tile is a button, and a button inside
// it is invalid. It acts on the **failed videos in the selection** and says how many that was, so selecting a whole
// screen and pressing it is a safe way to retry everything that failed.
function initRetryAction(ctx) {
  const sheet = document.getElementById('sheet');

  ctx.openers.retry = async () => {
    const failed = [];
    for (const cell of sheet.querySelectorAll('[data-kind="video"][data-status="failed"]')) {
      if (ctx.selected.has(cell.dataset.id)) failed.push(cell.dataset.id);
    }
    if (!failed.length) {
      ctx.actionNote.textContent = 'Ingen af de valgte er en video, der er fejlet.';
      return;
    }
    const queued = [];
    for (const id of failed) {
      try {
        const res = await ctx.fetch('/api/admin/videos/retry/' + encodeURIComponent(id), { method: 'POST' });
        if (res.ok) queued.push(id);
      } catch (_) { /* counted as not retried below */ }
    }
    // Waited for, so the reload shows the tiles as processing rather than still failed (task 437).
    const caught = await ctx.settledRows(queued, (row) => row && row.status !== 'failed');
    const ok = queued.length;
    let line = ok === failed.length
      ? (ok === 1 ? 'Videoen behandles igen.' : ok + ' videoer behandles igen.')
      : ok + ' af ' + failed.length + ' videoer kunne sættes i kø igen. Prøv igen om lidt.';
    if (!caught) line += ' ' + ctx.behindNote;
    ctx.actionNote.textContent = line;
    ctx.reloadSheet();
  };
}
