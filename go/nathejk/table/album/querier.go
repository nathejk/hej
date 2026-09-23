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
//
// # Every read joins the photograph, and every join filters it
//
// After PRD 022 §8.3 an album item is a membership and the photograph is `photo`'s row, so each read here
// joins the two. The joins are **inner** and every one of them requires `photo.deleted = 0`, which is
// what makes a curator's deletion take effect everywhere at once: a deleted photograph stops satisfying
// the join and therefore vanishes from the album page, the cover, the item count and the map without any
// of those reads knowing it happened.
//
// That is why `photo`'s delete fold deliberately leaves `album_item` alone. The guarantee lives here, in
// SQL, rather than in a fold that has to remember every table — see photo/consumer.go's handleDeleted.
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
//
// # Assembled from two tables
//
// After PRD 022 §8.3 the membership is `album_item`'s and everything else is `photo`'s, so this type is
// the result of a join rather than a row. The shape is unchanged from before the split, which is why
// `cmd/api`'s album page and removal path did not have to move with it.
type Item struct {
	Ordinal int

	// PhotoID is the library photograph at this position.
	//
	// New with the split, and it is what the curator's surfaces address a photograph by — the ordinal
	// identifies a *slot in this album*, which is not the same thing and is not stable across a reorder.
	PhotoID string

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
		         JOIN photo p ON p.photoId = i.photoId
		         WHERE i.albumId = a.albumId AND i.deleted = 0 AND p.deleted = 0) AS itemCount,
		       (SELECT MIN(i.ordinal) FROM album_item i
		         JOIN photo p ON p.photoId = i.photoId
		         WHERE i.albumId = a.albumId AND i.deleted = 0 AND p.deleted = 0) AS coverOrdinal
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

// items returns one album's live items, in curator order.
//
// The join to `photo` is inner and requires the photograph to be live, which is what makes a deleted
// photograph disappear from this album without anything having told the album so. An item whose
// photograph has not been folded yet behaves identically — invisible rather than a row with nothing in it
// — and that is the correct answer for two projections folded from independent streams, where the arrival
// order of two messages is not something a page should be able to notice.
func (q querier) items(albumID string) ([]Item, error) {
	rows, err := q.db.Query(`
		SELECT i.ordinal, i.photoId, p.blobRef, p.thumbRef, p.caption, p.width, p.height,
		       p.latitude, p.longitude, p.boundsVerdict
		FROM album_item i
		JOIN photo p ON p.photoId = i.photoId
		WHERE i.albumId = ? AND i.deleted = 0 AND p.deleted = 0
		ORDER BY i.ordinal ASC`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		var it Item
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&it.Ordinal, &it.PhotoID, &it.Ref, &it.ThumbRef, &it.Caption,
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
// before the album itself is visible — and the photograph must be live, so a curator's deletion takes a
// pin off the map as well as a picture off the page.
//
// Three tables and five conditions in one statement, deliberately. Assembling this in Go would mean the
// publication gate and the verdict gate lived in different places from each other, and the whole safety
// claim of this read is that neither can be forgotten independently.
func (q querier) Plottable(year string) ([]PlottableItem, error) {
	rows, err := q.db.Query(`
		SELECT i.albumId, a.slug, i.ordinal, p.latitude, p.longitude
		FROM album_item i
		JOIN album a ON a.albumId = i.albumId
		JOIN photo p ON p.photoId = i.photoId
		WHERE i.year = ? AND i.deleted = 0
		  AND p.deleted = 0 AND p.boundsVerdict = ?
		  AND a.deleted = 0 AND a.published = 1
		  AND p.latitude IS NOT NULL AND p.longitude IS NOT NULL
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

// RefsInUse was here, and its removal is the point rather than a tidy-up (PRD 022 §8.3, tasks 364/368).
//
// It answered "does any live album item still reference these bytes?", which the glimt and album delete
// paths united with glimt's own answer before purging an object. After the library split there are no
// refs in this file to answer it with — an item holds a `photoId` — so the question moved to
// `photo.Queries.RefsInUse`.
//
// **It could not simply be reimplemented here as a join, and this is the part worth reading.** A join
// through `album_item` answers a *narrower* question: which refs are used by photographs that are in an
// album. Narrower is the dangerous direction for a purge, because the caller deletes what is reported
// unused — so a library photograph in no album would have been reported unused by this read and had its
// bytes deleted by an unrelated glimt takedown. One subsumed-looking method would have been a
// data-loss bug rather than dead code.
//
// `photo`'s version asks the library directly and therefore covers every photograph, albumed or not.
// `placeholders` went with it, since nothing else here builds an IN clause.

var _ Queries = querier{}
