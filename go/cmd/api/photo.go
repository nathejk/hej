package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"

	// PNG and GIF are registered for their *decoders* only: a client may hand us
	// either, and everything is re-encoded to JPEG on the way out. Registering them
	// is also what makes image.Decode able to reject a file whose bytes do not match
	// its claimed type.
	_ "image/gif"
	_ "image/png"

	"nathejk.dk/internal/blob"
)

// maxPortraitUpload bounds the request body.
//
// The client downscales to ~1024px before uploading (task 106), which lands well under
// 500 KB; 4 MiB leaves generous room for a client that has not been updated, or a
// fallback `<input capture>` that hands over an untouched camera file, while staying far
// from a size that could exhaust memory. The limit is enforced on the *reader*, not on
// Content-Length, because a header is a claim rather than a fact.
const maxPortraitUpload = 4 << 20

// maxPortraitEdge is the longest edge of the stored image.
//
// Re-sizing here as well as on the client is not redundant: the server cannot assume the
// client did it, and PRD 007 sizes its offline sync against what is actually stored.
const maxPortraitEdge = 1024

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
// Re-encoding drops the EXIF orientation tag, so a photo the camera stored rotated with
// a "turn me" flag would come out sideways. The client is required to apply orientation
// before upload (task 106) — the correct place, since it has the tag and the canvas. The
// `<input capture>` fallback is the case to watch on real devices (task 108).
//
// @Summary      Upload own portrait
// @Description  Accepts a multipart form with a `photo` file field, or a raw image body. The bytes are validated by decoding them, re-encoded to JPEG (which strips all EXIF, including GPS), downscaled to a longest edge of 1024px, stored content-addressed, and published as a portrait event. The declared content type is ignored in favour of the actual bytes. Max 4 MiB.
// @Tags         me
// @Accept       mpfd
// @Produce      json
// @Success      200  {object}  map[string]string  "content hash of the stored portrait"
// @Failure      400  {object}  map[string]string  "not a decodable image"
// @Failure      401  {object}  map[string]string
// @Failure      413  {object}  map[string]string  "larger than 4 MiB"
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /me/photo [put]
func (app *application) updatePhotoHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
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

	encoded, meta, err := normalizePortrait(raw)
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
// Own portrait only: the ref comes from the caller's own projection row, never from the
// URL, so there is no path here that can be pointed at somebody else's face. Viewing
// *other* people's portraits is PRD 007, with its own access matrix and audit log — it
// must not be bolted onto this endpoint by adding a parameter.
//
// A row referencing bytes that have gone missing degrades to 404, per PRD 008 §8: "a
// replay that finds a missing object must degrade to 'no photo', never fail".
//
// @Summary      Own portrait
// @Description  Returns the signed-in user's own portrait as JPEG. Never another user's — cross-person viewing is a separate feature with its own access rules. 404 when no portrait is on file, or when the stored bytes are missing.
// @Tags         me
// @Produce      jpeg
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

	ref := blob.Ref(p.PortraitRef)
	if !ref.Valid() {
		// The projection refuses non-hash refs (task 103), so this is belt and braces —
		// but it is the one check standing between a database value and a filesystem
		// path, and it costs nothing.
		app.Logger.Error("portrait ref in projection is not a content hash",
			"userId", s.UserID, "ref", p.PortraitRef)
		app.NotFoundResponse(w, r)
		return
	}

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
	// Content-addressed, so the bytes behind a ref never change — but this is a
	// photograph of a minor, so it must not be held by shared caches. `private`
	// restricts it to the requesting user's own browser.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if _, err := io.Copy(w, reader); err != nil {
		// The response has already begun; there is nothing to say to the client that it
		// would still parse.
		app.Logger.Error("streaming portrait", "err", err, "userId", s.UserID)
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
			return nil, fmt.Errorf("forventede et billede i feltet \"photo\": %w", err)
		}
		defer file.Close()
		return readCapped(file)
	}

	return readCapped(r.Body)
}

func readCapped(src io.Reader) ([]byte, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		// MaxBytesReader's error is what a too-large body surfaces as, whether it came
		// via multipart or raw.
		return nil, errUploadTooLarge
	}
	if len(data) == 0 {
		return nil, errors.New("tomt billede")
	}
	if len(data) > maxPortraitUpload {
		return nil, errUploadTooLarge
	}
	return data, nil
}

// normalizePortrait decodes, downscales and re-encodes the upload.
//
// The decode is the validation: bytes that are not an image cannot get past it, and no
// header or filename is consulted. What comes out is always JPEG.
func normalizePortrait(raw []byte) ([]byte, portraitMeta, error) {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, portraitMeta{}, errNotAnImage
	}

	img = fitWithin(img, maxPortraitEdge)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, portraitMeta{}, fmt.Errorf("re-encode portrait: %w", err)
	}

	bounds := img.Bounds()
	return out.Bytes(), portraitMeta{
		ContentType: "image/jpeg",
		Width:       bounds.Dx(),
		Height:      bounds.Dy(),
	}, nil
}

// fitWithin scales img down so its longest edge is at most edge. Images already within
// the limit are returned untouched.
//
// Nearest-neighbour by hand rather than a resize dependency: this runs on an image that
// the client has usually already sized correctly, so it is a safety net rather than the
// normal path, and adding a dependency for the exceptional case is the wrong trade. Task
// 104 needs real resampling for thumbnails and can bring the library — with a visual
// check — if it turns out to matter.
func fitWithin(img image.Image, edge int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= edge && h <= edge {
		return img
	}

	newW, newH := w, h
	if w >= h {
		newW = edge
		newH = h * edge / w
	} else {
		newH = edge
		newW = w * edge / h
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + y*h/newH
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + x*w/newW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}
