package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/eventtime"
	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/photo"
)

// Downloading a whole album as one zip file.
//
// # Why this exists
//
// A curator who has assembled an album has produced a *selection* — an edit of a card dump that nothing else in
// the system records. Handing that selection to somebody outside the tool (a local paper, a parents' evening, the
// archive) previously meant right-clicking every photograph in order and renaming the lot by hand, which is both
// the work the album already did and the step where the order is lost.
//
// So the order is the point, and it is carried **twice**: the entries are written in the album's own sequence,
// and each name is prefixed with that sequence. Zip readers are free to list entries in any order they like —
// most sort by name — so a prefix is the only way the album's arrangement survives being unpacked.
//
// # Streamed, not assembled
//
// A hundred full-size photographs is several hundred megabytes, so nothing is buffered: the archive is written
// straight to the response as each object is read. That has one consequence worth stating plainly, because it
// cannot be designed away — the status line is sent before the first object is opened, so a read that fails
// halfway through cannot become a 500. It is logged and the connection is cut, leaving the client with a
// truncated file its zip reader will refuse. The alternative is buffering the whole archive in memory to be able
// to change our minds about the status code, which is a worse trade for a tool with three users.

// adminAlbumZipStored keeps the archive uncompressed.
//
// JPEG is already entropy-coded: deflating it saves a percent or two and costs CPU proportional to the whole
// album. `zip.Store` is the honest setting for a container of compressed images — the zip exists to make the
// selection one file in one order, not to make it smaller.
const adminAlbumZipStored = zip.Store

// largeEdge is the longest edge of the «large» download (1200px).
//
// Unlike `mediumEdge` this is **not** a stored rendition: nothing in the library holds a 1200px copy, so this size
// is rendered from the display image at download time. It is therefore a number that costs CPU rather than disk,
// which is the right way round for something three curators ask for a few times a year — and it is why there is no
// backfill to go with it.
//
// 1200 sits between the 800px rendition (a phone's screen) and the 1600px display image (a laptop's): big enough
// for a printed programme or a press e-mail, small enough to attach.
const largeEdge = 1200

// adminZipSizes are the sizes the download is offered in, and the only ones.
//
// # Three scales and one original
//
// `medium`, `large` and `xlarge` are **scales**: derived, metadata-stripped, named after their longest edge. `xlarge`
// is the 1600px display image, which is the largest *rendition* the archive holds — it is deliberately not called an
// original, because for years it was the only copy and calling it one trained everybody to expect the wrong thing from
// it.
//
// `original` is the photographer's file (PRD 027): full resolution, **metadata intact**. It is the only entry here that
// is not a rendition, and the only one that carries the camera's GPS coordinate — which is why this endpoint is behind
// `requireAdmin` and why `originalboundary_test.go` fails if any reader surface so much as names the variant.
//
// An edge of 0 means "stream what is stored, untouched". `xlarge` and `original` share that, and they are still two
// entries rather than one with a flag, because they read two different columns and one of them may be absent.
var adminZipSizes = map[string]struct {
	edge  int
	label string
	// original selects the photographer's file instead of a rendition.
	original bool
}{
	"medium":   {edge: mediumEdge, label: "800px"},
	"large":    {edge: largeEdge, label: "1200px"},
	"xlarge":   {edge: 0, label: "1600px"},
	"original": {edge: 0, label: "original", original: true},
}

// downloadAdminAlbumZipHandler streams an album's photographs as a zip file, in the album's order.
//
// @Summary      Download an album as a zip file
// @Description  Streams every live photograph of one album as a single zip file, in the album's own order. Entry names are prefixed with that order (`001-…`), because a zip reader is free to list entries alphabetically and the prefix is what makes the arrangement survive unpacking. `size` picks what is sent: `medium` (800px, the stored rendition), `large` (1200px, rendered on demand), `xlarge` (1600px, the stored display image — the default) or `original` (the photographer's own file, full resolution, **metadata intact including the camera's GPS coordinate**). A photograph with no stored original falls back to `xlarge`, which is its most original surviving form — that is every photograph uploaded before PRD 027. A photograph already smaller than a requested edge is sent as stored rather than re-encoded. The download is named `<slug>-<yyyymmdd>.zip`, dated in the event's timezone. Stored uncompressed — a JPEG does not deflate. Positions a curator removed, and photographs deleted from the library, are not included. The ids come out of the album projection and only then become blob refs, so this cannot be used to address arbitrary objects in the store. Requires the admin credential.
// @Tags         admin
// @Produce      application/zip
// @Param        albumId  path      string  true   "album id"
// @Param        size     query     string  false  "medium (800px), large (1200px), xlarge (1600px, the default) or original (the photographer's file)"
// @Param        year     query     string  false  "working year, when the X-Admin-Year header cannot be sent (a plain link)"
// @Success      200  {file}  binary
// @Failure      400  {object}  map[string]string  "no working year, or one the tool does not know (X-Admin-Year or ?year=); or an unrecognised size"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      404  {object}  map[string]string  "unknown album"
// @Failure      409  {object}  map[string]string  "the album holds no photographs to download"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the albums or the library are unavailable"
// @Router       /admin/albums/{albumId}/zip [get]
//
// A deleted album is **not** refused. Its photographs are still in the library and the curator can still open its
// editor; a takedown took the album off the public site, and taking a copy of work that still exists is not
// publication. The list hides the action for a deleted album, which is a judgement about the menu rather than a
// rule about the data.
func (app *application) downloadAdminAlbumZipHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}
	// The library read as well as the album one, because the size resolution is per photograph: the album's items
	// carry the display ref, and which rendition answers for a size depends on the photograph's dimensions and on
	// whether it has an 800px copy.
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return
	}

	// An unrecognised size is refused rather than defaulted. A typo'd `size` silently answering with the largest is a
	// curator wondering why their e-mail attachment is 400 MB.
	sizeKey := r.URL.Query().Get("size")
	if sizeKey == "" {
		sizeKey = "xlarge"
	}
	size, ok := adminZipSizes[sizeKey]
	if !ok {
		app.BadRequestResponse(w, r, fmt.Errorf("ukendt st\u00f8rrelse %q; v\u00e6lg medium, large, xlarge eller original", sizeKey))
		return
	}

	year := adminYear(r)
	albumID := httprouter.ParamsFromContext(r.Context()).ByName("albumId")

	a, items, found, err := app.models.AlbumCurator.Album(year, albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	// The items arrive in ordinal order, and **ordinal order is the album's order**: a sort mode is materialised
	// as a reorder when it is chosen or when photographs are added (see applyAlbumSortMode), so the projection
	// already holds the arrangement rather than a rule to re-derive. Sorting again here would be a second place
	// that decides what an album's order is, and the two disagreeing is precisely the bug nobody would see until
	// a zip file came out shuffled.
	type entry struct {
		photoID string
		name    string
	}
	entries := make([]entry, 0, len(items))
	for _, it := range items {
		// A removed position is not in the album; a deleted photograph is not in the library. Either way there is
		// nothing to put in the file, and the ordinal numbering is over what is included — a gap in the sequence
		// would read as a missing file.
		if it.Removed || it.PhotoDeleted {
			continue
		}
		entries = append(entries, entry{
			photoID: it.PhotoID,
			name:    fmt.Sprintf("%03d-%s", len(entries)+1, adminZipEntryName(it.SortFileName, it.PhotoID)),
		})
	}

	// An empty archive is a file that looks broken, so say so instead. 409 rather than 404: the album is there,
	// it just has nothing in it yet, and a curator who sees "not found" goes looking for a deleted album.
	if len(entries) == 0 {
		app.ConflictResponse(w, r, "albummet har ingen billeder at hente")
		return
	}

	// `no-store` comes from requireAdmin (task 371) and applies here too: a zip of the event's photographs left in
	// a shared laptop's HTTP cache outlives the session that fetched it. The file on disk is the curator's
	// deliberate copy; a cache entry is not.
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+adminAlbumZipFileName(a.Slug, a.ID, time.Now())+`"`)
	w.Header().Set("X-Robots-Tag", "noindex")

	zw := zip.NewWriter(w)
	for _, e := range entries {
		if err := app.writeAdminAlbumZipEntry(r, zw, year, e.photoID, e.name, size.edge, size.original); err != nil {
			// Past the status line, so this cannot become a 500 — see the file header. Logged with the entry that
			// failed, because "the download stopped" is otherwise unactionable, and the writer is deliberately
			// **not** closed: an unterminated archive is one a zip reader rejects, where a tidily closed one short
			// of its photographs is a file somebody would hand on without noticing.
			app.Logger.Error("the album zip failed midway; the download is truncated",
				"albumId", albumID, "size", sizeKey, "entry", e.name, "err", err)
			return
		}
	}
	if err := zw.Close(); err != nil {
		app.Logger.Error("closing the album zip failed; the download is truncated", "albumId", albumID, "err", err)
	}
}

// writeAdminAlbumZipEntry puts one photograph into the archive at the requested edge.
//
// `edge` is 0 for "as stored". The id is resolved through the library projection and the ref used is the one that
// came **out of the row** — the rule the media route's header explains at length, and the reason this takes a
// photograph id rather than a ref.
//
// A missing object is an error rather than a skipped entry, and deliberately so: the names carry the album's
// order, so quietly dropping one makes the sequence lie about what the album holds.
func (app *application) writeAdminAlbumZipEntry(
	r *http.Request, zw *zip.Writer, year, photoID, name string, edge int, wantOriginal bool,
) error {
	p, found, err := app.models.PhotoCurator.Photo(year, photoID)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	// Both of these are already excluded by the album walk, which reads the same two facts from its own join. This
	// is the second check rather than the first, and it stays because the two reads are not one transaction.
	if !found || p.Deleted {
		return fmt.Errorf("%s: the photograph is not in the library", name)
	}

	ref, derive := adminZipRendition(p, edge, wantOriginal)
	if !ref.Valid() {
		return fmt.Errorf("%s: the library row holds an unusable ref %q", name, p.Ref)
	}

	if derive == 0 {
		// Streamed, so a 300 MB download of originals never holds more than one object in memory.
		reader, rerr := app.blobs.Get(r.Context(), ref)
		if rerr != nil {
			if errors.Is(rerr, blob.ErrNotFound) {
				return fmt.Errorf("%s: the stored object is gone: %w", name, rerr)
			}
			return fmt.Errorf("%s: %w", name, rerr)
		}
		defer reader.Close()
		return adminZipCopy(zw, name, reader)
	}

	// Rendered on demand, and **not stored**: a 1200px copy is wanted by a curator a handful of times a year, so
	// writing one per photograph would add a rendition to the store — and to every future backup decision — for a
	// size nothing else reads. The cost is paid by the person who asked for it.
	//
	// Behind the decode gate (task 471), like every other decode in this service: a loop over 200 photographs at
	// ~83 MB a frame is exactly what the gate exists to keep away from live uploads. A slot per photograph rather
	// than one for the whole download, so an upload is never waiting on a curator's export.
	raw, err := app.readBlob(r.Context(), ref)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	var prepared imaging.Portrait
	if err := withDecodeSlot(r.Context(), func() error {
		var perr error
		prepared, perr = imaging.Prepare(raw, derive, nil, glimtJPEGQuality, false)
		return perr
	}); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if len(prepared.Full.Bytes) == 0 {
		return fmt.Errorf("%s: no bytes were rendered", name)
	}
	return adminZipCopy(zw, name, bytes.NewReader(prepared.Full.Bytes))
}

// adminZipRendition decides which stored object answers for a requested size, and whether it needs re-rendering.
//
// Returns the ref to read and the edge to render it to, where 0 means "send these bytes as they are".
//
// The original is asked for first and answered without any re-rendering at all, which is not an optimisation: an
// original that had been through `imaging` would not be an original. Nothing about this branch may ever derive, resize
// or re-encode.
//
// When no original is held — every photograph uploaded before PRD 027, permanently — it falls through to the display
// image and the entry is 1600px. That is the honest answer rather than a substitute: 1600px **is** that photograph's
// most original surviving form. The admin tool says so per album before the download (task 482), because a curator who
// cannot tell which entries are which has a zip they cannot trust.
//
// Two further cases avoid a re-encode, and both matter because a re-encode is a decode of the whole album plus a
// generation of JPEG loss:
//
//   - the stored 800px rendition **is** the medium size, so asking for medium reads it directly;
//   - a photograph smaller than the requested edge is already the answer. `imaging.Fit` does not upscale, so
//     rendering it would re-encode the same pixels — the one case where doing the work is both slower and worse.
func adminZipRendition(p photo.LibraryPhoto, edge int, wantOriginal bool) (blob.Ref, int) {
	full := blob.Ref(p.Ref)

	if wantOriginal {
		if ref := blob.Ref(p.OriginalRef); ref.Valid() {
			return ref, 0
		}
		return full, 0
	}
	if edge == 0 {
		return full, 0
	}
	if edge == mediumEdge && p.MediumRef != "" {
		if ref := blob.Ref(p.MediumRef); ref.Valid() {
			return ref, 0
		}
	}
	// Zero dimensions mean the row does not say — an early upload, or a projection written before the columns
	// existed. Re-rendering is then the safe answer: it is slower than it might need to be, never wrong.
	if p.Width > 0 && p.Height > 0 && p.Width <= edge && p.Height <= edge {
		return full, 0
	}
	return full, edge
}

// adminZipCopy writes one entry.
func adminZipCopy(zw *zip.Writer, name string, body io.Reader) error {
	part, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: adminAlbumZipStored})
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if _, err := io.Copy(part, body); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// adminZipEntryName is the photograph's own file name, reduced to something safe inside an archive.
//
// The photographer's name is kept because it is what they will recognise on their own laptop, and it is already
// a sort key the album may be ordered by (PRD 024 §6 R5). It is never anybody's name — see the column comment in
// photo/table.sql.
//
// Hostile-looking names are not hypothetical: the name comes from a file somebody chose, and a zip entry called
// `../../etc/passwd` is a real class of bug in whatever unpacks it. So only the last path element survives, and
// a name that reduces to nothing falls back to the photograph's id — which is always usable and always unique.
func adminZipEntryName(fileName, photoID string) string {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(fileName), `\`, "/"))
	base = strings.TrimSpace(strings.Trim(base, "./"))
	// Control characters and the separators a zip reader on either platform would interpret.
	base = strings.Map(func(ch rune) rune {
		if ch < 0x20 || ch == 0x7f || ch == '/' || ch == '\\' || ch == ':' {
			return '-'
		}
		return ch
	}, base)
	if base == "" {
		return photoID + ".jpg"
	}
	return base
}

// adminAlbumZipFileName is `<album>-<yyyymmdd>.zip`.
//
// # Why the slug rather than the title
//
// The title is Danish prose with spaces, «» quotes and æøå in it, and it arrives here through a
// `Content-Disposition` header — the one place in HTTP where a non-ASCII byte needs a second, differently
// escaped parameter to survive, and where the fallback a browser uses when it disagrees is mojibake in a file
// name. The slug is the title already reduced to a URL-safe form by the album's own rules, so it is both
// recognisable and representable, and it is the string the curator already sees in the album's address.
//
// The date is **when the copy was taken**, in the event's timezone (task 358: every rendered timestamp here goes
// through `eventtime`). That is what makes two exports of an album a curator has since extended distinguishable,
// which is the question somebody with two files in a downloads folder is actually asking. The album's creation
// date would make them identical.
//
// The size is **not** in the name, as asked. Two downloads of one album at two sizes therefore collide, and the
// browser resolves that the way it resolves every other repeat download — `(1)`. Worth knowing before somebody
// files both in the same folder.
func adminAlbumZipFileName(slug, albumID string, now time.Time) string {
	name := slug
	if name == "" {
		// A slug is assigned at creation and never changes, so this is the unreachable branch — but an empty one
		// would produce a file called `-20260920.zip`, and a curator cannot tell which album that was.
		name = albumID
	}
	return name + "-" + now.In(eventtime.Location()).Format("20060102") + ".zip"
}
