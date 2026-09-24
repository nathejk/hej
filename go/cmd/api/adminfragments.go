package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/nathejk/table/album"
)

// The admin tool's htmx fragments (task 395).
//
// # What these are, and what they replace
//
// A fragment handler renders a piece of a page as HTML for htmx to swap in. Each one replaces JavaScript that
// fetched JSON and assembled DOM nodes by hand — around 190 lines of it for the album list.
//
// The line count is not the reason. **The reason is that these are testable.** A fragment endpoint returns HTML
// an ordinary Go HTTP test can assert on; the JavaScript it replaces could only ever be checked by grepping its
// own source text, which this repo has 38 examples of and which produced three false positives in a single
// session — a needle matching the comment that explained it. Task 395 records that as the motivation.
//
// # Why these are not under /api/
//
// They return HTML, not JSON, and they exist for one client: this tool's own pages. Putting them under
// `/api/admin/` would add them to the OpenAPI guard's scope (task 380) and ask them to be documented as an API
// they are not. The maintainer confirmed the admin surface does not need OpenAPI specs, which is what makes
// this shape available.
//
// They are still `/admin/*`, so `requireAdmin` applies and `TestAdminRoutesUseOnlyTheAdminWrapper` enforces it.
//
// # Why they do not duplicate the JSON endpoints' logic
//
// Both write paths below publish the **same events** through the same helper the JSON handlers use
// (`publishAlbum`), with the same validation. What differs is only the response: an event log that disagreed
// with itself depending on which button produced the write would be the worst possible outcome of adopting
// htmx, so the rule is that a fragment handler may render differently and must never *decide* differently.

// adminAlbumListData is what the album list fragment renders.
type adminAlbumListData struct {
	Year string
	// Note is the line above the list — a count of unpublished albums, or the outcome of the action that
	// produced this render. Inside the fragment rather than beside it, so an outcome and the list it produced
	// arrive together.
	Note   string
	Albums []adminAlbumSummary
}

// showAdminAlbumsFragmentHandler renders the album list.
func (app *application) showAdminAlbumsFragmentHandler(w http.ResponseWriter, r *http.Request) {
	app.renderAdminAlbumList(w, r, "")
}

// createAdminAlbumFragmentHandler creates an album from the list's inline form and re-renders the list.
//
// Form-encoded rather than JSON, which is what htmx posts without an extension — and adding an extension to
// send JSON would be a fourth vendored dependency to avoid a `r.FormValue`.
func (app *application) createAdminAlbumFragmentHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	title, ok := app.adminAlbumTitleFromForm(w, r)
	if !ok {
		return
	}

	albumID, slug, err := app.createAdminAlbum(title, "", 0)
	if err != nil {
		// Rendered as a note in the fragment rather than as an error page: the curator is looking at a list and
		// mistyped a title, so the answer belongs where they are looking. A 4xx body htmx swapped in would
		// replace the list with a bare sentence and lose it.
		//
		// Only a *refusal* is shown verbatim. A broken read or a stalled event stream is not the curator's
		// sentence to read, and printing it into the page would leak an internal message onto a screen.
		var cerr *adminAlbumCreateError
		if errors.As(err, &cerr) && cerr.Kind == adminAlbumCreateRefused {
			app.renderAdminAlbumList(w, r, upperFirst(cerr.Err.Error())+".")
			return
		}
		app.Logger.Error("creating an album from the list fragment", "err", err)
		app.renderAdminAlbumList(w, r, "Kunne ikke oprette albummet. Prøv igen.")
		return
	}

	app.Logger.Info("admin created an album from the list fragment",
		"albumId", albumID, "slug", slug, "ip", clientIP(r))

	// Wait, briefly, for the fold to land the new album — because the answer to this request **is** the list, and
	// a list that comes back without the album the note says was just created reads as a failure. The JSON
	// endpoint has the same race and could ignore it: it answered 201 and the script then fetched the list as a
	// second request, by which time the fold had run.
	//
	// Bounded and best-effort, never an error: the album *was* created, the event is published, and refusing or
	// retrying would be a lie about what happened. Worst case the curator sees the note without the card, which
	// is what they saw before this waited at all. Observed live at well under the first tick.
	app.waitForAdminAlbum(albumID)

	app.renderAdminAlbumList(w, r,
		fmt.Sprintf("Albummet “%s” er oprettet som kladde.", title))
}

// waitForAdminAlbum blocks until the fold has an album, or until it has waited long enough.
//
// The fold runs in-process, so this is normally one tick or none. The ceiling is what keeps a stalled consumer
// from turning a create into a hung request.
func (app *application) waitForAdminAlbum(albumID string) {
	const (
		step = 20 * time.Millisecond
		max  = 500 * time.Millisecond
	)
	for waited := time.Duration(0); waited < max; waited += step {
		_, _, found, err := app.models.AlbumCurator.Album(app.config.eventYear, albumID)
		if err != nil || found {
			return
		}
		time.Sleep(step)
	}
}

// setAdminAlbumPublishedFragmentHandler publishes or unpublishes one album and re-renders the list.
//
// **Publication is published alone**, which is task 378's rule and the reason this is its own route rather than
// a general "patch the album" fragment: a curator pressing this has said one thing, and a shared endpoint would
// eventually let a half-typed title ride along with it.
func (app *application) setAdminAlbumPublishedFragmentHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	albumID := httprouter.ParamsFromContext(r.Context()).ByName("albumId")
	if albumID == "" {
		app.BadRequestResponse(w, r, errors.New("der skal angives et album"))
		return
	}
	// Parsed strictly: anything other than the two words is a broken client, and defaulting would mean a
	// mis-sent value silently unpublishing an album.
	var published bool
	switch r.FormValue("published") {
	case "true":
		published = true
	case "false":
		published = false
	default:
		app.BadRequestResponse(w, r, errors.New("published skal være true eller false"))
		return
	}

	a, _, found, err := app.models.AlbumCurator.Album(app.config.eventYear, albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	if perr := app.publishAlbum(album.VerbUpdated, albumID, album.Updated{
		AlbumID:   albumID,
		Year:      app.config.eventYear,
		Published: &published,
		UpdatedAt: time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	app.Logger.Info("admin changed an album's publication from the list fragment",
		"albumId", albumID, "slug", a.Slug, "published", published, "ip", clientIP(r))

	// The **frontpage** lags by up to a minute because its page is cached that long (task 335); the projection
	// itself folds in-process and is usually current by the time this re-reads it. Saying which is which is the
	// difference between a curator waiting and a curator pressing the button again.
	note := "Albummet er taget af forsiden. Det slår igennem inden for et minut."
	if published {
		note = "Albummet er udgivet. Det slår igennem på forsiden inden for et minut."
	}
	app.renderAdminAlbumList(w, r, note)
}

// deleteAdminAlbumFragmentHandler deletes a whole album and re-renders the list (task 396).
//
// # The same event as the Team section's takedown, on purpose
//
// `album.Deleted` is what `deleteAlbumHandler` publishes for the in-app takedown, and the fold already takes the
// album off the frontpage and marks every item removed (so the map drops them too). A second event meaning "the
// curator deleted it" would be a second way for an album to go, and two ways is how one of them forgets the map.
//
// # What it does not do
//
// It deletes **no photograph**. They stay in the library and in every other album they are in — which is the
// difference the confirm prompt in fragments.html has to make plain, for the same reason the photo delete sheet
// spells out "fjern" against "slet" (PRD 022 §5).
func (app *application) deleteAdminAlbumFragmentHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	albumID := httprouter.ParamsFromContext(r.Context()).ByName("albumId")
	if albumID == "" {
		app.BadRequestResponse(w, r, errors.New("der skal angives et album"))
		return
	}

	a, _, found, err := app.models.AlbumCurator.Album(app.config.eventYear, albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}
	// Already gone is answered with the list rather than a second event: a double click, or two curators, should
	// not append a no-op to the log.
	if a.Deleted {
		app.renderAdminAlbumList(w, r, "Albummet «"+a.Title+"» var allerede slettet.")
		return
	}

	if perr := app.publishAlbum(album.VerbDeleted, albumID, album.Deleted{
		AlbumID:   albumID,
		Year:      app.config.eventYear,
		DeletedAt: time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	app.Logger.Info("admin deleted an album from the list fragment",
		"albumId", albumID, "slug", a.Slug, "ip", clientIP(r))

	app.renderAdminAlbumList(w, r, "Albummet «"+a.Title+"» er slettet. Billederne ligger stadig i arkivet.")
}

// renderAdminAlbumList reads the year's albums and renders the fragment.
//
// `note` is the line above the list. Empty means "say how many are unpublished", which is the question a curator
// opens the page with — not "5 album" but "2 er ikke udgivet endnu".
func (app *application) renderAdminAlbumList(w http.ResponseWriter, r *http.Request, note string) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	rows, err := app.models.AlbumCurator.All(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	data := adminAlbumListData{Year: app.config.eventYear, Note: note}
	drafts := 0
	for _, a := range rows {
		if !a.Published && !a.Deleted {
			drafts++
		}
		data.Albums = append(data.Albums, adminAlbumSummary{
			AlbumID:      a.ID,
			Slug:         a.Slug,
			Title:        a.Title,
			Description:  a.Description,
			SortOrder:    a.SortOrder,
			Published:    a.Published,
			Deleted:      a.Deleted,
			ItemCount:    a.ItemCount,
			CoverPhotoID: a.CoverPhotoID,
		})
	}

	if note == "" {
		switch {
		case len(data.Albums) == 0:
			data.Note = "Der er ingen album endnu."
		case drafts == 0:
			data.Note = "Alle album er udgivet."
		case drafts == 1:
			data.Note = "1 album er ikke udgivet endnu."
		default:
			data.Note = fmt.Sprintf("%d album er ikke udgivet endnu.", drafts)
		}
	}

	app.renderAdminFragment(w, "albumlist", data)
}

// adminAlbumTitleFromForm validates a title posted from the list's form.
//
// Only the two checks the *form* can make: emptiness and length. Everything else — the slug, the collision — is
// `createAdminAlbum`'s, because those are decisions about the album rather than about the field, and duplicating
// them here is how the two surfaces would come to disagree.
func (app *application) adminAlbumTitleFromForm(w http.ResponseWriter, r *http.Request) (string, bool) {
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		app.renderAdminAlbumList(w, r, "Albummet skal have en titel.")
		return "", false
	}
	if len([]rune(title)) > maxAdminAlbumTitle {
		app.renderAdminAlbumList(w, r, "Titlen er for lang.")
		return "", false
	}
	return title, true
}

// upperFirst capitalises a message's first rune.
//
// The refusals in createAdminAlbum are written for a JSON `error` field, which this repo keeps lowercase. Shown
// to a curator they are a sentence, so they get a capital and a full stop rather than a second set of strings
// kept in step with the first.
func upperFirst(s string) string {
	for _, r := range s {
		return string(unicode.ToUpper(r)) + s[len(string(r)):]
	}
	return s
}

// ---------------------------------------------------------------------------
// The action sheets' pickers (step 4 of task 395).
//
// Each of these replaces a "fetch JSON, build option/label elements" block in page.js. They share a shape worth
// naming, because the remaining sheets should follow it:
//
//   - **The fragment renders what the server knows; the browser keeps what only it knows.** The selection is the
//     obvious case — it lives in a Set in page.js and no fragment goes looking for it. So the sheet's note
//     ("42 billeder bliver tagget") stays client-side and the fragment renders the albums, the patrol, the posts.
//   - **The fragment renders its own container, id and all.** The swap is `outerHTML`, so a listener bound
//     directly to a swapped element would be lost. The handlers in page.js are delegated from the sheet instead,
//     which is both what makes the swap safe and one fewer `getElementById` per control.
//   - **A fragment does not decide anything a JSON endpoint decides differently.** The create below goes through
//     the same `createAdminAlbum` as `/api/admin/albums`.
// ---------------------------------------------------------------------------

// adminAlbumPickerData is the add-to-album sheet's checkbox list.
type adminAlbumPickerData struct {
	Albums []adminAlbumPickerItem
	// Note is an outcome line, shown under the list. Empty for a plain open, because a sheet that greets the
	// curator with a sentence they did not ask for trains them to stop reading it.
	Note string
}

type adminAlbumPickerItem struct {
	AlbumID   string
	Title     string
	Published bool
	ItemCount int
	// Checked is whether the box is ticked when this render arrives.
	//
	// Carried in the render rather than reapplied by the browser afterwards, because the swap replaces the
	// elements: anything the client ticked and then re-derived would be a second source of truth for which
	// albums a selection is about to be filed into.
	Checked bool
}

// showAdminAlbumPickerHandler renders the add-to-album sheet's album checkboxes.
func (app *application) showAdminAlbumPickerHandler(w http.ResponseWriter, r *http.Request) {
	app.renderAdminAlbumPicker(w, r, checkedAlbumIDs(r), "")
}

// createAdminAlbumFromPickerHandler creates an album from inside the sheet and re-renders the picker with it
// ticked.
//
// **Ticked, because creating an album here is something a curator does *in order to* file the current selection
// into it.** The previous version had to create, re-fetch, then hunt the new checkbox and tick it — three steps
// that could disagree. One render cannot.
func (app *application) createAdminAlbumFromPickerHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	// The boxes already ticked, so a create does not cost the curator the albums they had chosen. The old
	// version rebuilt the list from scratch and lost them, which is the sort of thing nobody reports and
	// everybody works around.
	checked := checkedAlbumIDs(r)

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		app.renderAdminAlbumPicker(w, r, checked, "Albummet skal have en titel.")
		return
	}

	albumID, slug, err := app.createAdminAlbum(title, "", 0)
	if err != nil {
		var cerr *adminAlbumCreateError
		if errors.As(err, &cerr) && cerr.Kind == adminAlbumCreateRefused {
			app.renderAdminAlbumPicker(w, r, checked, upperFirst(cerr.Err.Error())+".")
			return
		}
		app.Logger.Error("creating an album from the add-to-album sheet", "err", err)
		app.renderAdminAlbumPicker(w, r, checked, "Kunne ikke oprette albummet. Prøv igen.")
		return
	}

	app.Logger.Info("admin created an album from the add-to-album sheet",
		"albumId", albumID, "slug", slug, "ip", clientIP(r))

	// The page's own album list is now stale by one. Told rather than redrawn, through the same
	// `albums-changed` event page.js fires — see the response header set in renderAdminAlbumPicker.
	app.waitForAdminAlbum(albumID)
	app.renderAdminAlbumPicker(w, r, append(checked, albumID),
		fmt.Sprintf("Albummet “%s” er oprettet som kladde og valgt.", title))
}

// renderAdminAlbumPicker renders the live albums, with the given ids ticked.
func (app *application) renderAdminAlbumPicker(w http.ResponseWriter, r *http.Request, checked []string, note string) {
	rows, ok := app.liveAdminAlbums(w, r)
	if !ok {
		return
	}

	tick := map[string]bool{}
	for _, id := range checked {
		tick[id] = true
	}

	data := adminAlbumPickerData{Note: note}
	for _, a := range rows {
		data.Albums = append(data.Albums, adminAlbumPickerItem{
			AlbumID:   a.ID,
			Title:     a.Title,
			Published: a.Published,
			ItemCount: a.ItemCount,
			Checked:   tick[a.ID],
		})
	}

	// Tells the page's album list to re-fetch, when this render created one. An htmx response header rather than
	// JavaScript dispatching the event, so the side that knows whether an album appeared is the side that says
	// so — the alternative was page.js guessing from a status code.
	if note != "" && strings.Contains(note, "oprettet") {
		w.Header().Set("HX-Trigger", "albums-changed")
	}
	app.renderAdminFragment(w, "albumpicker", data)
}

// checkedAlbumIDs reads the ticked boxes out of a request.
//
// Bounded, because this comes back from the browser and a picker cannot plausibly hold more albums than a year
// has. Unknown ids are harmless here: they only decide which boxes render ticked, and the add itself validates
// every album id again server-side.
func checkedAlbumIDs(r *http.Request) []string {
	if err := r.ParseForm(); err != nil {
		return nil
	}
	ids := r.Form["albumIds"]
	if len(ids) > maxAdminAlbumsPerPicker {
		ids = ids[:maxAdminAlbumsPerPicker]
	}
	return ids
}

// maxAdminAlbumsPerPicker bounds the ticked ids a picker render will echo back.
const maxAdminAlbumsPerPicker = 200

// adminDelAlbumPickerData is the delete sheet's album select.
type adminDelAlbumPickerData struct {
	Albums []adminAlbumPickerItem
}

// showAdminDelAlbumPickerHandler renders the delete sheet's album select.
func (app *application) showAdminDelAlbumPickerHandler(w http.ResponseWriter, r *http.Request) {
	rows, ok := app.liveAdminAlbums(w, r)
	if !ok {
		return
	}

	var data adminDelAlbumPickerData
	for _, a := range rows {
		data.Albums = append(data.Albums, adminAlbumPickerItem{
			AlbumID:   a.ID,
			Title:     a.Title,
			Published: a.Published,
			ItemCount: a.ItemCount,
		})
	}
	app.renderAdminFragment(w, "delalbumpicker", data)
}

// liveAdminAlbums reads the year's albums minus the deleted ones.
//
// Deleted albums are excluded here and **included** by the album list on the page. Both are right: the list
// answers "what have I got", and these two pickers answer "where can this go" — a deleted album is not somewhere
// a photograph can be filed, nor somewhere one can be removed from.
func (app *application) liveAdminAlbums(w http.ResponseWriter, r *http.Request) ([]album.CuratorAlbum, bool) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return nil, false
	}
	rows, err := app.models.AlbumCurator.All(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return nil, false
	}
	live := make([]album.CuratorAlbum, 0, len(rows))
	for _, a := range rows {
		if !a.Deleted {
			live = append(live, a)
		}
	}
	return live, true
}

// adminPatrolConfirmData is the line a curator confirms a patrol against.
type adminPatrolConfirmData struct {
	// Number is set only when a patrol was found, and is what enables the tag button. Empty for every other
	// outcome, so a failed lookup cannot leave a stale confirmation behind — which is the one way this could tag
	// the wrong patrol.
	Number string
	// Message is the whole of what the curator reads.
	Message string
}

// showAdminPatrolConfirmHandler resolves a typed patrol number and renders the confirmation.
//
// Every outcome is a **200 with a rendered line**, including "no such patrol". htmx does not swap a 4xx body by
// default, so answering 404 here would leave the previous confirmation on screen while the number in the box had
// changed — precisely the disagreement the confirmation exists to prevent. The JSON endpoint keeps its 404,
// because a status code is what a JSON client reads.
func (app *application) showAdminPatrolConfirmHandler(w http.ResponseWriter, r *http.Request) {
	render := func(number, message string) {
		app.renderAdminFragment(w, "patrolconfirm",
			adminPatrolConfirmData{Number: number, Message: message})
	}

	if app.models.PublicPatrols == nil {
		render("", "Patruljerne er ikke tilgængelige lige nu.")
		return
	}

	raw := strings.TrimSpace(r.FormValue("number"))
	if raw == "" {
		render("", "Skriv patruljens nummer.")
		return
	}
	// The same normalisation the public patrol page applies, so "042" and "42" are one patrol — which is what the
	// number on the sign means.
	number, ok := normalizePatrolNumber(raw)
	if !ok {
		render("", "Patruljenummeret skal være et tal.")
		return
	}

	p, found, err := app.models.PublicPatrols.ByNumber(app.config.eventYear, number)
	if err != nil {
		app.Logger.Error("resolving a patrol for the tag sheet", "err", err)
		render("", "Kunne ikke søge. Prøv igen.")
		return
	}
	if !found {
		// Named plainly. Unlike the *public* patrol page, this need not be indistinguishable from a closed gate:
		// the caller is already behind the credential, and a curator who typed 42 for 24 deserves to be told.
		render("", "Der er ingen patrulje med nummer "+number+" i år.")
		return
	}

	// The patrol, its group and its korps — never a person. The read has no field for one.
	bits := []string{}
	for _, s := range []string{p.Name, p.GroupName, p.KorpsLabel()} {
		if s != "" {
			bits = append(bits, s)
		}
	}
	message := "Patrulje " + p.Number
	if len(bits) > 0 {
		message += ": " + strings.Join(bits, " · ")
	}
	render(p.Number, message)
}

// renderAdminFragment executes one of the fragment templates.
func (app *application) renderAdminFragment(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminTemplates.ExecuteTemplate(w, name, data); err != nil {
		// Logged rather than answered: ExecuteTemplate may already have written a partial body, so there is no
		// status left to set. The same reasoning renderPublicPage gives.
		app.Logger.Error("rendering an admin fragment", "fragment", name, "err", err)
	}
}

// ---------------------------------------------------------------------------
// The position sheet's post picker (step 5 of task 395).
//
// **The Leaflet map is not touched.** It stays custom, per the maintainer's rule that JavaScript solving a problem
// no framework solves simply stays — and a WMS layer with token auth, a tile-retry policy read from a shared JSON
// file, and a marker that has to agree with a <select> is squarely that.
//
// The one hazard the migration had to respect is task 390's: Leaflet caches its pixel size at creation, so a map
// created in a hidden container gets a size of zero and the symptom is one tile in the corner. The order in
// page.js is therefore still open the sheet, then draw, then `invalidateSize` on the next frame — and the picker's
// swap deliberately does **not** touch `#posmap`, so no fragment can discard a live map instance.
// ---------------------------------------------------------------------------

// adminCheckpointPickerData is the position sheet's post select.
type adminCheckpointPickerData struct {
	Checkpoints []adminCheckpointView
}

// showAdminCheckpointPickerHandler renders the year's sited posts as a select.
//
// Rebuilt on every open rather than cached, because posts get sited during the season: a stale list would offer a
// post whose position the save then resolves differently, or not at all.
func (app *application) showAdminCheckpointPickerHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.CheckpointCurator == nil {
		app.ServiceUnavailableResponse(w, r, "posterne er ikke tilgængelige lige nu")
		return
	}

	points, err := app.models.CheckpointCurator.Positioned(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	data := adminCheckpointPickerData{Checkpoints: make([]adminCheckpointView, 0, len(points))}
	for _, c := range points {
		data.Checkpoints = append(data.Checkpoints, adminCheckpointView{
			ID:   string(c.ID),
			Name: c.Name,
			Lat:  c.Lat,
			Lng:  c.Lng,
		})
	}
	app.renderAdminFragment(w, "checkpointpicker", data)
}

// ---------------------------------------------------------------------------
// The contact sheet (step 6 of task 395).
//
// # The risk the task flagged, and why it turned out not to be one
//
// Task 395 named this the one real risk: PRD 022 §7 requires the selection to survive every action, and "a naive
// hx-swap over the grid destroys the selection set". The mitigations it offered were `hx-preserve` or Alpine owning
// the selection outside the swapped region.
//
// Neither was needed, because **the selection was never in the grid.** It is a Set of ids in page.js, held outside
// the DOM deliberately and for this exact reason — the file's own comment says a DOM-derived selection "would be
// lost by any re-render, and it would silently shrink to what is currently loaded". A swap replaces cells; the Set
// does not notice.
//
// What *is* read back from the DOM afterwards is `order`, the display order a shift-click range is resolved
// against. That is not state: it is the definition of "the cells currently shown", so deriving it from the cells
// currently shown cannot be wrong. The distinction between those two — selection outside the DOM, display order
// from it — is the whole answer to the question the task left open.
//
// # What did not migrate, and why
//
// The selection model, the shift-click ranges, the keyboard navigation and "select all matching this filter" stay
// custom. They are the ~250 lines the maintainer's rule covers: a bounded concurrent id-pager and a listbox with
// range selection are not things htmx expresses, and expressing them badly would cost the feature PRD 022 §3 calls
// the true shape of the work.
//
// "Select all matching this filter" still pages the **JSON** endpoint for ids, which is why both readers now go
// through one `readAdminLibraryPage`: the grid renders one page of a filter and select-all pages the ids of the
// same filter, and two interpretations of it disagreeing is exactly how a bulk action lands on photographs the
// curator never saw.
// ---------------------------------------------------------------------------

// adminContactSheetData is one page of thumbnails, plus everything around the grid that has to stay in step
// with it.
//
// The three out-of-band pieces are here rather than fetched separately because they are answers to the same
// question. A second request for the counts could return a different read.
type adminContactSheetData struct {
	Photos []adminLibraryPhoto
	Counts adminCountsView

	// ShownNote is the line above the grid: how many are on screen, and whether there are more.
	ShownNote string
	// HasMore and NextOffset drive the "more" button. NextOffset is where the next page starts, computed here
	// rather than by the browser adding up page sizes — the browser's idea of how many it has would drift the
	// first time a clamped limit differed from the one it asked for.
	HasMore    bool
	NextOffset int
}

// showAdminContactSheetHandler renders one page of the library as thumbnails.
func (app *application) showAdminContactSheetHandler(w http.ResponseWriter, r *http.Request) {
	page, ok := app.readAdminLibraryPage(w, r)
	if !ok {
		return
	}

	shown := page.Offset + len(page.Photos)
	data := adminContactSheetData{
		Photos:     page.Photos,
		Counts:     page.Counts,
		HasMore:    page.HasMore,
		NextOffset: shown,
	}

	switch {
	case shown == 0:
		// Said plainly. An empty grid under a filter is an ordinary answer, and the alternative is a curator
		// wondering whether the page is still loading.
		data.ShownNote = "Ingen billeder matcher."
	case page.HasMore:
		data.ShownNote = fmt.Sprintf("%d vist — der er flere", shown)
	default:
		data.ShownNote = fmt.Sprintf("%d vist", shown)
	}

	app.renderAdminFragment(w, "contactsheet", data)
}
