package photo

import (
	"database/sql"

	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the application.
//
// # This interface is not the privacy boundary the album one is
//
// `album.Queries`' doc comment calls its own shape the privacy boundary, because every read there is
// served to an unauthenticated page. That is emphatically **not** true here, and the difference is worth
// stating so nobody reasons from the wrong precedent in either direction.
//
// The library has no public surface at all. A photograph reaches the open web only by being referenced
// from a published album, and that resolution happens through `album`'s reads. So this interface is free
// to expose a curator's view of everything — which the richer reads task 366 adds do — without that being
// a widening of anything public.
//
// What it must still never carry is a **person**: no uploader, no curator, no member, no name. That rule
// is not about who can read this interface, it is about what the table is allowed to know, and task 381's
// structural walk enforces it here as well as on the public types.
//
// # What is here in task 363
//
// Only the two reads the projection needs to be verifiable and safe: one photograph by id, and the
// shared-blob check. The curator's filtered contact-sheet reads are task 366, and the deliberate
// staging is so that the fold and the blob-safety rule land with tests before any UI depends on them.
type Queries interface {
	// Get returns one live photograph.
	//
	// found is false for an unknown id and for a deleted one. Unlike `album.BySlug`, that conflation is
	// not a privacy measure — there is no public enumeration to prevent here — it is simply that a
	// deleted photograph is not a photograph any caller may act on. The curator's own reads (task 366)
	// may distinguish them, which is what an undelete would need (PRD 022 §11 Q6).
	Get(year, photoID string) (Photo, bool, error)

	// RefsInUse reports which of the given blob refs any live photograph still references.
	//
	// Exists for the blob-sharing hazard, and it is the read that stops a real bug: content addressing
	// means identical bytes are one object, so a library photograph, an album's photograph and a glimt
	// can all share a ref — and any one delete path would otherwise delete the others' bytes. See
	// cmd/api/glimtdelete.go and task 368.
	//
	// `excluding` names photographs whose references do not count, which is what makes the read usable
	// by the *library's own* delete path. Without it, a photograph being deleted would report its own
	// refs as in use — and since the fold is asynchronous, the row is still live at the moment the check
	// runs, so nothing would ever be deleted. Other callers pass nil: no photograph is being removed
	// there, so every live row that references the bytes is a reason to keep them.
	//
	// Both the full ref and the thumbnail count as a use: a thumbnail is as shareable as the image.
	RefsInUse(year string, excluding []string, refs []string) (map[string]bool, error)
}

// Photo is one photograph in the library.
type Photo struct {
	ID       string
	Ref      string
	ThumbRef string
	Caption  string
	Width    int
	Height   int
	Bytes    int

	// Lat/Lng are nil unless the photograph carries a usable coordinate. Nil rather than zero for the
	// reason the scan projection gives: 0,0 is the Atlantic off Ghana, and "not plottable" must be
	// distinguishable from "plotted at the equator".
	Lat *float64
	Lng *float64

	// BoundsVerdict is what the race-area check made of the coordinate. Carried through to the curator so
	// an out-of-bounds photograph is visibly rejected rather than mysteriously absent from the map.
	BoundsVerdict string
}

type querier struct {
	db cqrs.Reader
}

// Get returns one live photograph.
func (q querier) Get(year, photoID string) (Photo, bool, error) {
	// An empty id runs no query. The same rule `publicpatrol.ByNumber` follows: a read whose key is
	// empty is a caller bug, and answering it with a scan is how one becomes a table scan.
	if photoID == "" {
		return Photo{}, false, nil
	}

	rows, err := q.db.Query(`
		SELECT photoId, blobRef, thumbRef, caption, width, height, bytes,
		       latitude, longitude, boundsVerdict
		FROM photo
		WHERE year = ? AND photoId = ? AND deleted = 0`, year, photoID)
	if err != nil {
		return Photo{}, false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return Photo{}, false, rows.Err()
	}
	var p Photo
	var lat, lng sql.NullFloat64
	if err := rows.Scan(&p.ID, &p.Ref, &p.ThumbRef, &p.Caption, &p.Width, &p.Height, &p.Bytes,
		&lat, &lng, &p.BoundsVerdict); err != nil {
		return Photo{}, false, err
	}
	// Both halves or neither. A row with one is not a position, and the fold does not write one — but a
	// read that trusted a single NULL check would turn a future data bug into a marker at the equator.
	if lat.Valid && lng.Valid {
		latVal, lngVal := lat.Float64, lng.Float64
		p.Lat, p.Lng = &latVal, &lngVal
	}
	return p, true, rows.Err()
}

// RefsInUse reports which of refs any live photograph still references.
//
// # Why `deleted = 0` and not every row
//
// A soft-deleted photograph's bytes are no longer reachable from any page, so keeping them alive would
// mean a takedown never frees disk. The cost of being wrong in this direction is bounded and recoverable
// (an object deleted while a removed photograph still names it — and that photograph is already gone from
// every view), while the other direction blanks a live album.
//
// Note this is the one place PRD 022 §11 Q2's "photographs are never purged" does **not** reach. That
// resolution is about retention — we run no job that expires anything — not about a curator's deliberate
// deletion, which must still free its bytes or "take it down" would leave them on disk indefinitely.
//
// # Why the exclusion is applied in Go rather than in SQL
//
// The set is one photograph for a single deletion, or a selection's worth for a bulk one — never large.
// Building a `photoId NOT IN (...)` chain into the WHERE clause would add per-call SQL construction and a
// second placeholder-counting bug waiting to happen, to save filtering a handful of rows.
//
// Empty refs in, empty out, and no query. The same rule album's copy follows: treating an empty filter as
// "everything" is how a narrow read becomes a table scan — and here it would answer "every ref is in
// use", which would stop the purge from ever deleting anything.
func (q querier) RefsInUse(year string, excluding []string, refs []string) (map[string]bool, error) {
	inUse := map[string]bool{}
	if len(refs) == 0 {
		return inUse, nil
	}

	excluded := make(map[string]bool, len(excluding))
	for _, id := range excluding {
		excluded[id] = true
	}

	args := make([]any, 0, len(refs)*2+1)
	args = append(args, year)
	for _, ref := range refs {
		args = append(args, ref)
	}
	for _, ref := range refs {
		args = append(args, ref)
	}

	marks := placeholders(len(refs))
	rows, err := q.db.Query(`
		SELECT photoId, blobRef, thumbRef
		FROM photo
		WHERE year = ? AND deleted = 0
		  AND (blobRef IN (`+marks+`) OR thumbRef IN (`+marks+`))`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wanted := make(map[string]bool, len(refs))
	for _, ref := range refs {
		wanted[ref] = true
	}
	for rows.Next() {
		var id, full, thumb string
		if err := rows.Scan(&id, &full, &thumb); err != nil {
			return nil, err
		}
		if excluded[id] {
			continue
		}
		// Filtered against what was asked for, because a matching row carries both its refs and only one
		// of them may be the one in question.
		if wanted[full] {
			inUse[full] = true
		}
		if wanted[thumb] {
			inUse[thumb] = true
		}
	}
	return inUse, rows.Err()
}

// placeholders renders n comma-separated `?` marks.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, '?')
	}
	return string(out)
}

var _ Queries = querier{}
