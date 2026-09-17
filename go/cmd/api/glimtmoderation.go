package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/commands"
	"nathejk.dk/nathejk/table/glimt"
)

// Team-section moderation (PRD 019 §0, §6, §8, task 308).
//
// # Three endpoints, one gate
//
// The queue, hide, and unhide. All three require the **current** Team-section assignment, looked up
// per request (task 300) rather than read from a session claim — so taking the assignment away takes
// the capability away on the next request, which a signed cookie could never promise.
//
// # This is the widest read in the service
//
// `glimt.Queries.Moderation` takes no visibility filter, because there is no narrowing to apply: the
// Team section sees every scope, including group-scoped glimt from groups they are not in. That makes
// the gate below the **only** thing standing between this handler and every photograph in the event.
// It is checked first, before the id is even parsed, and there is no path through this file that
// reaches a read without it.
//
// PRD 019 §6 requires that reach to be *disclosed* rather than discovered — the composer and the
// privacy page say so (tasks 317, 321). That is a UI obligation, but it is recorded here because this
// file is what makes it true.
//
// # Hiding is not deleting
//
// `hide` sets a column and publishes an event; the media survives, so `unhide` can put it back. Only
// the author's DELETE destroys bytes (task 306). That asymmetry is what makes hiding cheap enough to
// be the automatic response to a report: if a takedown were irreversible, the safe reaction to an
// ambiguous report would be to wait for a human, and waiting is what the public scope cannot afford.

// maxGlimtModerationReason bounds the note a moderator leaves.
const maxGlimtModerationReason = 500

type moderateGlimtRequest struct {
	Reason string `json:"reason"`
}

// glimtModerationResponse is the queue.
type glimtModerationResponse struct {
	Glimt []moderationGlimtResponse `json:"glimt"`
}

// requireGlimtModerator resolves the caller and refuses anyone without the Team-section assignment.
//
// Returns the person id and false when it has already written a response, so callers can `return`
// immediately. Written as a guard rather than middleware because the lookup needs `app`, and because
// a reader of any of the three handlers below should see the check in the handler rather than have to
// trust a route registration elsewhere.
func (app *application) requireGlimtModerator(w http.ResponseWriter, r *http.Request) (string, bool) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return "", false
	}
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, "glimt er ikke tilgængelige lige nu")
		return "", false
	}
	if !app.isGlimtModerator(s.UserID) {
		// 403, not 404. Hiding the endpoint's existence from a logged-in member buys
		// nothing — the client bundle names it — and an honest refusal is easier to
		// diagnose when somebody's assignment has not been made yet.
		app.ForbiddenResponse(w, r)
		return "", false
	}
	return s.UserID, true
}

// listGlimtModerationHandler serves the review queue.
//
// @Summary      Moderation queue (Team section only)
// @Description  Every glimt at every scope, including group-scoped ones and ones already hidden, ordered with reported-and-not-yet-hidden first, then by report count, then newest. Requires the Team section assignment, which is read from the caller's current record on every request rather than from the session — so revoking the assignment revokes access immediately. This is the only response in the API that carries the author of a glimt, because a report cannot be answered against an anonymous author.
// @Tags         glimt-moderation
// @Produce      json
// @Param        limit   query     int  false  "page size (default 20, max 200)"
// @Param        offset  query     int  false  "rows to skip"
// @Success      200  {object}  glimtModerationResponse
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not in the Team section"
// @Failure      503  {object}  map[string]string
// @Router       /glimt/moderation [get]
func (app *application) listGlimtModerationHandler(w http.ResponseWriter, r *http.Request) {
	moderatorID, ok := app.requireGlimtModerator(w, r)
	if !ok {
		return
	}

	limit, offset := pageParams(r)
	rows, err := app.models.Glimt.Moderation(app.config.eventYear, limit, offset)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	out := glimtModerationResponse{Glimt: make([]moderationGlimtResponse, 0, len(rows))}
	for _, g := range rows {
		entry := moderationGlimtResponse{
			glimtResponse:  newGlimtResponse(g, moderatorID),
			AuthorPersonID: g.AuthorPersonID,
			ReportCount:    g.ReportCount,
			HiddenBy:       g.HiddenBy,
		}
		// Resolved because a bare person id is not something a human moderating at 03:00 can
		// act on. Best effort: a missing person row leaves the name empty rather than
		// failing the queue, since an unattributable glimt is exactly one somebody needs to
		// look at.
		if p, found := app.person(g.AuthorPersonID); found {
			entry.AuthorName = p.Name
		}
		out.Glimt = append(out.Glimt, entry)
	}

	// Never cached. The queue is a live operational view during an event, and a stale one would
	// have a moderator reviewing something already handled — or worse, believing a reported
	// glimt is still up.
	w.Header().Set("Cache-Control", "no-store")

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// hideGlimtHandler takes a glimt down.
//
// @Summary      Hide a glimt (Team section only)
// @Description  Takes a glimt down from every audience. Reversible and non-destructive: the row and the stored media survive, so /unhide can restore it, and only the author's DELETE destroys bytes. Records who hid it. Hiding an already-hidden glimt is harmless. Requires the current Team section assignment.
// @Tags         glimt-moderation
// @Accept       json
// @Produce      json
// @Param        glimtId  path      string                true   "glimt id"
// @Param        request  body      moderateGlimtRequest  false  "optional reason"
// @Success      204  "hidden"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not in the Team section"
// @Failure      404  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /glimt/items/{glimtId}/hide [post]
func (app *application) hideGlimtHandler(w http.ResponseWriter, r *http.Request) {
	app.moderateGlimt(w, r, glimt.VerbHidden)
}

// unhideGlimtHandler restores a glimt a report or a moderator took down.
//
// @Summary      Restore a hidden glimt (Team section only)
// @Description  Clears the hidden state so the glimt returns to its original audience — which is unchanged, because audience is immutable. This is what makes hiding cheap enough to be the automatic response to a report: an irreversible takedown would push a moderator towards waiting for a human, and the public scope cannot afford waiting. Requires the current Team section assignment.
// @Tags         glimt-moderation
// @Accept       json
// @Produce      json
// @Param        glimtId  path      string                true   "glimt id"
// @Param        request  body      moderateGlimtRequest  false  "optional reason"
// @Success      204  "restored"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not in the Team section"
// @Failure      404  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /glimt/items/{glimtId}/unhide [post]
func (app *application) unhideGlimtHandler(w http.ResponseWriter, r *http.Request) {
	app.moderateGlimt(w, r, glimt.VerbUnhidden)
}

// moderateGlimt is hide and unhide, which differ only in the verb and the body type.
//
// One function rather than two near-identical ones: everything that could go wrong — the gate, the
// lookup, the reason length, the publish failure — is identical, and two copies would be two places
// for the gate to be forgotten.
func (app *application) moderateGlimt(w http.ResponseWriter, r *http.Request, verb string) {
	moderatorID, ok := app.requireGlimtModerator(w, r)
	if !ok {
		return
	}

	glimtID := httprouter.ParamsFromContext(r.Context()).ByName("glimtId")

	var in moderateGlimtRequest
	if r.ContentLength > 0 {
		if err := app.ReadJSON(w, r, &in); err != nil {
			app.BadRequestResponse(w, r, err)
			return
		}
	}
	reason := strings.TrimSpace(in.Reason)
	if len([]rune(reason)) > maxGlimtModerationReason {
		app.BadRequestResponse(w, r, errors.New("beskrivelsen er for lang"))
		return
	}

	// Confirmed to exist before publishing, so a typo'd id is a 404 rather than an event about
	// nothing that a replay would then apply to a glimt created later with that id.
	if _, found, err := app.models.Glimt.Get(app.config.eventYear, glimtID); err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	} else if !found {
		app.NotFoundResponse(w, r)
		return
	}

	subject, serr := glimt.Subject(app.config.eventYear, glimtID, verb)
	if serr != nil {
		app.ServerErrorResponse(w, r, serr)
		return
	}

	var body any
	now := time.Now().UTC()
	if verb == glimt.VerbHidden {
		body = glimt.Hidden{
			GlimtID: glimtID,
			Year:    app.config.eventYear,
			// Attributable to a person, not to "the system". A takedown of a
			// participant's photograph is a decision somebody made.
			HiddenBy: moderatorID,
			Reason:   reason,
			HiddenAt: now,
		}
	} else {
		body = glimt.Unhidden{
			GlimtID:    glimtID,
			Year:       app.config.eventYear,
			UnhiddenBy: moderatorID,
			Reason:     reason,
			UnhiddenAt: now,
		}
	}

	if perr := app.commands.Publish(subject, body); perr != nil {
		if errors.Is(perr, commands.ErrNoPublisher) {
			app.ServiceUnavailableResponse(w, r, "kunne ikke gemme handlingen, prøv igen")
			return
		}
		app.ServerErrorResponse(w, r, perr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
