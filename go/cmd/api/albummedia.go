package main

import (
	"context"
	"errors"
	"fmt"

	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/publicpatrol"
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

// albumMediaPrepared is one photograph ready to be named in an ItemAdded event.
type albumMediaPrepared struct {
	Ref      string
	ThumbRef string
	Width    int
	Height   int
	Bytes    int

	// Lat/Lng are nil unless the file carried a usable coordinate. Nil rather than zero, because 0,0 is
	// a real place and also what a camera with no fix writes — `imaging.ReadGPS` already refuses it, and
	// pointers are what keep that refusal from arriving downstream as a coordinate off Ghana.
	Lat *float64
	Lng *float64

	// BoundsVerdict is one of album.Bounds*.
	BoundsVerdict string
}

// storeAlbumImage normalizes and stores one curated photograph.
//
// The ceilings and rate limits the glimt upload applies are deliberately **not** applied here. They
// exist to bound what an unaccountable participant on a phone can push into the store; an album ingest
// is an organizer action on a known set of photographs, and a limit that stopped a curator halfway
// through an album would cost more than it protects. The upload size cap stays, because a
// hundred-megabyte file is a mistake whoever sent it.
func (app *application) storeAlbumImage(ctx context.Context, raw []byte) (albumMediaPrepared, error) {
	// Read first. After Prepare there is nothing left to read — which is the whole design.
	lat, lng, hasCoordinate := imaging.ReadGPS(raw)

	prepared, err := imaging.Prepare(raw, maxGlimtEdge, glimtThumbEdges, glimtJPEGQuality, false)
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

	// A thumbnail failure does not fail the ingest, for the reason the glimt path records: the page
	// falls back to the full image, so losing a thumbnail costs bandwidth rather than the photograph.
	thumbRef := ""
	if len(prepared.Thumbs) > 0 && len(prepared.Thumbs[0].Bytes) > 0 {
		if tr, terr := app.blobs.Put(ctx, prepared.Thumbs[0].Bytes); terr != nil {
			app.Logger.Warn("storing album thumbnail", "err", terr)
		} else {
			thumbRef = tr.String()
		}
	}

	out := albumMediaPrepared{
		Ref:           ref.String(),
		ThumbRef:      thumbRef,
		Width:         prepared.Full.Width,
		Height:        prepared.Full.Height,
		Bytes:         len(prepared.Full.Bytes),
		BoundsVerdict: album.BoundsNone,
	}
	if hasCoordinate {
		out.Lat, out.Lng = &lat, &lng
		out.BoundsVerdict = app.albumBoundsVerdict(lat, lng)
	}
	return out, nil
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
func (app *application) albumBoundsVerdict(lat, lng float64) string {
	if app.models.RaceAreas == nil {
		return album.BoundsUnknown
	}
	area, ok, err := app.models.RaceAreas.RaceArea(app.config.eventYear)
	if err != nil {
		// Cannot judge, so we say so rather than guessing in either direction. An error here becoming
		// `outside` would silently unplot a whole batch because of a transient database problem.
		app.Logger.Error("reading the race area for an album bounds check", "err", err)
		return album.BoundsUnknown
	}
	if !ok {
		return album.BoundsUnknown
	}
	if withinRaceBounds(area, lat, lng) {
		return album.BoundsInside
	}
	return album.BoundsOutside
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
