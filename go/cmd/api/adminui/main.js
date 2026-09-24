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

  initSheetShell(ctx);
  initContactSheet(ctx);

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
}

initAdminTool();
