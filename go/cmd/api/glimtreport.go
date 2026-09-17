package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
)

// Reporting a glimt (PRD 019 §0, §6, task 307).
//
// # This is the safety mechanism, not a safety valve
//
// The public scope publishes with no approval queue in front of it (PRD 019 §0) — a decision taken
// knowingly, in exchange for shipping. What makes that acceptable is that **a report acts
// immediately and without waiting for a human**. Everything about this endpoint follows from that:
//
//   - The hide happens in the same projection fold as the report is recorded (task 301), so there is
//     no window in which the objected-to thing is still on the open web.
//   - It is reachable in one tap from every card (task 316), not behind a long-press.
//   - The reporter never learns who posted it, because nobody outside moderation does.
//   - It is rate-limited, but generously: the cost of a spurious report is a moderator glancing at
//     something, and the cost of a blocked one is a photograph staying up.
//
// A member may only report what they can see. That is not about protecting authors — it is that a
// report on an invisible glimt would be a way to confirm a glimt exists, which is the one thing the
// 403 in the media handler is careful not to do.

// maxGlimtReportReason bounds the free-text note.
//
// Short on purpose. This is a pointer for whoever reviews it — "det er ikke ok" — not an incident
// report, and a long field invites writing things about other people that then live in a log.
const maxGlimtReportReason = 500

type reportGlimtRequest struct {
	Reason string `json:"reason"`
}

// reportGlimtHandler flags a glimt and hides it from the public scope.
//
// @Summary      Report a glimt
// @Description  Flags a glimt for the Team section to review, and hides it from the public feed immediately — before any human looks at it, because the public scope has no approval queue in front of it. The optional `reason` is a short free-text note for whoever reviews it. Reporting the same glimt twice has no additional effect. A member may only report a glimt they can see. The response never reveals who posted it.
// @Tags         glimt
// @Accept       json
// @Produce      json
// @Param        glimtId  path      string              true   "glimt id"
// @Param        request  body      reportGlimtRequest  false  "optional reason"
// @Success      204  "reported and hidden from the public feed"
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "not shared with you"
// @Failure      404  {object}  map[string]string
// @Failure      429  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /glimt/items/{glimtId}/report [post]
func (app *application) reportGlimtHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, "glimt er ikke tilgængelige lige nu")
		return
	}

	// Generous, and deliberately looser than the create limit. The asymmetry is the point: the
	// cost of a spurious report is a moderator glancing at something, while the cost of a
	// throttled one is a photograph somebody objected to staying up. A limit exists only so the
	// endpoint cannot be hammered.
	if app.glimtReportLimiter != nil && !app.glimtReportLimiter.Allow(s.UserID) {
		app.RateLimitMessageResponse(w, r,
			"Du har anmeldt mange glimt på kort tid. Prøv igen om lidt.")
		return
	}

	glimtID := httprouter.ParamsFromContext(r.Context()).ByName("glimtId")

	// An absent body is fine — the reason is optional, and the common case is a member tapping
	// "Anmeld" and confirming. Only a malformed body is an error.
	var in reportGlimtRequest
	if r.ContentLength > 0 {
		if err := app.ReadJSON(w, r, &in); err != nil {
			app.BadRequestResponse(w, r, err)
			return
		}
	}
	reason := strings.TrimSpace(in.Reason)
	if len([]rune(reason)) > maxGlimtReportReason {
		app.BadRequestResponse(w, r, errors.New("beskrivelsen er for lang"))
		return
	}

	g, found, err := app.models.Glimt.Get(app.config.eventYear, glimtID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	viewer, vfound := app.models.Users.Get(s.UserID)
	if !vfound {
		app.NotFoundResponse(w, r)
		return
	}
	// Only what the caller can see. Not to protect the author — a report on an invisible glimt
	// would be a way to confirm that one exists, which is exactly what the media handler's 403
	// is careful not to leak.
	if !users.MaySeeGlimt(app.glimtViewer(s.UserID, viewer.Role), glimtSubjectOf(g)) {
		app.ForbiddenResponse(w, r)
		return
	}

	subject, serr := glimt.Subject(app.config.eventYear, glimtID, glimt.VerbReported)
	if serr != nil {
		app.ServerErrorResponse(w, r, serr)
		return
	}

	if perr := app.commands.Publish(subject, glimt.Reported{
		GlimtID: glimtID,
		Year:    app.config.eventYear,
		// The reporter is recorded, and it is the primary key that makes the count honest:
		// without it one person tapping twice would read as two people objecting, and that
		// count is what orders the moderation queue.
		ReporterPersonID: s.UserID,
		Reason:           reason,
		ReportedAt:       time.Now().UTC(),
	}); perr != nil {
		if errors.Is(perr, commands.ErrNoPublisher) {
			// 503, and the client should say so plainly. A report that silently failed
			// would be the worst possible lie in this feature: the member believes they
			// have acted, and the photograph stays up.
			app.ServiceUnavailableResponse(w, r, "kunne ikke anmelde glimtet, prøv igen")
			return
		}
		app.ServerErrorResponse(w, r, perr)
		return
	}

	// 204 rather than the reported glimt. There is nothing useful to return, and returning the
	// glimt would invite the client to render a state ("reported by you") that is not worth the
	// payload — the card simply disappears from the public feed and the reporter moves on.
	w.WriteHeader(http.StatusNoContent)
}
