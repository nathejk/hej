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

	// CoverRef is the cover photograph's display ref, CoverMediumRef its 800px rendition or "", and HasCover
	// says whether there is a cover at all.
	//
	// The cover is the curator's chosen photograph while it is a live item, otherwise the lowest live ordinal —
	// `coverOrder` (task 396). Until then it was only the lowest ordinal, on the reasoning that ordering the album
	// *was* choosing its cover; the maintainer asked for a cover that need not be the first photograph.
	//
	// # Refs rather than the ordinal this used to carry (task 461)
	//
	// The frontpage addressed its covers by **position**, which cost two things once PRD 024 let albums re-sort
	// themselves. Its media URL stopped being cacheable — task 456 had to drop `immutable` for ordinals, because
	// an ordinal's meaning moves — and the frontpage is the page a whole event opens at once on the Sunday
	// morning. And `srcset` needs to know whether the 800px rendition **exists**: naming one that does not is the
	// single thing `srcset` must never be told (see the album page's comment), and only the photograph's row can
	// say.
	//
	// Both are answered by the same widening, so the ordinal is gone rather than kept alongside: which bytes is
	// one fact, and "which slot, then which bytes" was two that could disagree.
	CoverRef       string
	CoverMediumRef string
	HasCover       bool
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

	Ref string
	// ThumbRef is the 320px grid rendition and MediumRef the 800px one (task 409). Either may be "", in
	// which case the page falls back to Ref — which is what lets a rendition ship with no backfill.
	ThumbRef  string
	MediumRef string
	Caption   string

	// Kind is "photo" or "video" (PRD 029). For a video, Ref is the 720p MP4, Thumb/Medium the poster and SdRef
	// the 480p MP4 or "". Status is "ready" for every photograph; a video is only served once it is.
	Kind       string
	Status     string
	DurationMs int
	SdRef      string

	// Credit is the photographer's credit line, or "" when there is none (task 393).
	//
	// Comes from the **photograph**, like the caption and for the same reason (PRD 022 §8.3): one place to edit,
	// so every album shows the same attribution and two copies of one fact cannot drift.
	//
	// This is the one field on the public surface that names a human being, and it is the narrow, consented
	// exception PRD 011's "names no person" claim now carries — a photographer's own credit, typed by a curator,
	// never derived from the `person` projection. See the column comment in photo/table.sql for the bounds.
	Credit string

	// CreditCrewID is the crew member credited instead of a typed name, or "" (PRD 025).
	//
	// **Not for rendering.** It is resolved to a name by `person.CreditNames` — in the *handler*, not here: this
	// package folds one stream and the person projection folds another, and a SQL join across the two would put
	// the credit's four bounds in a statement rather than in the one function that owns them. The handler merges
	// the resolved name into `Credit` before the page sees either.
	CreditCrewID string

	Width  int
	Height int

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

// coverOrder ranks an album's live items for the cover (task 396): the curator's chosen photograph first, then
// the lowest ordinal. Used with `ORDER BY … LIMIT 1` inside a subquery over `album_item i` joined to album `a`.
//
// **The one statement of the cover rule in SQL**, shared by the frontpage, the curator's list and the album
// view, so a chosen cover cannot show in one and not another. `pickCover` below is the same rule for a list
// already read into Go.
const coverOrder = `(i.photoId = a.coverPhotoId) DESC, i.ordinal ASC`

// pickCover applies coverOrder to live items already in ordinal order.
func pickCover(items []Item, chosen string) (Item, bool) {
	if len(items) == 0 {
		return Item{}, false
	}
	for _, it := range items {
		if chosen != "" && it.PhotoID == chosen {
			return it, true
		}
	}
	return items[0], true
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
	// The cover is picked once, as a photograph id, and its renditions come from a join on that — rather than one
	// correlated subquery per column. `coverOrder` stays the only statement of the rule, and the alternative was
	// three near-identical subqueries differing in their SELECT list, which is three places to get it wrong.
	rows, err := q.db.Query(`
		SELECT a.albumId, a.slug, a.title, a.description, a.sortOrder,
		       (SELECT COUNT(*) FROM album_item i
		         JOIN photo p ON p.photoId = i.photoId
		         WHERE i.albumId = a.albumId AND i.deleted = 0 AND p.deleted = 0) AS itemCount,
		       cp.blobRef, cp.mediumRef
		FROM album a
		LEFT JOIN photo cp ON cp.photoId = (
		         SELECT i.photoId FROM album_item i
		           JOIN photo p ON p.photoId = i.photoId
		           WHERE i.albumId = a.albumId AND i.deleted = 0 AND p.deleted = 0
		           ORDER BY `+coverOrder+` LIMIT 1)
		WHERE a.year = ? AND a.deleted = 0 AND a.published = 1
		ORDER BY a.sortOrder ASC, a.albumId ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Album{}
	for rows.Next() {
		var a Album
		// NULL when the album has no live items, which is why these are not strings. `mediumRef` is separately
		// nullable in effect — a photograph uploaded before that rendition existed holds "" — so an album can
		// have a cover and no medium, which is exactly the case `srcset` must not lie about.
		var cover, medium sql.NullString
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Description, &a.SortOrder,
			&a.ItemCount, &cover, &medium); err != nil {
			return nil, err
		}
		if cover.Valid && cover.String != "" {
			a.CoverRef = cover.String
			a.CoverMediumRef = medium.String
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
		SELECT albumId, slug, title, description, sortOrder, coverPhotoId
		FROM album
		WHERE year = ? AND slug = ? AND deleted = 0 AND published = 1`, year, slug)
	if err != nil {
		return Album{}, nil, false, err
	}
	defer rows.Close()

	var a Album
	var chosen string
	if !rows.Next() {
		return Album{}, nil, false, rows.Err()
	}
	if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Description, &a.SortOrder, &chosen); err != nil {
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
	if cover, ok := pickCover(items, chosen); ok {
		a.CoverRef = cover.Ref
		a.CoverMediumRef = cover.MediumRef
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
		SELECT i.ordinal, i.photoId, p.blobRef, p.thumbRef, p.mediumRef, p.caption, p.credit, p.creditCrewId,
		       p.width, p.height, p.latitude, p.longitude, p.boundsVerdict,
		       p.kind, p.status, p.durationMs, p.videoSdRef
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
		if err := rows.Scan(&it.Ordinal, &it.PhotoID, &it.Ref, &it.ThumbRef, &it.MediumRef,
			&it.Caption, &it.Credit, &it.CreditCrewID,
			&it.Width, &it.Height, &lat, &lng, &it.BoundsVerdict,
			&it.Kind, &it.Status, &it.DurationMs, &it.SdRef); err != nil {
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
