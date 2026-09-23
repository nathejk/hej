package main

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/nathejk/table/album"
)

// Curator removal of a photograph or an album (PRD 011 §0b.2, §6; task 335).
//
// # Why this exists when there is no rights model
//
// Permission to publish the album photographs was obtained upstream — "if we do not have permission to
// show a picture it will not be there" (maintainer, 2026-09-19) — so this feature builds no consent
// register, no per-subject flag and no approval state. The safety property is *the set*.
//
// What that decision still owes is the ability to take something back out, because **permission is
// withdrawable and mistakes happen**. That is this file. It is the one operation on this surface whose
// effect is irreversible in the sense that matters: once a photograph is down, somebody's objection has
// been honoured, and putting it back should require the same deliberate act as publishing it did.
//
// # Soft, not destructive
//
// The event marks the row rather than deleting it, following `checkgroup`, `checkpoint` and `person`.
// The reason here is specific rather than habitual: an accidental removal must be recoverable, and a
// destructive delete would make "put it back" require re-uploading bytes we deliberately destroyed.
//
// # The blob hazard, in the other direction
//
// Task 333 fixed the glimt delete path so it does not delete bytes an album still uses. This is the
// mirror image and it is just as real: a curated album photograph may be the same bytes as a glimt a
// participant posted, so removing the album item must not delete the glimt's photograph either. Same
// union check, same failure rule — if we cannot tell, nothing is deleted.

// removeAlbumItemRequest is the optional note a curator leaves.
type removeAlbumItemRequest struct {
	// Reason is recorded on the event, not shown anywhere. It is the audit trail behind a takedown,
	// which is the question anyone reviewing one will actually ask: *why did this photograph go?*
	Reason string `json:"reason"`
}

// maxAlbumRemovalReason bounds the note.
const maxAlbumRemovalReason = 500

// removeAlbumItemHandler takes one photograph out of an album.
//
// # Authorization is the Team section, reusing Glimt's moderation check
//
// Not a new role and not a new assignment. `isGlimtModerator` already answers "does this member
// currently hold the Team section?", it is a per-request lookup rather than a session claim (so
// revoking the assignment revokes access), and the people who moderate photographs in the app are the
// people who should be able to take one off the public page. A second, parallel notion of "curator"
// would be a second thing to assign and a second thing to get wrong.
//
// This is deliberately narrower than PRD 011 §11 Q5, which is still open about where curation *lives*.
// Removal cannot wait for that answer, because it is the safety valve.
//
// @Summary      Remove one photograph from an album
// @Description  Takes one photograph out of a curated album. A soft removal: the row survives so an accidental takedown is recoverable, and the bytes are deleted only if nothing else — no other album item, no glimt — references them, since the store is content-addressed. Requires the Team section, re-checked per request. The photograph leaves every public surface within the page cache window (60s).
// @Tags         public-site
// @Accept       json
// @Produce      json
// @Param        albumId  path      string                   true   "album id"
// @Param        ordinal  path      int                      true   "position within the album"
// @Param        request  body      removeAlbumItemRequest   false  "optional reason"
// @Success      204  "removed"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not a Team-section member"
// @Failure      404  {object}  map[string]string  "unknown album or ordinal"
// @Failure      503  {object}  map[string]string  "albums or the event stream are unavailable"
// @Router       /albums/{albumId}/items/{ordinal} [delete]
func (app *application) removeAlbumItemHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if !app.isGlimtModerator(s.UserID) {
		app.ForbiddenResponse(w, r)
		return
	}
	if app.models.Albums == nil {
		app.ServiceUnavailableResponse(w, r, "billederne er ikke tilgængelige lige nu")
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	albumID := params.ByName("albumId")
	ordinal, err := strconv.Atoi(params.ByName("ordinal"))
	if err != nil {
		app.NotFoundResponse(w, r)
		return
	}

	reason, ok := app.readRemovalReason(w, r)
	if !ok {
		return
	}

	// Looked up first so that an album or an ordinal that does not exist answers 404 rather than putting
	// a no-op on an append-only log.
	//
	// The item's **photo id** is what the event carries (task 386): this route is addressed by position, because
	// that is what the page's markup has, but a position only identifies a membership until the album is
	// reordered. Resolving it here means the log records which photograph was taken down rather than which slot
	// was occupied at the time.
	//
	// Before PRD 022 the item's refs also decided what bytes could go; now no bytes go at all (see the note
	// below).
	item, found, err := app.albumItemAt(albumID, ordinal)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	subject, err := album.Subject(app.config.eventYear, albumID, album.VerbItemRemoved)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if err := app.commands.Publish(subject, album.ItemRemoved{
		AlbumID:   albumID,
		Year:      app.config.eventYear,
		PhotoID:   item.PhotoID,
		Ordinal:   ordinal,
		Reason:    reason,
		RemovedAt: time.Now().UTC(),
	}); err != nil {
		app.writeAlbumPublishFailure(w, r, err)
		return
	}

	// Event first, and — new with PRD 022 — that is the *whole* operation. See the note on
	// albumItemRemovalKeepsTheBytes below for why no blobs are purged here any more.

	w.WriteHeader(http.StatusNoContent)
}

// # Removing an item no longer deletes bytes, and that is a behaviour change worth stating
//
// Before PRD 022, `album_item` *was* the photograph: taking it out of the album was the only way that
// photograph existed, so the removal was a takedown and it purged the objects nothing else referenced.
//
// After the library split (§8.3) removing an item takes one photograph out of **one album** and leaves it
// in the library and in every other album it belongs to. Purging its bytes would blank it everywhere else
// — the exact bug the old code's sharing check existed to prevent, reintroduced by a stale assumption
// rather than by a missing check.
//
// Deleting the *photograph* is the act that frees bytes, and it lives in the library's own delete path
// (task 379) where `photo.RefsInUse` can ask the question properly. A curator has both actions and PRD 022
// §5 requires the copy to make the difference obvious, because one of the two is what an organizer means
// when they say "take it down".
//
// The consequence accepted here: an item removed from every album leaves its bytes on disk until somebody
// deletes the photograph. That is a photograph the curator still has in the library, so the bytes are not
// orphaned — they are simply not published. PRD 022 §11 Q2 settled that we do not expire photographs, so
// there is nothing for a cleanup to do either.

// deleteAlbumHandler takes a whole album down.
//
// @Summary      Delete an album
// @Description  Takes a whole curated album off the public site, marking it and every membership in it removed. A soft removal, like the per-photograph one, so an accidental deletion is recoverable. The photographs themselves stay in the library and keep their bytes — deleting an album is not deleting its photographs. Requires the Team section, re-checked per request.
// @Tags         public-site
// @Accept       json
// @Produce      json
// @Param        albumId  path      string                   true   "album id"
// @Param        request  body      removeAlbumItemRequest   false  "optional reason"
// @Success      204  "deleted"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not a Team-section member"
// @Failure      404  {object}  map[string]string  "unknown album"
// @Failure      503  {object}  map[string]string  "albums or the event stream are unavailable"
// @Router       /albums/{albumId} [delete]
func (app *application) deleteAlbumHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if !app.isGlimtModerator(s.UserID) {
		app.ForbiddenResponse(w, r)
		return
	}
	if app.models.Albums == nil {
		app.ServiceUnavailableResponse(w, r, "billederne er ikke tilgængelige lige nu")
		return
	}

	albumID := httprouter.ParamsFromContext(r.Context()).ByName("albumId")

	reason, ok := app.readRemovalReason(w, r)
	if !ok {
		return
	}

	// Looked up first so an unknown album answers 404 rather than appending a no-op to the log. Before
	// PRD 022 this also gathered every item's refs before the album vanished from the published read;
	// the photographs now outlive the album, so there is nothing to gather.
	_, found, err := app.albumItems(albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	subject, err := album.Subject(app.config.eventYear, albumID, album.VerbDeleted)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if err := app.commands.Publish(subject, album.Deleted{
		AlbumID:   albumID,
		Year:      app.config.eventYear,
		Reason:    reason,
		DeletedAt: time.Now().UTC(),
	}); err != nil {
		app.writeAlbumPublishFailure(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// readRemovalReason reads and bounds the optional note.
func (app *application) readRemovalReason(w http.ResponseWriter, r *http.Request) (string, bool) {
	var in removeAlbumItemRequest
	if r.ContentLength > 0 {
		if err := app.ReadJSON(w, r, &in); err != nil {
			app.BadRequestResponse(w, r, err)
			return "", false
		}
	}
	if len([]rune(in.Reason)) > maxAlbumRemovalReason {
		app.BadRequestResponse(w, r, errors.New("begrundelsen er for lang"))
		return "", false
	}
	return in.Reason, true
}

// albumItemAt finds one item of a published album.
//
// Goes through the published read for the same reason the media route does: there is one place the
// publication filter lives, and a second lookup would be a second place to forget it. The consequence
// worth naming is that an **unpublished** album's items cannot be removed through this endpoint — which
// is acceptable, because an unpublished album is not on the public web and the whole album can be
// deleted instead.
func (app *application) albumItemAt(albumID string, ordinal int) (album.Item, bool, error) {
	items, found, err := app.albumItems(albumID)
	if err != nil || !found {
		return album.Item{}, false, err
	}
	for _, it := range items {
		if it.Ordinal == ordinal {
			return it, true, nil
		}
	}
	return album.Item{}, false, nil
}

// albumItems returns a published album's items, by id.
func (app *application) albumItems(albumID string) ([]album.Item, bool, error) {
	published, err := app.models.Albums.Published(app.config.eventYear)
	if err != nil {
		return nil, false, err
	}
	for _, a := range published {
		if a.ID != albumID {
			continue
		}
		_, items, found, err := app.models.Albums.BySlug(app.config.eventYear, a.Slug)
		return items, found, err
	}
	return nil, false, nil
}
