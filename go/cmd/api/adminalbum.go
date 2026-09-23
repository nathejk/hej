package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

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
}

// listAdminAlbumsHandler returns every album in the year, drafts included.
//
// @Summary      List the year's albums
// @Description  Returns every album in the configured event year, **including unpublished and deleted ones**, in curator order. That is the opposite of the public read, which shows only published albums and answers the same "not found" for unknown, unpublished and deleted so drafts cannot be enumerated — which is why this one is on the curator interface and behind the admin credential. Unpaged: a year holds three to five albums, or tens at most. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  listAdminAlbumsResponse
// @Failure      401  {object}  map[string]string  "missing or wrong admin credential"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
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
			AlbumID:     a.ID,
			Slug:        a.Slug,
			Title:       a.Title,
			Description: a.Description,
			SortOrder:   a.SortOrder,
			Published:   a.Published,
			Deleted:     a.Deleted,
			ItemCount:   a.ItemCount,
		})
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
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
// @Failure      401  {object}  map[string]string  "missing or wrong admin credential"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
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
	if title == "" {
		app.BadRequestResponse(w, r, errors.New("albummet skal have en titel"))
		return
	}
	if len([]rune(title)) > maxAdminAlbumTitle {
		app.BadRequestResponse(w, r, errors.New("titlen er for lang"))
		return
	}
	if len([]rune(in.Description)) > maxAdminAlbumDesc {
		app.BadRequestResponse(w, r, errors.New("beskrivelsen er for lang"))
		return
	}

	slug := slugifyAlbumTitle(title)
	if slug == "" {
		// A title of only punctuation or emoji. Refused with a reason rather than given a generated slug,
		// because the slug is the public address and "album-1" is not something a curator chose.
		app.BadRequestResponse(w, r,
			errors.New("titlen kan ikke bruges i en adresse — brug bogstaver eller tal"))
		return
	}

	// Deleted albums count as taking a slug. The schema makes it unique per year, so reusing a deleted one
	// would fail on the insert — and if the deletion were ever undone the two would collide. Refusing here
	// lets the tool say which it is instead of surfacing a database error.
	taken, err := app.models.AlbumCurator.SlugTaken(app.config.eventYear, slug)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if taken {
		app.BadRequestResponse(w, r,
			fmt.Errorf("adressen %q er brugt af et andet album — vælg en anden titel", slug))
		return
	}

	albumID := newAlbumID()
	// **Unpublished, unconditionally.** Not a default the caller may override: there is no field for it on the
	// request and no branch here. `album.Created` carries no `published` either (task 363), so publishing is
	// expressible only as a separate update — which is what keeps a half-assembled album off the open web.
	if err := app.publishAlbum(album.VerbCreated, albumID, album.Created{
		AlbumID:     albumID,
		Year:        app.config.eventYear,
		Slug:        slug,
		Title:       title,
		Description: in.Description,
		SortOrder:   in.SortOrder,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		app.writeAlbumPublishFailure(w, r, err)
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
// @Failure      401  {object}  map[string]string  "missing or wrong admin credential"
// @Failure      404  {object}  map[string]string  "an unknown album"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
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
	billeder := "billeder"
	if added == 1 {
		billeder = "billede"
	}
	if albums == 1 {
		return fmt.Sprintf("%d %s lagt i albummet.", added, billeder)
	}
	return fmt.Sprintf("%d %s lagt i %d album.", added, billeder, albums)
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
