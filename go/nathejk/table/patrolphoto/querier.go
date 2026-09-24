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

	// Teams lists the year's patrols that Cover has a photograph for, by patrol number.
	//
	// The admin tool's "diploma photographs into an album" action walks this (task 397); a patrol whose
	// Fototilladelse records a refusal is not listed, for the reason Cover returns nothing for it.
	Teams(year string) ([]Team, error)

	// Refused lists the year's patrols whose Fototilladelse records a refusal.
	Refused(year string) ([]string, error)
}

// Team is one patrol with a usable photograph.
type Team struct {
	TeamID string
	// Number is the patrol's number from `public_patrol`, or "" when it has not been given one.
	Number string
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
// # A refused patrol's photographs are never used, and never fetched
//
// When hq's Fototilladelse says the patrol, or anyone on it, refused (`patrol_photo_consent.refused`), no
// photograph of it is returned — the refusal is in the WHERE clause, so the ref never leaves this package and
// `internal/photobytes` is never asked for the bytes. "Never served" is a property of the query rather than a rule
// a caller has to remember.
//
// The crew's `attention` flag used to be that filter (2026-09-22). It no longer is: consent is the rule
// (2026-09-24), and a flagged photograph is a candidate like any other.
//
// # What is left, in the order it is applied
//
//  1. **An explicit choice wins**: if `patrol_photo_cover` names a ref that still exists, that is the cover.
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
	// no consent sort key, because a refused patrol's rows are not in the result at all.
	rows, err := q.db.Query(`
		SELECT p.ref, p.thumbRef, p.contentType, p.width, p.height, p.type
		FROM patrol_photo p
		LEFT JOIN patrol_photo_cover c
		  ON c.year = p.year AND c.teamId = p.teamId AND c.ref = p.ref AND c.ref <> ''
		LEFT JOIN patrol_photo_consent k
		  ON k.year = p.year AND k.teamId = p.teamId
		WHERE p.year = ? AND p.teamId = ? AND COALESCE(k.refused, 0) = 0
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

// Teams lists the patrols Cover would answer for.
//
// The same two conditions as Cover's WHERE clause — a photograph in the year, and no refusal — so the two cannot
// disagree about which patrols have a picture. The number is joined from `public_patrol` rather than asked of it,
// because that projection deliberately has no list read (see publicpatrol.Queries) and this is a curator's read.
func (q querier) Teams(year string) ([]Team, error) {
	if year == "" {
		return nil, nil
	}
	rows, err := q.db.Query(`
		SELECT p.teamId, COALESCE(MIN(pp.teamNumber), '')
		FROM patrol_photo p
		LEFT JOIN patrol_photo_consent k
		  ON k.year = p.year AND k.teamId = p.teamId
		LEFT JOIN public_patrol pp
		  ON pp.year = p.year AND pp.teamId = p.teamId
		WHERE p.year = ? AND COALESCE(k.refused, 0) = 0
		GROUP BY p.teamId
		ORDER BY CAST(COALESCE(MIN(pp.teamNumber), '') AS UNSIGNED) ASC, p.teamId ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Team
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.TeamID, &t.Number); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Refused lists the team ids whose newest photo consent decision is a refusal.
func (q querier) Refused(year string) ([]string, error) {
	if year == "" {
		return nil, nil
	}
	rows, err := q.db.Query(`
		SELECT teamId FROM patrol_photo_consent
		WHERE year = ? AND refused = 1
		ORDER BY teamId ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

var _ Queries = querier{}
