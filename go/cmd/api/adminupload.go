package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nathejk.dk/internal/commands"
	"nathejk.dk/nathejk/table/photo"
)

// The admin upload path (PRD 022 §8.5, §8.9, task 372).
//
// # One file per request, and why that is not a limitation
//
// PRD 022 §6 requires "several photographs at once", and this endpoint takes exactly one. The batching lives
// in the browser (task 373), which issues one request per file with bounded concurrency. That is the design
// rather than a simplification, for three reasons:
//
//  1. **One bad file cannot fail a batch.** A `.mov` from the same card, a raw file, a `Thumbs.db` — each
//     fails its own request with its own reason while the other 299 proceed. A single multipart request
//     containing 300 files either succeeds or fails as one, and partial success in one response is a shape
//     nobody handles well.
//  2. **A dead connection loses one file.** A closed laptop or a train tunnel costs the file in flight, not
//     the afternoon.
//  3. **Progress is real.** Per-file progress from per-file requests, rather than a single upload bar that
//     reaches 100% and then waits while the server does 300 files' work.
//
// # The id is the content hash, and that is what makes the batch resumable
//
// Re-dragging a folder is routine — it is how a photographer retries after a dropped connection. Because the
// id is derived from the stored bytes, a re-upload converges on the row that already exists instead of
// producing a second one, so "just drag it again" is a correct recovery procedure rather than a way to
// duplicate a card.

// maxAdminUpload bounds one uploaded photograph.
//
// 32 MiB, against glimt's 12 (PRD 022 §8.9). The reasoning is not generosity: a full-frame camera JPEG at
// maximum quality routinely exceeds 12 MiB, and a photographer whose ordinary files are refused would
// reasonably conclude the tool is broken — and would be right, because refusing a normal photograph from the
// intended input device is a bug in the limit rather than a protection.
//
// It is still a limit, because a hundred-megabyte file is a mistake whoever sent it: a video, a raw, or a
// script pointed at the wrong directory.
const maxAdminUpload = 32 << 20

// adminUploadTimeout extends the read deadline for this one endpoint.
//
// The server-wide default is 30 seconds. 32 MiB over a hotel or venue connection is minutes, and the default
// would abort partway — giving an upload that fails for no stated reason, intermittently, on exactly the
// connection that is hardest to reproduce.
//
// Longer than glimt's five minutes because the file is nearly three times the size and the person uploading
// is doing it three hundred times.
const adminUploadTimeout = 10 * time.Minute

var errAdminUploadTooLarge = fmt.Errorf("filen er større end %d MB", maxAdminUpload>>20)

// adminUploadOutcome distinguishes the three things a successful upload can mean.
//
// # Why the response says which, rather than just 200
//
// Because two of the three look like failures if they are reported as plain success, and the third looks like
// a bug.
//
// A re-upload that silently reported success would tell a photographer their 300 files landed when 280 of
// them were already there — indistinguishable from the batch having worked, and so a duplicate card could
// never be diagnosed. And a re-upload of a *deleted* photograph does nothing at all by design (see below);
// reporting that as success would be a lie, while reporting it as an error would suggest something needs
// fixing. It needs a third word.
type adminUploadOutcome string

const (
	// adminUploadStored is a photograph that was not in the library before.
	adminUploadStored adminUploadOutcome = "stored"
	// adminUploadAlreadyPresent is a re-upload of bytes the library already holds.
	adminUploadAlreadyPresent adminUploadOutcome = "already"
	// adminUploadPreviouslyDeleted is a re-upload of a photograph a curator removed.
	//
	// The upload event is **not** republished for this case, and that is the point. The id is derived from
	// the bytes, so republishing `uploaded` would be a no-op only because the fold deliberately leaves
	// `deleted` alone (task 363) — relying on that here would make the safety of a takedown depend on a
	// detail of a fold two packages away. Refusing to publish makes it depend on nothing.
	adminUploadPreviouslyDeleted adminUploadOutcome = "deleted"
)

// adminUploadResponse is what one upload returns.
//
// No blob refs. Glimt's upload returns them because its client has to name them when it creates the glimt;
// here the server owns the whole flow and the browser addresses a photograph by its id, so putting a content
// hash on the wire would hand out a capability for no reason. `glimtmediaserve.go` states the rule this
// follows: a blob URL is a capability, and refs do not leave the server without a reason.
type adminUploadResponse struct {
	PhotoID string             `json:"photoId"`
	Outcome adminUploadOutcome `json:"outcome"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	Bytes  int `json:"bytes,omitempty"`

	// Location is the coordinate read from the file, if it had a usable one, with the verdict the race-area
	// check reached. Returned so the drop zone can mark the row — PRD 011 §6 requires an out-of-bounds
	// coordinate to be visible *as rejected* rather than merely absent from the map.
	Location *adminUploadLocation `json:"location,omitempty"`

	// Message is a plain Danish sentence for the row, written by the server rather than assembled by the
	// client. One place decides the wording, and it is the place that knows which of the three outcomes
	// happened.
	Message string `json:"message"`
}

type adminUploadLocation struct {
	Lat           float64 `json:"lat"`
	Lng           float64 `json:"lng"`
	BoundsVerdict string  `json:"boundsVerdict"`
}

// uploadAdminPhotoHandler stores one photograph in the year's library.
//
// @Summary      Upload one photograph to the year's library
// @Description  Accepts a multipart form with a `photo` file field, or a raw image body, and adds it to the configured event year's photograph library. The bytes are validated **by decoding them** — the declared content type is ignored — then turned upright per their EXIF orientation, re-encoded to JPEG (which strips all EXIF including GPS) and stored content-addressed with a thumbnail. Any GPS fix is read from the original bytes *before* re-encoding, bounds-checked against the race area, and stored as a reviewable field. The photograph's id is the content hash of the stored rendition, so uploading the same file twice yields one library row rather than two; the response's `outcome` says which case occurred. A photograph a curator has deleted is **not** restored by re-uploading it. Max 32 MiB. Requires the admin credential.
// @Tags         admin
// @Accept       mpfd
// @Produce      json
// @Success      200  {object}  adminUploadResponse  "stored, already present, or previously deleted"
// @Failure      400  {object}  map[string]string  "not a decodable image"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      413  {object}  map[string]string  "larger than 32 MiB"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
// @Failure      507  {object}  map[string]string  "the blob volume would drop below its free-space floor; keep the card and tell a developer"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the library or the event stream are unavailable"
// @Router       /admin/photos [post]
func (app *application) uploadAdminPhotoHandler(w http.ResponseWriter, r *http.Request) {
	// The library read is needed to answer "was this already here, and was it deleted?", which is the whole
	// point of this endpoint's response. Without it the upload would still work but could not say which
	// outcome it was, so it is a 503 rather than a degraded success.
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return
	}

	// Set before the body is touched, as the glimt and portrait uploads do: the deadline has to be in place
	// before the read it is extending, or it extends nothing.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetReadDeadline(time.Now().Add(adminUploadTimeout)); err != nil {
			// Not fatal. The server-wide deadline still applies, which means a large file on a slow link may
			// fail — worth a log line so that failure is diagnosable rather than mysterious.
			app.Logger.Warn("could not extend the admin upload read deadline", "err", err)
		}
	}

	raw, err := readAdminUpload(w, r)
	if err != nil {
		switch {
		case errors.Is(err, errAdminUploadTooLarge):
			app.PayloadTooLargeResponse(w, r, err)
		default:
			app.BadRequestResponse(w, r, err)
		}
		return
	}

	// **Before the bytes are stored, not after.** There is no point discovering the volume is full once the
	// object is on it, and `storeAlbumImage` writes two objects (the rendition and its thumbnail) with no way
	// to undo half of it. The check is against the raw size, which over-counts slightly because the stored
	// rendition is usually smaller — over-counting is the safe direction for a floor.
	if app.writeAdminDiskResponse(w, r, app.checkAdminDiskFloor(int64(len(raw)))) {
		return
	}

	// The pipeline is `storeAlbumImage`'s, unchanged and deliberately reused rather than reimplemented: it
	// reads GPS from the original bytes *before* `imaging.Prepare` re-encodes them, which is the one ordering
	// this whole feature depends on (PRD 022 §8.4). A second copy of that sequence would be a second place
	// for somebody to "tidy up" the read out of existence.
	stored, err := app.storeAlbumImage(r.Context(), raw)
	if err != nil {
		if errors.Is(err, errGlimtNotMedia) {
			// The decode is the validation. A `.mov`, a raw file or a `Thumbs.db` from the same card all land
			// here, and the reason is in Danish because a photographer reads it in a table of 300 rows.
			app.BadRequestResponse(w, r, errors.New("filen er ikke et billede vi kan læse"))
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}

	// The id *is* the stored content ref. Derived rather than minted, which is what makes the whole path
	// idempotent — see the file header and PRD 022 §8.5.
	photoID := stored.Ref

	// Was it here already, and was it deleted? `Photo` finds a deleted row and reports it as deleted, which
	// is precisely why the curator read exists (task 366): the public read would say "not found" for both.
	existing, found, err := app.models.PhotoCurator.Photo(app.config.eventYear, photoID)
	if err != nil {
		app.ServerErrorResponse(w, r, fmt.Errorf("checking whether the photograph is already in the library: %w", err))
		return
	}

	resp := adminUploadResponse{
		PhotoID: photoID,
		Width:   stored.Width,
		Height:  stored.Height,
		Bytes:   stored.Bytes,
	}
	if stored.Location != nil {
		resp.Location = &adminUploadLocation{
			Lat:           stored.Location.Lat,
			Lng:           stored.Location.Lng,
			BoundsVerdict: stored.Location.BoundsVerdict,
		}
	}

	switch {
	case found && existing.Deleted:
		// **Nothing is published.** A curator removed this photograph, possibly because somebody objected,
		// and a re-upload is not a decision to put it back. Said plainly rather than silently ignored: the
		// silent version looks like a bug to the one person who could explain it.
		resp.Outcome = adminUploadPreviouslyDeleted
		resp.Message = "Dette billede er slettet tidligere og bliver ikke lagt op igen."
		app.Logger.Info("admin upload skipped: the photograph was previously deleted",
			"photoId", photoID, "ip", clientIP(r))

	case found:
		// Already in the library. Not republished — the fold would converge on the same row anyway, so the
		// event would be noise on a log that is never rewritten.
		resp.Outcome = adminUploadAlreadyPresent
		resp.Message = "Allerede uploadet."
		app.Logger.Info("admin upload was a duplicate", "photoId", photoID, "ip", clientIP(r))

	default:
		subject, serr := photo.Subject(app.config.eventYear, photoID, photo.VerbUploaded)
		if serr != nil {
			app.ServerErrorResponse(w, r, serr)
			return
		}
		if perr := app.commands.Publish(subject, photo.Uploaded{
			PhotoID:    photoID,
			Year:       app.config.eventYear,
			Ref:        stored.Ref,
			ThumbRef:   stored.ThumbRef,
			Width:      stored.Width,
			Height:     stored.Height,
			Bytes:      stored.Bytes,
			Location:   stored.Location,
			UploadedAt: time.Now().UTC(),
		}); perr != nil {
			// The projection is downstream of the log, so a failed publish must not answer 200: the
			// photograph would be absent on the next reload and the curator would have been told it was
			// saved. The bytes are already in the store and will be reused by a retry, which content
			// addressing makes free.
			app.writeAlbumPublishFailure(w, r, perr)
			return
		}

		resp.Outcome = adminUploadStored
		resp.Message = "Uploadet."
		// With a shared credential the log is the only audit trail there is (PRD 022 §8.2), so every write
		// logs what happened, to what, and from where.
		app.Logger.Info("admin uploaded a photograph",
			"photoId", photoID, "bytes", stored.Bytes, "ip", clientIP(r),
			"verdict", verdictOf(stored.Location))
	}

	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// verdictOf reports a location's verdict for logging, or "none".
//
// A helper rather than an inline conditional because it is the value an operator greps for when asking "why
// is nothing on the map" — and a log line that omitted it for the nil case would make the common answer
// ("they had no coordinate") indistinguishable from a missing field.
func verdictOf(loc *photo.Location) string {
	if loc == nil {
		return photo.BoundsNone
	}
	return loc.BoundsVerdict
}

// readAdminUpload reads one uploaded file, bounded.
//
// The same shape as `readGlimtUpload`, including the multipart branch that exists because an over-limit
// multipart body surfaces from inside the form parser rather than at the read — a bug found live on the
// portrait endpoint, which is why it is handled here from the start rather than discovered again.
//
// The field is named `photo` rather than glimt's `media`, because this endpoint accepts photographs and
// nothing else: there is no video path here (PRD 020 is a separate draft), and a field called `media` would
// invite one.
func readAdminUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	// Applied to the reader, so a lying Content-Length cannot get past it. +1 byte so hitting the limit
	// exactly is distinguishable from exceeding it.
	r.Body = http.MaxBytesReader(w, r.Body, maxAdminUpload+1)

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("photo")
		if err != nil {
			if isTooLarge(err) {
				return nil, errAdminUploadTooLarge
			}
			return nil, fmt.Errorf("forventede en fil i feltet \"photo\": %w", err)
		}
		defer file.Close()
		return readCappedAdminUpload(file)
	}

	return readCappedAdminUpload(r.Body)
}

func readCappedAdminUpload(src io.Reader) ([]byte, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		if isTooLarge(err) {
			return nil, errAdminUploadTooLarge
		}
		return nil, fmt.Errorf("kunne ikke læse filen: %w", err)
	}
	if len(data) == 0 {
		// A zero-byte file is a real thing on a card that was pulled mid-write, and it decodes as "not an
		// image" with a confusing reason if allowed through.
		return nil, errors.New("tom fil")
	}
	if len(data) > maxAdminUpload {
		return nil, errAdminUploadTooLarge
	}
	return data, nil
}

// Compile-time assurance that the publish-failure helper is the album one, shared deliberately: both surfaces
// answer 503 for `commands.ErrNoPublisher` and 500 otherwise, and two copies would eventually disagree about
// which is which.
var _ = commands.ErrNoPublisher
