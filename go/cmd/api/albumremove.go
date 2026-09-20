package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
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

	// The item is looked up before publishing, for two reasons: a removal of something that does not
	// exist should answer 404 rather than putting a no-op on an append-only log, and the refs are
	// needed to decide what bytes may go.
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
		Ordinal:   ordinal,
		Reason:    reason,
		RemovedAt: time.Now().UTC(),
	}); err != nil {
		app.writeAlbumPublishFailure(w, r, err)
		return
	}

	// Event first, bytes second — the same order `purgeOneGlimt` argues for, and for the same reason.
	// If the bytes went first and the publish then failed, the retry would come back to a row whose
	// refs are already gone, and anything sharing them would be blank permanently with no record of
	// why. Publishing first means a failure leaves the photograph in place and visible, which for a
	// takedown is the wrong direction — but it is the *recoverable* wrong direction, and a curator who
	// sees the photograph still there will press the button again.
	//
	// The item being removed is excluded from the sharing check: the fold is asynchronous, so its row
	// is still live right now and would otherwise report its own bytes as in use.
	app.purgeAlbumItemBlobs(r.Context(), albumID,
		[]album.ItemKey{{AlbumID: albumID, Ordinal: ordinal}}, refsOfAlbumItem(item))

	w.WriteHeader(http.StatusNoContent)
}

// deleteAlbumHandler takes a whole album down.
//
// @Summary      Delete an album
// @Description  Takes a whole curated album off the public site, marking it and every photograph in it removed. A soft removal, like the per-photograph one, so an accidental deletion is recoverable. Bytes are deleted only where nothing else references them. Requires the Team section, re-checked per request.
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

	// Every item's refs, gathered before the album disappears from the published read — after the
	// fold there would be no way to find out which bytes the album held without replaying the log.
	items, found, err := app.albumItems(albumID)
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

	var refs []string
	var excluding []album.ItemKey
	for _, it := range items {
		refs = append(refs, refsOfAlbumItem(it)...)
		// Every item in this album is on its way out, so none of them counts as a reason to keep bytes.
		excluding = append(excluding, album.ItemKey{AlbumID: albumID, Ordinal: it.Ordinal})
	}
	app.purgeAlbumItemBlobs(r.Context(), albumID, excluding, refs)

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

// refsOfAlbumItem returns the blob refs one item holds.
func refsOfAlbumItem(it album.Item) []string {
	var refs []string
	if it.Ref != "" {
		refs = append(refs, it.Ref)
	}
	if it.ThumbRef != "" {
		refs = append(refs, it.ThumbRef)
	}
	return refs
}

// purgeAlbumItemBlobs deletes the objects nothing else references.
//
// # The mirror of task 333's fix
//
// Task 333 stopped the glimt delete path from deleting bytes an **album** still used. This is the same
// hazard in the other direction: an album photograph may be byte-identical to a glimt a participant
// posted — content addressing makes those one object — so removing the album item must not blank the
// glimt.
//
// Both owners are asked, and a ref in use by either is kept. And as there, if either cannot be asked,
// **nothing is deleted**: leaking disk is recoverable, blanking somebody else's photograph is not, and
// it would be invisible to us.
func (app *application) purgeAlbumItemBlobs(ctx context.Context, albumID string, excluding []album.ItemKey, refs []string) {
	if len(refs) == 0 {
		return
	}

	inUse, err := app.albumRefsUsedElsewhere(excluding, refs)
	if err != nil {
		app.Logger.Error("could not check whether album media is shared; leaving the objects in place",
			"err", err, "albumId", albumID)
		return
	}

	for _, ref := range refs {
		if inUse[ref] {
			app.Logger.Debug("album media kept; something else references the same bytes",
				"albumId", albumID, "ref", ref)
			continue
		}
		r := blob.Ref(ref)
		if !r.Valid() {
			app.Logger.Error("skipping non-hash album ref during removal", "albumId", albumID, "ref", ref)
			continue
		}
		if derr := app.blobs.Delete(ctx, r); derr != nil {
			// Logged and carried on. The item is already gone from every view; what remains is
			// unreferenced bytes, which no URL can reach because the media handler needs a row.
			app.Logger.Error("deleting album media", "err", derr, "albumId", albumID, "ref", ref)
		}
	}
}

// albumRefsUsedElsewhere reports which refs anything other than the items being removed still
// references.
//
// The counterpart of `refsUsedElsewhere` in glimtdelete.go, and the same rule applies: **every owner of
// the blob store must be listed here.** Today that is other albums and the glimt.
//
// # The exclusion is not optional, and a test caught that
//
// `RefsInUse` is a year-wide question, so without naming the items being removed they would report
// their **own** refs as in use — and because the fold is asynchronous, their rows are still live at the
// moment this runs. The check would have looked correct and deleted nothing, ever. That is the kind of
// bug that never shows up as a failure, only as a disk filling quietly over years.
func (app *application) albumRefsUsedElsewhere(excluding []album.ItemKey, refs []string) (map[string]bool, error) {
	inUse := map[string]bool{}

	if app.models.Albums != nil {
		albumRefs, err := app.models.Albums.RefsInUse(app.config.eventYear, excluding, refs)
		if err != nil {
			return nil, fmt.Errorf("checking album media: %w", err)
		}
		for ref := range albumRefs {
			inUse[ref] = true
		}
	}

	// Nil when there is no database or the projection did not build: no glimt to protect, so nothing
	// this check would have found. Distinct from a *failing* glimt read, which is an error.
	if app.models.Glimt != nil {
		// "" as the excluded glimt id, because no glimt is being removed here — every glimt that
		// references these bytes is a reason to keep them.
		glimtRefs, err := app.models.Glimt.RefsUsedElsewhere(app.config.eventYear, "", refs)
		if err != nil {
			return nil, fmt.Errorf("checking glimt media: %w", err)
		}
		for ref := range glimtRefs {
			inUse[ref] = true
		}
	}
	return inUse, nil
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
