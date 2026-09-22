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
// # The rule, in the order it is applied
//
//  1. **An explicit choice wins.** If `patrol_photo_cover` names a ref that still exists, that is the cover —
//     including when the photograph carries the `attention` flag. A human looked at the patrol's pictures and
//     chose this one; second-guessing that with a flag the crew set while shooting would make the choice in the
//     organizer tool mean nothing.
//  2. **Otherwise the start photograph**, newest first. The diploma's subject is the patrol at the start line,
//     which is what `diplom` printed and what the maintainer asked for.
//  3. **Otherwise any photograph**, newest first, so a patrol photographed only at the finish still gets one.
//
// Steps 2 and 3 skip anything flagged `attention`. That flag is the crew saying *"somebody should look at this"*
// — blurred, wrong team, or something that should not be published — and nobody has looked. An unreviewed
// photograph is fine to hold and wrong to put on a public certificate by default. A human can still choose it
// deliberately, which is step 1.
//
// # Why one statement rather than three reads
//
// Because "the cover, or else the newest start, or else the newest" is one ordering, and expressing it as
// ORDER BY keeps the tie-breaks in one place. A caller doing three reads would have to reimplement them, and the
// gallery will be the second caller.
func (q querier) Cover(year, teamID string) (Photo, bool, error) {
	if year == "" || teamID == "" {
		return Photo{}, false, nil
	}

	// The joined cover ref decides the first sort key: 0 for the chosen photograph, 1 for everything else.
	// `attention` then pushes unreviewed pictures behind reviewed ones, and `type='start'` ahead of other
	// categories, before capturedAt breaks the remaining ties newest-first.
	rows, err := q.db.Query(`
		SELECT p.ref, p.thumbRef, p.contentType, p.width, p.height, p.type
		FROM patrol_photo p
		LEFT JOIN patrol_photo_cover c
		  ON c.year = p.year AND c.teamId = p.teamId AND c.ref = p.ref AND c.ref <> ''
		WHERE p.year = ? AND p.teamId = ?
		ORDER BY
		  CASE WHEN c.ref IS NOT NULL THEN 0 ELSE 1 END ASC,
		  p.attention ASC,
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
