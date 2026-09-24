package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"

	"nathejk.dk/nathejk/table/album"
)

// The curator's album writes (PRD 022 §6, tasks 375 and 378).
//
// # This is the action the whole model change exists for
//
// PRD 022 §2: a photograph often belongs in more than one album — "Natten", "Målet", "Postmandskabet". Before
// the library split that was two independent rows with two independent coordinates, free to disagree, with
// nothing to notice when they did. Now it is two membership rows referencing one photograph, so its location,
// caption and tags are the same in every album it appears in.

// maxAdminAlbumTitle bounds a title, and maxAdminSelection bounds a bulk action.
//
// The selection ceiling is not about protecting the server — an organizer adding a card's worth to an album is
// the intended use. It is about the *event log*: one request becomes one event per (album, photo) pair, so a
// runaway client could append tens of thousands of messages to a log that is never rewritten. 2000 is several
// times a real card and still a bounded mistake.
const (
	maxAdminAlbumTitle = 120
	maxAdminAlbumDesc  = 2000
	maxAdminSelection  = 2000
)

// listAdminAlbumsResponse is every album in the year, as the curator sees them.
type listAdminAlbumsResponse struct {
	Albums []adminAlbumSummary `json:"albums"`
}

// adminAlbumSummary is one album in the curator's list.
//
// Carries `published` and `deleted`, which the public summary type deliberately does not: the public read
// filters on them instead, and a public page has no business knowing an album it cannot see exists.
type adminAlbumSummary struct {
	AlbumID     string `json:"albumId"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sortOrder"`
	Published   bool   `json:"published"`
	Deleted     bool   `json:"deleted,omitempty"`
	ItemCount   int    `json:"itemCount"`

	// CoverPhotoID is the photograph the album opens with, or absent when it has none (task 391).
	//
	// A photo id rather than a URL, so the client builds the admin media address and the server does not
	// hard-code one. Omitted rather than empty when an album has no live items, which is how a new album and an
	// album whose photographs were all deleted both look — and both are states the list must render honestly
	// rather than hide.
	CoverPhotoID string `json:"coverPhotoId,omitempty"`
}

// listAdminAlbumsHandler returns every album in the year, drafts included.
//
// @Summary      List the year's albums
// @Description  Returns every album in the configured event year, **including unpublished and deleted ones**, in curator order. That is the opposite of the public read, which shows only published albums and answers the same "not found" for unknown, unpublished and deleted so drafts cannot be enumerated — which is why this one is on the curator interface and behind the admin credential. Unpaged: a year holds three to five albums, or tens at most. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  listAdminAlbumsResponse
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums are unavailable"
// @Router       /admin/albums [get]
func (app *application) listAdminAlbumsHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	rows, err := app.models.AlbumCurator.All(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	out := listAdminAlbumsResponse{Albums: make([]adminAlbumSummary, 0, len(rows))}
	for _, a := range rows {
		out.Albums = append(out.Albums, adminAlbumSummary{
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

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// updateAdminAlbumRequest edits an album's editorial fields.
//
// # Every field is a pointer, and the slug is absent
//
// Pointers so that "not mentioned" and "set to empty" are different messages — a curator clearing a description
// and a curator renaming an album must not be the same request, or one silently wipes the other's field. The
// same delta shape `album.Updated` uses, for the same reason.
//
// **There is no slug field, and there must never be one.** The slug is the album's public address, frozen at
// creation, because a retitled album that answers 404 is a dead link in a family's chat history. `album.Updated`
// has no slug field either, so the fold could not apply one even if this accepted it — two layers, as with
// `published` on create.
type updateAdminAlbumRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	SortOrder   *int    `json:"sortOrder,omitempty"`

	// Published is how an album reaches the frontpage. Only expressible here, never on create.
	Published *bool `json:"published,omitempty"`
}

// updateAdminAlbumHandler edits an album.
//
// @Summary      Edit an album
// @Description  Changes an album's title, description, sort order, or whether it is published. Every field is optional and only the ones sent are written, so editing one cannot wipe another. **The slug cannot be changed**: it is the album's public address and a retitled album answering 404 is a dead link in somebody's chat history. Publishing is only expressible here, never on create, because an album is assembled over several sittings. Unpublishing removes the album from the public frontpage within that page's 60-second cache window. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        albumId  path      string                   true  "album id"
// @Param        request  body      updateAdminAlbumRequest  true  "the fields to change"
// @Success      200  {object}  adminAlbumSummary
// @Failure      400  {object}  map[string]string  "nothing to change, or an unusable value"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "unknown album"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums or the event stream are unavailable"
// @Router       /admin/albums/{albumId} [patch]
func (app *application) updateAdminAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	albumID := httprouter.ParamsFromContext(r.Context()).ByName("albumId")

	var in updateAdminAlbumRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	if in.Title == nil && in.Description == nil && in.SortOrder == nil && in.Published == nil {
		// Refused rather than treated as a no-op: a request that changes nothing is a broken client, and the fold
		// would silently drop it.
		app.BadRequestResponse(w, r, errors.New("der er ingen ændringer i forespørgslen"))
		return
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			app.BadRequestResponse(w, r, errors.New("albummet skal have en titel"))
			return
		}
		if len([]rune(title)) > maxAdminAlbumTitle {
			app.BadRequestResponse(w, r, errors.New("titlen er for lang"))
			return
		}
		in.Title = &title
	}
	if in.Description != nil && len([]rune(*in.Description)) > maxAdminAlbumDesc {
		app.BadRequestResponse(w, r, errors.New("beskrivelsen er for lang"))
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
		AlbumID:     albumID,
		Year:        app.config.eventYear,
		Title:       in.Title,
		Description: in.Description,
		SortOrder:   in.SortOrder,
		Published:   in.Published,
		UpdatedAt:   time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	// Publishing is the one edit with a public consequence, so it is logged as its own fact rather than folded
	// into "updated": "when did this go live" is a question somebody will ask.
	if in.Published != nil {
		app.Logger.Info("admin changed an album's publication",
			"albumId", albumID, "slug", a.Slug, "published", *in.Published, "ip", clientIP(r))
	} else {
		app.Logger.Info("admin edited an album", "albumId", albumID, "slug", a.Slug, "ip", clientIP(r))
	}

	// The response reflects what was asked for, applied over what was read. The projection is downstream of the
	// log so it has not caught up yet, and echoing the stale row would show the curator their edit failing.
	out := adminAlbumSummary{
		AlbumID:     a.ID,
		Slug:        a.Slug,
		Title:       a.Title,
		Description: a.Description,
		SortOrder:   a.SortOrder,
		Published:   a.Published,
		Deleted:     a.Deleted,
		ItemCount:   a.ItemCount,
	}
	if in.Title != nil {
		out.Title = *in.Title
	}
	if in.Description != nil {
		out.Description = *in.Description
	}
	if in.SortOrder != nil {
		out.SortOrder = *in.SortOrder
	}
	if in.Published != nil {
		out.Published = *in.Published
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// reorderAdminAlbumItemsRequest states an album's new order.
type reorderAdminAlbumItemsRequest struct {
	// PhotoIDs is the album's live items in their new order. Position is the new ordinal.
	PhotoIDs []string `json:"photoIds"`
}

// reorderAdminAlbumItemsHandler rewrites an album's order.
//
// @Summary      Reorder an album's photographs
// @Description  Rewrites the order of an album's photographs. The request carries the album's live items in their new sequence, and position in that list becomes the new ordinal — one event for the whole order rather than one per moved item, because `album_item` is keyed on both the position and the photograph, so moving one item into a position another holds cannot be expressed as independent writes. The **cover is the first live item**, so reordering changes the cover; there is no separate cover field to set. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        albumId  path      string                         true  "album id"
// @Param        request  body      reorderAdminAlbumItemsRequest  true  "the live items in their new order"
// @Success      204  "reordered"
// @Failure      400  {object}  map[string]string  "an empty order, a duplicate, or a photograph not in this album"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "unknown album"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums or the event stream are unavailable"
// @Router       /admin/albums/{albumId}/items [patch]
func (app *application) reorderAdminAlbumItemsHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	albumID := httprouter.ParamsFromContext(r.Context()).ByName("albumId")

	var in reorderAdminAlbumItemsRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	photoIDs, err := dedupeAdminIDs(in.PhotoIDs, "billeder")
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	// De-duplication above quietly drops a repeat, which would leave the order shorter than the curator sent and
	// therefore not the order they were shown. Refused instead.
	if len(photoIDs) != len(in.PhotoIDs) {
		app.BadRequestResponse(w, r, errors.New("samme billede optræder flere gange i rækkefølgen"))
		return
	}

	_, items, found, err := app.models.AlbumCurator.Album(app.config.eventYear, albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	// Every named photograph must actually be in this album. Without the check a typo'd id would be published, the
	// fold's per-photograph UPDATE would match nothing, and the album would come back in an order the curator did
	// not ask for — with one item stranded in the offset range (see handleItemsReordered).
	member := make(map[string]bool, len(items))
	for _, it := range items {
		if !it.Removed {
			member[it.PhotoID] = true
		}
	}
	for _, photoID := range photoIDs {
		if !member[photoID] {
			app.BadRequestResponse(w, r,
				fmt.Errorf("et af billederne ligger ikke i dette album"))
			return
		}
	}

	if perr := app.publishAlbum(album.VerbItemsReordered, albumID, album.ItemsReordered{
		AlbumID:     albumID,
		Year:        app.config.eventYear,
		PhotoIDs:    photoIDs,
		ReorderedAt: time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	app.Logger.Info("admin reordered an album",
		"albumId", albumID, "count", len(photoIDs), "ip", clientIP(r))

	w.WriteHeader(http.StatusNoContent)
}

// moveAdminAlbumItemsRequest moves some of an album's photographs next to another (task 396).
type moveAdminAlbumItemsRequest struct {
	// PhotoIDs are the photographs to move. They land together, in the order they already had in the album —
	// not the order they are listed here, which for a selection is the order they were clicked.
	PhotoIDs []string `json:"photoIds"`
	// BeforePhotoID or AfterPhotoID, exactly one: the photograph they land next to. Both exist because a drop
	// after the last *loaded* cell has no loaded neighbour to be "before".
	BeforePhotoID string `json:"beforePhotoId,omitempty"`
	AfterPhotoID  string `json:"afterPhotoId,omitempty"`
}

// moveAdminAlbumItemsHandler moves photographs within an album — the server side of drag-and-drop.
//
// # Why the server computes the order
//
// `ItemsReordered` must carry the album's **whole** live order (see reorderAdminAlbumItemsHandler), and with the
// album view scrolling in pages the browser may not hold it: dragging photograph 180 to the front of an album of
// 200 happens with maybe 120 loaded. So the request says only what moved and where to, and the order is built
// here from the projection. It publishes the same event as a full reorder, so the fold has one way to reorder.
//
// @Summary      Move photographs within an album
// @Description  Moves the named photographs, together and keeping their current relative order, to just before or just after another photograph in the same album. The server builds the album's complete new order and publishes it as one reorder, so the client does not need to have loaded the whole album. Exactly one of `beforePhotoId` and `afterPhotoId` must be given, and it must not be one of the photographs being moved. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        albumId  path      string                      true  "album id"
// @Param        request  body      moveAdminAlbumItemsRequest  true  "what moves, and where to"
// @Success      204  "moved"
// @Failure      400  {object}  map[string]string  "nothing to move, no target or two, a target that is itself moving, or a photograph not in this album"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "unknown album"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums or the event stream are unavailable"
// @Router       /admin/albums/{albumId}/move [patch]
func (app *application) moveAdminAlbumItemsHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	albumID := httprouter.ParamsFromContext(r.Context()).ByName("albumId")

	var in moveAdminAlbumItemsRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	moving, err := dedupeAdminIDs(in.PhotoIDs, "billeder")
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	before, after := strings.TrimSpace(in.BeforePhotoID), strings.TrimSpace(in.AfterPhotoID)
	if (before == "") == (after == "") {
		app.BadRequestResponse(w, r, errors.New("angiv enten beforePhotoId eller afterPhotoId"))
		return
	}
	target := before + after

	_, items, found, err := app.models.AlbumCurator.Album(app.config.eventYear, albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	order, err := moveAlbumOrder(items, moving, target, after != "")
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	if perr := app.publishAlbum(album.VerbItemsReordered, albumID, album.ItemsReordered{
		AlbumID:     albumID,
		Year:        app.config.eventYear,
		PhotoIDs:    order,
		ReorderedAt: time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	app.Logger.Info("admin moved photographs within an album",
		"albumId", albumID, "moved", len(moving), "ip", clientIP(r))

	w.WriteHeader(http.StatusNoContent)
}

// moveAlbumOrder returns the album's live order with `moving` taken out and put back next to `target`.
//
// `items` is in ordinal order, as `CuratorQueries.Album` returns it. Live means not removed from the album —
// the same membership the full reorder checks, so a photograph deleted from the library keeps its place.
func moveAlbumOrder(items []album.CuratorItem, moving []string, target string, placeAfter bool) ([]string, error) {
	isMoving := make(map[string]bool, len(moving))
	for _, id := range moving {
		isMoving[id] = true
	}
	if isMoving[target] {
		return nil, errors.New("billedet der flyttes hen til, kan ikke selv flyttes")
	}

	var live, moved, rest []string
	for _, it := range items {
		if it.Removed {
			continue
		}
		live = append(live, it.PhotoID)
		if isMoving[it.PhotoID] {
			moved = append(moved, it.PhotoID)
		} else {
			rest = append(rest, it.PhotoID)
		}
	}
	if len(moved) != len(moving) {
		return nil, errors.New("et af billederne ligger ikke i dette album")
	}

	at := -1
	for i, id := range rest {
		if id == target {
			at = i
			break
		}
	}
	if at < 0 {
		return nil, errors.New("billedet der flyttes hen til, ligger ikke i dette album")
	}
	if placeAfter {
		at++
	}

	out := make([]string, 0, len(live))
	out = append(out, rest[:at]...)
	out = append(out, moved...)
	out = append(out, rest[at:]...)
	return out, nil
}

// createAdminAlbumRequest creates an album.
type createAdminAlbumRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sortOrder,omitempty"`
}

// createAdminAlbumResponse reports the new album.
type createAdminAlbumResponse struct {
	AlbumID string `json:"albumId"`
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	// Published is always false here, and is returned rather than assumed so a client cannot quietly come to
	// believe otherwise. See the handler's note.
	Published bool `json:"published"`
}

// createAdminAlbumHandler opens a new, unpublished album.
//
// @Summary      Create an album
// @Description  Creates an empty album in the configured event year. **Always unpublished**: an album is assembled over several sittings, and a create that could publish would put the first photograph on the open web before the second was chosen. Publishing is a separate edit. The slug is derived from the title and frozen at creation, because it is the album's public address and a retitled album must not break a link somebody already shared. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        request  body      createAdminAlbumRequest  true  "title, and optionally a description and sort order"
// @Success      201  {object}  createAdminAlbumResponse
// @Failure      400  {object}  map[string]string  "missing or unusable title, or the slug is taken"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums or the event stream are unavailable"
// @Router       /admin/albums [post]
func (app *application) createAdminAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	var in createAdminAlbumRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	title := strings.TrimSpace(in.Title)
	albumID, slug, err := app.createAdminAlbum(title, in.Description, in.SortOrder)
	if err != nil {
		app.writeAdminAlbumCreateFailure(w, r, err)
		return
	}

	app.Logger.Info("admin created an album",
		"albumId", albumID, "slug", slug, "ip", clientIP(r))

	if err := app.WriteJSON(w, http.StatusCreated, createAdminAlbumResponse{
		AlbumID:   albumID,
		Slug:      slug,
		Title:     title,
		Published: false,
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// adminAlbumCreateKind says why createAdminAlbum refused, so each caller can answer in its own idiom — a status
// code for the JSON endpoint, a note above the list for the htmx fragment — without re-deriving the distinction.
//
// Classified rather than left as a bare error because the three outcomes are genuinely different: the curator
// mistyped something, the database is unhappy, or the event could not be published. Flattening them would have
// meant the fragment showing "prøv igen" for a title that will never be accepted.
type adminAlbumCreateKind int

const (
	// adminAlbumCreateRefused is the curator's mistake: no title, too long, unusable as an address, or a slug
	// another album already holds. Answered 400, and shown as-is to the curator.
	adminAlbumCreateRefused adminAlbumCreateKind = iota
	// adminAlbumCreateBroken is ours: a read failed.
	adminAlbumCreateBroken
	// adminAlbumCreatePublish is the event stream's. It has its own writer because the distinction between "the
	// stream is down" and "the write was rejected" is one `writeAlbumPublishFailure` already draws.
	adminAlbumCreatePublish
)

// adminAlbumCreateError carries a refusal out of createAdminAlbum with its kind intact.
type adminAlbumCreateError struct {
	Kind adminAlbumCreateKind
	Err  error
}

func (e *adminAlbumCreateError) Error() string { return e.Err.Error() }
func (e *adminAlbumCreateError) Unwrap() error { return e.Err }

// createAdminAlbum validates a title, derives the slug and publishes `album.Created`.
//
// Shared by the JSON endpoint above and the list fragment's inline form (adminfragments.go). **Shared rather than
// reimplemented**, because the two differ only in how they answer: an event log that disagreed with itself
// depending on which button produced the write would be the worst possible outcome of adding a second surface.
func (app *application) createAdminAlbum(title, description string, sortOrder int) (string, string, error) {
	refuse := func(err error) (string, string, error) {
		return "", "", &adminAlbumCreateError{Kind: adminAlbumCreateRefused, Err: err}
	}

	if title == "" {
		return refuse(errors.New("albummet skal have en titel"))
	}
	if len([]rune(title)) > maxAdminAlbumTitle {
		return refuse(errors.New("titlen er for lang"))
	}
	if len([]rune(description)) > maxAdminAlbumDesc {
		return refuse(errors.New("beskrivelsen er for lang"))
	}

	slug := slugifyAlbumTitle(title)
	if slug == "" {
		// A title of only punctuation or emoji. Refused with a reason rather than given a generated slug,
		// because the slug is the public address and "album-1" is not something a curator chose.
		return refuse(errors.New("titlen kan ikke bruges i en adresse — brug bogstaver eller tal"))
	}

	// Deleted albums count as taking a slug. The schema makes it unique per year, so reusing a deleted one
	// would fail on the insert — and if the deletion were ever undone the two would collide. Refusing here
	// lets the tool say which it is instead of surfacing a database error.
	taken, err := app.models.AlbumCurator.SlugTaken(app.config.eventYear, slug)
	if err != nil {
		return "", "", &adminAlbumCreateError{Kind: adminAlbumCreateBroken, Err: err}
	}
	if taken {
		return refuse(fmt.Errorf("adressen “%s” er brugt af et andet album — vælg en anden titel", slug))
	}

	albumID := newAlbumID()
	// **Unpublished, unconditionally.** Not a default the caller may override: there is no parameter for it and
	// no branch here. `album.Created` carries no `published` either (task 363), so publishing is expressible
	// only as a separate update — which is what keeps a half-assembled album off the open web.
	if err := app.publishAlbum(album.VerbCreated, albumID, album.Created{
		AlbumID:     albumID,
		Year:        app.config.eventYear,
		Slug:        slug,
		Title:       title,
		Description: description,
		SortOrder:   sortOrder,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return "", "", &adminAlbumCreateError{Kind: adminAlbumCreatePublish, Err: err}
	}
	return albumID, slug, nil
}

// writeAdminAlbumCreateFailure answers a createAdminAlbum error as JSON.
func (app *application) writeAdminAlbumCreateFailure(w http.ResponseWriter, r *http.Request, err error) {
	var cerr *adminAlbumCreateError
	if !errors.As(err, &cerr) {
		// Unreachable today. A 500 rather than a 400, because an unclassified error is ours and not the
		// curator's, and guessing the friendlier status would hide a bug.
		app.ServerErrorResponse(w, r, err)
		return
	}
	switch cerr.Kind {
	case adminAlbumCreateRefused:
		app.BadRequestResponse(w, r, cerr.Err)
	case adminAlbumCreatePublish:
		app.writeAlbumPublishFailure(w, r, cerr.Err)
	default:
		app.ServerErrorResponse(w, r, cerr.Err)
	}
}

// addAdminAlbumItemsRequest adds a selection to one or more albums.
type addAdminAlbumItemsRequest struct {
	PhotoIDs []string `json:"photoIds"`
	AlbumIDs []string `json:"albumIds"`
}

// addAdminAlbumItemsResponse reports what happened, per album.
//
// Per album rather than one total, because "40 added" hides the case a curator most needs to see: that one of
// the three albums they picked already held most of the selection.
type addAdminAlbumItemsResponse struct {
	Albums  []addAdminAlbumItemsResult `json:"albums"`
	Message string                     `json:"message"`
}

type addAdminAlbumItemsResult struct {
	AlbumID string `json:"albumId"`
	Title   string `json:"title,omitempty"`
	// Added is how many memberships this request created.
	Added int `json:"added"`
	// AlreadyThere is how many of the selection the album already held.
	//
	// Reported rather than silently folded into `Added`, because re-selecting is routine when a curator works
	// through a filter over several sittings — and a tool that said "40 added" when it added three would be
	// lying about the one thing the curator is tracking.
	AlreadyThere int `json:"alreadyThere"`
}

// addAdminAlbumItemsHandler puts a selection of photographs into one or more albums.
//
// @Summary      Add photographs to albums
// @Description  Adds a selection of library photographs to one or more albums in a single action, publishing one item-added event per (album, photograph) pair. A photograph the album already holds is a **no-op** — no duplicate row and no ordinal change — because re-selecting is routine when a curator works through a filter over several sittings. The response reports, per album, how many were added and how many were already there. Ordinals are appended after the album's current maximum, including positions previously removed, so a removed item's position is never reused. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        request  body      addAdminAlbumItemsRequest  true  "the photograph ids and the album ids"
// @Success      200  {object}  addAdminAlbumItemsResponse
// @Failure      400  {object}  map[string]string  "no photographs, no albums, or too many"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "an unknown album"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums or the event stream are unavailable"
// @Router       /admin/albums/items [post]
func (app *application) addAdminAlbumItemsHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	var in addAdminAlbumItemsRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	photoIDs, err := dedupeAdminIDs(in.PhotoIDs, "billeder")
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	albumIDs, err := dedupeAdminIDs(in.AlbumIDs, "album")
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	// The pair count is what becomes events, so it is what needs bounding — twenty albums by two hundred
	// photographs is four thousand messages from one click.
	if len(photoIDs)*len(albumIDs) > maxAdminSelection {
		app.BadRequestResponse(w, r,
			fmt.Errorf("for mange på én gang (%d × %d); del det op",
				len(photoIDs), len(albumIDs)))
		return
	}

	out := addAdminAlbumItemsResponse{Albums: make([]addAdminAlbumItemsResult, 0, len(albumIDs))}
	totalAdded := 0

	for _, albumID := range albumIDs {
		// Read the album immediately before writing to it, once per album, for two things at once: to reject an
		// unknown id, and to learn which of the selection it already holds.
		a, items, found, rerr := app.models.AlbumCurator.Album(app.config.eventYear, albumID)
		if rerr != nil {
			app.ServerErrorResponse(w, r, rerr)
			return
		}
		if !found {
			app.NotFoundResponse(w, r)
			return
		}

		// Existing membership, including **removed** items. A photograph whose membership was removed and is
		// now being re-added is not "already there" — it needs a new event to clear the removal — but it also
		// must not take a second ordinal, or the album would hold it twice once the fold runs. Re-publishing at
		// its existing ordinal clears `deleted` and puts it back exactly where it was.
		at := make(map[string]int, len(items))
		live := make(map[string]bool, len(items))
		for _, it := range items {
			at[it.PhotoID] = it.Ordinal
			if !it.Removed {
				live[it.PhotoID] = true
			}
		}

		// Ordinals are appended after the current maximum, **including positions previously removed** — see
		// `NextOrdinal`. Reusing a removed position would resurrect that row's soft delete through the upsert,
		// silently putting a taken-down photograph back on the page.
		next, rerr := app.models.AlbumCurator.NextOrdinal(app.config.eventYear, albumID)
		if rerr != nil {
			app.ServerErrorResponse(w, r, rerr)
			return
		}

		result := addAdminAlbumItemsResult{AlbumID: albumID, Title: a.Title}
		now := time.Now().UTC()

		for _, photoID := range photoIDs {
			if live[photoID] {
				result.AlreadyThere++
				continue
			}

			ordinal, existing := at[photoID]
			if !existing {
				ordinal = next
				next++
			}

			if perr := app.publishAlbum(album.VerbItemAdded, albumID, album.ItemAdded{
				AlbumID: albumID,
				Year:    app.config.eventYear,
				Ordinal: ordinal,
				PhotoID: photoID,
				AddedAt: now,
			}); perr != nil {
				// Published so far, failed here. The response must not claim the selection is filed, so this is
				// a failure for the whole request even though some events landed — the curator reloads and sees
				// what actually happened, which is the honest outcome. Partial success reported as success is
				// how a photograph goes missing from an album nobody checks again.
				app.Logger.Error("admin album add failed partway",
					"albumId", albumID, "photoId", photoID, "added", totalAdded, "err", perr)
				app.writeAlbumPublishFailure(w, r, perr)
				return
			}
			result.Added++
			totalAdded++
		}

		out.Albums = append(out.Albums, result)
		app.Logger.Info("admin added photographs to an album",
			"albumId", albumID, "added", result.Added, "already", result.AlreadyThere,
			"ip", clientIP(r))
	}

	out.Message = adminAddedMessage(totalAdded, len(albumIDs))
	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// adminAddedMessage writes the one sentence the action bar shows.
//
// Written here rather than assembled by the client for the reason the upload path gives: one place decides the
// wording, and it is the place that knows what happened.
func adminAddedMessage(added, albums int) string {
	if added == 0 {
		return "Billederne lå allerede i de valgte album."
	}
	// "i albummet" when there is one, because naming the count of a thing there is only one of reads as a form
	// rather than a sentence.
	if albums == 1 {
		return fmt.Sprintf("%s lagt i albummet.", photoCount(added))
	}
	return fmt.Sprintf("%s lagt i %s.", photoCount(added), albumCount(albums))
}

// dedupeAdminIDs cleans a selection: trimmed, de-duplicated, order preserved, non-empty.
//
// De-duplication matters because the client sends a Set but JSON has no such thing, and a duplicated id would
// otherwise produce two events for one pair — harmless to the fold, which upserts, but it would make the
// response's counts wrong, and those counts are what the curator reads.
func dedupeAdminIDs(in []string, what string) ([]string, error) {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, id := range in {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("vælg mindst ét %s", what)
	}
	if len(out) > maxAdminSelection {
		return nil, fmt.Errorf("for mange %s på én gang", what)
	}
	return out, nil
}

// slugifyAlbumTitle turns a Danish title into a URL segment.
//
// # Why the folding is explicit rather than generic
//
// `album.validSlug` accepts lowercase letters, digits and hyphens only — narrow on purpose, because the string
// is concatenated into a public URL and matched back out of one. So "Lørdag morgen" has to become
// `loerdag-morgen`, and the Danish letters need their conventional two-letter forms rather than being stripped:
// `lrdag-morgen` is not a slug anybody would have typed, and `lordag` is a different word.
//
// A generic Unicode-decomposition library would give `lordag`, which is why this is done by hand for the three
// letters that matter here.
func slugifyAlbumTitle(title string) string {
	var b strings.Builder
	lastHyphen := true // leading hyphens are suppressed

	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == 'æ':
			b.WriteString("ae")
			lastHyphen = false
		case r == 'ø':
			b.WriteString("oe")
			lastHyphen = false
		case r == 'å':
			b.WriteString("aa")
			lastHyphen = false
		default:
			// Everything else — spaces, punctuation, emoji, other alphabets — becomes a single hyphen.
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}

// newAlbumID mints an album id.
//
// A UUID, matching how every other entity this app creates is identified (`glimtfeed.go`, `patrolreport.go`) —
// and deliberately **not** the slug.
//
// The slug is the album's *address* and the id is its *identity*. The slug is frozen at creation precisely so a
// retitle cannot break a link somebody already shared, and conflating the two would leave a future decision to
// allow retitling-with-a-redirect nowhere to stand. It would also make the id carry editorial content, which is
// how an id ends up being edited.
func newAlbumID() string {
	return uuid.NewString()
}
