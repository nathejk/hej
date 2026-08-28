package main

import (
	"errors"
	"net/http"
	"time"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/track"
)

// trackRequest is the batch a client uploads.
//
// Note what is NOT here: any field naming the person. The session decides whose track this
// is, and because ReadJSON sets DisallowUnknownFields, a body that tries to name someone
// else is rejected with a 400 rather than quietly ignored. That is a stronger property than
// "we overwrite it server-side" — the attempt cannot even be expressed.
type trackRequest struct {
	Points []track.Point `json:"points"`
}

// createTrackHandler accepts a batch of recorded positions from the signed-in user and
// publishes it to the telemetry stream. Runs behind requireAuth.
//
// It writes no SQL, by design and not by omission: every state change in this service is an
// event, and SQL is only ever a projection of the log (PRD 008 §8). The track has no
// projection in this repo at all — 086 reads it back off the stream.
//
// @Summary      Upload a batch of recorded positions
// @Description  Publishes the signed-in user's position batch to the telemetry stream. The person is taken from the session; the body cannot name anyone. Invalid points are dropped and counted rather than failing the batch. Writes no SQL.
// @Tags         track
// @Accept       json
// @Produce      json
// @Param        request  body      trackRequest  true  "Recorded positions"
// @Success      202      {object}  map[string]int  "accepted and dropped point counts"
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Failure      413      {object}  map[string]string
// @Failure      429      {object}  map[string]string
// @Failure      503      {object}  map[string]string  "broker unreachable — retry"
// @Router       /track [post]
func (app *application) createTrackHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Per USER, not per IP. Participants share networks — a patrol walks together on one
	// phone's hotspot, and a whole klan can sit behind one carrier NAT — so an IP limit
	// would throttle innocent members in groups while still allowing one member to flood
	// on their own. The user is the meaningful axis for an authenticated ingest endpoint.
	if !app.trackLimiter.Allow(s.UserID) {
		app.RateLimitResponse(w, r)
		return
	}

	var input trackRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	if len(input.Points) == 0 {
		app.BadRequestResponse(w, r, track.ErrEmptyBatch)
		return
	}
	if len(input.Points) > track.MaxPointsPerBatch {
		// A distinct 413 rather than a 400: the client can act on "too big" by splitting
		// the batch, which is exactly what task 083's chunking does, and cannot act on a
		// generic bad request.
		app.PayloadTooLargeResponse(w, r, track.ErrBatchTooLarge)
		return
	}

	kept, dropped := track.Clean(input.Points, time.Now())

	// Every point was junk. Deliberately a 202 with accepted: 0 and not an error: there is
	// nothing to publish, but there is also nothing for the client to fix by retrying, and
	// a 4xx would make it retry the same batch forever. Accepting it lets the client mark
	// the points done and move on, and the dropped count is what makes that visible.
	if len(kept) == 0 {
		app.respondTrackAccepted(w, r, 0, dropped)
		return
	}

	subject, err := track.Subject(app.config.eventYear, s.UserID)
	if err != nil {
		// A person id or configured year that cannot be a subject token is our problem,
		// not the caller's.
		app.ServerErrorResponse(w, r, err)
		return
	}

	body := track.Reported{PersonID: s.UserID, Year: app.config.eventYear, Points: kept}
	if err := app.commands.Publish(subject, body); err != nil {
		if errors.Is(err, commands.ErrNoPublisher) {
			// The one failure mode this endpoint must get right: the batch has NOT been
			// stored anywhere, so saying 202 would tell the client to delete the only
			// copy that exists (it lives in the phone's IndexedDB until accepted). A 503
			// keeps the points pending and retried, which is task 083's contract.
			app.ServiceUnavailableResponse(w, r, "event stream unavailable, retry later")
			return
		}
		// Same reasoning for any other publish failure — including a subject no stream
		// claims, which the acked JetStream publish does surface as an error.
		app.ServiceUnavailableResponse(w, r, "could not publish track, retry later")
		return
	}

	app.respondTrackAccepted(w, r, len(kept), dropped)
}

func (app *application) respondTrackAccepted(w http.ResponseWriter, r *http.Request, accepted, dropped int) {
	payload := map[string]int{"accepted": accepted, "dropped": dropped}
	if err := app.WriteJSON(w, http.StatusAccepted, payload, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
