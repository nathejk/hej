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
    // Lucide: cloud-download, copied from the icons repo so the path data is the published one rather than
    // something redrawn from memory.
    //
    // The maintainer's choice, and a better one than the `download` tray glyph this first used. That one was picked
    // for pairing — same box as `share` above, arrow reversed — which is precisely the problem: the two then differ
    // only in the direction of a small arrow, sitting side by side, on a phone, at arm's length. Confusing them
    // means sharing a photograph of somebody's child when you meant to keep it, which is not a symmetrical mistake.
    //
    // A cloud has a different **silhouette**, so the pair is told apart at a glance rather than on inspection. It
    // also says where the bytes come from, which is what this action does: pull the photograph off the server onto
    // the device.
    'cloud-download':
      '<path d="M12 13v8l-4-4"/><path d="m12 21 4-4"/>' +
      '<path d="M4.393 15.269A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.436 8.284"/>',
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

    // The frame holds everything, and exists for one reason: **it is the element that goes fullscreen**
    // (task 419).
    //
    // The dialog itself cannot be — Chromium refuses a fullscreen request for an element already in the top
    // layer, which `showModal` puts it in. The stage alone was tried and left the filmstrip out of the fullscreen
    // view, because the strip is its sibling. So the fullscreen element has to be an ordinary div that contains
    // the whole viewer, which is this.
    var frame = document.createElement('div');
    frame.className = 'hv-frame';

    var stage = document.createElement('div');
    stage.className = 'hv-stage';

    var prev = button('hv-nav hv-prev', 'prev', 'Forrige billede');
    var next = button('hv-nav hv-next', 'next', 'Næste billede');

    var img = document.createElement('img');
    img.className = 'hv-img';
    // Decorative here: the caption is in the panel below, and an alt repeating it would have a screen reader
    // read the same sentence twice.
    img.alt = '';

    // The player for a library video (PRD 029, task 499). One element for the whole album, like the <img>: it is
    // pointed at each video as the visitor reaches it and emptied when they leave, so only the item on screen ever
    // plays or downloads. Native controls, because they are the seek bar, the time and the fullscreen every
    // visitor already knows, and they are keyboard- and screen-reader-operable without our help.
    var video = document.createElement('video');
    video.className = 'hv-video';
    video.controls = true;
    video.playsInline = true;
    // iOS reads the attribute, not the property, in older versions; without it a video opens fullscreen.
    video.setAttribute('playsinline', '');
    video.preload = 'metadata';

    // Shown while a video plays muted — every video reached by swiping, and any the browser would only autoplay
    // silently. The native mute control does the same thing; this one is large, labelled and impossible to miss.
    var unmute = document.createElement('button');
    unmute.type = 'button';
    unmute.className = 'hv-unmute';
    unmute.textContent = 'Slå lyd til';
    unmute.hidden = true;

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
    stage.appendChild(video);
    stage.appendChild(unmute);
    stage.appendChild(missing);
    stage.appendChild(next);
    // **The info panel lives over the photograph, not under it** (task 416).
    //
    // It used to be a row of the dialog's grid, which meant a caption changed the height of the stage — so the
    // arrows moved between one photograph and the next depending on whether it had a caption. Inside the stage and
    // positioned over it, the geometry is the same for every photograph.
    stage.appendChild(info);
    frame.appendChild(stage);
    frame.appendChild(strip);
    dialog.appendChild(frame);
    document.body.appendChild(dialog);

    ui = {
      dialog: dialog,
      frame: frame,
      stage: stage,
      img: img,
      video: video,
      unmute: unmute,
      // Whether the next video shown may play with sound because a click or tap opened the viewer on it, and
      // whether the visitor has chosen sound for the rest of this viewing (PRD 029 §6).
      gesture: false,
      soundOn: false,
      // Set while *we* change `muted`, so the volumechange that follows is not mistaken for the visitor's choice.
      settingMute: false,
      bar: bar,
      info: info,
      strip: strip,
      // Watches the strip's thumbnails so only the ones near it are fetched (task 432). Rebuilt per album.
      stripObserver: null,
      prev: prev,
      next: next,
      // The open album: its items, where we are in it, and what the host page asked for.
      items: [],
      // URLs already handed to a prefetch, so a sliding window asks for each photograph once (task 434).
      // Reset per album, where items are.
      prefetched: {},
      index: 0,
      opener: null,
      config: {},
      pushed: false,
      reflected: false,
    };

    prev.addEventListener('click', function () { move(-1); });
    next.addEventListener('click', function () { move(1); });

    img.addEventListener('load', function () {
      dialog.classList.remove('is-loading');
      dialog.classList.remove('is-missing');
      // Prefetch from here rather than from `show`, because this is the first moment `currentSrc` reports which
      // rendition this viewport actually resolved to (task 410). It also keeps the prefetches from competing
      // with the photograph the visitor is waiting for.
      prefetchAround(ui.index);
    });
    img.addEventListener('error', function () {
      // Not an error state to recover from: a photograph can be taken down between the page loading and a
      // visitor reaching it. Say so, and let the arrows keep working.
      dialog.classList.remove('is-loading');
      dialog.classList.add('is-missing');
    });

    video.addEventListener('loadeddata', function () {
      dialog.classList.remove('is-loading');
      dialog.classList.remove('is-missing');
      prefetchAround(ui.index);
    });
    video.addEventListener('error', function () {
      // Only while a video is what is on screen: emptying the element on the way out raises an error too.
      if (!dialog.classList.contains('is-video') || !video.getAttribute('src')) return;
      dialog.classList.remove('is-loading');
      dialog.classList.add('is-missing');
    });
    video.addEventListener('volumechange', function () {
      if (ui.settingMute) {
        ui.settingMute = false;
        return;
      }
      // The visitor used the native control. Their choice holds for every video after this one.
      ui.soundOn = !video.muted;
      paintUnmute();
    });
    unmute.addEventListener('click', function () {
      setMuted(false);
      ui.soundOn = true;
      paintUnmute();
      // The button hides itself, and a hidden element cannot keep the focus: it would fall to <body>, outside the
      // dialog, and every key after it — Space, the arrows, Esc — would stop reaching the viewer. Close is where
      // the focus starts on open, so it goes back there.
      var close = ui.bar.querySelector('.hv-close');
      if (close) close.focus();
    });

    dialog.addEventListener('keydown', onKeydown);
    dialog.addEventListener('close', onClose);
    // The backdrop is part of the dialog's box, so a click that lands on the dialog or on the frame rather than
    // on any of their children is a click outside the photograph.
    dialog.addEventListener('click', function (event) {
      if (event.target === dialog || event.target === frame) dialog.close();
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
        // The photographer's own filename (task 475). Set by the curator's tool and by nothing else: the public
        // album page does not emit it, so on that surface this is "" and the panel shows no such line. The same
        // shape as `deleted` below — a field the shared component can carry and only one host provides.
        filename: node.getAttribute('data-filename') || '',
        ordinal: node.getAttribute('data-viewer-ordinal') || '',
        // The photograph's durable address, when the host page has one (task 447). The viewer knows nothing about
        // albums or refs — it is handed a URL and prefers it when sharing, because the page that rendered it is
        // the only thing that knows what a durable address looks like on that surface.
        permalink: node.getAttribute('data-viewer-permalink') || '',
        id: node.getAttribute('data-viewer-id') || '',
        deleted: node.getAttribute('data-viewer-deleted') === 'true',
        // A video (PRD 029): `full` is then its 720p MP4, `sd` its 480p one or "", and thumb/medium its poster.
        kind: node.getAttribute('data-kind') === 'video' ? 'video' : 'photo',
        sd: node.getAttribute('data-sd') || '',
        durationMs: parseInt(node.getAttribute('data-duration-ms') || '0', 10) || 0,
      });
    }
    return out;
  }

  // creditLine renders a credit as the public sentence: always "Foto: " and then the name.
  //
  // # Why the prefix is added here rather than stored
  //
  // The maintainer's instruction (task 473) is that the single-image view always shows the credit prefixed. The
  // stored value cannot be relied on to carry it: a credit picked from the crew roster resolves to a **bare name**
  // ("Anne Sørensen"), while a typed one is whatever a curator wrote — and they have been writing "Foto: Anne
  // Sørensen" since task 393, because until now the stored string was the whole rendered line.
  //
  // So both forms exist in the data, and adding the prefix unconditionally would render "Foto: Foto: Anne Sørensen"
  // for every credit typed before today. An existing prefix is therefore stripped before one is added, which makes
  // this idempotent and makes the two kinds render identically — which is the point of the instruction.
  //
  // Case-insensitive and tolerant of the missing space, because the variants are what people type. It does **not**
  // touch anything else: a credit reading "Billede: X" or "Photo: X" keeps its own words and gets the prefix, which
  // is a little redundant and much better than this function guessing at prose.
  var FOTO_PREFIX = /^\s*foto\s*:\s*/i;
  function creditLine(credit) {
    return 'Foto: ' + String(credit).replace(FOTO_PREFIX, '').trim();
  }

  // The one place a plain image URL is chosen: the display image, or the thumbnail if a photograph somehow has
  // no display rendition. This is what a browser that ignores `srcset` ends up with, and what the filmstrip
  // falls back to.
  //
  // **For a video it is the poster, never `full`.** `full` is then an MP4 of up to a few hundred megabytes, and this
  // function feeds the prefetch and the filmstrip, which would otherwise download clips nobody opened.
  function pictureFor(item) {
    if (item.kind === 'video') return item.medium || item.thumb;
    return item.full || item.thumb;
  }

  // The `sizes` value is a deliberate lie about layout, and it must stay one (task 410, PRD 023 §7.9).
  //
  // A phone's slot really is ~100vw. Declaring that truthfully on a 3x display asks the browser for ~1170px of
  // image, which selects the 1600w candidate — re-introducing the exact cost this change exists to remove.
  // Declaring 400px instead caps a narrow viewport at the 800px rendition, which is still a genuine 2x image on
  // a 390pt screen. Only a 3x flagship gives anything up, and 800px over 390pt is not perceptible.
  //
  // Under-declaring `sizes` is a known technique rather than a slip: the number is intentionally *not* `100vw`.
  // "Fixing" it to `100vw` silently undoes the whole feature — the photographs still look right, so nothing
  // tells you the phone went back to downloading twice the bytes.
  var SIZES = '(max-width: 40rem) 400px, 100vw';

  // Point the overlay's image at a photograph, offering both renditions with their real widths.
  //
  // The widths are the renditions' longest-edge bounds (task 409): 800 and 1600. `srcset`/`sizes` rather than a
  // JS branch, for the two reasons §7.9 gives. Device pixel ratio is half the decision and the browser already
  // knows it, so a 390pt phone at 3x and an iPad at 2x get different answers without us writing that arithmetic
  // down. And rotating or resizing re-runs the choice for free, where a branch taken once at open is wrong the
  // moment a phone turns sideways — which matters because this viewer is for desktop *and* mobile (§2).
  //
  // With no medium rendition — every photograph uploaded before task 409 — we emit no `srcset` at all, rather
  // than a single candidate plus a `sizes` hint that would then be a lie with nothing to gain by it. The display
  // image is what such a photograph has: same rule and same reason as the missing-thumbnail fallback, because a
  // rendition is an optimisation and losing one costs bandwidth rather than the photograph.
  //
  // `src` is set last and always: the candidate list has to be in place before the browser makes its selection,
  // and `src` stays the answer for anything that ignores `srcset`.
  function applyPicture(img, item) {
    if (item.medium && item.full) {
      img.srcset = item.medium + ' 800w, ' + item.full + ' 1600w';
      img.sizes = SIZES;
    } else {
      img.removeAttribute('srcset');
      img.removeAttribute('sizes');
    }
    img.src = pictureFor(item);
  }

  function show(index) {
    var state = ui;
    if (index < 0 || index >= state.items.length) return;
    state.index = index;

    var item = state.items[index];
    state.dialog.classList.add('is-loading');
    state.dialog.classList.remove('is-missing');
    // Whatever was playing stops before anything else is shown: only the item on screen plays (PRD 029 §6).
    stopVideo();
    var isVideo = item.kind === 'video';
    state.dialog.classList.toggle('is-video', isVideo);
    if (isVideo) {
      state.img.removeAttribute('srcset');
      state.img.removeAttribute('src');
      playVideo(item);
    } else {
      applyPicture(state.img, item);
    }
    var missing = state.stage.querySelector('.hv-missing');
    if (missing) missing.textContent = isVideo ? 'Videoen er ikke tilgængelig lige nu.' : 'Billedet er ikke tilgængeligt.';

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
      credit.textContent = creditLine(item.credit);
      state.info.appendChild(credit);
    }
    // **Below the credit**, as asked for (task 475), and only where the host page provides one — which is the
    // curator's tool. Rendered as the filename it is, with no prefix and no attempt to make it a sentence: a
    // photographer recognises their own filing, and dressing it up would be the tool interpreting it.
    if (item.filename) {
      var filename = document.createElement('span');
      filename.className = 'hv-filename';
      filename.textContent = item.filename;
      state.info.appendChild(filename);
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

  // --- video (PRD 029, task 499) ---------------------------------------------

  // Clips this short loop, like a live photo; longer ones stop on their last frame (PRD 029 §11 Q4).
  var LOOP_MS = 15000;
  // How far Shift+arrow and J/L seek.
  var SEEK_S = 10;

  function setMuted(muted) {
    var v = ui.video;
    if (v.muted === muted) return;
    ui.settingMute = true;
    v.muted = muted;
  }

  function paintUnmute() {
    ui.unmute.hidden = !ui.dialog.classList.contains('is-video') || !ui.video.muted;
  }

  // playVideo points the player at a video and starts it.
  //
  // # The sound rule
  //
  // **With sound when a click or tap opened the viewer on this video**, because that click is the user gesture
  // browsers require before they allow sound, and a visitor who chose a video expects to hear it. **Muted when the
  // visitor swiped or arrowed onto it**: a swipe may not count as that gesture, and a clip blaring out mid-browse is
  // the thing that makes people close a page. Once they unmute, sound stays on for the rest of this viewing.
  //
  // If the browser refuses sound anyway it is retried muted, and if it refuses even that (iOS Low Power Mode, data
  // saver) the native controls show the poster with a play button — which is the honest state, not a failure.
  function playVideo(item) {
    var v = ui.video;
    var withSound = ui.soundOn || ui.gesture;
    // The gesture belongs to the item it opened, not to whatever comes next.
    ui.gesture = false;

    v.poster = item.medium || item.thumb || '';
    v.loop = item.durationMs > 0 && item.durationMs <= LOOP_MS;
    setMuted(!withSound);
    v.src = item.full;
    var started = v.play();
    if (started && typeof started.catch === 'function') {
      started.catch(function () {
        if (!v.muted) {
          setMuted(true);
          paintUnmute();
          var again = v.play();
          if (again && typeof again.catch === 'function') again.catch(function () {});
        }
      });
    }
    paintUnmute();
  }

  // stopVideo stops the player and lets go of the file, so a clip left behind does not keep downloading.
  function stopVideo() {
    var v = ui.video;
    if (!v.getAttribute('src')) return;
    v.pause();
    v.removeAttribute('src');
    v.removeAttribute('poster');
    // `load()` with no source is what actually aborts the network request; removing the attribute alone does not.
    v.load();
    ui.unmute.hidden = true;
  }

  // onVideoKey handles the player's keys, and reports whether it did.
  //
  // Space plays and pauses; M mutes. **Seeking is Shift+←/→ (and J/L), not the bare arrows**: the bare arrows
  // already move between photographs, and a visitor arrowing through an album would otherwise be trapped inside
  // the first video. Nothing here fires while the focus is in a text field — the curator's caption editor sits in
  // this dialog.
  function onVideoKey(event) {
    var item = ui.items[ui.index];
    if (!item || item.kind !== 'video') return false;
    var t = event.target;
    if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.isContentEditable)) {
      return false;
    }
    var v = ui.video;
    var key = event.key;
    if (key === ' ' || key === 'Spacebar') {
      if (v.paused) {
        var p = v.play();
        if (p && typeof p.catch === 'function') p.catch(function () {});
      } else {
        v.pause();
      }
      return true;
    }
    if (key === 'm' || key === 'M') {
      setMuted(!v.muted);
      ui.soundOn = !v.muted;
      paintUnmute();
      return true;
    }
    var back = key === 'j' || key === 'J' || (event.shiftKey && key === 'ArrowLeft');
    var fwd = key === 'l' || key === 'L' || (event.shiftKey && key === 'ArrowRight');
    if (back || fwd) {
      var to = (v.currentTime || 0) + (fwd ? SEEK_S : -SEEK_S);
      if (isFinite(v.duration)) to = Math.min(to, v.duration);
      v.currentTime = Math.max(0, to);
      return true;
    }
    return false;
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

  // The filmstrip's thumbnails are loaded through an observer, not by `loading="lazy"` (task 432).
  //
  // `lazy` is not broken here, it is simply the wrong instrument at this density. Its scroll-distance
  // threshold is thousands of pixels — tuned for a page of full-width images — while a strip button is about
  // 62px wide, so "just off screen" covers something like fifty thumbnails. Opening a 45-photograph album
  // therefore fired ~45 requests at once, most for thumbnails the grid had never fetched, and they competed
  // with the photograph the visitor was actually waiting for.
  //
  // That is the thing PRD 023 §6 exists to prevent — it budgets **two ahead and one back** — so the strip
  // quietly spending two orders of magnitude more than the prefetch is not a tuning detail.
  //
  // `root: strip` is the load-bearing part: the strip is the scroll container, so intersection has to be
  // measured against it rather than the viewport. A generous horizontal `rootMargin` keeps scrolling smooth
  // without going back to a whole-album fetch.
  var STRIP_ROOT_MARGIN = '0px 300px';

  function observeStripThumb(img) {
    // No observer (or none possible) means load it now: a visible thumbnail beats a clever one, and this is a
    // bandwidth optimisation rather than a correctness property.
    if (!ui.stripObserver) {
      hydrateStripThumb(img);
      return;
    }
    ui.stripObserver.observe(img);
  }

  function hydrateStripThumb(img) {
    var src = img.getAttribute('data-src');
    if (!src) return;
    img.removeAttribute('data-src');
    img.src = src;
  }

  function fillStrip() {
    var strip = ui.strip;

    // Torn down and rebuilt with the strip, or the observer would hold every <img> of every album the viewer
    // has ever opened — the admin tool reopens this overlay repeatedly against a changing sheet.
    if (ui.stripObserver) {
      ui.stripObserver.disconnect();
      ui.stripObserver = null;
    }
    if (typeof IntersectionObserver === 'function') {
      ui.stripObserver = new IntersectionObserver(function (entries) {
        for (var i = 0; i < entries.length; i++) {
          if (!entries[i].isIntersecting) continue;
          hydrateStripThumb(entries[i].target);
          // Once loaded there is nothing left to watch for, and an observer that keeps every thumbnail of a
          // 200-photograph album under observation is its own small leak.
          ui.stripObserver.unobserve(entries[i].target);
        }
      }, { root: strip, rootMargin: STRIP_ROOT_MARGIN });
    }

    strip.innerHTML = '';
    for (var i = 0; i < ui.items.length; i++) {
      var item = ui.items[i];
      var thumb = document.createElement('button');
      thumb.type = 'button';
      thumb.setAttribute('aria-label', item.caption || 'Billede ' + (i + 1));
      var img = document.createElement('img');
      // The URL goes on data-src and is promoted to src by the observer. Deliberately not both: an <img> with a
      // src is already a request, whatever an observer decides afterwards.
      img.setAttribute('data-src', item.thumb || pictureFor(item));
      img.alt = '';
      img.decoding = 'async';
      thumb.appendChild(img);
      thumb.addEventListener('click', jumpTo(i));
      strip.appendChild(thumb);
      observeStripThumb(img);
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
  //
  // **Of the variant this viewport resolved to**, which is why the neighbours are not simply `pictureFor`
  // (task 410). Prefetching 1600px images to a phone that will then display 800px ones is this task's own bug
  // arriving by the back door: the visible photograph gets cheaper and the invisible two ahead do not.
  //
  // The variant comes from `currentSrc` — what the browser actually picked for the photograph on screen — rather
  // than from a second, independent guess here. A guess would have to re-derive viewport width and DPR, and two
  // implementations of one decision drift apart exactly when a device is unusual. An empty `currentSrc` means
  // nothing has resolved yet, so we fall back to the display image: the same answer a browser without `srcset`
  // gets, wrong only in costing bandwidth.
  function prefetchAround(index) {
    var current = ui.items[index];
    var resolved = ui.img.currentSrc || '';
    // A suffix match, because `currentSrc` is absolute and the item's URLs come off the page as written. Nothing
    // about the variant's spelling is known here — the shared viewer holds no endpoint (PRD 023 §7.7).
    var medium = !!(current && current.medium) && resolved.indexOf(current.medium) >= 0;

    var wanted = [index + 1, index + 2, index - 1];
    for (var i = 0; i < wanted.length; i++) {
      var at = wanted[i];
      if (at < 0 || at >= ui.items.length) continue;
      var item = ui.items[at];
      var url = (medium && item.medium) || pictureFor(item);
      if (!url) continue;
      // Asked for once per album, however many times the window slides over it (task 434).
      //
      // A ±2 window necessarily overlaps: walking 0→1→2→3 asks for photograph 2 at every step. Relying on
      // the HTTP cache to make the repeats free is *nearly* right — the responses are `immutable` with a
      // year's `max-age`, so a browser normally serves them without a request — but "nearly" is doing a lot
      // of work there. It is not true with the cache disabled, not true after an eviction, and on a phone
      // looking at 900 kB photographs an eviction is an ordinary event rather than a corner case. Keeping a
      // set of what we have already asked for costs one string per photograph and removes the assumption.
      if (ui.prefetched[url]) continue;
      ui.prefetched[url] = true;

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
  //
  // **`pushed` and `reflected` are two different facts** (task 421), and conflating them was a bug:
  //
  //   - `reflected` means the address already names the current photograph, so the next move should replace
  //     rather than push;
  //   - `pushed` means *we* added a history entry, so closing may wind it back.
  //
  // A viewer opened from a `?foto=` link is reflected but not pushed — the entry it is sitting on belongs to
  // whoever sent the link. Treating that as pushed made closing press back, which either went nowhere (a fresh
  // tab has nothing behind it, so the address kept `?foto=` and a reload reopened the photograph) or left the
  // album entirely.
  function reflectURL(item) {
    var param = ui.config.history;
    if (!param || !item.ordinal || !window.history) return;

    var url = new URL(window.location.href);
    url.searchParams.set(param, item.ordinal);
    url.hash = param + '-' + item.ordinal;

    if (ui.reflected) {
      window.history.replaceState({ hv: true }, '', url.toString());
      return;
    }
    window.history.pushState({ hv: true }, '', url.toString());
    ui.reflected = true;
    ui.pushed = true;
  }

  // Take the photograph back out of the address (task 421).
  //
  // Used when closing a viewer whose entry we did not push — one opened from a `?foto=` link. `replaceState`
  // rather than `back()`, because there may be nothing behind it to go back to and because whatever *is* behind
  // it belongs to the sender of the link rather than to this album.
  //
  // The fragment goes with the parameter. It would be harmless to leave — a hash scrolls, it does not open
  // anything — but half a cleaned address is the kind of thing somebody reports as a bug a second time.
  function unreflectURL() {
    var param = ui.config.history;
    if (!param || !window.history) return;

    var url = new URL(window.location.href);
    if (!url.searchParams.has(param) && !url.hash) return;
    url.searchParams.delete(param);
    url.hash = '';
    window.history.replaceState({}, '', url.toString());
  }

  function onKeydown(event) {
    if (onVideoKey(event)) {
      // Space would otherwise press whichever button has the focus — on open, that is Close.
      event.preventDefault();
      return;
    }
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
        // A drag along the bottom of a video is the seek bar, not a swipe (PRD 029 §7): changing item mid-scrub
        // would throw the visitor out of the clip they were trying to find a moment in.
        if (event.target === ui.video) {
          var rect = ui.video.getBoundingClientRect();
          if (event.clientY > rect.bottom - 64) return;
        }
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
      // The frame rather than the whole overlay, for the one control that needs an element to hand to a platform
      // API. See the fullscreen action: the dialog itself cannot be the one (tasks 418, 419).
      frame: ui.frame,
      config: ui.config,
      refresh: function () { show(ui.index); },
    };
  }

  function closeButton() {
    var el = button('hv-close', 'close', 'Luk');
    el.addEventListener('click', function () { ui.dialog.close(); });
    return el;
  }

  // `byGesture` says a click or tap opened it, which is what allows a video to start with sound (see playVideo).
  // The admin tool's "Vis stort" is a click too, so the public API treats a missing argument as true.
  function open(container, index, byGesture) {
    build();
    ui.gesture = byGesture !== false;
    ui.soundOn = false;

    ui.items = itemsIn(container);
    if (!ui.items.length) return;
    if (index < 0 || index >= ui.items.length) index = 0;

    // Cleared with the items, not kept across opens. The admin tool reopens this overlay against a sheet that
    // may have changed underneath it, and a stale entry would suppress a prefetch for a photograph that is no
    // longer the one we fetched.
    ui.prefetched = {};

    ui.config = {
      history: container.getAttribute('data-viewer-history') || '',
      shareTitle: container.getAttribute('data-share-title') || document.title,
      captionEndpoint: container.getAttribute('data-caption-endpoint') || '',
      year: container.getAttribute('data-year') || '',
    };
    ui.pushed = false;
    ui.reflected = false;
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
    stopVideo();
    ui.dialog.classList.remove('is-video');

    // The strip's observer goes with the strip's contents. Left connected it would keep every thumbnail <img>
    // of the album alive for as long as the page lives, which the admin tool would accumulate one album at a
    // time.
    if (ui.stripObserver) {
      ui.stripObserver.disconnect();
      ui.stripObserver = null;
    }

    // Leaving fullscreen with the viewer, because the thing that was fullscreen is the thing being closed.
    // Without this, closing while fullscreen leaves the browser filling the screen with the album page behind —
    // no chrome, no viewer, and no obvious way back.
    if (fullscreenElement()) exitFullscreen();

    // Announced for the same reason as hv:show: a host page may have work it deliberately deferred until the
    // overlay was out of the way — the admin tool reloads its sheet here rather than swapping 120 thumbnails out
    // from under somebody who is still looking at one.
    ui.dialog.dispatchEvent(new CustomEvent('hv:close'));

    // Wind the address back to the album, one of two ways (task 421).
    //
    // If we pushed an entry, going back both closes that entry and restores the address in one move — and leaves
    // no leftover entry that looks like a place you can return to. If we did not, there is nothing of ours to go
    // back through, so the parameter is taken out of the address in place. The distinction is the whole of the
    // bug this replaced: closing used to press back either way, which in a fresh tab went nowhere and left
    // `?foto=` in the address, so reloading reopened the photograph.
    if (ui.pushed && window.history) {
      ui.pushed = false;
      window.history.back();
    } else {
      unreflectURL();
    }
    ui.reflected = false;
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
      open(container, index, true);
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
          // A link someone sent is not a gesture on this page: a video opened from one starts muted.
          open(container, j, false);
          // Opened from the address rather than from a click. The address already names this photograph, so
          // moving replaces rather than pushes — but **nothing of ours is in the history**, so closing must take
          // the parameter out in place rather than pressing back. Those are two different flags for a reason;
          // see reflectURL.
          ui.reflected = true;
          ui.pushed = false;
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
    // How a credit is rendered publicly (task 473). Exposed because the admin tool's credit editor shows a preview
    // of exactly this line, and a preview rendered by a second copy of the rule is a preview that will eventually
    // disagree with the page it is previewing.
    creditLine: creditLine,
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
        // **The frame — not the dialog, not the page, and not the stage alone** (task 419).
        //
        // Four targets have been tried on a real browser and three of them are wrong. All four are recorded
        // because each is the obvious next guess for somebody who has just met one of the others:
        //
        //   - **The dialog is refused.** It is in the top layer, put there by `showModal`, and Chromium will not
        //     fullscreen an element that is already there — the promise rejects (Brave, macOS, task 417).
        //   - **The document element renders wrong.** It is accepted, and joins the top layer *after* the dialog,
        //     so the album page paints over the photograph and the info panel falls behind it (task 416).
        //   - **The stage loses the filmstrip**, which is its sibling rather than its child, so a fullscreen view
        //     had no way to see where you were in the album (task 418).
        //   - **The frame works**: an ordinary div, not in the top layer itself, containing the whole viewer.
        enterFullscreen(ctx.frame);
      },
    });
  }
  // Share (task 405, PRD 023 §4, §7.8).
  //
  // # A link, never the bytes
  //
  // `navigator.share` accepts `files`, and this deliberately never passes any. The distinction is the takedown
  // path: a shared **link** stops working when a photograph is taken down, and a shared **JPEG** does not. It is
  // also what keeps this a way to say "look at this" rather than a republishing tool. Somebody who wants the file
  // can save it from the photograph; we are simply not the ones putting a copy into a group chat.
  //
  // **Task 488 added a save button beside this one, and it does not weaken the rule above** — see its comment. The
  // short version: this control is about what *we* hand to a third party on somebody's behalf, and that still never
  // includes bytes. Saving is the visitor acting on the photograph in front of them, which long-press and
  // right-click have always allowed. A takedown governs what we serve, and that is unchanged by either.
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
    // A permalink when the page supplied one, because that is the address that still means this photograph after
    // the album has been re-sorted (task 447). The ordinal form below is this page's own state — right now, and
    // not necessarily next week — so it is the fallback rather than the answer.
    if (ctx.item.permalink) {
      return new URL(ctx.item.permalink, window.location.href).toString();
    }

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

  // Save this photograph (task 488).
  //
  // # What it hands over, and what it deliberately does not
  //
  // **The display image** — `item.full`, the same bytes already on screen. That is the "fair size" the maintainer
  // asked for: a 1600px JPEG, a few hundred kilobytes, enough for a print at postcard size or an attachment, and
  // already what the page loaded, so on a cache hit this costs no transfer at all.
  //
  // Not the photographer's file. The viewer has no way to ask for one and must not gain one: that copy keeps the
  // camera's metadata, including where the photograph was taken, and it is reachable only behind the curator's
  // credential (PRD 027). This file is one copy shared by the public album page and the admin tool, so a variant
  // added here for a curator's convenience would ship to the open web the same afternoon — which is why there is a
  // test that fails if this file so much as names one.
  //
  // # Why this does not contradict the share button above
  //
  // Share says, at length, that it never passes `files` because a shared **link** stops working after a takedown and
  // a shared **JPEG** does not — "we are simply not the ones handing out copies that outlive a takedown". That is
  // still true of *sharing*, and it is the reason this is a separate control rather than files added to the share
  // sheet.
  //
  // What changed is narrower than it looks. The same comment already conceded the capability: "somebody who wants the
  // file can save it from the photograph". Long-press on a phone and right-click on a laptop have always produced
  // exactly these bytes, so this button removes friction from something a visitor could already do, deliberately,
  // while looking at the photograph. It does not hand out copies on somebody's behalf into a group chat, and the
  // bytes it saves are the stripped rendition rather than the file the camera wrote.
  //
  // The honest summary: a takedown still cannot recall a copy somebody saved, and it never could. What a takedown
  // governs is what we **serve**, and that is unchanged.
  //
  // # An anchor rather than fetch-into-a-blob
  //
  // `<a download>` lets the browser do the work: no second copy of the bytes in JavaScript memory, and the request is
  // the same same-origin URL the `<img>` already has, so a cached response is reused and the media route's
  // `immutable` year applies. Fetching into a blob would re-download a few hundred kilobytes on a phone to achieve
  // the same file, and would need the response to be readable — a needless requirement on an `<img>` that already
  // displayed.
  //
  // The `download` attribute is honoured because this is same-origin; cross-origin it would be ignored and the
  // browser would navigate instead. Nothing here is cross-origin, and if that ever changes this control breaks
  // loudly (a navigation away from the album) rather than quietly.
  function downloadName(ctx) {
    // Derived from the permalink when the page supplied one, because that is the only string here that knows what
    // album this is: `/2026/album/natten/f/766ec78fe2c3` gives "natten". The viewer itself knows nothing about
    // albums (PRD 023 §7.7) and must not start.
    var base = '';
    if (ctx.item.permalink) {
      var parts = ctx.item.permalink.split('/');
      // The segment after "album", when the address has that shape. Read by name rather than by index so a future
      // change to the path's depth cannot silently pick the year.
      for (var i = 0; i < parts.length - 1; i++) {
        if (parts[i] === 'album') {
          base = parts[i + 1];
          break;
        }
      }
    }
    if (!base) {
      // No permalink: the album title, reduced to something a filesystem will not argue with. Lowercased ASCII
      // letters and digits, everything else a hyphen — deliberately cruder than the server's slug rules, because a
      // filename does not have to round-trip and a second slug implementation that nearly matches is worse than an
      // obviously different one.
      base = (ctx.config.shareTitle || 'foto')
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '');
    }
    if (!base) base = 'foto';

    // The ordinal, which is **a label on a copy and not an address**. It moves when the album is re-sorted, and that
    // is fine here in a way it was not fine for the permalink (task 447): nobody resolves a filename. It is also the
    // number the visitor can see on the page, so two saved files are told apart the way they were seen.
    var n = ctx.item.ordinal;
    // A video saves as the MP4 it is (PRD 029).
    var ext = ctx.item.kind === 'video' ? '.mp4' : '.jpg';
    return n ? base + '-' + n + ext : base + ext;
  }

  // Registered unconditionally, unlike share: there is no capability to feature-detect. An `<a download>` works
  // everywhere this viewer runs, and a browser that ignored the attribute would open the photograph rather than
  // save it — a worse outcome than a download, and still not a broken one.
  //
  // Which pages *get* the control is the host page's decision, as it is for every action here: the public album page
  // declares it, the curator's tool does not, because that tool has the whole album as a zip and the photographer's
  // file besides.
  window.hejViewer.register('download', {
    icon: 'cloud-download',
    // Names the thing rather than the mechanism, like "Del" above: a visitor wants the photograph, not a transfer.
    label: 'Hent dette billede',
    activate: function (ctx) {
      if (!ctx.item.full) return;

      var a = document.createElement('a');
      a.href = ctx.item.full;
      a.download = downloadName(ctx);
      // Appended before clicking and removed after: Firefox ignores a click on an anchor that is not in the
      // document. `display:none` would not help — it has to be connected, not visible.
      a.style.display = 'none';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    },
  });

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
