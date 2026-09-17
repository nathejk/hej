package glimt

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

// The read side (PRD 019 §8, task 301).
//
// # Visibility is applied here, but not decided here
//
// Every read takes a Filter, which the caller derives from internal/users.GlimtFeedFilterFor. This
// package turns that into a WHERE clause; it does not decide what the clause should be. The rows
// that come back are expected to be passed through users.MaySeeGlimt on the way out, so the SQL is
// a narrowing and the predicate is the authority. A test in internal/users asserts the two agree.
//
// Writing the clause from a struct rather than letting handlers assemble SQL is the point: a
// handler that built its own WHERE would be a second definition of who may see what, and the one
// that drifts is always the one nobody is looking at.

// Glimt is one row of the feed, with its media attached.
//
// AuthorPersonID is present on the struct because DELETE authorization and the moderation queue
// need it — but it must not reach a response. The HTTP layer projects it out in its response type
// (task 302), the same discipline person.PhoneParent gets. Do not add a JSON tag to this struct as
// a shortcut for serialising it.
type Glimt struct {
	GlimtID string
	Year    string

	AuthorPersonID string
	AuthorGroup    string

	// TeamNumber and TeamName are the hold's, frozen at creation (PRD 019 §0b). Empty number
	// is normal for crew.
	TeamNumber string
	TeamName   string

	Audience string
	Caption  string

	CreatedAt  time.Time
	MediaCount int

	// HiddenAt is nil when visible. Non-nil rows reach a caller only if they are that
	// caller's own or the caller moderates — the filter enforces it.
	HiddenAt    *time.Time
	HiddenBy    string
	ReportCount int

	Media []Media
}

// Filter is the visibility narrowing a read applies, mirroring users.GlimtFeedFilter.
//
// Duplicated as a local type rather than importing the users one, because this package may not
// import internal/... (see the package doc). The caller converts. It is three fields, and the
// conversion is in one place in cmd/api.
type Filter struct {
	// Denied means the caller may see nothing. A read with this set returns no rows without
	// touching the database — the fail-closed case must not depend on a WHERE clause being
	// assembled correctly.
	Denied bool
	// Unrestricted is a moderator: every audience, including hidden rows.
	Unrestricted bool
	// PersonID is the caller, whose own rows are always visible to them.
	PersonID string
	// Group limits `group`-scoped rows. Empty means none are visible.
	Group string
}

// where renders the visibility clause, and nothing else.
//
// Returns "0" — never "1" — for a denied filter, so a bug that lets a denied read reach SQL yields
// an empty result rather than the whole table. The clause is built with placeholders because
// PersonID and Group originate in a session and a stored row; unlike the consumer, the read side
// has a real driver to escape for it and there is no reason not to use it.
func (f Filter) where() (string, []any) {
	if f.Denied {
		return "0", nil
	}
	if f.Unrestricted {
		return "1", nil
	}

	// Own rows first in the OR chain because they are unconditional: they survive both a
	// narrower audience and a takedown.
	clauses := []string{"g.authorPersonId = ?"}
	args := []any{f.PersonID}

	visible := "g.hiddenAt IS NULL"
	audience := []string{
		fmt.Sprintf("g.audience IN (%s, %s)", "?", "?"),
	}
	args = append(args, AudienceNathejk, AudiencePublic)
	if f.Group != "" {
		audience = append(audience, "(g.audience = ? AND g.authorGroup = ?)")
		args = append(args, AudienceGroup, f.Group)
	}
	clauses = append(clauses, fmt.Sprintf("(%s AND (%s))", visible, strings.Join(audience, " OR ")))

	return "(" + strings.Join(clauses, " OR ") + ")", args
}

// Audience values, duplicated from users.GlimtAudience for the same reason Filter is.
//
// They are a wire format on three sides — the event body, the SQL column and the JSON response — so
// changing one is a data migration rather than a rename.
const (
	AudienceGroup   = "group"
	AudienceNathejk = "nathejk"
	AudiencePublic  = "public"
)

// glimtColumns is the select list, named once so the three reads cannot drift apart in what they
// select and therefore in what scanGlimt expects.
const glimtColumns = `g.glimtId, g.year, g.authorPersonId, g.authorGroup, g.teamNumber, g.teamName,
	g.audience, g.caption, g.createdAt, g.mediaCount, g.hiddenAt, g.hiddenBy, g.reportCount`

type querier struct {
	db cqrs.Reader
}

// Queries is the read API the app depends on, so handlers take an interface they can fake rather
// than a *Table they cannot.
type Queries interface {
	Feed(year string, f Filter, limit, offset int) ([]Glimt, error)
	ByHold(year, teamNumber string, f Filter, limit, offset int) ([]Glimt, error)
	Get(year, glimtID string) (Glimt, bool, error)
	Holds(year string, f Filter) ([]Hold, error)
	Moderation(year string, limit, offset int) ([]Glimt, error)
	PublicFeed(year string, notBefore time.Time, limit, offset int) ([]Glimt, error)
	Expired(year string, before time.Time, limit int) ([]Expired, error)
	RefsUsedElsewhere(year, glimtID string, refs []string) (map[string]bool, error)
	Version(year string, f Filter) (string, error)
}

// Hold is one entry in the index of holds that have posted something (PRD 019 §0a.1).
type Hold struct {
	TeamNumber string
	TeamName   string
	Group      string
	Count      int
	// LatestAt is the newest glimt in this hold's collection, so the index can be ordered by
	// recent activity rather than by number.
	LatestAt time.Time
}

// Expired is one glimt the retention sweep should remove, with the blobs to delete (task 310).
type Expired struct {
	GlimtID string
	Refs    []string
}

// Feed returns the glimt visible to the caller, newest first.
//
// Paginated, and deliberately without a total count: a count means a second full scan of the same
// filtered set, and nothing in the UI shows one. The client pages until a short page comes back.
func (q querier) Feed(year string, f Filter, limit, offset int) ([]Glimt, error) {
	if f.Denied {
		return nil, nil
	}
	clause, args := f.where()
	query := `
		SELECT ` + glimtColumns + `
		FROM glimt g
		WHERE g.year = ? AND g.deleted = 0 AND ` + clause + `
		ORDER BY g.createdAt DESC, g.glimtId DESC
		LIMIT ? OFFSET ?`

	all := append([]any{year}, args...)
	all = append(all, clampLimit(limit), clampOffset(offset))
	return q.list(query, all...)
}

// ByHold returns one hold's collection, **oldest first**.
//
// Oldest first because a race reads forward in time: the post-race browse is somebody reliving an
// evening in order, not scanning a news feed (PRD 019 §0a.1). The feed's ordering is the opposite
// and that is not an inconsistency — the two views answer different questions.
func (q querier) ByHold(year, teamNumber string, f Filter, limit, offset int) ([]Glimt, error) {
	if f.Denied {
		return nil, nil
	}
	if teamNumber == "" {
		// Guard against matching the column default, which would return every crew
		// glimt as though it were one hold's collection.
		return nil, nil
	}
	clause, args := f.where()
	query := `
		SELECT ` + glimtColumns + `
		FROM glimt g
		WHERE g.year = ? AND g.teamNumber = ? AND g.deleted = 0 AND ` + clause + `
		ORDER BY g.createdAt ASC, g.glimtId ASC
		LIMIT ? OFFSET ?`

	all := append([]any{year, teamNumber}, args...)
	all = append(all, clampLimit(limit), clampOffset(offset))
	return q.list(query, all...)
}

// Get returns one glimt, with no visibility filtering.
//
// Unfiltered on purpose, and the only read that is: it backs DELETE authorization and the media
// handler, both of which need the row in order to *decide*. Callers must pass what they get to
// users.MaySeeGlimt before serving any of it. Named Get rather than something reassuring so that a
// handler using it without a check reads as obviously incomplete.
func (q querier) Get(year, glimtID string) (Glimt, bool, error) {
	if glimtID == "" {
		return Glimt{}, false, nil
	}
	row := q.db.QueryRow(`
		SELECT `+glimtColumns+`
		FROM glimt g
		WHERE g.year = ? AND g.glimtId = ? AND g.deleted = 0`, year, glimtID)

	g, err := scanGlimt(row)
	if err == sql.ErrNoRows {
		return Glimt{}, false, nil
	}
	if err != nil {
		return Glimt{}, false, err
	}
	media, err := q.mediaFor(year, []string{g.GlimtID})
	if err != nil {
		return Glimt{}, false, err
	}
	g.Media = media[g.GlimtID]
	return g, true, nil
}

// Holds lists the holds with at least one glimt the caller may see.
//
// Filtered by the same clause as the feed, so a hold whose only glimt is invisible to this caller
// does not appear at all. Showing it with a count of zero would leak that a group posted something,
// which is a small leak and still one the caller was not meant to have.
func (q querier) Holds(year string, f Filter) ([]Hold, error) {
	if f.Denied {
		return nil, nil
	}
	clause, args := f.where()
	query := `
		SELECT g.teamNumber, MAX(g.teamName), MAX(g.authorGroup), COUNT(*), MAX(g.createdAt)
		FROM glimt g
		WHERE g.year = ? AND g.deleted = 0 AND g.teamNumber <> '' AND ` + clause + `
		GROUP BY g.teamNumber
		ORDER BY MAX(g.createdAt) DESC`

	all := append([]any{year}, args...)
	rows, err := q.db.Query(query, all...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Hold
	for rows.Next() {
		var h Hold
		var latest sql.NullTime
		if err := rows.Scan(&h.TeamNumber, &h.TeamName, &h.Group, &h.Count, &latest); err != nil {
			return nil, err
		}
		if latest.Valid {
			h.LatestAt = latest.Time
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Moderation returns every glimt, reported-and-not-yet-reviewed first.
//
// No filter parameter, because there is no narrowing to apply — but that means the *handler* is the
// only thing standing between this and every group-scoped photo in the event. It must check the
// Team-section assignment (task 300) before calling, and the check is a per-request lookup rather
// than a session claim precisely so that it can be revoked.
//
// Ordering: unreviewed reports first (reported and still visible is the urgent state), then by
// report count, then newest.
func (q querier) Moderation(year string, limit, offset int) ([]Glimt, error) {
	query := `
		SELECT ` + glimtColumns + `
		FROM glimt g
		WHERE g.year = ? AND g.deleted = 0
		ORDER BY (g.reportCount > 0 AND g.hiddenAt IS NULL) DESC,
		         g.reportCount DESC,
		         g.createdAt DESC,
		         g.glimtId DESC
		LIMIT ? OFFSET ?`
	return q.list(query, year, clampLimit(limit), clampOffset(offset))
}

// PublicFeed returns what the open web sees: public, not hidden, newer than the cutoff, newest first.
//
// Takes no Filter and no caller at all — that is the design, not an omission. The public page is on
// the same host as the app, so a logged-in member's browser will send its session cookie to it; if
// this read could be influenced by a caller, the public page would silently become a different page
// for members than for parents and "is this public-safe?" would stop being testable (PRD 019 §8).
//
// `notBefore` is the public retention window, applied here rather than by mutating rows: the audience
// is immutable and hiding is a moderation act, so neither may be repurposed by a timer (see
// cmd/api/glimtpurge.go). A zero `notBefore` means no cutoff.
func (q querier) PublicFeed(year string, notBefore time.Time, limit, offset int) ([]Glimt, error) {
	clause := ""
	args := []any{year, AudiencePublic}
	if !notBefore.IsZero() {
		clause = " AND g.createdAt >= ?"
		args = append(args, notBefore.UTC().Format("2006-01-02 15:04:05"))
	}
	args = append(args, clampLimit(limit), clampOffset(offset))

	query := `
		SELECT ` + glimtColumns + `
		FROM glimt g
		WHERE g.year = ? AND g.deleted = 0 AND g.audience = ? AND g.hiddenAt IS NULL` + clause + `
		ORDER BY g.createdAt DESC, g.glimtId DESC
		LIMIT ? OFFSET ?`
	return q.list(query, args...)
}

// Expired returns glimt created before the cutoff, with every blob ref to delete.
//
// Both the full and thumbnail refs, because forgetting the thumbnails would leave recognisable
// images on disk while technically having "purged the glimt" — the mistake the portrait purge test
// exists to catch.
func (q querier) Expired(year string, before time.Time, limit int) ([]Expired, error) {
	rows, err := q.db.Query(`
		SELECT g.glimtId,
		       GROUP_CONCAT(m.blobRef),
		       GROUP_CONCAT(NULLIF(m.thumbRef, ''))
		FROM glimt g
		LEFT JOIN glimt_media m ON m.glimtId = g.glimtId AND m.year = g.year
		WHERE g.year = ? AND g.deleted = 0 AND g.createdAt < ?
		GROUP BY g.glimtId
		ORDER BY g.createdAt ASC
		LIMIT ?`, year, before.UTC().Format("2006-01-02 15:04:05"), clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Expired
	for rows.Next() {
		var e Expired
		var full, thumbs sql.NullString
		if err := rows.Scan(&e.GlimtID, &full, &thumbs); err != nil {
			return nil, err
		}
		e.Refs = append(splitRefs(full), splitRefs(thumbs)...)
		out = append(out, e)
	}
	return out, rows.Err()
}

// RefsUsedElsewhere reports which of these refs are still referenced by a *different* glimt.
//
// # Why deletion needs this at all
//
// The blob store is content-addressed, so identical bytes are one object with one ref. That is what
// makes uploads idempotent and retries free — and it means **deleting the object for one glimt can
// blank the media of another**. Two members of the same patrulje posting the photo one of them
// AirDropped to the other is not a contrived case; nor is one member posting the same picture in two
// glimt. Without this check, deleting the second post would silently break the first, and the
// evidence would be a grey box in someone else's feed with nothing in any log.
//
// Only non-deleted rows count, and the glimt being deleted is excluded — its own rows are exactly
// the ones that are going away.
func (q querier) RefsUsedElsewhere(year, glimtID string, refs []string) (map[string]bool, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(refs)-1) + "?"
	args := make([]any, 0, len(refs)*2+2)
	args = append(args, year, glimtID)
	for _, r := range refs {
		args = append(args, r)
	}
	for _, r := range refs {
		args = append(args, r)
	}

	// Both columns, because a ref can be one glimt's full image and another's thumbnail — a
	// 320px upload is stored once and referenced as both.
	rows, err := q.db.Query(`
		SELECT m.blobRef, m.thumbRef
		FROM glimt_media m
		JOIN glimt g ON g.glimtId = m.glimtId AND g.year = m.year
		WHERE m.year = ? AND m.glimtId <> ? AND g.deleted = 0
		  AND (m.blobRef IN (`+placeholders+`) OR m.thumbRef IN (`+placeholders+`))`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wanted := map[string]bool{}
	for _, r := range refs {
		wanted[r] = true
	}
	inUse := map[string]bool{}
	for rows.Next() {
		var full, thumb string
		if err := rows.Scan(&full, &thumb); err != nil {
			return nil, err
		}
		if wanted[full] {
			inUse[full] = true
		}
		if wanted[thumb] {
			inUse[thumb] = true
		}
	}
	return inUse, rows.Err()
}

// Version returns a cheap fingerprint of what this caller's feed currently contains.
//
// Built from the count and the newest change rather than hashing the payload: the client only needs
// to know *whether* to refetch (PRD 019 §8), and hashing a page of rows to answer that would cost
// as much as sending it. Includes hidden-state changes, so a takedown invalidates the cache — a
// version that only tracked creations would leave a reported glimt in every client that already
// held it.
func (q querier) Version(year string, f Filter) (string, error) {
	if f.Denied {
		return "", nil
	}
	clause, args := f.where()
	all := append([]any{year}, args...)

	var count int
	var newest, hidden sql.NullTime
	row := q.db.QueryRow(`
		SELECT COUNT(*), MAX(g.createdAt), MAX(g.hiddenAt)
		FROM glimt g
		WHERE g.year = ? AND g.deleted = 0 AND `+clause, all...)
	if err := row.Scan(&count, &newest, &hidden); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d-%d-%d", count, nullUnix(newest), nullUnix(hidden)), nil
}

// list runs a query returning glimt rows and attaches their media in one further query.
//
// Two queries rather than a join, because a join multiplies every parent row by its media count and
// the post-race browse reads pages of ten glimt with up to ten items each. One extra round trip
// beats a hundred-row result set that has to be de-duplicated in Go.
func (q querier) list(query string, args ...any) ([]Glimt, error) {
	rows, err := q.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Glimt
	var ids []string
	var year string
	for rows.Next() {
		g, err := scanGlimt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
		ids = append(ids, g.GlimtID)
		year = g.Year
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	media, err := q.mediaFor(year, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Media = media[out[i].GlimtID]
	}
	return out, nil
}

// mediaFor loads the media for a page of glimt, keyed by glimt id and in ordinal order.
func (q querier) mediaFor(year string, ids []string) (map[string][]Media, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	args := make([]any, 0, len(ids)+1)
	args = append(args, year)
	for _, id := range ids {
		args = append(args, id)
	}

	rows, err := q.db.Query(`
		SELECT glimtId, ordinal, blobRef, thumbRef, kind, contentType, bytes, width, height, durationMs
		FROM glimt_media
		WHERE year = ? AND glimtId IN (`+placeholders+`)
		ORDER BY glimtId, ordinal`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]Media{}
	for rows.Next() {
		var id string
		var m Media
		if err := rows.Scan(&id, &m.Ordinal, &m.Ref, &m.ThumbRef, &m.Kind,
			&m.ContentType, &m.Bytes, &m.Width, &m.Height, &m.DurationMs); err != nil {
			return nil, err
		}
		out[id] = append(out[id], m)
	}
	return out, rows.Err()
}

// rowScanner is what both *sql.Row and *sql.Rows satisfy, so one scan function serves both.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanGlimt(s rowScanner) (Glimt, error) {
	var g Glimt
	var hiddenAt sql.NullTime
	var createdAt sql.NullTime
	err := s.Scan(
		&g.GlimtID, &g.Year, &g.AuthorPersonID, &g.AuthorGroup, &g.TeamNumber, &g.TeamName,
		&g.Audience, &g.Caption, &createdAt, &g.MediaCount, &hiddenAt, &g.HiddenBy, &g.ReportCount,
	)
	if err != nil {
		return Glimt{}, err
	}
	if createdAt.Valid {
		g.CreatedAt = createdAt.Time
	}
	if hiddenAt.Valid {
		t := hiddenAt.Time
		g.HiddenAt = &t
	}
	return g, nil
}

// clampLimit keeps a page size sane whatever a handler passes.
//
// A zero or negative limit becomes the default rather than "no limit": the post-race browse is a
// caller with thousands of rows behind it, and an unbounded query there is the difference between a
// slow page and a service that stops answering.
func clampLimit(limit int) int {
	const (
		def = 20
		max = 200
	)
	if limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}

func clampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func splitRefs(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return nil
	}
	parts := strings.Split(s.String, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nullUnix(t sql.NullTime) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.Unix()
}
