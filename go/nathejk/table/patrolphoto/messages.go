package patrolphoto

import "time"

// The events this projection folds, as a reader needs them.
//
// Owned by **foto** (hq's `nathejk/table/photo` and `nathejk/table/photocover` hold the authoritative copies).
// Only the fields this app reads are declared; JSON unmarshalling ignores the rest.
//
// Two fields are deliberately *not* declared, and their absence is the point:
//
//   - `original` — the uploaded file, which keeps its EXIF and may carry the location a child was photographed
//     in. foto never serves it. A struct with no field for it cannot pass it on.
//   - `source` — where foto fetched the bytes from. Provenance is foto's record and hq's; a URL into a camera
//     app's web server is not something this app should hold, let alone render.

// photographed is `NATHEJK.<year>.patrulje.<teamId>.photographed`.
type photographed struct {
	TeamID      string `json:"teamId"`
	Year        string `json:"year"`
	Type        string `json:"type,omitempty"`
	Attention   bool   `json:"attention,omitempty"`
	Ref         string `json:"ref"`
	ContentType string `json:"contentType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`

	// Renditions may be empty for a photograph recorded before rendition sets existed. The fold degrades to
	// `ref` rather than treating that as broken.
	Renditions []rendition `json:"renditions,omitempty"`

	CapturedAt time.Time `json:"capturedAt"`
}

// rendition is one cached smaller version.
type rendition struct {
	Name        string `json:"name"`
	Ref         string `json:"ref"`
	ContentType string `json:"contentType"`
	Bytes       int    `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

// photoPurged is `NATHEJK.<year>.patrulje.<teamId>.photopurged`.
//
// Its own verb rather than a photographed with an empty ref, so a replay can tell a deletion from a malformed
// message — which matters here more than usual: the thing being deleted is a picture of a minor, and somebody
// auditing that deletion needs the log to say so.
type photoPurged struct {
	TeamID string   `json:"teamId"`
	Year   string   `json:"year"`
	Refs   []string `json:"refs"`
}

// coverSelected is `NATHEJK.<year>.patrulje.<teamId>.photocoverselected`.
//
// An empty Ref clears the choice, and is stored as such rather than deleted — see table.sql.
type coverSelected struct {
	TeamID     string    `json:"teamId"`
	Year       string    `json:"year"`
	Ref        string    `json:"ref"`
	SelectedAt time.Time `json:"selectedAt"`
}

// smallestRendition returns the ref of the smallest rendition, or "" when there is none worth recording.
//
// By pixel width rather than byte size: the column exists so a grid can ask for a small image, and a JPEG that
// happens to compress badly is still the small one. A rendition with an invalid ref is skipped rather than
// stored — an unusable ref in a column a gallery reads is worse than an empty one, which has a defined fallback.
func smallestRendition(rs []rendition) string {
	best := ""
	bestWidth := 0
	for _, r := range rs {
		if !validRef(r.Ref) || r.Width <= 0 {
			continue
		}
		if best == "" || r.Width < bestWidth {
			best, bestWidth = r.Ref, r.Width
		}
	}
	return best
}

// validRef reports whether a ref looks like a content hash from a blob store: 64 lowercase hex characters.
//
// Duplicated from `internal/blob`'s Ref.Valid rather than called, because packages under nathejk/table may not
// import internal/... — the same rule and the same duplication hq documents.
//
// This is not cosmetic. A ref arrives in an event body from outside the process, is interpolated into SQL here,
// and later appears in a URL path and a filesystem lookup. `../../etc/passwd` is a ref-shaped string. Uppercase
// is rejected too: the stores produce lowercase, so an uppercase ref is not something we wrote, and two
// spellings of one hash would defeat the primary key that makes this projection idempotent.
func validRef(ref string) bool {
	if len(ref) != 64 {
		return false
	}
	for _, c := range ref {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
