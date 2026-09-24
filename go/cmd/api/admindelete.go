package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
)

// The curator's two removals (PRD 022 §5, §6, task 379).
//
// # These are different acts, and the copy is the substance of this task
//
// PRD 022 §5 and §7 both single this out, and the reason is worth stating rather than assuming:
//
//   - **Removing a photograph from an album** takes it out of *that album*. It stays in the library and in every
//     other album it belongs to. Reversible from the contact sheet in a few seconds.
//   - **Deleting a photograph from the library** takes it out of *everything* — every album, every public read —
//     and frees its bytes unless something else still holds them. Recoverable only by a maintainer replaying the
//     log.
//
// *One of those two is the thing an organizer means when they say "take it down."* If the two buttons read alike,
// the wrong one gets pressed under exactly the pressure that makes it matter — a parent on the phone, a curator
// who has been sorting photographs for three hours. So the Danish sentences below are written rather than
// generated, the destructive one is visually distinct, and the endpoints are separate.
//
// # Both are soft
//
// The removal honours somebody's objection, and an accidental one must be recoverable without republishing a
// photograph that was taken down on purpose. That reasoning is already in `album/table.sql` and it applies twice
// over here, because PRD 022 §11 Q2 resolved that photographs are **never purged automatically** — so a curator's
// delete is now the *only* way a photograph ever leaves.
//
// v1 ships no undelete button (§11 Q6). If a curator deletes forty photographs with a mis-aimed "select all", the
// only recovery is a maintainer. That is a choice being made, not an oversight, and it is why the confirmation
// says how many.

// removeAdminAlbumItemHandler takes one photograph out of one album.
//
// @Summary      Remove a photograph from an album
// @Description  Takes one photograph out of **this album only**. It stays in the library and in every other album it belongs to, and none of its bytes are deleted — this is not a takedown. A soft removal, so the photograph can be put back from the contact sheet and lands in its original position when it is. Addressed by photograph rather than by position, because a position is a slot in one album while the photograph is what a curator selected. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        albumId  path      string  true  "album id"
// @Param        photoId  path      string  true  "library photograph id"
// @Success      204  "removed"
// @Failure      400  {object}  map[string]string  "a missing id"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "unknown album, or the photograph is not in it"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums or the event stream are unavailable"
// @Router       /admin/albums/{albumId}/items/{photoId} [delete]
func (app *application) removeAdminAlbumItemHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	albumID := strings.TrimSpace(params.ByName("albumId"))
	photoID := strings.TrimSpace(params.ByName("photoId"))
	if albumID == "" || photoID == "" {
		app.BadRequestResponse(w, r, errors.New("både album og billede skal angives"))
		return
	}

	// The event is keyed on the **ordinal**, because that is what `album_item` is keyed on — but the curator
	// selected a photograph, not a slot. Resolved here rather than asking the client to know the ordinal: a stale
	// ordinal in a browser would remove whatever now occupies that position, which is the wrong photograph.
	_, items, found, err := app.models.AlbumCurator.Album(adminYear(r), albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	ordinal := -1
	for _, it := range items {
		if it.PhotoID == photoID && !it.Removed {
			ordinal = it.Ordinal
			break
		}
	}
	if ordinal < 0 {
		// Either not in this album, or already removed. One answer for both: a second removal is not an error
		// worth a different status, and the curator's view is the same either way.
		app.NotFoundResponse(w, r)
		return
	}

	// The event carries the **photograph**, and the ordinal alongside it for the record (task 386). The fold
	// matches on the photograph: a position identifies a membership only until the album is reordered, and a
	// removal keyed on the slot removes whoever moved into it.
	//
	// The ordinal is still resolved here because `album_item`'s primary key is `(albumId, ordinal)` and the
	// event is the log's record of what happened — but nothing reads it except the pre-386 fallback.
	if perr := app.publishAlbum(adminYear(r), album.VerbItemRemoved, albumID, album.ItemRemoved{
		AlbumID:   albumID,
		Year:      adminYear(r),
		PhotoID:   photoID,
		Ordinal:   ordinal,
		RemovedAt: time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	// **No blob purge.** The photograph outlives the album (PRD 022 §8.3), so freeing its bytes here would blank
	// it in the library and in every other album — the exact bug the old sharing check existed to prevent,
	// reintroduced from the other end. See the note in albumremove.go.
	app.Logger.Info("admin removed a photograph from an album",
		"albumId", albumID, "photoId", photoID, "ordinal", ordinal, "ip", clientIP(r))

	w.WriteHeader(http.StatusNoContent)
}

// deleteAdminPhotoRequest carries the optional reason.
type deleteAdminPhotoRequest struct {
	// Reason is the curator's note. Recorded on the event, never shown publicly, and the one durable record of
	// *why* a photograph was taken down — which is the question anyone auditing a takedown actually asks.
	Reason string `json:"reason,omitempty"`
}

// deleteAdminPhotoHandler removes one photograph from the library.
//
// @Summary      Delete a photograph from the library
// @Description  Removes one photograph from the year's library. It disappears from **every** album and every public read, and its bytes are deleted unless another library photograph or a glimt still references the same content — content addressing means identical bytes are one object, so an organizer's photograph and a participant's glimt can be the same file. A soft removal: the row survives so the deletion is recoverable from the event log, and so a re-upload of the same bytes cannot silently undo it. This is **not** the same as removing a photograph from an album, which leaves it in the library; this is the one that means "take it down". Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        photoId  path      string                   true   "library photograph id"
// @Param        request  body      deleteAdminPhotoRequest  false  "optional reason"
// @Success      204  "deleted"
// @Failure      400  {object}  map[string]string  "a missing id or an over-long reason"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "unknown photograph"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the library or the event stream are unavailable"
// @Router       /admin/photos/{photoId} [delete]
func (app *application) deleteAdminPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return
	}

	photoID := strings.TrimSpace(httprouter.ParamsFromContext(r.Context()).ByName("photoId"))
	if photoID == "" {
		app.BadRequestResponse(w, r, errors.New("der skal angives et billede"))
		return
	}

	var in deleteAdminPhotoRequest
	if r.ContentLength > 0 {
		if err := app.ReadJSON(w, r, &in); err != nil {
			app.BadRequestResponse(w, r, err)
			return
		}
	}
	if len([]rune(in.Reason)) > maxAdminRemovalReason {
		app.BadRequestResponse(w, r, errors.New("begrundelsen er for lang"))
		return
	}

	// Looked up first, for two reasons: a delete of something that does not exist should answer 404 rather than
	// appending a no-op to a log that is never rewritten, and the refs have to be captured **before** the fold
	// runs — afterwards there is no way to learn which bytes the photograph held without replaying.
	p, found, err := app.models.PhotoCurator.Photo(adminYear(r), photoID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}
	if p.Deleted {
		// Already gone. 204 rather than 404: a curator clicking twice, or two curators acting on the same
		// selection, should not meet an error for reaching the state they wanted.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	refs := make([]string, 0, 2)
	if p.Ref != "" {
		refs = append(refs, p.Ref)
	}
	if p.ThumbRef != "" {
		// A thumbnail is as shareable as the image, so it is checked and purged on the same terms.
		refs = append(refs, p.ThumbRef)
	}

	subject, serr := photo.Subject(adminYear(r), photoID, photo.VerbDeleted)
	if serr != nil {
		app.BadRequestResponse(w, r, serr)
		return
	}
	if perr := app.commands.Publish(subject, photo.Deleted{
		PhotoID:   photoID,
		Year:      adminYear(r),
		Reason:    in.Reason,
		DeletedAt: time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	// Event first, bytes second — the order `purgeOneGlimt` argues for and for the same reason. If the bytes went
	// first and the publish then failed, a retry would find a row whose objects are already gone, and anything
	// sharing them would be blank permanently with no record of why. Publishing first means a failure leaves the
	// photograph visible, which for a takedown is the wrong direction — but it is the *recoverable* wrong
	// direction, and a curator who still sees it will press the button again.
	//
	// The photograph being deleted is excluded from the sharing check: the fold is asynchronous, so its row is
	// still live right now and would otherwise report its own bytes as in use — and nothing would ever be freed.
	app.purgeBlobs(r.Context(), "photo", photoID,
		blobOwnerExclusions{PhotoIDs: []string{photoID}}, refs)

	app.Logger.Info("admin deleted a photograph from the library",
		"photoId", photoID, "reason", in.Reason, "ip", clientIP(r))

	w.WriteHeader(http.StatusNoContent)
}

// maxAdminRemovalReason bounds the curator's note.
//
// The same number `albumremove.go` uses for its own reason field, deliberately: two limits on the same kind of
// text would eventually differ, and a curator meeting one of them would not know which surface they were on.
const maxAdminRemovalReason = 500
