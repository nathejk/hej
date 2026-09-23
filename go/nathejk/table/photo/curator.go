package photo

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
)

// The curator's reads (PRD 022 §8.8, task 366).
//
// # Why this is a second interface rather than a flag on the first
//
// `Queries` above is what public handlers hold. `CuratorQueries` is the opposite in almost every respect:
// it shows drafts, removals, and photographs no album references.
//
// The cheap alternative was an `includeDeleted bool` on the existing reads. It is rejected for the reason
// `publicpatrol/table.go` states about not reading the organizers' table from a public handler: **"It is
// not here" is a property; "we do not select it" is a habit.** A public handler holding only
// `app.models.Photos` is *structurally unable* to reach a curator read, whereas a handler holding a read
// with a flag is one argument away from serving one — and the guarantee degrades from something the type
// system enforces to something a reviewer has to notice.
//
// The cost is some duplicated SQL. That is the right trade here: the duplication is visible and local,
// while the failure it prevents is a draft album on the open web.
//
// # This interface still may not carry a person
//
// The split is about *publication*, not about privacy, and the no-person rule is not relaxed by being
// behind a credential. There is no uploader, no curator, no member \u2014 and after PRD 022 §8.2 there could
// not honestly be one anyway, since the credential is shared. Task 381's structural walk covers these
// types too.

// CuratorQueries is the read API for the admin tool.
//
// Deliberately **not** embedded in or embedding `Queries`. Embedding either way would mean a handler with
// one has the other, which is exactly the boundary this split exists to draw.
type CuratorQueries interface {
	// Library returns one page of the year's photographs, newest first.
	//
	// Newest first is the only ordering offered, because it is the only one a contact sheet of a card
	// dump wants: the curator is looking for what they just uploaded. Paged because a card is hundreds of
	// photographs and a page that renders all of them at once is a page nobody can use.
	Library(year string, f Filter, limit, offset int) ([]LibraryPhoto, error)

	// Counts returns the header's numbers for the whole year, ignoring paging.
	//
	// One query rather than one per number: they are shown together, they must agree with each other, and
	// six round trips to render one line of text is how a page gets slow for no reason.
	Counts(year string) (Counts, error)

	// Photo returns one photograph including a deleted one, with its tags.
	//
	// Unlike `Queries.Get`, a deleted photograph is **found** here and reports itself deleted. A curator
	// has to be able to see what they removed \u2014 that is the whole difference between a soft delete and a
	// destructive one, and it is what an undelete would be built on (PRD 022 §11 Q6).
	Photo(year, photoID string) (LibraryPhoto, bool, error)

	// Tags returns the patrols a photograph is attributed to.
	//
	// Returns the id and the number only. The patrol's **name** is deliberately not joined here: the
	// resolution from a number to a name already exists in `publicpatrol.ByNumber`, and duplicating that
	// join would mean two places decide what a patrol is called. The handler resolves names for display.
	Tags(year, photoID string) ([]Tag, error)
}

// Filter narrows the library read.
//
// Zero value means "everything live", which is the contact sheet's default view.
//
// # Why the tri-state fields are pointers
//
// `HasLocation` and `Tagged` each have three meaningful states — yes, no, and don't care — and the
// curator's filter bar offers all three. A plain bool would collapse two of them, making "show me the
// photographs with no location" inexpressible, which is the filter the whole bulk-position workflow is
// built around.
type Filter struct {
	// InNoAlbum limits the result to photographs no live album membership references.
	//
	// The filter that makes the workflow work: after a card dump this is "what have I not sorted yet".
	InNoAlbum bool

	// HasLocation selects photographs with, or without, a coordinate. Nil means either.
	HasLocation *bool

	// Verdict limits to one bounds verdict. "" means any.
	//
	// The curator's \"out of bounds\" view is `Verdict: BoundsOutside`, and it exists because PRD 011 §6
	// requires a rejected coordinate to be *visible as rejected* rather than merely absent from the map.
	// `BoundsUnknown` is separately selectable for the reason the constants record: it is a statement
	// about us, and a curator who sees a batch of them should be told to site the checkpoints, not to
	// re-take the photographs.
	Verdict string

	// Tagged selects photographs with, or without, at least one patrol tag. Nil means either.
	Tagged *bool

	// IncludeDeleted adds soft-deleted photographs to the result.
	//
	// Off by default so the ordinary contact sheet shows what exists. On, it is how a curator finds
	// something they removed by accident.
	IncludeDeleted bool
}

// Counts is the header's summary of the year.
type Counts struct {
	// Total is every live photograph in the year.
	Total int
	// InNoAlbum is how many no live album references — the size of the unsorted pile.
	InNoAlbum int
	// WithLocation is how many carry a coordinate, of any verdict.
	WithLocation int
	// Plottable is how many would actually appear on the public map: `inside`, and nothing else.
	//
	// Separate from WithLocation on purpose. A curator who has placed forty coordinates and sees
	// "40 med position / 12 på kortet" has been told something true and actionable; one number would hide
	// twenty-eight rejections behind an encouraging total.
	Plottable int
	// OutOfBounds is how many carry a coordinate that was judged not at the event.
	OutOfBounds int
	// Unknown is how many carry a coordinate we could not judge, because there was no race area.
	Unknown int
	// Tagged is how many carry at least one patrol attribution.
	Tagged int
	// Deleted is how many the curator has removed. Shown so a soft delete is visible rather than silent.
	Deleted int
}

// LibraryPhoto is one photograph as the curator sees it.
//
// A wider type than `Photo` because the contact sheet shows what the public never does: whether the
// photograph is in an album, whether it was deleted, and how many patrols it is attributed to.
type LibraryPhoto struct {
	ID       string
	Ref      string
	ThumbRef string
	Caption  string
	Width    int
	Height   int
	Bytes    int

	// Lat/Lng are nil unless the photograph carries a usable coordinate.
	Lat *float64
	Lng *float64

	// BoundsVerdict is what the race-area check made of the coordinate. Shown to the curator, which is
	// the point of storing it rather than only acting on it.
	BoundsVerdict string

	// AlbumCount is how many live albums reference this photograph.
	//
	// A count rather than the album list: the contact sheet needs "is this sorted yet", and fetching each
	// photograph's albums to render a dot would be a query per thumbnail.
	AlbumCount int

	// TagCount is how many patrols it is attributed to. Same reasoning as AlbumCount.
	TagCount int

	// Deleted is whether the curator removed it. Only ever true when Filter.IncludeDeleted was set.
	Deleted bool

	// UploadedAt is when it entered the library, which is what the contact sheet orders by.
	UploadedAt string
}

// Tag is one patrol attribution.
//
// The id and the number, and no name — see CuratorQueries.Tags. And no person, ever: a photograph is
// attributed to a patrulje (PRD 011 §4, `.rules`).
type Tag struct {
	TeamID string
	Number string
}

// curatorQuerier is a distinct type from `querier`, not a second method set on it.
//
// If they were one type, `photo.Table` would satisfy both interfaces and `photoQueriesOrNil` could hand a
// public handler something that answers curator reads. Two types means the wiring has to choose, and the
// choice is visible in `main.go`.
type curatorQuerier struct {
	db cqrs.Reader
}

// libraryColumns is the select list both the page read and the single-photograph read use.
//
// Shared so the two cannot disagree about what a LibraryPhoto contains — they scan into the same struct,
// and a column added to one and not the other is a silent zero value.
const libraryColumns = `
	p.photoId, p.blobRef, p.thumbRef, p.caption, p.width, p.height, p.bytes,
	p.latitude, p.longitude, p.boundsVerdict, p.deleted, p.uploadedAt,
	(SELECT COUNT(*) FROM album_item i
	  WHERE i.photoId = p.photoId AND i.deleted = 0) AS albumCount,
	(SELECT COUNT(*) FROM photo_patrol t
	  WHERE t.photoId = p.photoId AND t.year = p.year AND t.deleted = 0) AS tagCount`

// where renders the filter as SQL conditions plus their arguments.
//
// Built here rather than in each read so that "live unless asked" and the year scope are applied in one
// place. The year is always first and always present: a curator read that could span years would show one
// event's photographs while writing to another's.
func (f Filter) where(year string) (string, []any) {
	conds := []string{"p.year = ?"}
	args := []any{year}

	if !f.IncludeDeleted {
		conds = append(conds, "p.deleted = 0")
	}
	if f.InNoAlbum {
		conds = append(conds,
			`NOT EXISTS (SELECT 1 FROM album_item i
			              WHERE i.photoId = p.photoId AND i.deleted = 0)`)
	}
	if f.HasLocation != nil {
		if *f.HasLocation {
			conds = append(conds, "p.latitude IS NOT NULL AND p.longitude IS NOT NULL")
		} else {
			conds = append(conds, "p.latitude IS NULL")
		}
	}
	if f.Verdict != "" {
		conds = append(conds, "p.boundsVerdict = ?")
		args = append(args, f.Verdict)
	}
	if f.Tagged != nil {
		exists := `EXISTS (SELECT 1 FROM photo_patrol t
		                    WHERE t.photoId = p.photoId AND t.year = p.year AND t.deleted = 0)`
		if *f.Tagged {
			conds = append(conds, exists)
		} else {
			conds = append(conds, "NOT "+exists)
		}
	}
	return strings.Join(conds, " AND "), args
}

// Library returns one page of the year's photographs, newest first.
func (q curatorQuerier) Library(year string, f Filter, limit, offset int) ([]LibraryPhoto, error) {
	// A read with no bound is a read that eventually returns a card dump into a template. The default is
	// applied here rather than trusted from the caller, and the ceiling is applied even when one is given.
	if limit <= 0 {
		limit = 120
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	where, args := f.where(year)
	// `photoId` breaks ties on the timestamp. Without it a page boundary falling inside a batch uploaded
	// in the same second could show one photograph twice and skip another, because MariaDB is under no
	// obligation to order equal keys consistently between two queries.
	rows, err := q.db.Query(`
		SELECT `+libraryColumns+`
		FROM photo p
		WHERE `+where+`
		ORDER BY p.uploadedAt DESC, p.photoId DESC
		LIMIT `+fmt.Sprint(limit)+` OFFSET `+fmt.Sprint(offset), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []LibraryPhoto{}
	for rows.Next() {
		p, err := scanLibraryPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Photo returns one photograph, including a deleted one.
func (q curatorQuerier) Photo(year, photoID string) (LibraryPhoto, bool, error) {
	if photoID == "" {
		return LibraryPhoto{}, false, nil
	}
	rows, err := q.db.Query(`
		SELECT `+libraryColumns+`
		FROM photo p
		WHERE p.year = ? AND p.photoId = ?`, year, photoID)
	if err != nil {
		return LibraryPhoto{}, false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return LibraryPhoto{}, false, rows.Err()
	}
	p, err := scanLibraryPhoto(rows)
	if err != nil {
		return LibraryPhoto{}, false, err
	}
	return p, true, rows.Err()
}

// scanLibraryPhoto reads one row of libraryColumns.
func scanLibraryPhoto(rows *sql.Rows) (LibraryPhoto, error) {
	var p LibraryPhoto
	var lat, lng sql.NullFloat64
	var deleted int
	if err := rows.Scan(&p.ID, &p.Ref, &p.ThumbRef, &p.Caption, &p.Width, &p.Height, &p.Bytes,
		&lat, &lng, &p.BoundsVerdict, &deleted, &p.UploadedAt,
		&p.AlbumCount, &p.TagCount); err != nil {
		return LibraryPhoto{}, err
	}
	// Both halves or neither, as everywhere else: a row with one is not a position.
	if lat.Valid && lng.Valid {
		latVal, lngVal := lat.Float64, lng.Float64
		p.Lat, p.Lng = &latVal, &lngVal
	}
	p.Deleted = deleted != 0
	return p, nil
}

// Counts returns the header's numbers.
//
// Conditional aggregates in one pass rather than eight queries. They are displayed on one line and must
// agree with each other; eight reads could each be correct about a different instant.
func (q curatorQuerier) Counts(year string) (Counts, error) {
	var c Counts
	err := q.db.QueryRow(`
		SELECT
			COALESCE(SUM(p.deleted = 0), 0),
			COALESCE(SUM(p.deleted = 0 AND NOT EXISTS (
				SELECT 1 FROM album_item i
				 WHERE i.photoId = p.photoId AND i.deleted = 0)), 0),
			COALESCE(SUM(p.deleted = 0 AND p.latitude IS NOT NULL), 0),
			COALESCE(SUM(p.deleted = 0 AND p.boundsVerdict = ?), 0),
			COALESCE(SUM(p.deleted = 0 AND p.boundsVerdict = ?), 0),
			COALESCE(SUM(p.deleted = 0 AND p.boundsVerdict = ?), 0),
			COALESCE(SUM(p.deleted = 0 AND EXISTS (
				SELECT 1 FROM photo_patrol t
				 WHERE t.photoId = p.photoId AND t.year = p.year AND t.deleted = 0)), 0),
			COALESCE(SUM(p.deleted = 1), 0)
		FROM photo p
		WHERE p.year = ?`,
		BoundsInside, BoundsOutside, BoundsUnknown, year,
	).Scan(&c.Total, &c.InNoAlbum, &c.WithLocation, &c.Plottable, &c.OutOfBounds, &c.Unknown,
		&c.Tagged, &c.Deleted)
	if err != nil {
		return Counts{}, err
	}
	return c, nil
}

// Tags returns the patrols a photograph is attributed to.
func (q curatorQuerier) Tags(year, photoID string) ([]Tag, error) {
	if photoID == "" {
		return nil, nil
	}
	rows, err := q.db.Query(`
		SELECT teamId, teamNumber
		FROM photo_patrol
		WHERE year = ? AND photoId = ? AND deleted = 0
		ORDER BY teamNumber ASC, teamId ASC`, year, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.TeamID, &t.Number); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

var _ CuratorQueries = curatorQuerier{}
