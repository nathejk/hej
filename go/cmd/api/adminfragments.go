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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminTemplates.ExecuteTemplate(w, "albumlist", data); err != nil {
		// Logged rather than answered: ExecuteTemplate may already have written a partial body, so there is no
		// status left to set. The same reasoning renderPublicPage gives.
		app.Logger.Error("rendering the album list fragment", "err", err)
	}
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
