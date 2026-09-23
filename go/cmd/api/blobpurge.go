package main

import (
	"context"
	"fmt"

	"nathejk.dk/internal/blob"
)

// The shared blob purge (PRD 022 §8.9, tasks 368 and 379).
//
// # Why this is one file and one pair of functions
//
// Two paths delete photographs' bytes: a glimt takedown (`glimtdelete.go`) and a curator deleting from the
// library (`admindelete.go`). PRD 022 Â§8.9 states the requirement plainly — *"Two doors is correct; the two
// disagreeing about blob purging is not"* — and the way two copies disagree is not by being written differently
// but by one of them not being updated when a **third owner** of the blob store appears.
//
// So the union of owners is asked in exactly one place: `blobRefsInUse`. Adding an owner is one edit there, and
// every delete path gets it.
//
// # The rule that matters more than the mechanism
//
// **If any owner cannot be asked, nothing is deleted.** Not "assume unused", which is the tempting reading of a
// failed query — that is how a database hiccup in one feature becomes permanent data loss in another. Leaking
// disk is recoverable and visible in a graph; blanking somebody else's photograph is neither.
//
// This matters because content addressing makes sharing the *expected* case, not a corner one: an organizer
// curating an album from a photograph a participant also posted publicly produces one object with two owners, and
// PRD 011 §0b.2 describes that as the workflow rather than an accident.

// blobOwnerExclusions names the rows whose references must not count in a purge check.
//
// # Why an exclusion is required rather than optional
//
// The fold is asynchronous. At the moment a delete path asks "is anything still using these bytes?", the row it
// has just published a removal for is **still live in the projection** — so without naming it, it reports its own
// refs as in use and nothing is ever deleted. The check would look entirely correct and quietly never free a byte.
//
// A test caught exactly that once on the album path (task 335), which is why this is a named type rather than a
// pair of bare arguments somebody could pass in the wrong order.
type blobOwnerExclusions struct {
	// GlimtID is the glimt being deleted, or "" when none is.
	GlimtID string
	// PhotoIDs are the library photographs being deleted, or nil when none are.
	PhotoIDs []string
}

// blobRefsInUse reports which of refs any owner of the blob store still references, ignoring the exclusions.
//
// # Every owner of the blob store must be listed here
//
// One function asking each owner in turn, because there is no way to discover an owner automatically and a missed
// one means deleted bytes on somebody else's live page. Today there are two:
//
//   - **glimt** — what participants shared (PRD 019);
//   - **the photograph library** — what our photographers handed in (PRD 022). This covers album photographs
//     too, and covers them *better* than asking the album side would: a photograph in no album still owns its
//     bytes, and an album-side check would have reported it unused. See the note where `album.RefsInUse` used to
//     be, in album/querier.go.
//
// **Add the next owner here**, and add a test like `TestGlimtDeleteKeepsBytesAnAlbumStillUses` with it.
//
// # Nil and error are not the same answer
//
// A nil projection means that owner has nothing to protect — no database, no rows — so there is nothing this
// check would have found and the purge may proceed. A *failing* read means we cannot tell, and the whole answer
// is an error. Conflating them is how a database-free run never frees disk, or a transient failure deletes a live
// photograph.
func (app *application) blobRefsInUse(exclude blobOwnerExclusions, refs []string) (map[string]bool, error) {
	inUse := map[string]bool{}
	if len(refs) == 0 {
		return inUse, nil
	}

	if app.models.Glimt != nil {
		glimtRefs, err := app.models.Glimt.RefsUsedElsewhere(app.config.eventYear, exclude.GlimtID, refs)
		if err != nil {
			return nil, fmt.Errorf("checking glimt media: %w", err)
		}
		for ref := range glimtRefs {
			inUse[ref] = true
		}
	}

	if app.models.Photos != nil {
		photoRefs, err := app.models.Photos.RefsInUse(app.config.eventYear, exclude.PhotoIDs, refs)
		if err != nil {
			return nil, fmt.Errorf("checking library photographs: %w", err)
		}
		for ref := range photoRefs {
			inUse[ref] = true
		}
	}

	return inUse, nil
}

// purgeBlobs deletes the objects no owner still references.
//
// `what` and `id` are for the log only, so a purge line says which feature freed which bytes — with a shared
// credential the log is the only audit trail there is (PRD 022 §8.2), and "some bytes were deleted" is not one.
//
// Best-effort by design: every failure is logged and the loop continues. What remains after a failed delete is
// unreferenced bytes, which no URL can reach because every media handler needs a row to find them (see
// `glimtmediaserve.go` and `adminlibrary.go`). The photograph is already gone from every view either way.
func (app *application) purgeBlobs(ctx context.Context, what, id string, exclude blobOwnerExclusions, refs []string) {
	if len(refs) == 0 {
		return
	}

	inUse, err := app.blobRefsInUse(exclude, refs)
	if err != nil {
		// Cannot tell whether these are shared, so nothing is deleted. See the file header: this is the direction
		// to fail in.
		app.Logger.Error("could not check whether media is shared; leaving the objects in place",
			"err", err, "what", what, "id", id)
		return
	}

	for _, ref := range refs {
		if inUse[ref] {
			// A normal outcome, not a fault: content addressing means sharing is the expected case.
			app.Logger.Debug("media kept; something else references the same bytes",
				"what", what, "id", id, "ref", ref)
			continue
		}
		r := blob.Ref(ref)
		if !r.Valid() {
			// Not a hash, so not something this store put there. Skipped rather than passed to Delete, which is
			// the one place a bad ref could become a filesystem path.
			app.Logger.Error("skipping non-hash ref during purge", "what", what, "id", id, "ref", ref)
			continue
		}
		if derr := app.blobs.Delete(ctx, r); derr != nil {
			app.Logger.Error("deleting media", "err", derr, "what", what, "id", id, "ref", ref)
		}
	}
}
