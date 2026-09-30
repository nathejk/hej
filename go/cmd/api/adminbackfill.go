package main

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/photo"
)

// Backfilling the 800px rendition onto photographs uploaded before it existed (task 433).
//
// # Why a backfill is needed at all, when task 430 rebuilds renditions on demand
//
// They solve different halves and it is worth being precise, because the names sound interchangeable.
//
// Task 430 **repairs** a rendition the projection already names: a row holds `mediumRef` but the bytes have
// gone, so a serve that misses rebuilds them under that same ref. It cannot help here, because these rows hold
// no `mediumRef` at all — there is no ref to fill.
//
// This is the other half: **producing** the rendition, and recording its ref on the row. That needs an event,
// which is why it is a write path behind the admin credential rather than something a read can do (PRD 008 §8:
// the projection is written by the consumer, never by the writer).
//
// # Why it is an endpoint and not a boot-time job
//
// A boot-time pass would decode and re-encode the whole library on every deploy, inside the window before the
// HTTP server starts listening — the exact failure shape task 431 was about, where housekeeping got in front
// of serving. An operator action is also honest about what this is: a one-off with a visible cost, run when
// somebody chooses to.
//
// # Why it is synchronous and batched rather than a background job
//
// Because a background job needs somewhere to report to, and the thing it would report is already the
// response here. Batching makes it resumable with no state: run it, read the counts, run it again until it
// reports nothing left. No queue, no progress table, no job that fails silently at 03:00.
//
// The batch is bounded by `adminBackfillMaxBatch` for the same reason `MissingMedium` insists on a limit: each
// row is a decode, a resize and an encode, so an unbounded batch is an unbounded amount of CPU in one request.

// adminBackfillDefaultBatch is how many photographs one call processes when the caller does not say.
//
// 25 rather than hundreds: at PRD 022 §6's figures a photograph is a ~3 MB decode, so this is a few seconds of
// work — long enough to make progress, short enough that a curator who runs it by accident is not stuck
// watching a spinner, and short enough to stay well inside any proxy's read timeout.
const adminBackfillDefaultBatch = 25

// adminBackfillMaxBatch caps what a caller may ask for.
//
// A ceiling rather than trust, because `?limit=` is a query parameter and the cost of a large one is paid by
// the process every other request is sharing.
const adminBackfillMaxBatch = 200

// adminBackfillResponse reports what one pass did.
//
// Counts rather than a list of ids: the caller is a person deciding whether to run it again, and three
// hundred hashes is not that. The ids of anything that failed go to the log, where an operator can act on
// them.
type adminBackfillResponse struct {
	// Examined is how many photographs lacking the rendition this pass looked at.
	//
	// **This is the loop condition.** Zero means there is nothing left to do; anything else means run it
	// again. Deliberately reported separately from Produced, because a pass that examined 25 and produced 0 is
	// a pass that is stuck, and conflating them would present that as "finished".
	Examined int `json:"examined"`

	// Produced is how many renditions were made and recorded.
	Produced int `json:"produced"`

	// Failed is how many could not be produced. Their ids are logged.
	//
	// A photograph whose source bytes have gone will fail here on every run, forever, which is why Examined
	// is the loop condition and not Failed: a library with one unreadable photograph must still be able to
	// finish, and the honest end state is "examined 25, produced 24, failed 1" repeating.
	Failed int `json:"failed"`

	Message string `json:"message"`
}

// backfillAdminRenditionsHandler produces the 800px rendition for photographs that have none.
//
// @Summary      Backfill the 800px rendition
// @Description  Produces the 800px rendition for photographs in the year's library that were uploaded before it existed, and records its ref. Processes one bounded batch per call and is safe to re-run: call it until `examined` is 0. The rendition is derived from the stored display image and stored in the blob store's **cache** class, so it is reproducible and outside the backup scope. A photograph whose stored bytes are missing is counted in `failed`, logged, and skipped — it will be retried on every run, so `examined` rather than `failed` is the loop condition. Nothing is overwritten: a photograph that already has the rendition is never examined. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        limit  query     int  false  "photographs to process, 1–200 (default 25)"
// @Success      200  {object}  adminBackfillResponse
// @Failure      400  {object}  map[string]string  "no working year, or one the tool does not know (X-Admin-Year or ?year=)"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the photograph library is unavailable"
// @Router       /admin/photos/renditions [post]
func (app *application) backfillAdminRenditionsHandler(w http.ResponseWriter, r *http.Request) {
	// The request is validated **before** the dependency is checked. A nonsense `limit` is wrong whatever the
	// state of the library, and answering 503 for it would invite a retry that fails identically; a 400 tells
	// the caller the thing they can actually fix.
	limit := adminBackfillDefaultBatch
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			app.BadRequestResponse(w, r, errors.New("limit skal v\u00e6re et positivt tal"))
			return
		}
		limit = min(n, adminBackfillMaxBatch)
	}

	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedbiblioteket er ikke tilg\u00e6ngeligt lige nu")
		return
	}
	// The working year is established by `requireAdminYear`, which wraps this route — see routes.go. Not
	// re-checked here, because two places deciding what the year is, is how they come to disagree.
	year := adminYear(r)

	rows, err := app.models.PhotoCurator.MissingMedium(year, limit)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	resp := adminBackfillResponse{Examined: len(rows)}
	for _, p := range rows {
		if app.produceMediumFor(r, year, p) {
			resp.Produced++
		} else {
			resp.Failed++
		}
	}

	switch {
	case resp.Examined == 0:
		resp.Message = "Alle billeder har en mellemstørrelse."
	case resp.Produced == 0:
		resp.Message = "Ingen billeder kunne behandles — se loggen."
	default:
		resp.Message = "Behandlede " + strconv.Itoa(resp.Produced) + " billede(r). Kør igen hvis der er flere."
	}

	app.Logger.Info("admin rendition backfill pass",
		"year", year, "examined", resp.Examined, "produced", resp.Produced, "failed", resp.Failed,
		"ip", clientIP(r))

	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// produceMediumFor makes and records one photograph's 800px rendition. Reports whether it succeeded.
//
// Order matters and is the same rule the upload path follows: **the bytes are stored before the event is
// published.** An event naming a ref that is not in the store yet is the unrecoverable direction — a reader
// would 404 on a rendition the row promises — while bytes with no event are merely an unreferenced object that
// the next pass will produce again and dedup for free.
func (app *application) produceMediumFor(r *http.Request, year string, p photo.LibraryPhoto) bool {
	ctx := r.Context()

	source := blob.Ref(p.Ref)
	if !source.Valid() {
		app.Logger.Error("backfill: a library row holds an unusable ref", "photoId", p.ID, "ref", p.Ref)
		return false
	}

	raw, err := app.readBlob(ctx, source)
	if err != nil {
		// The common failure, and a permanent one: the display image is gone, so there is nothing to derive
		// from. Warn rather than error — the library is still correct, this photograph simply cannot gain a
		// rendition, and it will be counted again on every run.
		app.Logger.Warn("backfill: cannot read a photograph's source bytes",
			"photoId", p.ID, "ref", p.Ref, "err", err)
		return false
	}

	// The same call the upload path makes, so a backfilled rendition is indistinguishable from an uploaded
	// one. Only the medium edge is asked for: the thumbnail already exists on these rows, and re-encoding it
	// would write a second copy of bytes the store already holds under a different hash.
	// Behind the decode gate (task 471), and this is the caller that needed it most: a backfill is a **loop**, so
	// without the gate it competes with every live upload for the whole event, at 83 MB a photograph. It yields a
	// slot between images rather than holding one for the run.
	var prepared imaging.Portrait
	if err := withDecodeSlot(ctx, func() error {
		var perr error
		prepared, perr = imaging.Prepare(raw, mediumEdge, []int{mediumEdge}, glimtJPEGQuality, false)
		return perr
	}); err != nil {
		app.Logger.Warn("backfill: cannot re-render a photograph", "photoId", p.ID, "err", err)
		return false
	}
	if len(prepared.Thumbs) == 0 || len(prepared.Thumbs[0].Bytes) == 0 {
		app.Logger.Warn("backfill: no rendition was produced", "photoId", p.ID)
		return false
	}

	// PutCache: derived from the display image, so outside the backup scope (task 429) and rebuildable on a
	// miss (task 430). This is the whole reason the backfill is cheap to justify — it adds nothing a backup
	// has to hold.
	ref, err := app.blobs.PutCache(ctx, prepared.Thumbs[0].Bytes)
	if err != nil {
		app.Logger.Error("backfill: storing a rendition", "photoId", p.ID, "err", err)
		return false
	}

	subject, err := photo.Subject(year, p.ID, photo.VerbMediumAdded)
	if err != nil {
		app.Logger.Error("backfill: building the subject", "photoId", p.ID, "err", err)
		return false
	}
	if err := app.commands.Publish(subject, photo.MediumAdded{
		PhotoID:   p.ID,
		Year:      year,
		MediumRef: ref.String(),
		AddedAt:   time.Now().UTC(),
	}); err != nil {
		// The bytes are stored and will be reused by the next pass, which content addressing makes free. So
		// this is a retry rather than a loss, and the count reports it as failed because the row did not
		// change.
		app.Logger.Error("backfill: publishing the rendition", "photoId", p.ID, "err", err)
		return false
	}
	return true
}
