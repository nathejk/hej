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
	"nathejk.dk/nathejk/table/glimt"
)

// Glimt media upload (PRD 019 §8, task 303).
//
// One item per request, returning the refs the client then names when it creates the glimt
// (task 304). Two-step rather than one multipart request carrying everything, because a glimt has
// up to ten items and is posted from a field on one bar of signal: uploading them one at a time
// means a failure costs one item instead of the whole post, and it is what lets the client's outbox
// resume a partial upload rather than starting over (task 314).
//
// Almost all of the work is the portrait's, reused: `internal/imaging` decodes (which *is* the
// validation — no header or filename is trusted), turns the image upright by its EXIF orientation,
// re-encodes to JPEG and thereby **strips all EXIF including GPS**, and `internal/blob` stores the
// result content-addressed. See photo.go for the reasoning behind each of those choices; this file
// only records where Glimt differs.

// maxGlimtUpload bounds one item.
//
// Larger than the portrait's 8 MiB because a glimt is a photograph of a scene rather than a face and
// the client has less reason to have pre-cropped it, but still small enough that ten of them do not
// make a single post unbearable on mobile data. Video gets its own, much larger, ceiling in task 322
// — deliberately a separate number, so raising one cannot silently raise the other.
const maxGlimtUpload = 12 << 20

// maxGlimtEdge is the longest edge of the stored display image.
//
// Bigger than the portrait's 1024: a portrait is shown in a list row or an avatar, while a glimt
// fills a phone screen and is opened full-screen in the viewer, where 1024 is visibly soft on a
// modern display.
const maxGlimtEdge = 1600

// glimtThumbEdges are the thumbnail sizes generated at upload.
//
// One size, and it is the size the hold-collection grid uses. That grid is the post-race browse
// (PRD 019 §0a.1), which pulls thumbnails by the thousand over a congested network, so this number
// is the single most load-bearing constant in the feature: 320px covers a grid tile at 2× on a phone
// and costs a few kilobytes.
var glimtThumbEdges = []int{320}

// glimtJPEGQuality matches the portrait's. Worth keeping identical so that "why does a glimt look
// different from a portrait" never becomes a question with an answer.
const glimtJPEGQuality = 85

// glimtUploadTimeout extends the read deadline for this one endpoint, as the portrait upload does.
//
// The server-wide default is 30 seconds. Twelve MiB at the ~50 KB/s a field at night actually
// provides is four minutes, and the default would abort the read partway — giving the member an
// upload that fails for no stated reason, intermittently, exactly where it is hardest to reproduce.
const glimtUploadTimeout = 5 * time.Minute

var errGlimtUploadTooLarge = fmt.Errorf("filen er større end %d MB", maxGlimtUpload>>20)

var errGlimtNotMedia = errors.New("filen er ikke et billede eller en video vi kan læse")

// glimtMediaStored is what an upload returns to the client.
//
// The refs are returned because the client has to name them when it creates the glimt — this is the
// one place a content hash legitimately crosses the wire, and it goes only to the person who just
// uploaded the bytes. Everywhere else media is addressed by ordinal (task 302), because a hash in a
// *feed* payload would be a forwardable capability.
type glimtMediaStored struct {
	Ref      string `json:"ref"`
	ThumbRef string `json:"thumb_ref,omitempty"`
	Kind     string `json:"kind"`
	Bytes    int    `json:"bytes"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	// DurationMs is 0 for a still. Populated by task 322 for video.
	DurationMs int `json:"duration_ms,omitempty"`
	// ContentType of the stored object, which is image/jpeg for every image regardless of
	// what was uploaded — the bytes are re-encoded.
	ContentType string `json:"content_type"`
}

// uploadGlimtMediaHandler stores one photo (or, from task 322, one video) and returns its refs.
//
// @Summary      Upload one Glimt media item
// @Description  Accepts a multipart form with a `media` file field, or a raw image body. The bytes are validated by decoding them, turned upright per their EXIF orientation, re-encoded to JPEG (which strips all EXIF, including GPS), downscaled to a longest edge of 1600px, and stored content-addressed together with a 320px thumbnail. The declared content type is ignored in favour of the actual bytes. The returned refs are named when creating a glimt; they are not URLs and grant nothing on their own. Max 12 MiB.
// @Tags         glimt
// @Accept       mpfd
// @Produce      json
// @Success      200  {object}  glimtMediaStored
// @Failure      400  {object}  map[string]string  "not a decodable image"
// @Failure      401  {object}  map[string]string
// @Failure      413  {object}  map[string]string  "larger than 12 MiB"
// @Failure      429  {object}  map[string]string  "upload rate, byte budget or per-member storage ceiling reached"
// @Failure      500  {object}  map[string]string
// @Failure      507  {object}  map[string]string  "the event's total storage ceiling is reached"
// @Router       /glimt/media [post]
func (app *application) uploadGlimtMediaHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Checked before the body is read, for the reason photo.go records: a member at the
	// ceiling should not get to push 12 MiB up a mobile link first, and the server should
	// not spend a decode on it. Keyed by user rather than IP because participants share
	// networks — a patrol on one hotspot must not throttle each other.
	if app.glimtMediaLimiter != nil && !app.glimtMediaLimiter.Allow(s.UserID) {
		app.RateLimitMessageResponse(w, r,
			"Du har uploadet mange filer på kort tid. Prøv igen om lidt.")
		return
	}

	// Set before the body is touched, or the read runs under the old deadline. Not fatal if
	// unsupported (a test double, say) — it then keeps the server default.
	if derr := http.NewResponseController(w).SetReadDeadline(
		time.Now().Add(glimtUploadTimeout),
	); derr != nil {
		app.Logger.Warn("could not extend the glimt upload read deadline", "err", derr)
	}

	raw, err := readGlimtUpload(w, r)
	if err != nil {
		if errors.Is(err, errGlimtUploadTooLarge) {
			// A distinct 413 so the client can say "vælg en mindre fil" rather than
			// reporting a generic failure the member cannot act on.
			app.PayloadTooLargeResponse(w, r, err)
			return
		}
		app.BadRequestResponse(w, r, err)
		return
	}

	// The byte half of the rate limit, and the storage ceiling. Both are checked **after** the body
	// is read and **before** the decode, which is the only place they can be: the size is not known
	// until the bytes have arrived, and the decode is the expensive part.
	//
	// The size charged is the *uploaded* size, not the re-encoded one. It is the honest measure of
	// what the member cost the link and the CPU, it is knowable here, and it cannot be gamed by
	// sending something that compresses well after resampling. The stored figure ends up smaller,
	// which means the ceiling refuses slightly early — the safe direction.
	incoming := int64(len(raw))
	if !app.glimtMediaBudget.Allow(s.UserID, incoming) {
		app.RateLimitMessageResponse(w, r,
			"Du har uploadet mange billeder på kort tid. Prøv igen om lidt.")
		return
	}
	if app.writeGlimtCeilingResponse(w, r, app.checkGlimtStorageCeiling(s.UserID, incoming)) {
		return
	}

	stored, err := app.storeGlimtImage(r, raw)
	if err != nil {
		if errors.Is(err, errGlimtNotMedia) {
			app.BadRequestResponse(w, r, err)
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, stored, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// storeGlimtImage normalizes and stores one image, returning what the create call will need.
//
// # Why no original is kept
//
// The portrait keeps a metadata-stripped original so renditions can be regenerated later (task 111).
// Glimt deliberately does not, and the reason is arithmetic: a portrait is one object per member for
// one event, while glimt are unbounded — ten items per post, many posts per member — and the blob
// store is the one thing in this service that cannot be rebuilt from the stream and therefore the
// one thing that must be backed up. Keeping originals would roughly double the only irreplaceable
// data in the service, to enable a re-rendering nobody has asked for. If a future feature needs
// sharper media it should raise maxGlimtEdge for new uploads rather than hoard originals for all of
// them.
func (app *application) storeGlimtImage(r *http.Request, raw []byte) (glimtMediaStored, error) {
	prepared, err := imaging.Prepare(raw, maxGlimtEdge, glimtThumbEdges, glimtJPEGQuality, false)
	if err != nil {
		if errors.Is(err, imaging.ErrNotAnImage) {
			// Translated to the message the client shows; the packaged error is for the log.
			return glimtMediaStored{}, errGlimtNotMedia
		}
		return glimtMediaStored{}, fmt.Errorf("prepare glimt media: %w", err)
	}

	ctx := r.Context()

	ref, err := app.blobs.Put(ctx, prepared.Full.Bytes)
	if err != nil {
		return glimtMediaStored{}, fmt.Errorf("store glimt media: %w", err)
	}

	// The thumbnail is stored but a failure to store it does **not** fail the upload — the
	// opposite of the portrait's rule, and deliberately so. PRD 007 relies on every portrait
	// having a thumbnail, so an incomplete rendition set there would be a state to handle
	// forever. Here the client already falls back to the full item when `has_thumb` is false
	// (task 302), so losing a thumbnail costs one grid tile some bandwidth rather than
	// costing the member their photo.
	thumbRef := ""
	if len(prepared.Thumbs) > 0 && len(prepared.Thumbs[0].Bytes) > 0 {
		tr, terr := app.blobs.Put(ctx, prepared.Thumbs[0].Bytes)
		if terr != nil {
			app.Logger.Warn("storing glimt thumbnail", "err", terr)
		} else {
			thumbRef = tr.String()
		}
	}

	return glimtMediaStored{
		Ref:         ref.String(),
		ThumbRef:    thumbRef,
		Kind:        glimt.MediaKindImage,
		Bytes:       len(prepared.Full.Bytes),
		Width:       prepared.Full.Width,
		Height:      prepared.Full.Height,
		ContentType: "image/jpeg",
	}, nil
}

// readGlimtUpload reads the bytes from either a multipart form or a raw body.
//
// Both, for the reason photo.go gives: the composer posts a Blob as multipart while a plain
// `<input capture>` fallback or a shell-driven test may send the file as the body, and accepting
// both removes a class of "works in the app, not from curl" confusion.
//
// The field is `media`, not `photo`, because from task 322 it carries video too and a field named
// for one of the two would be a lie in half the requests.
func readGlimtUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	// Applied to the reader, so a lying Content-Length cannot get past it. +1 byte so hitting
	// the limit exactly is distinguishable from exceeding it.
	r.Body = http.MaxBytesReader(w, r.Body, maxGlimtUpload+1)

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("media")
		if err != nil {
			// An over-limit multipart body surfaces here, from inside the form parser,
			// rather than at the read below. Without this branch it would be reported as
			// "we expected a file in the media field" with a 400 — doubly wrong: the field
			// was there, and the client is told to fix its request when the truth is the
			// file is too big. This exact bug was found live on the portrait endpoint
			// (see photo.go), so it is handled here from the start.
			if isTooLarge(err) {
				return nil, errGlimtUploadTooLarge
			}
			return nil, fmt.Errorf("forventede en fil i feltet \"media\": %w", err)
		}
		defer file.Close()
		return readCappedGlimt(file)
	}

	return readCappedGlimt(r.Body)
}

func readCappedGlimt(src io.Reader) ([]byte, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		if isTooLarge(err) {
			return nil, errGlimtUploadTooLarge
		}
		// A connection that died mid-upload — common on a weak mobile link, and worth
		// distinguishing so the client retries rather than shrinking a file it already
		// compressed.
		return nil, fmt.Errorf("kunne ikke læse filen: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("tom fil")
	}
	if len(data) > maxGlimtUpload {
		return nil, errGlimtUploadTooLarge
	}
	return data, nil
}

// glimtBlobRef parses a ref supplied by a client.
//
// Used by the create handler (task 304) to reject anything that is not a content hash before it
// reaches a SQL statement or a URL path. Returns the typed blob.Ref so the rest of the code cannot
// confuse it with an arbitrary string: "../../etc/passwd" is a Ref-shaped value, and
// blob.Ref.Valid is what says otherwise.
//
// # Why this is stricter than blob.Ref.Valid
//
// `blob.Ref.Valid` uses `hex.DecodeString`, which **accepts uppercase hex** — correct for its own
// job, which is deciding whether a string is safe to turn into a filesystem path. But
// `blob.ComputeRef` only ever emits lowercase, and the glimt projection's `validRef` only accepts
// lowercase. Left alone, those two facts combine into a silent data loss: a client that uppercased a
// ref would pass validation here, the ref would go out on the created event, and the **fold would
// drop that media item** — so a three-photo glimt would arrive with two, with no error logged
// anywhere and nothing for the member to retry.
//
// So the canonical form is required at the boundary. Fixing it here rather than loosening the
// projection is deliberate: lowercase-only is what the store actually produces, and a projection
// that accepted both spellings of the same hash would be a projection where two rows can reference
// one object.
func glimtBlobRef(s string) (blob.Ref, bool) {
	ref := blob.Ref(s)
	if !ref.Valid() {
		return "", false
	}
	if s != strings.ToLower(s) {
		return "", false
	}
	return ref, true
}
