package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/person"
)

// maxPortraitUpload bounds the request body.
//
// The app's own capture path sends a few hundred KB (client-side 2048px re-encode), so
// this limit exists for the **fallback**: `<input capture>` hands over the untouched
// camera file, and since task 112 that path is the only route to a camera-native
// original. A modern phone's "most compatible" JPEG is routinely 2–5 MB, which made the
// previous 4 MiB cap a coin flip — and a 413 on a photo the user just took reads as a
// bug, not as a limit.
//
// 8 MiB rather than unbounded because the cost here is *pixels*, not bytes: decoding is
// what allocates, so an 8 MiB JPEG (~12–20 MP) is tens of MB of transient RGBA per
// upload. That is comfortable for an authenticated, low-frequency endpoint and would not
// be if this were open or bulk.
//
// Enforced on the *reader*, not on Content-Length, because a header is a claim rather
// than a fact.
const maxPortraitUpload = 8 << 20

// portraitUploadTimeout is how long the body of an upload may take to arrive.
//
// Sized against the worst case this app is actually used in rather than a round number:
// 8 MiB at roughly 50 KB/s, which is a realistic single-bar mobile link, is a little over
// two and a half minutes. Three gives it room without letting a connection linger
// indefinitely.
const portraitUploadTimeout = 3 * time.Minute

// maxPortraitEdge is the longest edge of the stored image.
//
// Re-sizing here as well as on the client is not redundant: the server cannot assume the
// client did it, and PRD 007 sizes its offline sync against what is actually stored.
const maxPortraitEdge = 1024

// thumbnailEdges are the thumbnail sizes generated at upload (task 104).
//
// A list because more than one is expected: an identification grid (PRD 007) wants a
// different size from an avatar. Adding one here is all it takes — the event, the
// projection and the endpoint all carry a *set* of renditions, so no shape changes.
//
// 256 is sized against its first consumer: PRD 007 shows a grid of faces to identify
// someone in the dark and caches many of them offline. At 256px a face is still
// recognisable when tapped to fill a phone's width, while the file stays around 15–25 KB —
// so a few hundred is a handful of megabytes rather than a hundred.
//
// Note that changing this list does **not** rewrite existing portraits: renditions are
// produced at upload. A new size appears for portraits taken after the change, and a
// backfill would be its own task.
var thumbnailEdges = []int{256}

// jpegQuality is a deliberate trade. The portrait is used to recognise a face in the
// dark, so it must survive being enlarged on a phone screen; 85 is visually lossless for
// that purpose at this size and keeps a 1024px image around 100–200 KB, which matters
// because PRD 007 caches many of them on participants' devices.
const jpegQuality = 85

// errNotAnImage is returned when the bytes are not a decodable image.
var errNotAnImage = errors.New("filen er ikke et billede vi kan læse")

// updatePhotoHandler accepts the signed-in user's portrait. Runs behind requireAuth.
//
// # Why the bytes are re-encoded rather than stored
//
// Everything the client sends is decoded and re-encoded from pixels. That is the
// requirement from PRD 003 §6 Non-Functional, and it does several jobs at once:
//
//   - **EXIF is gone**, including GPS. This is a photograph taken by a minor during an
//     event; storing where it was taken is not something anyone asked for. Note that
//     stripping is a *consequence* of re-encoding rather than a separate step, which is
//     why it cannot be forgotten later.
//   - **The bytes are proven to be an image.** A polyglot file that is both a valid JPEG
//     and something a browser will execute does not survive a decode/encode round trip.
//   - **The stored format is known.** One content type, one codec, so PRD 007's sync and
//     task 104's thumbnails are not written against "whatever arrived".
//
// The declared Content-Type is never trusted: it is not even consulted. `image.Decode`
// sniffs the actual bytes, which is the only claim that means anything.
//
// # Orientation
//
// The EXIF orientation tag is read and **applied** before re-encoding (task 104), so a
// photo a phone stored rotated arrives upright. Doing it server-side covers the path the
// client cannot: the `<input capture>` fallback hands over an untouched camera file.
//
// # What is stored
//
// Three kinds of object, all content-addressed: the 1024px display image, one thumbnail
// per configured size (task 104), and — unless disabled — the **original** upload at its
// own resolution with all metadata stripped (task 111). The original is never served; it
// exists so renditions can be produced again later, which a 1024px re-encode cannot
// support.
//
// @Summary      Upload own portrait
// @Description  Accepts a multipart form with a `photo` file field, or a raw image body. The bytes are validated by decoding them, turned upright per their EXIF orientation, re-encoded to JPEG (which strips all EXIF, including GPS), downscaled to a longest edge of 1024px, and stored content-addressed together with a 256px thumbnail. The original is also retained at full resolution with its metadata stripped — but only when it holds more pixels than the display image — for future re-rendering, and is never served. The declared content type is ignored in favour of the actual bytes. Max 8 MiB.
// @Tags         me
// @Accept       mpfd
// @Produce      json
// @Success      200  {object}  map[string]string  "content hash of the stored portrait"
// @Failure      400  {object}  map[string]string  "not a decodable image"
// @Failure      401  {object}  map[string]string
// @Failure      413  {object}  map[string]string  "larger than 8 MiB"
// @Failure      429  {object}  map[string]string  "more than 10 uploads in the past hour"
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /me/photo [put]
func (app *application) updatePhotoHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Checked before the body is read, which is the point: a member who has hit the
	// ceiling should not get to push another 8 MiB up a mobile link first, and the server
	// should not spend a decode on it. Keyed by user, so a shared network is not throttled
	// as one client.
	//
	// A denied request is not recorded (see ratelimit.Allow), so retrying does not push the
	// window out — the wait is bounded by the ten uploads they actually made.
	if !app.photoLimiter.Allow(s.UserID) {
		app.RateLimitMessageResponse(w, r,
			"Du kan uploade 10 billeder i timen. Prøv igen senere.")
		return
	}

	// This one endpoint gets minutes, while the server keeps a 30-second default for
	// everything else (see app.Serve).
	//
	// A portrait is up to 8 MiB and is uploaded from wherever the member happens to be —
	// which for this app means a field at night on one bar of signal. At ~50 KB/s that is
	// well over two minutes, and the server-wide deadline would abort the read partway:
	// the member sees an upload that failed for no stated reason, intermittently, in the
	// place where it is hardest to reproduce.
	//
	// Set before the body is touched, or the deadline the read runs under is the old one.
	// A server that does not support deadline control (a test double, say) reports that
	// here and simply keeps its default, so this is deliberately not fatal.
	if derr := http.NewResponseController(w).SetReadDeadline(
		time.Now().Add(portraitUploadTimeout),
	); derr != nil {
		app.Logger.Warn("could not extend the upload read deadline", "err", derr)
	}

	raw, err := readPortraitUpload(w, r)
	if err != nil {
		if errors.Is(err, errUploadTooLarge) {
			// A distinct 413 so the client can say "vælg et mindre billede" instead of
			// a generic failure.
			app.PayloadTooLargeResponse(w, r, err)
			return
		}
		app.BadRequestResponse(w, r, err)
		return
	}

	encoded, meta, err := normalizePortrait(raw, app.config.portraitKeepOriginal)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	ref, err := app.storePortrait(r.Context(), s.UserID, encoded, meta)
	if err != nil {
		// A publish failure means the portrait is not saved as far as the app is
		// concerned (see storePortrait), and it is retryable — so 503, not 500. The
		// user is told to try again rather than being left believing there is a photo
		// on file.
		app.ServiceUnavailableResponse(w, r, "kunne ikke gemme billedet, prøv igen")
		app.Logger.Error("storing portrait", "err", err, "userId", s.UserID)
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, map[string]string{"ref": ref.String()}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// showPhotoHandler serves the signed-in user's own portrait. Runs behind requireAuth.
//
// `?size=thumb` serves the 256px thumbnail generated at upload (task 104). A query
// parameter rather than a second route because it selects a *representation* of the same
// resource, and because PRD 007 will need the same choice for other people's portraits —
// one convention is better than two.
//
// Own portrait only: the ref comes from the caller's own projection row, never from the
// URL, so there is no path here that can be pointed at somebody else's face. Viewing
// *other* people's portraits is PRD 007, with its own access matrix and audit log — it
// must not be bolted onto this endpoint by adding a parameter.
//
// A row referencing bytes that have gone missing degrades to 404, per PRD 008 §8: "a
// replay that finds a missing object must degrade to 'no photo', never fail".
//
// @Summary      Own portrait
// @Description  Returns the signed-in user's own portrait as JPEG. `size=thumb` serves the default (smallest) thumbnail, and `size=thumb256` (or `size=256`) names a specific rendition; anything unrecognised, or a portrait without that rendition, falls back to the full image. Never another user's — cross-person viewing is a separate feature with its own access rules. 404 when no portrait is on file, or when the stored bytes are missing.
// @Tags         me
// @Produce      jpeg
// @Param        size  query     string  false  "full (default), thumb, or a rendition name such as thumb256"
// @Success      200  {file}    binary
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /me/photo [get]
func (app *application) showPhotoHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if app.models.People == nil {
		// No database: portraits are unavailable rather than absent. 503 keeps the
		// difference visible instead of telling the user they have no photo.
		app.ServiceUnavailableResponse(w, r, "billeder er ikke tilgængelige lige nu")
		return
	}

	p, found, err := app.models.People.Get(app.config.eventYear, s.UserID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found || p.PortraitRef == "" {
		app.NotFoundResponse(w, r)
		return
	}

	// Which rendition to serve — see portraitRefForSize.
	stored := portraitRefForSize(p, r.URL.Query().Get("size"))

	ref := blob.Ref(stored)
	if !ref.Valid() {
		// The projection refuses non-hash refs (task 103), so this is belt and braces —
		// but it is the one check standing between a database value and a filesystem
		// path, and it costs nothing.
		app.Logger.Error("portrait ref in projection is not a content hash",
			"userId", s.UserID, "ref", stored)
		app.NotFoundResponse(w, r)
		return
	}

	app.streamPortrait(w, r, ref, "private, max-age=3600", s.UserID)
}

// portraitRefForSize picks which stored rendition to serve for a `size` query value.
//
// `size=thumb` means "the default thumbnail" (the smallest, denormalized onto the row);
// `size=thumb256` — or just `size=256` — names one explicitly, which is how a client asks
// for a specific size once there is more than one.
//
// Anything unrecognised, and any portrait with no such rendition, falls back to the full
// image rather than 404-ing: a client asking for something small would rather have
// something large than nothing.
//
// Shared by the own-portrait handler and the contacts photo handler (PRD 007) so the two
// cannot drift on what `?size=thumb` means — which would be a subtle way for the contacts
// pane to start shipping full-resolution faces.
func portraitRefForSize(p person.Person, sizeParam string) string {
	stored := p.PortraitRef
	size := strings.ToLower(strings.TrimSpace(sizeParam))
	if size == "" || size == "full" {
		return stored
	}

	switch {
	case size == "thumb" && p.PortraitThumbRef != "":
		stored = p.PortraitThumbRef
	default:
		name := size
		if !strings.HasPrefix(name, "thumb") {
			name = "thumb" + name
		}
		if t, ok := p.Thumb(name); ok {
			stored = t.Ref
		}
	}
	return stored
}

// streamPortrait writes the blob behind ref as JPEG.
//
// cacheControl is explicit at every call site rather than defaulted, because the right
// answer differs per surface and getting it wrong is a privacy bug rather than a
// performance one:
//
//   - a member's own portrait and a directory member's may be cached by the requesting
//     browser (`private, max-age=...`) — content-addressed bytes never change, and the
//     directory is meant to work offline;
//   - a patrol member's portrait must not be cached at all (`no-store`), because that is
//     what keeps spejder faces off devices (PRD 007 §8).
//
// `private` at minimum on every path: these are photographs of members, many of them minors,
// so they must never sit in a shared cache.
func (app *application) streamPortrait(w http.ResponseWriter, r *http.Request, ref blob.Ref, cacheControl, logID string) {
	reader, err := app.blobs.Get(r.Context(), ref)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			app.NotFoundResponse(w, r)
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", cacheControl)
	// The ref is a content hash, so it is a perfect ETag: same bytes, same value, and a
	// changed portrait is a different ref. This is what lets the sync engine skip images it
	// already holds without a size or date heuristic.
	w.Header().Set("ETag", `"`+string(ref)+`"`)

	if _, err := io.Copy(w, reader); err != nil {
		// The response has already begun; there is nothing to say to the client that it
		// would still parse.
		app.Logger.Error("streaming portrait", "err", err, "id", logID)
	}
}

// errUploadTooLarge marks a body over the limit, so the caller can answer 413.
var errUploadTooLarge = fmt.Errorf("billedet er større end %d MB", maxPortraitUpload>>20)

// readPortraitUpload reads the image bytes from either a multipart form or a raw body.
//
// Both are supported because the two clients differ: `PhotoCapture` produces a Blob and
// posts it as multipart, while a plain `<input capture>` fallback or a curl-driven test
// may send the file as the body. Accepting both is a few lines here and removes a class
// of "works in the app, not from the shell" confusion.
func readPortraitUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	// The limit is applied to the reader, so a lying Content-Length cannot get past it.
	// +1 byte so hitting the limit exactly is distinguishable from exceeding it.
	r.Body = http.MaxBytesReader(w, r.Body, maxPortraitUpload+1)

	if mediaType := r.Header.Get("Content-Type"); len(mediaType) >= 19 &&
		mediaType[:19] == "multipart/form-data" {
		file, _, err := r.FormFile("photo")
		if err != nil {
			// An over-limit multipart body surfaces here, from inside the form parser,
			// rather than at the read below — so without this it was reported as "we
			// expected an image in the photo field" with a 400. Which is doubly wrong: the
			// field was there, and the client is told to fix its request instead of being
			// told the picture is too big.
			//
			// Caught live on 2026-08-29 with an 11.6 MB upload; the existing test missed it
			// because it sent a raw body, which takes the other branch.
			if isTooLarge(err) {
				return nil, errUploadTooLarge
			}
			return nil, fmt.Errorf("forventede et billede i feltet \"photo\": %w", err)
		}
		defer file.Close()
		return readCapped(file)
	}

	return readCapped(r.Body)
}

// isTooLarge reports whether err is MaxBytesReader's limit being hit.
//
// Checks the typed error first. The string comparison is a fallback because the error
// travels up through mime/multipart, which is not documented to preserve wrapping — and
// getting this wrong means a 400 where the user needed a 413, i.e. "your request is
// malformed" where the truth is "your photo is too big".
func isTooLarge(err error) bool {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		return true
	}
	return strings.Contains(err.Error(), "request body too large")
}

func readCapped(src io.Reader) ([]byte, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		// MaxBytesReader's error is what a too-large body surfaces as, whether it came
		// via multipart or raw.
		if isTooLarge(err) {
			return nil, errUploadTooLarge
		}
		// Anything else is a connection that died mid-upload — common on a weak mobile
		// link, and worth distinguishing so the client can retry rather than shrink the
		// picture it already downscaled.
		return nil, fmt.Errorf("kunne ikke læse billedet: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("tomt billede")
	}
	if len(data) > maxPortraitUpload {
		return nil, errUploadTooLarge
	}
	return data, nil
}

// normalizePortrait decodes, turns upright, downscales and re-encodes the upload, and
// produces the thumbnail from the same decode.
//
// The decode is the validation: bytes that are not an image cannot get past it, and no
// header or filename is consulted. What comes out is always JPEG. The work itself lives
// in internal/imaging, where it is testable without a request.
func normalizePortrait(raw []byte, keepOriginal bool) ([]byte, portraitMeta, error) {
	prepared, err := imaging.Prepare(raw, maxPortraitEdge, thumbnailEdges, jpegQuality, keepOriginal)
	if err != nil {
		if errors.Is(err, imaging.ErrNotAnImage) {
			// Translated to the Danish message the client shows; the packaged error is
			// for the log.
			return nil, portraitMeta{}, errNotAnImage
		}
		return nil, portraitMeta{}, fmt.Errorf("prepare portrait: %w", err)
	}

	return prepared.Full.Bytes, portraitMeta{
		ContentType:         "image/jpeg",
		Width:               prepared.Full.Width,
		Height:              prepared.Full.Height,
		Thumbs:              prepared.Thumbs,
		Original:            prepared.Original,
		OriginalContentType: "image/" + prepared.Format,
		Orientation:         prepared.Orientation,
	}, nil
}
