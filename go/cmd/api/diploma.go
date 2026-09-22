package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/diploma"
	"nathejk.dk/internal/eventtime"
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
	// **There is no second gate any more** (task 360). It used to refuse a diploma unless the patrol finished,
	// so that "har gennemført" could never be printed for one that did not (task 346).
	//
	// The maintainer's instruction reverses it: *"there are two different wordings depending on if at least one
	// member from patrulje finished or not"*. `diplom` has printed both since 2024. So the finish now selects the
	// **sentence** rather than deciding whether the document exists — which is where the distinction belongs, and
	// it keeps a patrol that walked all night from being handed nothing.
	//
	// `FinishedAt` nil is exactly "nobody from this patrol reached the finish": the gate reads the patrol's scan
	// at the last checkgroup, and a checkpoint scans the patrol rather than its members.

	finishedAt := verdict.FinishedAt
	if finishedAt != nil {
		// In the event's own timezone, as `diplom` does: the clock on a certificate is the one the patrol
		// looked at when they crossed the line, not UTC.
		local := finishedAt.In(app.eventLocation())
		finishedAt = &local
	}

	// The photograph (task 361). Resolved here rather than in the renderer, which stays a pure function of a
	// patrol's facts plus bytes.
	photo, contentType := app.diplomaPhoto(r, patrol.TeamID)

	return diploma.Diploma{
		Number:           patrol.Number,
		Name:             patrol.Name,
		Title:            fmt.Sprintf("Nathejk %s", app.config.eventYear),
		Route:            app.diplomaRoute(),
		FinishedAt:       finishedAt,
		Photo:            photo,
		PhotoContentType: contentType,
	}, true
}

// diplomaPhoto resolves the patrol's photograph to bytes, or nothing.
//
// # Every failure is "no photograph", never "no diploma"
//
// There are five ways this comes back empty — no projection, no photograph for this patrol, no fetcher
// configured, foto unreachable, bytes that do not match their ref — and a family waiting for a certificate cares
// about none of them. So the photograph is decoration with a silent fallback, and the failures that deserve
// attention are logged where they happen (`internal/photobytes` logs a hash mismatch loudly, because that one is
// corruption or substitution rather than an outage).
//
// The request's context is passed through, so a visitor who closes the tab does not leave a fetch running — and
// the single-flight means the fetch continues for whoever else is waiting on the same photograph.
func (app *application) diplomaPhoto(r *http.Request, teamID string) ([]byte, string) {
	if app.models.PatrolPhotos == nil || app.photos == nil {
		return nil, ""
	}

	cover, found, err := app.models.PatrolPhotos.Cover(app.config.eventYear, teamID)
	if err != nil {
		app.Logger.Error("reading a patrol's cover photograph", "team", teamID, "err", err)
		return nil, ""
	}
	if !found {
		// Not photographed. Ordinary, and the diploma is worth printing without a picture.
		return nil, ""
	}

	data, err := app.photos.Bytes(r.Context(), blob.Ref(cover.Ref))
	if err != nil {
		// Info, not error: an unreachable foto or an object that has been purged is an expected state, and this
		// route is public enough that logging it at error level would be a way to fill the log from outside.
		app.Logger.Info("a patrol's photograph is unavailable", "team", teamID, "ref", cover.Ref, "err", err)
		return nil, ""
	}
	return data, cover.ContentType
}

// diplomaRoute is the route line, or "" to omit it.
//
// # Two sources, and why the operator's wins
//
// The **projection** is the data: hq's year entity carries `cityDeparture` and `cityDestination`, and task 357
// copied that fold into this app precisely so a diploma stops depending on a place name hardcoded in a sibling
// service. That is the normal path.
//
// `EVENT_ROUTE` stays as an override, and it takes precedence when set, because it is the only thing that can
// fix a wrong line without a release — the same argument as every other operational override here. An operator
// setting it has seen the diploma; the projection has only seen an event.
//
// # Every failure omits the line rather than breaking the diploma
//
// A missing projection, a database error, a year nobody has filled in, one city without the other: all of them
// return "", which is exactly what shipped before this existed. A certificate with no route reads fine; one
// that fails to render, or names half a journey, does not.
func (app *application) diplomaRoute() string {
	if app.config.eventRoute != "" {
		return app.config.eventRoute
	}
	if app.models.Years == nil {
		return ""
	}

	from, to, ok, err := app.models.Years.Route(app.config.eventYear)
	if err != nil {
		// Logged rather than returned: the caller's job is to produce a diploma, and this line is decoration on
		// it. Silence here would hide a broken projection, so it is logged once per request that needed it.
		app.Logger.Error("reading the event route", "year", app.config.eventYear, "err", err)
		return ""
	}
	if !ok {
		return ""
	}

	// The Danish phrase is composed here rather than in the renderer or the table: the table stores what the
	// organizers typed, and `internal/diploma` prints the line it is given. This is the one place that knows
	// two place names make a sentence.
	return fmt.Sprintf("fra %s til %s", from, to)
}

// eventLocation is the timezone the event happens in.
//
// Now a thin wrapper over `internal/eventtime`, which every rendered timestamp in this app goes through (task
// 358). It was the first place to get this right — the diploma has converted since task 345 — and being the only
// one is what made the public pages' UTC clocks hard to notice: one surface was correct.
//
// Kept as a method rather than inlined at the call site because the conversion there reads as *the diploma's*
// decision, and because the reason it is Copenhagen and not configurable belongs next to the certificate: a
// diploma prints the clock the patrol looked at when they crossed the line, and a configurable timezone would be
// a knob with exactly one correct setting. The UTC fallback and its reasoning now live in eventtime.
func (app *application) eventLocation() *time.Location {
	return eventtime.Location()
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
