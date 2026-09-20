package album

import (
	"database/sql"

	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the application.
//
// # The shape is the privacy boundary
//
// Everything here is read by **unauthenticated** handlers, so the discipline the checkpoint projection
// applies to positions applies here to the whole interface: there is no read that returns anything a
// public page may not show. In particular nothing here returns a curator, an uploader, or any person at
// all — an album knows who is *in* it only in the sense that a photograph does, and it does not record
// who put it there.
//
// That is a deliberate omission rather than an oversight. PRD 011 §0b.2 puts photo permission upstream
// of this feature, so there is no audit question this table is the answer to, and a `curatorPersonId`
// column would be a personal identifier on the one surface that must name no person (task 337).
type Queries interface {
	// Published returns the year's published albums in curator order, with a cover and an item count.
	//
	// Empty is a normal answer — before anybody has curated anything — and not an error.
	Published(year string) ([]Album, error)

	// BySlug returns one published album and its items, in curator order.
	//
	// found is false for an unknown slug, an unpublished album and a deleted one. **One answer for all
	// three**, deliberately: distinguishing them would let the open web enumerate drafts.
	BySlug(year, slug string) (Album, []Item, bool, error)

	// Plottable returns every item in the year that may be drawn on a map, across albums.
	//
	// Only `inside` verdicts, which is what `Plottable` means — see events.go. The read is filtered in
	// SQL rather than by the caller, so a handler cannot plot an unchecked coordinate by forgetting to
	// ask.
	Plottable(year string) ([]PlottableItem, error)

	// RefsInUse reports which of the given blob refs any live album item still references.
	//
	// Exists for the blob-sharing hazard, and it is the read that stops a real bug: content addressing
	// means identical bytes are one object, so an album photograph and a glimt can share a ref — and
	// the glimt delete path would otherwise delete the album's bytes. See cmd/api/glimtdelete.go.
	//
	// Both the full ref and the thumbnail count as a use: a thumbnail is as shareable as the image.
	RefsInUse(year string, refs []string) (map[string]bool, error)
}

// Album is one curated collection.
type Album struct {
	ID          string
	Slug        string
	Title       string
	Description string
	SortOrder   int

	// ItemCount is how many live items the album holds, for the frontpage's "12 billeder".
	ItemCount int

	// CoverOrdinal is the item shown as the cover, and HasCover says whether there is one.
	//
	// The lowest live ordinal rather than a `cover` column: a curator orders the album deliberately, so
	// the first photograph is the one they chose to open with. A separate column would be a second
	// thing to set and a second thing to get wrong for an album that has one.
	CoverOrdinal int
	HasCover     bool
}

// Item is one photograph in an album.
type Item struct {
	Ordinal  int
	Ref      string
	ThumbRef string
	Caption  string
	Width    int
	Height   int

	// Lat/Lng are nil unless the item carries a usable coordinate. Nil rather than zero for the reason
	// the scan projection gives: 0,0 is the Atlantic off Ghana, and "not plottable" must be
	// distinguishable from "plotted at the equator".
	Lat *float64
	Lng *float64

	// BoundsVerdict is what the race-area check made of the coordinate. Carried through to the curator
	// so an out-of-bounds item is visibly rejected rather than mysteriously absent from the map.
	BoundsVerdict string
}

// PlottableItem is one photograph the map may draw, with enough to address its bytes.
//
// A separate, narrower type from Item on purpose: the map endpoint is public and needs a position, an
// album and an ordinal — not a caption, not dimensions, and nothing that would grow into "the item, but
// on the map". A wider type here is how a future edit puts a curator's note on a public marker.
type PlottableItem struct {
	AlbumID   string
	AlbumSlug string
	Ordinal   int
	Lat       float64
	Lng       float64
}

type querier struct {
	db cqrs.Reader
}

// Published returns the year's published albums in curator order.
//
// The cover and the count come from a correlated subquery rather than a join with a GROUP BY: there are
// three to five albums, so this is a handful of index lookups, and the alternative has to cope with an
// album that has no items at all — which is a normal state while one is being assembled, and the state
// a GROUP BY quietly drops.
func (q querier) Published(year string) ([]Album, error) {
	rows, err := q.db.Query(`
		SELECT a.albumId, a.slug, a.title, a.description, a.sortOrder,
		       (SELECT COUNT(*) FROM album_item i
		         WHERE i.albumId = a.albumId AND i.deleted = 0) AS itemCount,
		       (SELECT MIN(i.ordinal) FROM album_item i
		         WHERE i.albumId = a.albumId AND i.deleted = 0) AS coverOrdinal
		FROM album a
		WHERE a.year = ? AND a.deleted = 0 AND a.published = 1
		ORDER BY a.sortOrder ASC, a.albumId ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Album{}
	for rows.Next() {
		var a Album
		// NULL when the album has no live items, which is why this is not an int.
		var cover sql.NullInt64
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Description, &a.SortOrder,
			&a.ItemCount, &cover); err != nil {
			return nil, err
		}
		if cover.Valid {
			a.CoverOrdinal = int(cover.Int64)
			a.HasCover = true
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// BySlug returns one published album and its items.
//
// Two queries rather than one join: an album's items are a list, and a join would return the album's
// columns once per photograph for a page that shows them once. The album is fetched first so an unknown
// slug costs one lookup rather than a scan of items belonging to nothing.
func (q querier) BySlug(year, slug string) (Album, []Item, bool, error) {
	rows, err := q.db.Query(`
		SELECT albumId, slug, title, description, sortOrder
		FROM album
		WHERE year = ? AND slug = ? AND deleted = 0 AND published = 1`, year, slug)
	if err != nil {
		return Album{}, nil, false, err
	}
	defer rows.Close()

	var a Album
	if !rows.Next() {
		return Album{}, nil, false, rows.Err()
	}
	if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Description, &a.SortOrder); err != nil {
		return Album{}, nil, false, err
	}
	if err := rows.Err(); err != nil {
		return Album{}, nil, false, err
	}

	items, err := q.items(a.ID)
	if err != nil {
		return Album{}, nil, false, err
	}
	a.ItemCount = len(items)
	if len(items) > 0 {
		a.CoverOrdinal = items[0].Ordinal
		a.HasCover = true
	}
	return a, items, true, nil
}

func (q querier) items(albumID string) ([]Item, error) {
	rows, err := q.db.Query(`
		SELECT ordinal, blobRef, thumbRef, caption, width, height, latitude, longitude,
		       boundsVerdict
		FROM album_item
		WHERE albumId = ? AND deleted = 0
		ORDER BY ordinal ASC`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		var it Item
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&it.Ordinal, &it.Ref, &it.ThumbRef, &it.Caption,
			&it.Width, &it.Height, &lat, &lng, &it.BoundsVerdict); err != nil {
			return nil, err
		}
		// Both halves or neither. A row with one is not a position, and the fold does not write one —
		// but a read that trusted a single NULL check would turn a future data bug into a marker at
		// the equator.
		if lat.Valid && lng.Valid {
			latVal, lngVal := lat.Float64, lng.Float64
			it.Lat, it.Lng = &latVal, &lngVal
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Plottable returns every drawable item in the year.
//
// The `boundsVerdict = 'inside'` filter is in the SQL and not in the caller, which is the point: a
// handler cannot plot an unchecked or out-of-bounds coordinate by forgetting a condition. The album
// must also be published and undeleted — an unpublished album's photographs must not appear on the map
// before the album itself is visible.
func (q querier) Plottable(year string) ([]PlottableItem, error) {
	rows, err := q.db.Query(`
		SELECT i.albumId, a.slug, i.ordinal, i.latitude, i.longitude
		FROM album_item i
		JOIN album a ON a.albumId = i.albumId
		WHERE i.year = ? AND i.deleted = 0 AND i.boundsVerdict = ?
		  AND a.deleted = 0 AND a.published = 1
		  AND i.latitude IS NOT NULL AND i.longitude IS NOT NULL
		ORDER BY i.albumId ASC, i.ordinal ASC`, year, BoundsInside)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PlottableItem{}
	for rows.Next() {
		var p PlottableItem
		if err := rows.Scan(&p.AlbumID, &p.AlbumSlug, &p.Ordinal, &p.Lat, &p.Lng); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RefsInUse reports which of refs any live album item still references.
//
// # Why `deleted = 0` and not every row
//
// A soft-deleted item's bytes are no longer reachable from any page, so keeping them alive would mean
// a takedown never frees disk. The cost of being wrong in this direction is bounded and recoverable
// (an object deleted while a removed item still names it — and that item's photograph is already gone
// from every view), while the other direction blanks a live album.
//
// Empty in, empty out, and no query. The same rule the checkpoint projection's bounded reads follow:
// treating an empty filter as "everything" is how a narrow read becomes a table scan — and here it
// would answer "every ref is in use", which would stop the purge from ever deleting anything.
func (q querier) RefsInUse(year string, refs []string) (map[string]bool, error) {
	inUse := map[string]bool{}
	if len(refs) == 0 {
		return inUse, nil
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
		SELECT blobRef, thumbRef
		FROM album_item
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
		var full, thumb string
		if err := rows.Scan(&full, &thumb); err != nil {
			return nil, err
		}
		// Filtered against what was asked for, because a matching row carries both its refs and only
		// one of them may be the one in question.
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
