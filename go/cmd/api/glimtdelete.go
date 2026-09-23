package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/commands"
	"nathejk.dk/nathejk/table/glimt"
)

// Deleting a glimt (PRD 019 §6, task 306).
//
// # Only the author, and that is the whole shape of it
//
// Not another member of the same hold, and **not the Team section**. Moderators hide (task 308);
// authors destroy. The asymmetry is deliberate and is what makes hiding cheap enough to be the
// default response to a report: a takedown that could not be undone would push a moderator towards
// waiting for a human, and waiting is what the public scope cannot afford. Meanwhile the person who
// took the photograph is the only one who gets to make it stop existing.
//
// Ownership is the stored `authorPersonId`, not the session, so a delete still works after a profile
// switch (PRD 012) and a glimt never becomes unowned.

// deleteGlimtHandler destroys the caller's own glimt and its media.
//
// @Summary      Delete your own glimt
// @Description  Publishes a deletion and purges the stored media. Only the author may do this — not other members of their hold, and not the Team section, which hides rather than deletes. Idempotent: deleting an already-deleted glimt succeeds. Media objects shared with another glimt are left in place, because storage is content-addressed and identical bytes are one object.
// @Tags         glimt
// @Produce      json
// @Param        glimtId  path      string  true  "glimt id"
// @Success      204  "deleted"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not yours"
// @Failure      404  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /glimt/items/{glimtId} [delete]
func (app *application) deleteGlimtHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, "glimt er ikke tilgængelige lige nu")
		return
	}

	glimtID := httprouter.ParamsFromContext(r.Context()).ByName("glimtId")
	g, found, err := app.models.Glimt.Get(app.config.eventYear, glimtID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		// Already gone, or never existed. 204 rather than 404 for the already-gone case is
		// tempting but they are indistinguishable here — the row is tombstoned either way —
		// and 404 is the honest answer to "delete this thing I cannot see".
		app.NotFoundResponse(w, r)
		return
	}

	if g.AuthorPersonID == "" || g.AuthorPersonID != s.UserID {
		// 403 and not 404: the caller can see this glimt (it is in their feed), so
		// pretending it does not exist would be a lie they can immediately disprove. Note
		// this refuses moderators too, deliberately.
		app.ForbiddenResponse(w, r)
		return
	}

	refs := glimtRefsOf(g)

	// Published **before** the blobs are deleted, which is the opposite order from the
	// retention sweep (portraitpurge.go) and deliberately so. The two have different failure
	// modes to protect against:
	//
	//   - The sweep is a background job that retries next tick, so deleting bytes first and
	//     failing to publish costs nothing: the row still points at missing objects, which
	//     PRD 008 §8 already requires to degrade to "no photo".
	//   - This is a member tapping "Slet" and watching. If the bytes went first and the
	//     publish failed, they would be told the delete failed while their glimt sat in
	//     everyone's feed with grey boxes where the photos were. Publishing first means the
	//     worst case is unreferenced bytes on disk — invisible, and unreachable, since the
	//     media handler cannot serve an object without a row pointing at it.
	subject, serr := glimt.Subject(app.config.eventYear, glimtID, glimt.VerbDeleted)
	if serr != nil {
		app.ServerErrorResponse(w, r, serr)
		return
	}
	if perr := app.commands.Publish(subject, glimt.Deleted{
		GlimtID:   glimtID,
		Year:      app.config.eventYear,
		Refs:      refs,
		DeletedAt: time.Now().UTC(),
	}); perr != nil {
		if errors.Is(perr, commands.ErrNoPublisher) {
			app.ServiceUnavailableResponse(w, r, "kunne ikke slette glimtet, prøv igen")
			return
		}
		app.ServerErrorResponse(w, r, perr)
		return
	}

	// Best effort, and after the fact. A failure here leaks disk space, not privacy.
	app.purgeGlimtBlobs(r.Context(), glimtID, refs)

	w.WriteHeader(http.StatusNoContent)
}

// glimtRefsOf collects every object a glimt references — full images and thumbnails.
//
// Both, because forgetting the thumbnails would leave recognisable images on disk while technically
// having deleted the glimt. That is the mistake the portrait purge test exists to catch, and it is
// worse here: a 320px thumbnail of a child's face is still a photograph of a child's face.
func glimtRefsOf(g glimt.Glimt) []string {
	refs := make([]string, 0, len(g.Media)*2)
	for _, m := range g.Media {
		if m.Ref != "" {
			refs = append(refs, m.Ref)
		}
		if m.ThumbRef != "" {
			refs = append(refs, m.ThumbRef)
		}
	}
	return refs
}

// purgeGlimtBlobs deletes the objects that nothing surviving still references.
//
// # The trap this exists for
//
// The store is content-addressed, so identical bytes are **one object with one ref**. Deleting it for
// one glimt therefore blanks the media of everything else that happens to reference it. That is
// not a contrived scenario: two members of a patrulje posting the same photo (one AirDropped it to
// the other) produce the same hash, and so does one member posting the same picture twice. Without
// the check below, deleting the newer post would silently break the older one, and the only evidence
// would be a grey box in somebody else's feed.
//
// # Two owners now, not one (PRD 011, task 333)
//
// The check originally asked only the other *glimt*. That was complete when glimt were the only thing
// in the blob store, and it stopped being complete the moment curated albums arrived: an organizer
// building an album out of a photograph a participant also posted publicly is **the expected
// workflow**, not an edge case, and it produces one object with two owners.
//
// Left alone, retention would eventually expire the glimt and delete bytes the album still shows —
// blanking a public page months later, with nothing to connect the two events. So both tables are
// asked, and a ref in use by either is kept.
//
// The lesson generalises and is worth stating for whoever adds the third owner: **anything that stores
// a blob must be added here.** There is no way for this function to discover a new owner on its own,
// which is why the sources are listed explicitly below rather than hidden behind one helper.
//
// A ref still in use is skipped and logged at debug — that is a normal outcome, not a fault.
//
// # This now delegates, and that is the point
//
// The union of owners and the "any error means nothing is deleted" rule live in `blobpurge.go`, shared with the
// curator's library delete (task 379). PRD 022 §8.9: *"Two doors is correct; the two disagreeing about blob
// purging is not"* — and two copies disagree not by being written differently but by one of them not being
// updated when a third owner of the blob store appears.
func (app *application) purgeGlimtBlobs(ctx context.Context, glimtID string, refs []string) {
	// No photograph is being deleted here, so every live library photograph that references these bytes is a
	// reason to keep them.
	app.purgeBlobs(ctx, "glimt", glimtID, blobOwnerExclusions{GlimtID: glimtID}, refs)
}

// refsUsedElsewhere reports which refs anything other than this glimt still references.
//
// A thin wrapper over the shared `blobRefsInUse` (blobpurge.go), kept because the glimt tests name it and because
// it reads better at its call site than the general form does.
//
// The reasoning that used to live here moved with the logic, and the part worth repeating is why the **library**
// is asked rather than the albums: before PRD 022 an album item carried the refs, and after the split an
// album-side check answers the narrower question "which refs are used by photographs that are *in an album*" — so
// a photograph nobody had curated yet would be reported unused and a glimt takedown would delete a
// photographer's bytes. See the note where `album.RefsInUse` used to be, in album/querier.go.
func (app *application) refsUsedElsewhere(glimtID string, refs []string) (map[string]bool, error) {
	return app.blobRefsInUse(blobOwnerExclusions{GlimtID: glimtID}, refs)
}
