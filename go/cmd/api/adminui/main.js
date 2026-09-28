// The admin tool's wiring (task 395, step 7).
//
// # Why this file exists
//
// page.js was 1,436 lines in one closure, and the eight features inside it shared state by simply being in the
// same scope. Task 394 made the tool's markup, CSS and JavaScript real files and deferred the split of the
// JavaScript itself to task 395; this is that split, and this file is what replaces the scope they shared.
//
// **`ctx` is the whole of the shared surface, and that is the point.** In one closure, any feature could reach any
// other's variables and nothing recorded which of them meant to. Written down, the surface turns out to be small:
// the selection, three functions for opening an overlay, a way to reload the grid, the line an action writes its
// outcome to, and two notifications. Anything a feature does not put here, no other feature can reach.
//
// # Why these are function declarations spliced into one script, and not modules
//
// ES modules would give the same split with real `import`s and no build step, and were rejected for one reason:
// every asset on this surface is served with `no-store` (task 371), because it sits behind the shared credential.
// Six module files would be six uncacheable requests on every page load, where the injection keeps the whole tool
// at one — and the person waiting is a photographer on a hotel connection the day after the event.
//
// So each file is a single `function init…(ctx)` declaration, which is a complete and valid JavaScript program on
// its own: a formatter, a linter and an editor can all read it, which was task 394's whole reason for making these
// real files. page.html splices them into one `<script>` inside one IIFE, so the declarations are scoped to it and
// nothing reaches `window`.
//
// # Why the order below is the order
//
// Function declarations hoist, so only the order of *invocation* matters. Two constraints:
//
//   1. **The shell first.** Every other feature opens an overlay, and none can before `ctx.openSheet` exists.
//   2. **The contact sheet before the actions**, because it publishes the selection they all act on — and before
//      the uploader, which reloads it when a batch finishes.
//
// The first page of thumbnails is fetched by neither: it is declared on `#sheet` in page.html as
// `hx-trigger="load"`. htmx is a deferred script and this one is inline, so `window.htmx` does not exist while any
// of this runs; a call here would silently do nothing and leave an empty grid. Declaring it in the markup puts the
// fetch on htmx's own initialisation, which is the one moment it is certain to be ready. Every *later* load goes
// through `load()`, which reads `window.htmx` lazily and by then finds it.
function initAdminTool() {
  // Built by assignment rather than as an object literal, so that **`ctx.x = ` is the only way anything is ever
  // provided.** That uniformity is what lets `TestEveryAdminContextMemberIsProvided` check the whole surface by
  // reading the files: a member used by a feature and provided by nobody is the one bug this split can introduce,
  // and JavaScript gives no compiler to catch it.
  const ctx = {};

  // fetch is `window.fetch` with the working year stamped on (task 392).
  //
  // Every JSON request the tool makes goes through this, because the API has no year in its path and refuses a
  // request that does not say which year it means — a default would be a silent wrong-year write. The year is
  // read from the header's badge, the one place the page states it. htmx requests get the same header from the
  // `hx-headers` on <body>. `TestTheAdminScriptsFetchOnlyThroughTheYear` holds that nothing calls fetch directly.
  const year = document.querySelector('.year').dataset.year;
  ctx.fetch = (url, opts) => {
    const o = Object.assign({}, opts);
    o.headers = Object.assign({}, o.headers, { 'X-Admin-Year': year });
    return window.fetch(url, o);
  };

  // openers maps a button's `data-act` to the function that opens its sheet. Each action registers its own, so
  // adding a sixth is one file rather than two.
  ctx.openers = {};

  // albumsChanged tells the page's album list that the set of albums, or their item counts, just changed.
  //
  // An event on `body` rather than a direct call, because the list is an htmx fragment listening for it (see
  // adminui/fragments.html) and nothing in JavaScript holds a reference to it. This is the one piece of coupling
  // between the script and a fragment, and it is deliberately one-way: the script announces, the fragment decides
  // whether to care.
  ctx.albumsChanged = () => {
    document.body.dispatchEvent(new Event('albums-changed'));
  };

  // photoCount renders "1 billede" or "12 billeder" (task 387).
  //
  // On the context rather than copied into each sheet, for the reason task 387 found the hard way: the same
  // ternary was written out at eight call sites here and six in Go, and the one place it was written *without* the
  // singular was the public frontpage, where it read "1 billeder" to every family that opened it. A grammatical
  // fact repeated fourteen times is a grammatical fact that will be got wrong.
  //
  // It cannot share the Go definition in `plural.go` — there is no build step on this surface and nothing compiles
  // these files together — so there are exactly two copies, which is the floor. `TestTheAdminCountsAgreeAcrossGo
  // AndJavaScript` holds them equal.
  ctx.photoCount = (n) => (n === 1 ? '1 billede' : n + ' billeder');

  // settled waits until the library can name every one of these ids, and says whether it got there (task 439).
  //
  // # Why every write in this tool needs this
  //
  // A write here **publishes an event**; every grid and sheet reads a projection a jetstream consumer folds from
  // that event. The two are milliseconds apart in a healthy system and not ordered at all in principle, so
  // reloading the instant a response lands is a read with nothing behind it. That is a real report, not a theory:
  // a curator was left with a list of uploaded photographs above a grid that did not have them (task 437).
  //
  // The condition being waited for is not a proxy for anything. Getting an id back from a **read** is the fold
  // having happened (task 438) — the id is in the write's response, so there is nothing to infer and nothing a
  // second curator working at the same time can satisfy on our behalf.
  //
  // # Why it lives here
  //
  // The uploader is the first caller and PRD 024 §8 brings the next; the eight actions that reload after a write
  // have the same race. Eight copies of a polling loop is the outcome this exists to prevent, so the loop is on
  // the context and the callers pass ids.
  //
  // Deliberately **not** given to a caller that has no ids to wait for: `settled([])` is honest about a batch that
  // stored nothing, but a caller that cannot name what it wrote needs its own answer to "which ids prove this
  // landed" — and for the removals that answer is an id *disappearing*, which is a different read.
  //
  // Answers a plain boolean, because both ways of failing — the deadline passed, or the read itself broke — leave
  // the caller with exactly one thing to do: refresh anyway and admit it may be behind. A caller that needs to
  // name the stragglers should get that added here rather than polling on its own.

  // How long the projection is given to catch up. Far longer than the fold takes, and short enough that a broken
  // consumer is reported rather than waited out.
  const SETTLE_MS = 10000;
  // Ids per presence request. The endpoint refuses more than 200 (`maxAdminLibraryIDs`), and a card is routinely
  // three hundred photographs, so asking is chunked. 100 keeps each answer prompt.
  const IDS_PER_ASK = 100;

  // seen asks which of these ids the library can see, and returns them as a Set.
  //
  // `ids=` is a presence read: the filter composes with the default "live only", so a photograph a curator
  // deleted correctly comes back absent. Returns null if the read fails — an answer we could not get is not an
  // answer that says "not yet".
  const seen = async (ids) => {
    try {
      const url = '/api/admin/photos?limit=' + ids.length + '&ids=' + ids.map(encodeURIComponent).join(',');
      const res = await ctx.fetch(url);
      if (!res.ok) return null;
      const out = await res.json();
      if (!out || !out.photos) return null;
      return new Set(out.photos.map((p) => p.id));
    } catch (err) {
      return null;
    }
  };

  // Ids already accounted for are dropped from the next round, so a batch of three hundred converges instead of
  // re-asking after the ones that arrived first. Backs off, so a slow fold costs a few requests rather than a poll
  // per frame.
  ctx.settled = async (ids) => {
    const deadline = Date.now() + SETTLE_MS;
    let waiting = ids.slice();
    for (let delay = 150; ; delay = Math.min(delay * 2, 1000)) {
      const still = [];
      for (let i = 0; i < waiting.length; i += IDS_PER_ASK) {
        const chunk = waiting.slice(i, i + IDS_PER_ASK);
        const there = await seen(chunk);
        if (there === null) return false; // a failed read is not a reason to keep asking
        for (const id of chunk) if (!there.has(id)) still.push(id);
      }
      waiting = still;
      if (!waiting.length) return true;
      if (Date.now() + delay > deadline) return false;
      await new Promise((r) => setTimeout(r, delay));
    }
  };

  // settledOrder waits until the album read agrees with the order the server says it published.
  //
  // `ctx.settled` is the wrong tool for a reorder and it is worth saying why, because reaching for it is the
  // obvious mistake: it waits for photographs to **exist**, and after a re-sort they already did. Their presence
  // says nothing about their positions, so a grid reloaded on that signal can still show the old order — the
  // exact complaint tasks 437 and 438 fixed for the uploader, in a new place.
  //
  // So this waits for the *positions*. The album read returns an album's photographs in ordinal order, which is
  // the projection's own answer to "what order is this album in" — the same question the grid asks.
  //
  // Same bound and backoff as `ctx.settled`, and the same honesty on giving up: false means "reload anyway and
  // say it may be behind", never "keep waiting".
  // sortModeName renders a sort mode the way the curator chose it (PRD 024 §6 R1).
  //
  // On the context because **two** features say it: the editor card confirms what it saved, and the warning
  // before a hand move names what the album is currently doing. Those two sentences have to agree, and the one
  // that would drift is the warning — the place it matters most.
  //
  // The `<select>`'s own option text is the same five strings, and that duplication is deliberate rather than
  // shared: the markup is what a curator reads when choosing, this is what they read afterwards, and a template
  // cannot be called from JavaScript on a surface with no build step. `TestTheSortModeLabelsAgree` holds them
  // equal.
  ctx.sortModeName = (mode) => ({
    'manual': 'Manuel',
    'time-asc': 'Tid, ældste først',
    'time-desc': 'Tid, nyeste først',
    'filename-asc': 'Filnavn A–Å',
    'filename-desc': 'Filnavn Å–A',
  }[mode] || mode);

  ctx.settledOrder = async (albumId, order) => {
    if (!albumId || !order || !order.length) return true;
    const deadline = Date.now() + SETTLE_MS;
    // Capped at the endpoint's ceiling. A longer album is checked by its first page, which is where a reorder is
    // visible anyway — the alternative is paging the whole album on every poll to answer a question the first
    // screenful already answers.
    const want = order.slice(0, IDS_PER_ASK);
    for (let delay = 150; ; delay = Math.min(delay * 2, 1000)) {
      let got = null;
      try {
        const res = await ctx.fetch('/api/admin/photos?limit=' + want.length +
          '&album=' + encodeURIComponent(albumId));
        if (res.ok) {
          const out = await res.json();
          if (out && out.photos) got = out.photos.map((p) => p.id);
        }
      } catch (err) {
        return false; // a failed read is not a reason to keep asking
      }
      if (got === null) return false;
      if (got.length === want.length && got.every((id, i) => id === want[i])) return true;
      if (Date.now() + delay > deadline) return false;
      await new Promise((r) => setTimeout(r, delay));
    }
  };

  initSheetShell(ctx);
  initContactSheet(ctx);

  // "Vis stort" (task 406). First of the actions, because it is the only one that changes nothing — a curator
  // reaches for it to *decide*, and the rest of the bar is what they do afterwards.
  initViewAction(ctx);
  initViewerEdit(ctx);
  initAlbumAction(ctx);
  initPositionAction(ctx);
  initPatrolAction(ctx);
  initCaptionAction(ctx);
  initCreditAction(ctx);
  initDeleteAction(ctx);

  // The two features that belong to one view each (task 396): upload is the all-photos view's, the editor card is
  // the album view's. The rest above is shared by both.
  if (document.getElementById('drop')) initUpload(ctx);
  initAlbumEditor(ctx);
  initAlbumOrder(ctx);
}

initAdminTool();
