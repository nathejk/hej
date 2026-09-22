package patrolphoto

import (
	"database/sql"

	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the application.
type Queries interface {
	// Cover returns the photograph that represents a patrol, and whether there is one.
	//
	// One read for both callers — the diploma now, the gallery shortly — so they cannot disagree about which
	// picture is a patrol's.
	Cover(year, teamID string) (Photo, bool, error)
}

// Photo is one photograph, as this app may use it.
//
// No original, no source URL, no uploader: the table has no such columns (see table.sql), so this type could not
// carry them even if a handler asked.
type Photo struct {
	// Ref is the display image's content hash — the key in this app's blob store once the bytes are fetched.
	Ref string

	// ThumbRef is the smallest rendition, or empty. A caller wanting a small image must fall back to Ref.
	ThumbRef string

	ContentType string
	Width       int
	Height      int

	// Type is the camera app's category: "start", "finish", …
	Type string
}

type querier struct {
	db cqrs.Reader
}

// Cover picks the photograph that represents a patrol.
//
// # A flagged photograph is never used, and never fetched
//
// The maintainer's rule (2026-09-22): *"if attention flag is raised, then skip photo, do not download"*. So
// `attention` is a **filter, not a preference** — flagged rows are excluded by the WHERE clause, which means their
// ref never leaves this package, and `internal/photobytes` is never asked for their bytes. "Do not download" is
// therefore a property of the query rather than a rule a caller has to remember.
//
// This overrides how it worked when the projection landed a day earlier, where an explicit cover selection won
// even if the photograph was flagged, on the grounds that a human had chosen it. The maintainer's rule is the
// safer one and the simpler one: the flag is the crew saying *"somebody should look at this"* — blurred, wrong
// team, or something that should not be published — and a public certificate is the wrong place to find out what
// they meant. A flagged cover falls through to the next unflagged candidate, so the patrol still gets a picture
// where one exists.
//
// # What is left, in the order it is applied
//
//  1. **An explicit choice wins**, among unflagged photographs: if `patrol_photo_cover` names a ref that still
//     exists and is not flagged, that is the cover.
//  2. **Otherwise the newest `start` photograph.** The diploma's subject is the patrol at the start line, which is
//     what `diplom` printed and what the maintainer asked for.
//  3. **Otherwise any photograph**, newest first, so a patrol photographed only at the finish still gets one.
//     (Observed in 2026's data, the finish type is spelled `maal`.)
//
// # Why one statement rather than three reads
//
// Because that is one ordering, and expressing it as ORDER BY keeps the tie-breaks in one place. A caller doing
// three reads would have to reimplement them, and the gallery will be the second caller.
func (q querier) Cover(year, teamID string) (Photo, bool, error) {
	if year == "" || teamID == "" {
		return Photo{}, false, nil
	}

	// The joined cover ref decides the first sort key: 0 for the chosen photograph, 1 for everything else. Then
	// `type='start'` ahead of other categories, before capturedAt breaks the remaining ties newest-first. There is
	// no `attention` sort key, because flagged rows are not in the result at all.
	rows, err := q.db.Query(`
		SELECT p.ref, p.thumbRef, p.contentType, p.width, p.height, p.type
		FROM patrol_photo p
		LEFT JOIN patrol_photo_cover c
		  ON c.year = p.year AND c.teamId = p.teamId AND c.ref = p.ref AND c.ref <> ''
		WHERE p.year = ? AND p.teamId = ? AND p.attention = 0
		ORDER BY
		  CASE WHEN c.ref IS NOT NULL THEN 0 ELSE 1 END ASC,
		  CASE WHEN p.type = 'start' THEN 0 ELSE 1 END ASC,
		  p.capturedAt DESC,
		  p.ref ASC
		LIMIT 1`, year, teamID)
	if err != nil {
		return Photo{}, false, err
	}
	defer rows.Close()

	if !rows.Next() {
		// No photographs. Normal: not every patrol is photographed, and the diploma omits the picture rather
		// than failing.
		return Photo{}, false, rows.Err()
	}

	var p Photo
	var thumb, contentType sql.NullString
	if err := rows.Scan(&p.Ref, &thumb, &contentType, &p.Width, &p.Height, &p.Type); err != nil {
		return Photo{}, false, err
	}
	p.ThumbRef = thumb.String
	p.ContentType = contentType.String
	return p, true, rows.Err()
}

var _ Queries = querier{}
