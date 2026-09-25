// The shared photo viewer (task 402, PRD 023 §6, §7.2, §7.7).
//
// One full-viewport overlay, used unchanged by the public album page and by the curator's admin tool. No
// framework: the public page loads neither htmx nor Alpine, and this has to be the same file on both surfaces.
// No build step either — it is served from the Go binary exactly as written.
//
// # It does not know which surface it is on, and that is load-bearing
//
// Everything page-specific arrives through `data-` attributes (PRD 023 §7.7): the list of photographs, their
// URLs, their captions, and **which controls the action row carries**. There is deliberately no `isAdmin`
// anywhere below. The same behaviour written as a branch would be one inverted boolean away from a public edit
// button — and since nothing in the Go test suite can execute this file, nobody would find out from a test.
// A control the host page never declares is a control this file never builds.
//
// # No template actions in here, ever
//
// This is a static asset served from a fixed map, not a template — the rule that already governs
// `adminui/*.js`. An action inside a script is escaped as JavaScript by `html/template`, which mangles values
// in ways nobody notices until a browser does something strange, and it stops the file being something a
// formatter or a linter can read.
//
// # What lives elsewhere
//
// The action row is a registry, and the controls that go in it are their own tasks: fullscreen (404), share
// (405), the caption editor (407) and the credit editor (408) each add one entry to `actions` below. A declared
// action this file does not know about is skipped in silence, so a page may ask for a control before it exists
// and simply not get one. That is what keeps those four tasks separable from this one.
//
// `pictureFor` is the single place an image URL is chosen, so task 410's `srcset` work is one function rather
// than a hunt.
(function () {
  'use strict';

  // Lucide, inline, because there is no build step to import a component through (.rules). Same icon set as
  // the app, different delivery.
  var ICONS = {
    prev: '<path d="m15 18-6-6 6-6"/>',
    next: '<path d="m9 18 6-6-6-6"/>',
    close: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
    share:
      '<path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/>' +
      '<line x1="12" x2="12" y1="2" y2="15"/>',
    maximize:
      '<path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/>' +
      '<path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/>',
    minimize:
      '<path d="M8 3v3a2 2 0 0 1-2 2H3"/><path d="M21 8h-3a2 2 0 0 1-2-2V3"/>' +
      '<path d="M3 16h3a2 2 0 0 1 2 2v3"/><path d="M16 21v-3a2 2 0 0 1 2-2h3"/>',
  };

  function icon(name) {
    return (
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" ' +
      'stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' +
      ICONS[name] +
      '</svg>'
    );
  }

  function button(className, iconName, label) {
    var el = document.createElement('button');
    el.type = 'button';
    el.className = className;
    el.innerHTML = icon(iconName);
    el.setAttribute('aria-label', label);
    el.title = label;
    return el;
  }

  // The action registry. Later tasks add entries; each is { icon, label, activate(ctx) } and gets one button in
  // the row, in the order the host page declared them.
  var actions = {};

  // The viewer is a singleton: one dialog in the document, whatever opened it. Two overlays over one page is a
  // state with no sensible meaning, and building the DOM once keeps the filmstrip's scroll position honest
  // between openings of the same album.
  var ui = null;

  function build() {
    if (ui) return ui;

    var dialog = document.createElement('dialog');
    dialog.className = 'hv';

    var stage = document.createElement('div');
    stage.className = 'hv-stage';

    var prev = button('hv-nav hv-prev', 'prev', 'Forrige billede');
    var next = button('hv-nav hv-next', 'next', 'Næste billede');

    var img = document.createElement('img');
    img.className = 'hv-img';
    // Decorative here: the caption is in the panel below, and an alt repeating it would have a screen reader
    // read the same sentence twice.
    img.alt = '';

    var missing = document.createElement('p');
    missing.className = 'hv-missing';
    missing.textContent = 'Billedet er ikke tilgængeligt.';

    var bar = document.createElement('div');
    bar.className = 'hv-bar';

    var info = document.createElement('div');
    info.className = 'hv-info';

    var strip = document.createElement('div');
    strip.className = 'hv-strip';

    stage.appendChild(bar);
    stage.appendChild(prev);
    stage.appendChild(img);
    stage.appendChild(missing);
    stage.appendChild(next);
    // **The info panel lives over the photograph, not under it** (task 416).
    //
    // It used to be a row of the dialog's grid, which meant a caption changed the height of the stage — so the
    // arrows moved between one photograph and the next depending on whether it had a caption. Inside the stage and
    // positioned over it, the geometry is the same for every photograph.
    stage.appendChild(info);
    dialog.appendChild(stage);
    dialog.appendChild(strip);
    document.body.appendChild(dialog);

    ui = {
      dialog: dialog,
      stage: stage,
      img: img,
      bar: bar,
      info: info,
      strip: strip,
      prev: prev,
      next: next,
      // The open album: its items, where we are in it, and what the host page asked for.
      items: [],
      index: 0,
      opener: null,
      config: {},
      pushed: false,
    };

    prev.addEventListener('click', function () { move(-1); });
    next.addEventListener('click', function () { move(1); });

    img.addEventListener('load', function () {
      dialog.classList.remove('is-loading');
      dialog.classList.remove('is-missing');
    });
    img.addEventListener('error', function () {
      // Not an error state to recover from: a photograph can be taken down between the page loading and a
      // visitor reaching it. Say so, and let the arrows keep working.
      dialog.classList.remove('is-loading');
      dialog.classList.add('is-missing');
    });

    dialog.addEventListener('keydown', onKeydown);
    dialog.addEventListener('close', onClose);
    // The backdrop is part of the dialog's box, so a click that lands on the dialog itself rather than on any
    // of its children is a click outside the photograph.
    dialog.addEventListener('click', function (event) {
      if (event.target === dialog) dialog.close();
    });

    bindSwipe(stage);

    return ui;
  }

  // Read the items out of the DOM, every time the viewer opens.
  //
  // Not cached, deliberately, and this is the same reasoning the admin contact sheet gives for rebuilding its
  // `order` after a swap: "the photographs currently on the page" is not state, it is a fact about the DOM, and
  // deriving it from the DOM cannot be wrong. It is also what makes the viewer work after a "Vis flere" link,
  // an htmx swap, or anything else that adds tiles.
  function itemsIn(container) {
    var nodes = container.querySelectorAll('[data-viewer-item]');
    var out = [];
    for (var i = 0; i < nodes.length; i++) {
      var node = nodes[i];
      out.push({
        node: node,
        full: node.getAttribute('data-full') || '',
        medium: node.getAttribute('data-medium') || '',
        thumb: node.getAttribute('data-thumb') || '',
        caption: node.getAttribute('data-caption') || '',
        credit: node.getAttribute('data-credit') || '',
        ordinal: node.getAttribute('data-viewer-ordinal') || '',
        id: node.getAttribute('data-viewer-id') || '',
        deleted: node.getAttribute('data-viewer-deleted') === 'true',
      });
    }
    return out;
  }

  // The one place an image URL is chosen.
  //
  // Task 410 gives this a `srcset` and a `sizes` hint so the browser can take an 800px rendition on a phone
  // instead of the 1600px display image. Until then the display image is all there is, and having one function
  // to change is the point of writing it this way now.
  function pictureFor(item) {
    return item.full || item.thumb;
  }

  function show(index) {
    var state = ui;
    if (index < 0 || index >= state.items.length) return;
    state.index = index;

    var item = state.items[index];
    state.dialog.classList.add('is-loading');
    state.dialog.classList.remove('is-missing');
    state.img.src = pictureFor(item);

    state.info.innerHTML = '';
    if (item.caption) {
      var caption = document.createElement('span');
      caption.className = 'hv-caption';
      caption.textContent = item.caption;
      state.info.appendChild(caption);
    }
    if (item.credit) {
      var credit = document.createElement('span');
      credit.className = 'hv-credit';
      credit.textContent = item.credit;
      state.info.appendChild(credit);
    }
    if (item.deleted) {
      // A photograph the curator has deleted from the library still appears in their sheet, marked. It must stay
      // marked here: presenting it as live is how somebody re-publishes something that was taken down on
      // purpose (PRD 023 §5).
      var gone = document.createElement('span');
      gone.className = 'hv-deleted';
      gone.textContent = 'Slettet fra arkivet.';
      state.info.appendChild(gone);
    }

    state.prev.disabled = index === 0;
    state.next.disabled = index === state.items.length - 1;

    markStrip(index);
    reflectURL(item);
    prefetchAround(index);

    // Announce the change, so a control that is showing something about *this* photograph can follow along.
    //
    // Generic on purpose: the viewer does not know that the admin tool's caption editor is listening, only that
    // something might be. It is what makes "caption this one, press the right arrow, caption the next" work
    // without the viewer knowing what a caption editor is.
    state.dialog.dispatchEvent(
      new CustomEvent('hv:show', { detail: { item: item, index: index } })
    );
  }

  function move(delta) {
    show(ui.index + delta);
  }

  function markStrip(index) {
    var buttons = ui.strip.children;
    for (var i = 0; i < buttons.length; i++) {
      var current = i === index;
      buttons[i].setAttribute('aria-current', current ? 'true' : 'false');
      if (current) {
        // `nearest` rather than `center` for the block axis: centring vertically would scroll the *page* behind
        // the dialog on some browsers, and the strip only ever needs to move sideways.
        buttons[i].scrollIntoView({ block: 'nearest', inline: 'center' });
      }
    }
  }

  function fillStrip() {
    var strip = ui.strip;
    strip.innerHTML = '';
    for (var i = 0; i < ui.items.length; i++) {
      var item = ui.items[i];
      var thumb = document.createElement('button');
      thumb.type = 'button';
      thumb.setAttribute('aria-label', item.caption || 'Billede ' + (i + 1));
      var img = document.createElement('img');
      img.src = item.thumb || pictureFor(item);
      img.alt = '';
      img.loading = 'lazy';
      img.decoding = 'async';
      thumb.appendChild(img);
      thumb.addEventListener('click', jumpTo(i));
      strip.appendChild(thumb);
    }
  }

  function jumpTo(index) {
    return function () { show(index); };
  }

  // Two ahead and one back (PRD 023 §6), and no further.
  //
  // Revised upward from one-each-way once §2 established that albums are read after the event on home
  // connections rather than on a congested cell at the finish line: the next photograph being there when you
  // press the arrow is most of how this feels, and the cost of guessing wrong is small. Still bounded —
  // prefetching a whole album is the thing PRD 023 exists to stop.
  function prefetchAround(index) {
    var wanted = [index + 1, index + 2, index - 1];
    for (var i = 0; i < wanted.length; i++) {
      var at = wanted[i];
      if (at < 0 || at >= ui.items.length) continue;
      var url = pictureFor(ui.items[at]);
      if (!url) continue;
      var pre = new Image();
      pre.src = url;
    }
  }

  // Reflect the current photograph in the address, when the host page asked for it.
  //
  // `data-viewer-history="foto"` names the query parameter. Opt-in rather than automatic because only the
  // public album page has a server that understands the parameter (task 401); the admin tool's sheet is a
  // filtered view whose address means something else entirely.
  //
  // Push on open, replace while moving: back should close the viewer, not walk back through every photograph
  // somebody swiped past. The fragment goes on too, so that a copied address scrolls to the tile — a query
  // string alone scrolls nowhere, and a fragment alone never reaches the server (task 401).
  function reflectURL(item) {
    var param = ui.config.history;
    if (!param || !item.ordinal || !window.history) return;

    var url = new URL(window.location.href);
    url.searchParams.set(param, item.ordinal);
    url.hash = param + '-' + item.ordinal;

    if (ui.pushed) {
      window.history.replaceState({ hv: true }, '', url.toString());
      return;
    }
    window.history.pushState({ hv: true }, '', url.toString());
    ui.pushed = true;
  }

  function onKeydown(event) {
    switch (event.key) {
      case 'ArrowLeft':
        event.preventDefault();
        move(-1);
        break;
      case 'ArrowRight':
        event.preventDefault();
        move(1);
        break;
      case 'Home':
        event.preventDefault();
        show(0);
        break;
      case 'End':
        event.preventDefault();
        show(ui.items.length - 1);
        break;
      case 'Escape':
        // **Esc in fullscreen leaves fullscreen, and nothing else** (task 404).
        //
        // Two features want this key: the browser exits fullscreen with it, and a modal dialog closes with it.
        // Which one wins is not something to leave to a browser to arbitrate — the orders differ, and "Esc
        // closed the whole viewer when I only wanted the window back" is a complaint nobody can reproduce on
        // demand. So it is decided here: in fullscreen, Esc is the fullscreen key; otherwise it is the dialog's.
        if (fullscreenElement()) {
          event.preventDefault();
          exitFullscreen();
        }
        break;
      default:
        // Tab is the dialog's own: showModal traps focus without help.
        break;
    }
  }

  function bindSwipe(stage) {
    var startX = 0;
    var startY = 0;
    var tracking = false;

    stage.addEventListener(
      'pointerdown',
      function (event) {
        if (event.pointerType === 'mouse') return;
        tracking = true;
        startX = event.clientX;
        startY = event.clientY;
      },
      { passive: true }
    );

    stage.addEventListener(
      'pointerup',
      function (event) {
        if (!tracking) return;
        tracking = false;
        var dx = event.clientX - startX;
        var dy = event.clientY - startY;
        // Horizontal intent only, and a threshold generous enough that a tap with a shaky thumb is still a tap.
        // Without the vertical comparison, a scroll attempt on a tall photograph changes the picture.
        if (Math.abs(dx) < 40 || Math.abs(dx) < Math.abs(dy)) return;
        move(dx < 0 ? 1 : -1);
      },
      { passive: true }
    );
  }

  function renderActions(names) {
    ui.bar.innerHTML = '';
    for (var i = 0; i < names.length; i++) {
      var name = names[i];
      var action = actions[name];
      // A declared action this build does not have is skipped rather than announced. A host page may ask for a
      // control whose task has not landed yet, and an empty gap beats a button that does nothing.
      if (!action) continue;
      ui.bar.appendChild(actionButton(name, action));
    }
    ui.bar.appendChild(closeButton());
  }

  function actionButton(name, action) {
    var el = document.createElement('button');
    el.type = 'button';
    el.className = 'hv-act hv-act-' + name;
    el.innerHTML = icon(action.icon);
    el.setAttribute('aria-label', action.label);
    el.title = action.label;
    el.addEventListener('click', function () {
      action.activate(context(el));
    });
    return el;
  }

  // What an action gets to work with. Narrow on purpose: the current item, the means to redraw it, and its own
  // button for state. An action that needed the whole `ui` would be an action that could move the viewer, and
  // the four that exist need none of that.
  function context(el) {
    return {
      item: ui.items[ui.index],
      button: el,
      dialog: ui.dialog,
      config: ui.config,
      refresh: function () { show(ui.index); },
    };
  }

  function closeButton() {
    var el = button('hv-close', 'close', 'Luk');
    el.addEventListener('click', function () { ui.dialog.close(); });
    return el;
  }

  function open(container, index) {
    build();

    ui.items = itemsIn(container);
    if (!ui.items.length) return;
    if (index < 0 || index >= ui.items.length) index = 0;

    ui.config = {
      history: container.getAttribute('data-viewer-history') || '',
      shareTitle: container.getAttribute('data-share-title') || document.title,
      captionEndpoint: container.getAttribute('data-caption-endpoint') || '',
      year: container.getAttribute('data-year') || '',
    };
    ui.pushed = false;
    ui.opener = document.activeElement;
    ui.dialog.setAttribute(
      'aria-label',
      container.getAttribute('data-viewer-label') || 'Billeder'
    );

    renderActions(declaredActions(container));
    fillStrip();

    lockScroll();
    ui.dialog.showModal();
    show(index);
    // After showModal, so the dialog is focusable. The close button rather than the photograph: it is the one
    // control every viewer has, and landing on it means the first Tab goes forwards through the row.
    var close = ui.bar.querySelector('.hv-close');
    if (close) close.focus();
  }

  function declaredActions(container) {
    var raw = container.getAttribute('data-viewer-actions') || '';
    var names = raw.split(',');
    var out = [];
    for (var i = 0; i < names.length; i++) {
      var name = names[i].trim();
      if (name) out.push(name);
    }
    return out;
  }

  function onClose() {
    unlockScroll();
    ui.img.removeAttribute('src');

    // Leaving fullscreen with the viewer, because the thing that was fullscreen is the thing being closed.
    // Without this, closing while fullscreen leaves the browser filling the screen with the album page behind —
    // no chrome, no viewer, and no obvious way back.
    if (fullscreenElement()) exitFullscreen();

    // Announced for the same reason as hv:show: a host page may have work it deliberately deferred until the
    // overlay was out of the way — the admin tool reloads its sheet here rather than swapping 120 thumbnails out
    // from under somebody who is still looking at one.
    ui.dialog.dispatchEvent(new CustomEvent('hv:close'));

    // Wind the address back to the album. Only when we put an entry there, or a viewer that never touched
    // history would send the visitor off the page.
    if (ui.pushed && window.history) {
      ui.pushed = false;
      window.history.back();
    }
    if (ui.opener && typeof ui.opener.focus === 'function') {
      ui.opener.focus();
    }
    ui.opener = null;
  }

  // Scroll lock, with the position remembered.
  //
  // `showModal` makes the page behind inert but does not reliably stop it scrolling, and a viewer that returns
  // you to a different part of the album than you left is disorienting in a way nobody can quite name.
  var scrollY = 0;

  function lockScroll() {
    scrollY = window.scrollY || 0;
    document.documentElement.style.overflow = 'hidden';
  }

  function unlockScroll() {
    document.documentElement.style.overflow = '';
    window.scrollTo(0, scrollY);
  }

  // A click on a tile opens the viewer instead of following the link.
  //
  // Delegated from the container, so tiles appended later — a "Vis flere" page, an htmx swap — need no
  // registration.
  //
  // **Modified clicks are left alone.** A cmd-click, middle-click or shift-click on a tile means "open the
  // photograph in a new tab or window", and the whole reason every tile is a real `<a href>` (PRD 023 §6) is
  // that the plain page has to work. Swallowing those would take a working browser gesture away in order to
  // show an overlay the visitor did not ask for.
  function bindContainer(container) {
    if (container.getAttribute('data-viewer-bound') === 'true') return;
    container.setAttribute('data-viewer-bound', 'true');

    // **A container may keep its clicks** (task 406), with `data-viewer-click="none"`.
    //
    // The curator's contact sheet is the case this exists for: a click there **selects**, and PRD 022 §7 builds
    // the whole tool on that. Intercepting it to show an overlay would make the one gesture a curator uses three
    // hundred times a sitting mean two things. That sheet opens the viewer from its action bar instead, through
    // `hejViewer.open`.
    //
    // Opt-out rather than opt-in, so the public page — where a tile is a link to a photograph and clicking it
    // obviously means "show me" — needs no attribute to get the obvious behaviour.
    if (container.getAttribute('data-viewer-click') === 'none') return;

    container.addEventListener('click', function (event) {
      if (event.defaultPrevented) return;
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;

      var opener = event.target.closest('[data-viewer-open], [data-viewer-item]');
      if (!opener) return;
      var item = opener.closest('[data-viewer-item]');
      if (!item) return;

      var items = container.querySelectorAll('[data-viewer-item]');
      var index = Array.prototype.indexOf.call(items, item);
      if (index < 0) return;

      event.preventDefault();
      open(container, index);
    });
  }

  function bindAll() {
    var containers = document.querySelectorAll('[data-viewer]');
    for (var i = 0; i < containers.length; i++) {
      bindContainer(containers[i]);
    }
  }

  // Open on load when the address names a photograph (task 401's `?foto=`).
  //
  // The server has already rendered the page that holds it, so the item is in the DOM; this only opens the
  // overlay on it. Without JavaScript the same address is the right page scrolled to the right tile, which is
  // the whole point of the parameter being a query rather than app state.
  function openFromURL() {
    var containers = document.querySelectorAll('[data-viewer][data-viewer-history]');
    for (var i = 0; i < containers.length; i++) {
      var container = containers[i];
      var param = container.getAttribute('data-viewer-history');
      var wanted = new URL(window.location.href).searchParams.get(param);
      if (!wanted) continue;

      var items = itemsIn(container);
      for (var j = 0; j < items.length; j++) {
        if (items[j].ordinal === wanted) {
          open(container, j);
          // Opened from the address rather than from a click, so there is nothing to push: the entry the
          // visitor arrived on already names this photograph, and back belongs to wherever they came from.
          ui.pushed = true;
          return;
        }
      }
    }
  }

  window.addEventListener('popstate', function () {
    // Back was pressed while the viewer was open. Close it without winding history again — `onClose` only does
    // that for an entry this file pushed, and the press we are answering has already consumed it.
    if (ui && ui.dialog.open) {
      ui.pushed = false;
      ui.dialog.close();
    }
  });

  function start() {
    bindAll();
    openFromURL();
  }

  // The seam the other tasks use: fullscreen (404), share (405), the caption editor (407) and the credit
  // editor (408) each call `register` once. Exposed on `window` rather than kept private so that a host page can
  // register its own controls — which is what the admin tool's editors do — and rebind after replacing its tiles.
  window.hejViewer = {
    register: function (name, action) { actions[name] = action; },
    bind: bindAll,
    // For a host page that opens the viewer itself rather than on a click — see `data-viewer-click="none"`.
    open: open,
    icons: ICONS,
  };
  // Fullscreen (task 404, PRD 023 §7.5).
  //
  // # Why this is a real feature and not the same as the overlay
  //
  // The overlay already fills the viewport. Fullscreen escapes the browser *chrome* — the address bar, the
  // tabs, the toolbar — which the overlay cannot do, and on a laptop that is most of the screen. PRD 023 §2
  // establishes that albums are read after the event, often on a laptop, so this is not a nicety.
  //
  // # Three platform facts, built in rather than discovered
  //
  //   - **Safari needs the prefixed call.** Two lines; without them the button silently does nothing on a Mac.
  //   - **An iPhone has no element fullscreen at all.** iOS Safari implements fullscreen only for video;
  //     iPadOS supports it for elements. So the control is **absent** rather than inert where the API is
  //     missing: a visible button that does nothing is worse than no button, and it costs an iPhone nothing
  //     real, because the overlay already covers the viewport and an installed PWA is standalone anyway.
  //   - **The state is not ours to track.** A visitor leaves fullscreen with Esc, a swipe or a system gesture,
  //     none of which come through our button, so the icon and the label follow `fullscreenchange` rather than
  //     a variable we increment.
  function fullscreenElement() {
    return document.fullscreenElement || document.webkitFullscreenElement || null;
  }

  function fullscreenSupported() {
    if (document.fullscreenEnabled || document.webkitFullscreenEnabled) return true;
    return false;
  }

  function exitFullscreen() {
    if (document.exitFullscreen) {
      document.exitFullscreen();
    } else if (document.webkitExitFullscreen) {
      document.webkitExitFullscreen();
    }
  }

  function enterFullscreen(el) {
    if (el.requestFullscreen) {
      var request = el.requestFullscreen();
      if (request && typeof request.catch === 'function') {
        request.catch(function () {
          // **Reported, not swallowed** (task 416). The first version caught this and did nothing, which is how
          // "the fullscreen icon is not working" reached a maintainer rather than the person who wrote it.
          flash('Fuld skærm er ikke tilgængelig her.');
        });
      }
      return;
    }
    if (el.webkitRequestFullscreen) {
      el.webkitRequestFullscreen();
      return;
    }
    flash('Fuld skærm er ikke tilgængelig her.');
  }

  function paintFullscreenButton() {
    if (!ui) return;
    var el = ui.bar.querySelector('.hv-act-fullscreen');
    if (!el) return;

    var on = !!fullscreenElement();
    var label = on ? 'Afslut fuld skærm' : 'Fuld skærm';
    el.innerHTML = icon(on ? 'minimize' : 'maximize');
    el.setAttribute('aria-label', label);
    el.title = label;
  }

  document.addEventListener('fullscreenchange', paintFullscreenButton);
  document.addEventListener('webkitfullscreenchange', paintFullscreenButton);

  if (fullscreenSupported()) {
    window.hejViewer.register('fullscreen', {
      icon: 'maximize',
      label: 'Fuld skærm',
      activate: function (ctx) {
        if (fullscreenElement()) {
          exitFullscreen();
          return;
        }
        // **The document element, not the dialog** (task 416).
        //
        // Asking the dialog itself was the obvious thing and it did nothing at all — reported from Brave on
        // macOS, where the button appeared and had no effect. The dialog is in the **top layer**, put there by
        // `showModal`, and a top-layer element is precisely the awkward case for the fullscreen API: two
        // mechanisms that both mean "render this above everything", with engines disagreeing about the
        // combination.
        //
        // Fullscreening the page sidesteps the argument and looks identical, because the dialog already covers
        // the viewport and stays in the top layer over it. There is nothing of the page left to see.
        enterFullscreen(document.documentElement);
      },
    });
  }
  // Share (task 405, PRD 023 §4, §7.8).
  //
  // # A link, never the bytes
  //
  // `navigator.share` accepts `files`, and this deliberately never passes any. The distinction is the takedown
  // path: a shared **link** stops working when a photograph is taken down, and a shared **JPEG** does not. It is
  // also what keeps this a way to say "look at this" rather than a republishing tool. Somebody who wants the
  // file can save it from the photograph; we are simply not the ones handing out copies that outlive a
  // takedown.
  //
  // # The title is the album's, never the caption
  //
  // From `data-share-title`. Not fussiness: a caption is free text a curator typed and the one place a person's
  // name could plausibly end up, while a share sheet's title is the string that gets quoted into a group chat.
  //
  // # Why the URL is built here rather than read off the address bar
  //
  // Because the address bar is only right when `data-viewer-history` is set, and a host page that does not
  // reflect the current photograph would share whatever page happens to be showing. Building it from the
  // ordinal means the link names the photograph on screen either way.
  //
  // Query **and** fragment, for the reason task 401 recorded: the query is what lets the server render the page
  // holding the item, the fragment is what scrolls the recipient to the tile, and neither can do the other's
  // job.
  function shareURL(ctx) {
    var url = new URL(window.location.href);
    var param = ctx.config.history || 'foto';
    if (ctx.item.ordinal) {
      url.searchParams.set(param, ctx.item.ordinal);
      url.hash = param + '-' + ctx.item.ordinal;
    }
    // The window this page was cut to is not part of what is being shared: the server derives it from the
    // ordinal, and a stale one in a link somebody keeps would be a page number that no longer means anything.
    url.searchParams.delete('side');
    return url.toString();
  }

  // A short confirmation in the info panel, which clears itself.
  //
  // In the panel rather than as an alert, because an alert is a second modal over a modal and needs dismissing;
  // this is a receipt, not a question.
  function flash(message) {
    if (!ui) return;
    var note = document.createElement('span');
    note.className = 'hv-credit';
    note.textContent = message;
    ui.info.appendChild(note);
    window.setTimeout(function () {
      if (note.parentNode) note.parentNode.removeChild(note);
    }, 2500);
  }

  var canShare = !!(navigator.share && typeof navigator.share === 'function');
  var canCopy = !!(navigator.clipboard && typeof navigator.clipboard.writeText === 'function');

  if (canShare || canCopy) {
    window.hejViewer.register('share', {
      icon: 'share',
      // "Del" rather than "Kopier link" even when copying is what will happen: the visitor's intent is the same
      // and the label should name the intent, not the mechanism. The confirmation says which happened.
      label: 'Del dette billede',
      activate: function (ctx) {
        var url = shareURL(ctx);
        if (canShare) {
          // No `files`. See the comment above; this is the line that keeps the promise.
          var shared = navigator.share({ title: ctx.config.shareTitle, url: url });
          if (shared && typeof shared.catch === 'function') {
            shared.catch(function () {
              // A cancelled share sheet rejects, and cancelling is not a failure — it is somebody changing their
              // mind. Nothing to report.
            });
          }
          return;
        }
        navigator.clipboard.writeText(url).then(
          function () { flash('Linket er kopieret.'); },
          function () { flash('Kunne ikke kopiere linket.'); }
        );
      },
    });
  }

  // **Last, and that is the fix for a real bug** (task 415).
  //
  // `start()` binds the containers and — crucially — opens the viewer immediately when the address carries
  // `?foto=`. It used to run halfway up this file, before the controls below were registered, so a cold load of a
  // shared link built its action row from an **empty registry**: a viewer with nothing but a close button, no
  // fullscreen and no share. Reported from Brave on macOS, and reproducible every time by reloading the page the
  // viewer had just put `?foto=` on.
  //
  // The ordering is the whole guarantee, so it is asserted by test rather than left to whoever adds the next
  // control to notice.
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
})();
