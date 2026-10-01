package photo

import (
	"database/sql"
	"strings"

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
	//
	// **Every ref column on this table must be named here.** A column this query does not ask about has two
	// possible outcomes and both are bad: bytes orphaned on disk forever, or — worse — a live object deleted
	// because nothing claimed it. Neither shows up in a test that only exercises uploads. Task 368's
	// reasoning, and the reason task 409 called this out before adding `mediumRef`.
	//
	// Since PRD 027 that includes `originalRef`, and it is the column where the second outcome stops being
	// recoverable: a rendition deleted in error can be rebuilt (task 430), while the photographer's file cannot be
	// produced again from anything. The implementation now derives the select list and the scan from one column
	// list rather than repeating it, so "every ref column" is enforced by construction instead of by this sentence.
	RefsInUse(year string, excluding []string, refs []string) (map[string]bool, error)
}

// Photo is one photograph in the library.
type Photo struct {
	ID  string
	Ref string
	// ThumbRef and MediumRef may be "", in which case readers serve Ref. See table.sql.
	ThumbRef  string
	MediumRef string
	Caption   string
	Width     int
	Height    int
	Bytes     int

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
		SELECT photoId, blobRef, thumbRef, mediumRef, caption, width, height, bytes,
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
	if err := rows.Scan(&p.ID, &p.Ref, &p.ThumbRef, &p.MediumRef, &p.Caption, &p.Width, &p.Height, &p.Bytes,
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

	// **One list drives everything**: the WHERE clause, the argument arithmetic, the select list and the scan.
	//
	// It used to drive only the first two, with the `SELECT` and the `Scan` written out by hand — and the comment
	// here warned that the arithmetic "silently breaks when a column is added", which was true of the half it
	// covered and not of the half it did not. Adding `originalRef` for PRD 027 meant touching four places that had to
	// agree, in a function whose own doc says a missed column either orphans bytes forever or **deletes a live
	// object because nothing claimed it**. So the duplication is gone rather than extended.
	//
	// `originalRef` is the newest and the one with the most at stake: it is the photographer's file, the only copy of
	// it, and it is what a library takedown has to be able to free (PRD 027 R8).
	columns := []string{"blobRef", "thumbRef", "mediumRef", "originalRef"}

	args := make([]any, 0, len(refs)*len(columns)+1)
	args = append(args, year)
	for range columns {
		for _, ref := range refs {
			args = append(args, ref)
		}
	}

	marks := placeholders(len(refs))
	clauses := make([]string, 0, len(columns))
	for _, col := range columns {
		clauses = append(clauses, col+" IN ("+marks+")")
	}

	rows, err := q.db.Query(`
		SELECT photoId, `+strings.Join(columns, ", ")+`
		FROM photo
		WHERE year = ? AND deleted = 0
		  AND (`+strings.Join(clauses, " OR ")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wanted := make(map[string]bool, len(refs))
	for _, ref := range refs {
		wanted[ref] = true
	}
	for rows.Next() {
		var id string
		// Sized from `columns` for the same reason the clause is: a scan with one target too few is a runtime error
		// inside a delete path, and one too many cannot happen at all.
		found := make([]string, len(columns))
		targets := make([]any, 0, len(columns)+1)
		targets = append(targets, &id)
		for i := range found {
			targets = append(targets, &found[i])
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		if excluded[id] {
			continue
		}
		// Filtered against what was asked for, because a matching row carries all of its refs and only one
		// of them may be the one in question.
		for _, ref := range found {
			if wanted[ref] {
				inUse[ref] = true
			}
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
