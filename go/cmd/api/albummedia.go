package main

import (
	"context"
	"errors"
	"fmt"

	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/patrolphoto"
	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/publicpatrol"
	"nathejk.dk/nathejk/table/year"
)

// Album media ingest (PRD 011 §6 section 1, §8; task 333).
//
// # What is different from the glimt upload, and it is one thing
//
// Everything about storing the bytes is the glimt path's, reused wholesale: `internal/imaging` decodes
// (which *is* the validation), turns the image upright by its EXIF orientation, re-encodes to JPEG —
// **which strips all EXIF including GPS** — and `internal/blob` stores the result content-addressed
// with a thumbnail. None of that changes here, and it must not.
//
// The one addition is that the coordinate is **read before it is destroyed**, and written to a column.
//
// That is not a loophole in the stripping rule, it is the point of it. A photograph of a child must not
// carry where it was taken around inside a file that nobody has looked at (PRD 003 §6). A *curated*
// album photograph may legitimately be plotted on the public map, and the honest way to hold that fact
// is a column a curator can see, correct, bounds-check and delete. So:
//
//   - a coordinate in a column is a decision somebody made;
//   - a coordinate inside a stored file is a leak waiting to happen.
//
// **Never** change the pipeline to preserve EXIF because this feature wants a coordinate.
//
// # Why the bounds check is here and not at read time
//
// Because the race area moves. Organizers site checkpoints up to the event, so a verdict recomputed
// next week would be a different verdict for a photograph nobody touched — silently. The verdict
// recorded is the one that was true when the photograph was accepted, which is also the one a curator
// was shown.

// albumMediaPrepared is one photograph ready to be named in a photo.Uploaded event.
type albumMediaPrepared struct {
	Ref      string
	ThumbRef string

	// MediumRef is the 800px rendition (task 409), or "" if it could not be produced.
	//
	// Empty is a normal answer rather than a failure: readers fall back to the full image, which is what
	// lets this rendition exist without a backfill for everything uploaded before it.
	MediumRef string

	Width  int
	Height int
	Bytes  int

	// Location is nil unless the file carried a usable coordinate, which is the common case.
	//
	// A `*photo.Location` rather than a lat, a lng and a verdict side by side, for the reason that type's
	// doc gives at length: a coordinate and the judgement made about it are one fact, and three separate
	// fields make "moved the point, forgot the verdict" expressible. Nil rather than zero because 0,0 is a
	// real place and also what a camera with no fix writes — `imaging.ReadGPS` already refuses it, and this
	// shape is what keeps that refusal from arriving downstream as a coordinate off Ghana.
	Location *photo.Location
}

// libraryThumbEdges are the rendition sizes a **library** photograph gets, longest edge in pixels.
//
// # Why this is not `glimtThumbEdges`
//
// It was, until task 409 added the 800px rendition. PRD 023 §11 Q8 asked whether glimt should get that
// rendition too, and the answer is **not decided here**, because `glimtThumbEdges` is glimt's storage
// decision and belongs to PRD 019. Editing a shared constant would have changed a second feature's
// storage for every future upload, silently, in a diff about album pages — which is precisely the kind of
// change nobody reviews. So the constant splits and glimt keeps exactly what it had.
//
// The two are free to differ, and there is a reason to expect they will: an album photograph is opened
// full-screen in a viewer on a laptop, while a glimt is a grid tile and a phone-sized view. What they must
// not do is differ *by accident*.
//
// Recorded in PRD 023 §11 Q8 as well, because a decision that only exists in a comment is a decision the
// next person re-takes.
//
// Order matters to nothing here — renditions are looked up by name (`imaging.ThumbName`), never by index,
// exactly so that adding a size cannot silently re-point an existing one.
var libraryThumbEdges = []int{mediumEdge, glimtThumbEdges[0]}

// mediumEdge is the longest edge of the 800px rendition (task 409, PRD 023 §7.9).
//
// 800 because a 390pt phone shows about that at 2×, so the 1600px display image is four times the pixels
// for no visible gain. It is also the `800w` candidate the viewer names in its `srcset` (task 410): change
// this and that width has to change with it, or the browser is told a size the bytes do not have.
const mediumEdge = 800

// storeAlbumImage normalizes and stores one curated photograph.
//
// The ceilings and rate limits the glimt upload applies are deliberately **not** applied here. They
// exist to bound what an unaccountable participant on a phone can push into the store; an album ingest
// is an organizer action on a known set of photographs, and a limit that stopped a curator halfway
// through an album would cost more than it protects. The upload size cap stays, because a
// hundred-megabyte file is a mistake whoever sent it.
func (app *application) storeAlbumImage(ctx context.Context, year string, raw []byte) (albumMediaPrepared, error) {
	// Read first. After Prepare there is nothing left to read — which is the whole design.
	lat, lng, hasCoordinate := imaging.ReadGPS(raw)

	prepared, err := imaging.Prepare(raw, maxGlimtEdge, libraryThumbEdges, glimtJPEGQuality, false)
	if err != nil {
		if errors.Is(err, imaging.ErrNotAnImage) {
			return albumMediaPrepared{}, errGlimtNotMedia
		}
		return albumMediaPrepared{}, fmt.Errorf("prepare album media: %w", err)
	}

	ref, err := app.blobs.Put(ctx, prepared.Full.Bytes)
	if err != nil {
		return albumMediaPrepared{}, fmt.Errorf("store album media: %w", err)
	}

	// A rendition failure does not fail the ingest, for the reason the glimt path records: the page
	// falls back to the full image, so losing one costs bandwidth rather than the photograph.
	//
	// Which is also why these are PutCache: the same fallback that makes the failure survivable makes the
	// bytes worth excluding from the backup, and they are reproducible from the full rendition above.
	// That one stays an original — an admin upload keeps no separate original either (§8.5), so it is the
	// only copy of the photographer's work, and PRD 022 §11 Q2 says it is never purged.
	//
	// **By name, not by index.** `prepared.Thumbs` is in the order of `libraryThumbEdges`, so indexing it
	// would mean every reader here silently depends on that order — and adding a rendition would re-point
	// the thumbnail to whatever now sits at [0]. That is a data bug that looks like a layout bug.
	thumbRef := app.storeRendition(ctx, prepared, glimtThumbEdges[0], "thumbnail")
	mediumRef := app.storeRendition(ctx, prepared, mediumEdge, "medium")

	out := albumMediaPrepared{
		Ref:       ref.String(),
		ThumbRef:  thumbRef,
		MediumRef: mediumRef,
		Width:     prepared.Full.Width,
		Height:    prepared.Full.Height,
		Bytes:     len(prepared.Full.Bytes),
	}
	if hasCoordinate {
		// The verdict is decided here, at ingest, and travels with the coordinate. Never recomputed on
		// read — see the file header.
		out.Location = &photo.Location{
			Lat:           lat,
			Lng:           lng,
			BoundsVerdict: app.albumBoundsVerdict(year, lat, lng),
		}
	}
	return out, nil
}

// storeRendition stores one prepared rendition, identified by its edge, and returns its ref or "".
//
// Looks the rendition up **by name** rather than by position in `prepared.Thumbs`, which is the whole
// reason this is a function. `imaging.ThumbName` derives the name from the edge, so the lookup cannot
// disagree with what `Prepare` produced — whereas an index depends on the order of `libraryThumbEdges`,
// and adding a size would silently re-point every existing reader.
//
// Never returns an error: a missing rendition is a normal state that readers handle by falling back to the
// full image. `what` names it in the log so a volume or encoder problem is visible as something other than
// pages that quietly got heavier.
func (app *application) storeRendition(
	ctx context.Context, prepared imaging.Portrait, edge int, what string,
) string {
	name := imaging.ThumbName(edge)
	for _, r := range prepared.Thumbs {
		if r.Name != name || len(r.Bytes) == 0 {
			continue
		}
		// PutCache: derived from the full rendition, so outside the backup scope and rebuildable on a miss
		// (tasks 429 and 430).
		ref, err := app.blobs.PutCache(ctx, r.Bytes)
		if err != nil {
			app.Logger.Warn("storing an album rendition", "rendition", what, "edge", edge, "err", err)
			return ""
		}
		return ref.String()
	}
	// Reached when `Prepare` produced nothing at this edge — today only if the edge is not in
	// `libraryThumbEdges`, which would be a wiring mistake worth seeing in a log.
	app.Logger.Warn("no rendition was produced at this edge", "rendition", what, "edge", edge)
	return ""
}

// albumBoundsVerdict judges a coordinate against the race area.
//
// # Three outcomes, and the third is the one worth having
//
//   - `inside`  — plottable.
//   - `outside` — a real coordinate that is not in the race area: a phone with a stale fix, a
//     photograph taken at home, a mistyped edit. Kept and never plotted.
//   - `unknown` — there was a coordinate but **no area to judge it against**, because no checkpoint has
//     a position yet. Distinct from `outside` on purpose: `outside` is a statement about the
//     photograph, `unknown` is a statement about us. Folding them together would permanently
//     condemn every photograph uploaded before the course was sited, with nothing to
//     distinguish those from genuinely stray coordinates.
//
// Both non-inside verdicts keep the value. A curator has to be able to see that an item was rejected
// rather than wonder why it is absent from the map — and discarding the coordinate destroys the only
// evidence that there was ever anything to reject.
//
// # Why the bounding box and not the polygon
//
// The race area is a hull buffered by 3 km (PRD 002 §11.2), and the question here is "is this plausibly
// at the event?", not "is this on the route?". A box is the generous reading of a generous margin, and
// generous is right: the cost of admitting a photograph from a car park just outside the hull is a pin
// slightly off the edge, while the cost of rejecting one is a curator wondering why their photograph
// will not appear.
//
// Judged against **the photograph's** year's race area (task 392): a 2025 photograph placed against 2026's area
// would be rejected or accepted by the wrong boundary, and the verdict decides whether it reaches the public map.
func (app *application) albumBoundsVerdict(year string, lat, lng float64) string {
	if app.models.RaceAreas == nil {
		return photo.BoundsUnknown
	}
	area, ok, err := app.models.RaceAreas.RaceArea(year)
	if err != nil {
		// Cannot judge, so we say so rather than guessing in either direction. An error here becoming
		// `outside` would silently unplot a whole batch because of a transient database problem.
		app.Logger.Error("reading the race area for an album bounds check", "err", err)
		return photo.BoundsUnknown
	}
	if !ok {
		return photo.BoundsUnknown
	}
	if withinRaceBounds(area, lat, lng) {
		return photo.BoundsInside
	}
	return photo.BoundsOutside
}

// withinRaceBounds reports whether a coordinate is inside the area's bounding box.
//
// A pure function of the area and the point, so the rule is testable without a database — the same
// reasoning that put the race-area derivation in its own file.
func withinRaceBounds(area checkpoint.RaceArea, lat, lng float64) bool {
	return lat >= area.SouthWest.Lat && lat <= area.NorthEast.Lat &&
		lng >= area.SouthWest.Lng && lng <= area.NorthEast.Lng
}

// albumQueriesOrNil narrows the projection to its read API, or nil.
//
// The same shape as glimtQueriesOrNil and peopleOrNil, and for the same reason: a typed nil
// `*album.Table` stored in an interface field is not `== nil`, so every "is this available?" check
// would pass and then panic on the first call. Converting here is what makes those checks honest —
// and one of them is in the glimt delete path, where a panic would take out a takedown.
func albumQueriesOrNil(t *album.Table) album.Queries {
	if t == nil {
		return nil
	}
	return t
}

// albumCuratorOrNil and photoCuratorOrNil narrow the projections to the **admin tool's** read APIs, or nil
// (PRD 022 §8.8, task 366).
//
// The same typed-nil guard as the others, and one extra reason to keep them separate from
// `albumQueriesOrNil`/`photoQueriesOrNil` rather than returning both from one: these two return the
// draft-visible interfaces, and their call sites are what an auditor greps for to answer "who can see an
// unpublished album?". Two names, two greps, one answer each.
func albumCuratorOrNil(t *album.Table) album.CuratorQueries {
	if t == nil {
		return nil
	}
	return t
}

func photoCuratorOrNil(t *photo.Table) photo.CuratorQueries {
	if t == nil {
		return nil
	}
	return t
}

// photoQueriesOrNil narrows the photograph library to its read API, or nil (PRD 022, task 363).
//
// The same typed-nil guard as albumQueriesOrNil, and the consequence of getting it wrong is the same one:
// `RefsInUse` is consulted by the glimt and album delete paths, so a typed nil would pass their
// availability check and panic inside a takedown — which is the one code path in this service that must not
// fail halfway.
func photoQueriesOrNil(t *photo.Table) photo.Queries {
	if t == nil {
		return nil
	}
	return t
}

// publicPatrolQueriesOrNil narrows the projection to its read API, or nil.
//
// The same typed-nil guard, and here the consequence of getting it wrong is specific: the patrol page's
// availability check decides whether to serve or to answer not-yet, and a typed nil would pass that check
// and panic inside a request on an unauthenticated route.
func publicPatrolQueriesOrNil(t *publicpatrol.Table) publicpatrol.Queries {
	if t == nil {
		return nil
	}
	return t
}

// patrolPhotoQueriesOrNil is the same guard for the patrol photograph projection (task 361).
func patrolPhotoQueriesOrNil(t *patrolphoto.Table) patrolphoto.Queries {
	if t == nil {
		return nil
	}
	return t
}

// yearQueriesOrNil is the same guard for the event-year projection (task 357).
//
// A typed nil in an interface is not nil, which is the bug this shape exists to prevent — and here the cost of
// getting it wrong is a panic while rendering a diploma rather than a missing route line.
func yearQueriesOrNil(t *year.Table) year.Queries {
	if t == nil {
		return nil
	}
	return t
}
