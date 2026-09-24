// the bulk position (task 376)
//
// Two ways to give the point, and the post picker is the one a curator actually uses: they know "Post 3", not
// a coordinate. The id is sent rather than the coordinate, so the server resolves it — a stale coordinate in
// this browser must not become a pin on a public map.
//
// Registered under `position` so the action bar can open it; see contactsheet.js.
function initPositionAction(ctx) {
  const posPanel = document.getElementById('pospanel');
  const posNote = document.getElementById('posnote');
  const posMapEl = document.getElementById('posmap');
  const posPicked = document.getElementById('pospicked');
  const doPosition = document.getElementById('doposition');

  // Re-queried rather than held, because the picker is a fragment and the swap replaces this element.
  function cpPick() { return document.getElementById('cppick'); }

  // The point chosen by clicking the map, if any. A post chosen in the select wins, because it is the more
  // precise statement of intent — and the server is told which of the two, never both.
  let clicked = null;
  let posMap = null;
  let posMarker = null;

  function describeChoice() {
    const sel = cpPick();
    if (sel && sel.value) {
      posPicked.textContent = 'Valgt: ' + sel.options[sel.selectedIndex].textContent;
      doPosition.disabled = false;
      return;
    }
    if (clicked) {
      posPicked.textContent = 'Valgt på kortet: ' + clicked.lat.toFixed(5) + ', ' + clicked.lng.toFixed(5);
      doPosition.disabled = false;
      return;
    }
    posPicked.textContent = '';
    doPosition.disabled = true;
  }

  // # Why the map is drawn last, and why it is measured again after (task 390)
  //
  // Leaflet computes its pixel size when the map is created and caches it. Creating one inside a container
  // that is hidden — or that the browser has not laid out yet — gives it a size of zero, and the symptom is a
  // map that loads one tile in the corner and ignores every drag. So 'drawPositionMap' runs after 'openSheet'
  // has made the sheet visible, and 'invalidateSize' is called on the frame after that, once layout has
  // settled. It was safe while the panel was an inline card that was already in flow; it is not safe now.
  //
  // The post picker's fragment (task 395) deliberately targets its own wrapper and never '#posmap', so no swap
  // can discard a live map instance — which would strand Leaflet's listeners and leak the tile layer.
  async function openPositionPanel() {
    ctx.openSheet(posPanel);
    clicked = null;
    posNote.textContent = ctx.selected.size === 1
      ? '1 billede får positionen.'
      : ctx.selected.size + ' billeder får positionen.';

    ctx.fragment('GET', '/admin/fragments/checkpointpicker', '#cppickwrap');
    describeChoice();
    await drawPositionMap();
    if (posMap) requestAnimationFrame(() => posMap.invalidateSize());
  }

  // The map island, reusing the vendored Leaflet and the shared layer config the app and the public map use
  // (task 353), so the curator places points on the same base map everybody else sees.
  //
  // # Why this is more than three lines (task 389)
  //
  // The first version read 'layers.layers[0]' and drew nothing at all, because 'maplayers.json' keys 'layers'
  // by **layer id** ('dtk25', 'dtk50', 'orto') rather than as an array — so 'base' was always 'undefined' and
  // the container sat empty. It also skipped three things the layer needs to actually render:
  //
  //   - the **token**. These are Dataforsyningen WMS endpoints and refuse an unauthenticated request; the
  //     token comes from '/api/config', the same place the Vue app reads it.
  //   - the 'layers' and 'format' **parameters**. A WMS URL without them is not a tile request.
  //   - **retry**. Leaflet has none: one failed image leaves that tile grey until something recreates it, and
  //     the file's own comment records that patchy rural data makes that the normal case rather than the
  //     exception. Ported from 'EventMap.vue''s 'attachTileRetry' rather than reinvented — same backoff, same
  //     jitter, same cache-buster, and the numbers come from the shared file so the two cannot drift.
  //
  // This page has no build step, so the app's TypeScript cannot be imported. The mitigation is that every
  // value is read from 'maplayers.json' at runtime: if the layer definitions or the retry policy change, this
  // map follows without an edit here.
  async function drawPositionMap() {
    if (typeof L === 'undefined') return; // Leaflet blocked or still loading: the picker is enough
    if (posMap) { posMap.invalidateSize(); return; }

    let cfg = null, token = '';
    try {
      const [layersRes, confRes] = await Promise.all([
        fetch('/maplayers.json'),
        fetch('/api/config'),
      ]);
      if (layersRes.ok) cfg = await layersRes.json();
      if (confRes.ok) token = (await confRes.json()).dataforsyningen_token || '';
    } catch (err) { /* fall through: an empty map is still clickable */ }

    posMapEl.hidden = false;
    const minZoom = (cfg && cfg.minZoom) || 7;
    const maxZoom = (cfg && cfg.maxZoom) || 18;
    posMap = L.map(posMapEl, { minZoom: minZoom, maxZoom: maxZoom }).setView([55.7332, 12.2648], 11);

    // The file's own default, by key. Falling back to the first entry rather than to a hard-coded id, so a
    // renamed default does not empty the map again — which is the bug this function had.
    const defs = (cfg && cfg.layers) || {};
    const key = (cfg && cfg.default && defs[cfg.default]) ? cfg.default : Object.keys(defs)[0];
    const base = key ? defs[key] : null;

    if (base && base.url) {
      const layer = L.tileLayer.wms(base.url, {
        layers: base.layer,
        format: base.format,
        transparent: false,
        crossOrigin: 'anonymous',
        attribution: base.attribution || (cfg && cfg.attribution) || '',
        token: token,
        maxZoom: maxZoom,
      });
      attachTileRetry(layer, (cfg && cfg.retry) || {});
      layer.addTo(posMap);
    } else {
      // Said out loud rather than left as a grey rectangle. The picker below still works, so this is a
      // degraded map and not a broken panel — but a curator staring at an empty square should be told which.
      posNote.textContent = 'Kortet kan ikke hentes lige nu. Du kan stadig vælge en post i listen.';
    }

    posMap.on('click', (e) => {
      clicked = { lat: e.latlng.lat, lng: e.latlng.lng };
      // Choosing on the map clears the post, so the two cannot both be sent — the server refuses that, and it
      // should never have to.
      const sel = cpPick();
      if (sel) sel.value = '';
      if (posMarker) posMarker.remove();
      posMarker = L.marker(e.latlng).addTo(posMap);
      describeChoice();
    });
  }

  // Retry a failed tile a few times before giving up, with exponential backoff and jitter.
  //
  // A port of EventMap.vue's attachTileRetry, deliberately faithful rather than simplified:
  //
  //   - re-assigning 'src' on the **same** <img> keeps Leaflet's own load/error handlers attached, so a late
  //     success still fades the tile in normally. Creating a new image would lose that.
  //   - the '_retry=' suffix defeats negative caching of the failed response by the browser or a proxy.
  //   - jitter stops a whole screen of failed tiles retrying in lockstep and hammering the service.
  //   - 'isConnected' guards a tile Leaflet has since discarded by panning or a layer swap.
  function attachTileRetry(layer, retry) {
    const limit = retry.limit || 3;
    const baseDelay = retry.baseDelayMs || 400;
    const jitter = retry.jitterMs || 250;

    layer.on('tileerror', (event) => {
      const tile = event.tile;
      if (!tile) return;
      const attempt = (tile._hejRetries || 0) + 1;
      if (attempt > limit) return;
      tile._hejRetries = attempt;

      const delay = baseDelay * Math.pow(2, attempt - 1) + Math.random() * jitter;
      const original = tile.src.replace(/&_retry=\d+$/, '');
      window.setTimeout(() => {
        if (!tile.isConnected) return;
        tile.src = original + '&_retry=' + attempt;
      }, delay);
    });
  }

  // Delegated from the sheet rather than bound to the select, because the fragment swap replaces that element.
  posPanel.addEventListener('change', (e) => {
    if (e.target.id !== 'cppick') return;
    if (e.target.value) {
      clicked = null;
      if (posMarker) { posMarker.remove(); posMarker = null; }
      const o = e.target.options[e.target.selectedIndex];
      if (posMap && o.dataset.lat) {
        const ll = [parseFloat(o.dataset.lat), parseFloat(o.dataset.lng)];
        posMarker = L.marker(ll).addTo(posMap);
        posMap.setView(ll, 14);
      }
    }
    describeChoice();
  });

  document.getElementById('closepos').addEventListener('click', () => { ctx.closeSheet(); });

  async function sendPosition(payload) {
    posNote.textContent = 'Gemmer…';
    try {
      const res = await fetch('/api/admin/photos', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(Object.assign({ photoIds: Array.from(ctx.selected) }, payload)),
      });
      const out = await res.json().catch(() => null);
      if (!res.ok) {
        posNote.textContent = (out && out.error) || 'Kunne ikke gemme positionen.';
        return;
      }
      // The server writes the sentence, because it is the side that ran the bounds check and knows which of the
      // three verdicts happened — and two of them are refusals the curator must not mistake for a fault at
      // their end.
      ctx.actionNote.textContent = out.message || 'Gemt.';
      ctx.closeSheet();
      ctx.selected.clear();
      ctx.reloadSheet();
    } catch (err) {
      posNote.textContent = 'Kunne ikke gemme positionen. Prøv igen.';
    }
  }

  doPosition.addEventListener('click', () => {
    const sel = cpPick();
    if (sel && sel.value) { sendPosition({ checkpointId: sel.value }); return; }
    if (clicked) { sendPosition({ location: { lat: clicked.lat, lng: clicked.lng } }); return; }
    posNote.textContent = 'Vælg en post eller klik på kortet.';
  });

  document.getElementById('doclear').addEventListener('click', () => {
    sendPosition({ clearLocation: true });
  });

  ctx.openers.position = openPositionPanel;
}
