package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/diploma"
)

// The diploma on a patrol's public page (PRD 011 §6, §11 Q6; task 345).
//
// # Generated here, not linked to
//
// PRD 011 §8 leaned on linking the visitor's browser to the sibling `diplom` service — a link is not a
// service-to-service call, so the org's architecture rule held. The maintainer decided on 2026-09-21 to move
// the generation here instead, which is §11 Q6 option (c). `internal/diploma` carries the reasoning and, more
// importantly, the one thing that could not come with it: **the patrol photograph**.
//
// # One definition of "finished", still
//
// The finish time comes from `publicgate.Verdict.FinishedAt` — the scan at the last checkgroup — which is the
// same value the gate opens on (task 330's trigger 1) and the same one the page prints. `diplom` derives it the
// same way. That is deliberate and worth keeping: three surfaces disagreeing about when a patrol finished is
// the bug this avoids.
//
// # Both routes are gated twice, and the second gate is not redundant
//
// `openPatrol` decides whether the patrol's page exists at all; `verdict.Finished()` decides whether there is a
// diploma. A backstop-opened patrol has a page and **no** diploma (task 346), so a route that checked only the
// first would hand out a certificate saying "har gennemført" to a patrol that did not.

// diplomaThumbEdge is the longest side of the thumbnail, in pixels.
//
// 600 rather than the ~200 the slot renders at: the page is read on phones with 2–3× pixel density, and a
// diploma is the one image on this page somebody will pinch to look at. It is one JPEG of the artwork, shared
// by every patrol and cached for an hour, so the cost of being generous is nothing.
const diplomaThumbEdge = 600

// diplomaCacheControl is a shorter window than media, and longer than the pages.
//
// The bytes are not content-addressed — they are generated per request from the artwork embedded in the binary
// — so `immutable` would be a lie the day the artwork is replaced (which it will be: see
// `diploma.ReplaceBeforeLaunch`). An hour is long enough that a burst is served from caches and short enough
// that a new deploy's artwork appears the same afternoon.
const diplomaCacheControl = "public, max-age=3600"

// patrolDiplomaHandler serves a finished patrol's diploma as a PDF.
//
// @Summary      A patrol's diploma (PDF)
// @Description  The diploma for a patrol that finished: the event's artwork, the patrol's own name, and the minute they crossed the line. Generated here rather than fetched from the `diplom` service (PRD 011 §11 Q6 option c), and **deliberately without the patrol photograph** that service includes — this surface is unauthenticated, and a photograph of eight children has been through no consent gate (PRD 011 §0b.2). Answers 404 for a patrol that did not finish, identically to one that does not exist: a page opened by the backstop has no diploma at all. Unauthenticated; ignores the session cookie.
// @Tags         public-site
// @Produce      application/pdf
// @Param        number  path      string  true  "patrol number"
// @Success      200  {file}    binary  "the diploma"
// @Failure      404  {object}  map[string]string  "unknown patrol, page not open, or the patrol did not finish"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Router       /public/patrol/{number}/diploma [get]
func (app *application) patrolDiplomaHandler(w http.ResponseWriter, r *http.Request) {
	// The media budget, not the page one (task 347): a PDF is a file a visitor saves, and it belongs with the
	// thumbnails rather than with the HTML.
	if !app.allowPublicMediaRead(w, r) {
		return
	}

	d, ok := app.patrolDiploma(r)
	if !ok {
		app.NotFoundResponse(w, r)
		return
	}

	// Inline, with a filename: the browser's own viewer is the whole delivery mechanism here (PRD 011 §4
	// rules out printing, posting and emailing), and a download that lands as `diploma.pdf` in a folder of
	// other people's diplomas is no use to anybody.
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`inline; filename="nathejk-diplom-patrulje-%s.pdf"`, d.Number))
	w.Header().Set("Cache-Control", diplomaCacheControl)

	if err := diploma.PDF(d, w); err != nil {
		// The response has begun — a PDF is streamed — so there is nothing to answer with. Logged, as the
		// public templates do for the same reason.
		app.Logger.Error("rendering a diploma", "patrol", d.Number, "err", err)
	}
}

// patrolDiplomaThumbHandler serves the thumbnail the patrol page shows.
//
// @Summary      A patrol's diploma thumbnail (JPEG)
// @Description  The diploma's artwork, scaled for the slot on the patrol page. Carries no text — it is a picture of the diploma rather than a small diploma, since a patrol's name at this size would be illegible (see internal/diploma.Thumbnail). Gated exactly as the PDF is: a patrol that did not finish has no thumbnail either, so the page cannot show a frame with nothing behind it. Unauthenticated.
// @Tags         public-site
// @Produce      image/jpeg
// @Param        number  path      string  true  "patrol number"
// @Success      200  {file}    binary  "the thumbnail"
// @Failure      404  {object}  map[string]string  "unknown patrol, page not open, or the patrol did not finish"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      500  {object}  map[string]string  "the artwork could not be rendered"
// @Router       /public/patrol/{number}/diploma/thumb [get]
func (app *application) patrolDiplomaThumbHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicMediaRead(w, r) {
		return
	}

	if _, ok := app.patrolDiploma(r); !ok {
		app.NotFoundResponse(w, r)
		return
	}

	bytes, err := app.diplomaThumbnail()
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", diplomaCacheControl)
	if _, err := w.Write(bytes); err != nil {
		app.Logger.Error("writing a diploma thumbnail", "err", err)
	}
}

// patrolDiploma resolves a request to the diploma it may have, or reports that there is none.
//
// One place for both routes, so the PDF and the thumbnail cannot disagree about who has a diploma — the same
// reason `openPatrol` exists for the page and the map.
func (app *application) patrolDiploma(r *http.Request) (diploma.Diploma, bool) {
	number, ok := normalizePatrolNumber(httprouter.ParamsFromContext(r.Context()).ByName("number"))
	if !ok {
		return diploma.Diploma{}, false
	}

	patrol, verdict, open := app.openPatrol(number)
	if !open {
		return diploma.Diploma{}, false
	}
	// **The second gate.** A page the backstop opened has no diploma to give (task 346), and "har gennemført"
	// is not a sentence to print for a patrol that did not.
	if !verdict.Finished() {
		return diploma.Diploma{}, false
	}

	finishedAt := verdict.FinishedAt
	if finishedAt != nil {
		// In the event's own timezone, as `diplom` does: the clock on a certificate is the one the patrol
		// looked at when they crossed the line, not UTC.
		local := finishedAt.In(app.eventLocation())
		finishedAt = &local
	}

	return diploma.Diploma{
		Number:     patrol.Number,
		Name:       patrol.Name,
		Title:      fmt.Sprintf("Nathejk %s", app.config.eventYear),
		Route:      app.config.eventRoute,
		FinishedAt: finishedAt,
	}, true
}

// eventLocation is the timezone the event happens in.
//
// Fixed to Copenhagen rather than configurable: Nathejk is a Danish night race, the diploma prints a local
// clock time, and a configurable timezone would be a knob with exactly one correct setting. Falls back to UTC
// if the zoneinfo database is missing from the image, which would be a deployment fault rather than a reason
// to fail a render — an hour's error on a certificate beats no certificate.
func (app *application) eventLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		app.Logger.Error("loading Europe/Copenhagen; diploma times will be UTC", "err", err)
		return time.UTC
	}
	return loc
}

// diplomaThumbnail renders the thumbnail once and keeps it.
//
// The artwork is identical for every patrol and embedded in the binary, so decoding and scaling a 1240×1754
// JPEG per request would be pure waste on an unauthenticated route — the same reasoning as the merged track's
// cache (task 347), at a fraction of the cost.
func (app *application) diplomaThumbnail() ([]byte, error) {
	app.diplomaThumbOnce.Do(func() {
		app.diplomaThumbBytes, app.diplomaThumbErr = diploma.Thumbnail(diplomaThumbEdge)
	})
	return app.diplomaThumbBytes, app.diplomaThumbErr
}
