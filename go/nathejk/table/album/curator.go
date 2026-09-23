package album

import (
	"database/sql"

	"github.com/jrgensen/cqrs"
)

// The curator's album reads (PRD 022 §8.8, task 366).
//
// # Why this is a second interface rather than a flag on `Queries`
//
// `Queries` is publication-filtered in SQL and read by unauthenticated handlers; its doc comment calls that
// shape the privacy boundary, and `BySlug` returns the same "not found" for unknown, unpublished and
// deleted precisely so the open web cannot enumerate drafts.
//
// The obvious cheap alternative was an `includeUnpublished bool` on those reads. It is rejected because it
// would put every public handler **one argument** away from serving a draft album, turning a property of
// the type into a habit of the caller. The reasoning `publicpatrol/table.go` records applies exactly: *"It
// is not here" is a property; "we do not select it" is a habit.*
//
// So a handler holding only `app.models.Albums` is structurally unable to see a draft, and that is worth a
// second interface and some duplicated SQL.

// CuratorQueries is the album read API for the admin tool.
//
// Deliberately not embedding, and not embedded in, `Queries` — either direction would mean a handler with
// one has the other, which is the boundary this exists to draw.
type CuratorQueries interface {
	// All returns every album in the year, including unpublished and deleted ones, in curator order.
	//
	// Unpaged, unlike the library: PRD 011 §6 puts three to five albums on the frontpage, and even a
	// generous event has tens rather than hundreds. A page control here would be ceremony.
	All(year string) ([]CuratorAlbum, error)

	// Album returns one album by id with its items in ordinal order, including removed ones.
	//
	// By id rather than by slug, which is the opposite of the public read and deliberate: the slug is a
	// *public address*, while the curator is editing a specific row and may be looking at one whose slug
	// nobody has ever visited. Removed items are included so a curator can see what they took out.
	Album(year, albumID string) (CuratorAlbum, []CuratorItem, bool, error)

	// SlugTaken reports whether the year already has an album with this slug, deleted ones included.
	//
	// Deleted ones count. The slug is unique per year in the schema, so a create that reused a deleted
	// album's slug would fail on the insert — and, worse, if the delete were ever undone the two would
	// collide. Refusing up front lets the tool say "den slug er brugt" instead of surfacing a database
	// error.
	SlugTaken(year, slug string) (bool, error)

	// NextOrdinal returns the position a new item should take in an album.
	//
	// Needed because ordinals are chosen by the publisher, not assigned by the fold (see table.sql): there
	// is nothing in the projection that allocates one. Returns max(ordinal)+1 over **every** row including
	// removed ones, so a removed item's position is never reused — reusing it would resurrect that row's
	// soft delete via the upsert, silently putting a taken-down photograph back on the page.
	NextOrdinal(year, albumID string) (int, error)
}

// CuratorAlbum is one album as the curator sees it.
//
// Wider than the public `Album`: it carries the publication and deletion state, which the public type has
// no business knowing because the public read filters on them instead.
type CuratorAlbum struct {
	ID          string
	Slug        string
	Title       string
	Description string
	SortOrder   int

	// Published is whether it is on the public frontpage.
	Published bool
	// Deleted is whether the curator took it down.
	Deleted bool

	// ItemCount counts live items whose photograph is also live — the same definition the public count
	// uses, so the admin tool and the frontpage never disagree about how many photographs an album has.
	ItemCount int

	CreatedAt string
}

// CuratorItem is one position in an album as the curator sees it.
type CuratorItem struct {
	Ordinal int
	PhotoID string

	// Ref and ThumbRef come from the photograph, for rendering the editor's thumbnails.
	Ref      string
	ThumbRef string
	Caption  string

	// Removed is whether this position was taken out of the album.
	Removed bool
	// PhotoDeleted is whether the photograph itself was deleted from the library.
	//
	// Distinct from Removed, and the distinction is the one PRD 022 §5 insists the copy must make clear: a
	// removed item is a photograph that left *this album*, while a deleted photograph left *everywhere*.
	// A curator looking at a gap needs to know which happened, and only one of the two is fixed by
	// re-adding it here.
	PhotoDeleted bool
}

// curatorQuerier is a distinct type from `querier`, so that `album.Table` satisfying one interface does not
// hand a public handler the other. See photo/curator.go for the same note.
type curatorQuerier struct {
	db cqrs.Reader
}

// All returns every album in the year.
func (q curatorQuerier) All(year string) ([]CuratorAlbum, error) {
	rows, err := q.db.Query(`
		SELECT a.albumId, a.slug, a.title, a.description, a.sortOrder,
		       a.published, a.deleted, a.createdAt,
		       (SELECT COUNT(*) FROM album_item i
		         JOIN photo p ON p.photoId = i.photoId
		         WHERE i.albumId = a.albumId AND i.deleted = 0 AND p.deleted = 0) AS itemCount
		FROM album a
		WHERE a.year = ?
		ORDER BY a.deleted ASC, a.sortOrder ASC, a.albumId ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CuratorAlbum{}
	for rows.Next() {
		var a CuratorAlbum
		var published, deleted int
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Description, &a.SortOrder,
			&published, &deleted, &a.CreatedAt, &a.ItemCount); err != nil {
			return nil, err
		}
		a.Published = published != 0
		a.Deleted = deleted != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

// Album returns one album and its items, including removed ones.
func (q curatorQuerier) Album(year, albumID string) (CuratorAlbum, []CuratorItem, bool, error) {
	if albumID == "" {
		return CuratorAlbum{}, nil, false, nil
	}

	rows, err := q.db.Query(`
		SELECT a.albumId, a.slug, a.title, a.description, a.sortOrder,
		       a.published, a.deleted, a.createdAt,
		       (SELECT COUNT(*) FROM album_item i
		         JOIN photo p ON p.photoId = i.photoId
		         WHERE i.albumId = a.albumId AND i.deleted = 0 AND p.deleted = 0) AS itemCount
		FROM album a
		WHERE a.year = ? AND a.albumId = ?`, year, albumID)
	if err != nil {
		return CuratorAlbum{}, nil, false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return CuratorAlbum{}, nil, false, rows.Err()
	}
	var a CuratorAlbum
	var published, deleted int
	if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Description, &a.SortOrder,
		&published, &deleted, &a.CreatedAt, &a.ItemCount); err != nil {
		return CuratorAlbum{}, nil, false, err
	}
	a.Published = published != 0
	a.Deleted = deleted != 0
	if err := rows.Err(); err != nil {
		return CuratorAlbum{}, nil, false, err
	}

	items, err := q.items(year, albumID)
	if err != nil {
		return CuratorAlbum{}, nil, false, err
	}
	return a, items, true, nil
}

// items returns one album's positions, including removed ones and ones whose photograph was deleted.
//
// A LEFT JOIN, unlike the public read's inner one. The public page wants a deleted photograph to vanish; the
// curator wants to see that the position exists and that what was in it is gone, because those are two
// different things to fix.
func (q curatorQuerier) items(year, albumID string) ([]CuratorItem, error) {
	rows, err := q.db.Query(`
		SELECT i.ordinal, i.photoId, i.deleted,
		       COALESCE(p.blobRef, ''), COALESCE(p.thumbRef, ''), COALESCE(p.caption, ''),
		       COALESCE(p.deleted, 1)
		FROM album_item i
		LEFT JOIN photo p ON p.photoId = i.photoId
		WHERE i.year = ? AND i.albumId = ?
		ORDER BY i.ordinal ASC`, year, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CuratorItem{}
	for rows.Next() {
		var it CuratorItem
		var removed, photoDeleted int
		if err := rows.Scan(&it.Ordinal, &it.PhotoID, &removed,
			&it.Ref, &it.ThumbRef, &it.Caption, &photoDeleted); err != nil {
			return nil, err
		}
		it.Removed = removed != 0
		// A missing photo row COALESCEs to 1, which reads correctly: from the curator's point of view a
		// photograph that is not there and one that was deleted are the same problem.
		it.PhotoDeleted = photoDeleted != 0
		out = append(out, it)
	}
	return out, rows.Err()
}

// SlugTaken reports whether the year already uses this slug, deleted albums included.
func (q curatorQuerier) SlugTaken(year, slug string) (bool, error) {
	if slug == "" {
		return false, nil
	}
	var n int
	if err := q.db.QueryRow(`
		SELECT COUNT(*) FROM album WHERE year = ? AND slug = ?`, year, slug).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// NextOrdinal returns max(ordinal)+1 over every row, removed ones included.
func (q curatorQuerier) NextOrdinal(year, albumID string) (int, error) {
	var max sql.NullInt64
	if err := q.db.QueryRow(`
		SELECT MAX(ordinal) FROM album_item WHERE year = ? AND albumId = ?`,
		year, albumID).Scan(&max); err != nil {
		return 0, err
	}
	if !max.Valid {
		return 0, nil
	}
	return int(max.Int64) + 1, nil
}

var _ CuratorQueries = curatorQuerier{}
