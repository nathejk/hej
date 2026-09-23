package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/nathejk/table/photo"
)

// The curator's patrol tags (PRD 022 §8.6, §11 Q1, task 377).
//
// # No roster, and no picker
//
// `publicpatrol.Queries` has exactly one method — `ByNumber(year, number)` — and its doc comment says the absence
// of a list read is deliberate: *"a list read is what a scraper would ask for"*. There is no patrol list anywhere
// in this service at any layer.
//
// An autocomplete picker would need one, so v1 does not build one. The curator types the number **from the
// patrol's sign**, which is how they know it; the tool resolves it through the existing read and shows the name
// back for confirmation. That gets the feature with no new read, no new enumerable surface, and no widening of a
// type unauthenticated handlers read through.
//
// Note this is the opposite decision from the checkpoint picker (task 376), and the difference is the point.
// There, enumerating the course is something the *curator* legitimately needs and no scraper can reach, so a
// separate interface was worth adding. Here the thing enumerated would be **every patrol in the event**, the
// curator already knows the one number they want, and the confirmation step covers the only thing a list would
// have added. If a picker is ever wanted it must be a separate authenticated interface, not a method on
// `publicpatrol.Queries`.
//
// # In v1 a tag has no public effect
//
// PRD 022 §11 Q1: tags will eventually surface a photograph on that patrol's own public page, and deliberately
// not yet — so the tagging can be used in anger and corrected before a mistag can put a photograph on the wrong
// family's page. `photo`'s own test asserts the public read cannot touch `photo_patrol`.

// adminPatrolResponse is one resolved patrol, for the curator to confirm.
//
// # What is here, and why none of it is a person
//
// A patrol's own name ("Ørnene"), its scout group and its korps label — exactly the attribution the public patrol
// page already carries. There is no member, no contact, no phone: `publicpatrol.Patrol` has no field for one, and
// the upstream event it is folded from carries a leader's name, phone and email that reach no column (see that
// package's doc). PRD 022 §4 repeats the rule specifically because a tagging UI is where somebody reaches for a
// name.
type adminPatrolResponse struct {
	// TeamID is the resolved identity, and what a tag is keyed on.
	TeamID string `json:"teamId"`
	// Number is what the curator typed, echoed back.
	Number string `json:"number"`
	// Name is the **patrol's** name, not a person's.
	Name string `json:"name,omitempty"`
	// Group is the scout group, e.g. "1. Søllerød Gruppe".
	Group string `json:"group,omitempty"`
	// Korps is the korps label rather than its slug, because a slug is an internal token and "DDS" is what a
	// human recognises.
	Korps string `json:"korps,omitempty"`
}

// resolveAdminPatrolHandler turns a patrol number into a patrol, for confirmation.
//
// @Summary      Resolve a patrol number
// @Description  Resolves the number printed on a patrol's sign to that patrol, so a curator can confirm which one they got before tagging photographs with it. Returns the patrol's own name, its scout group and its korps label — **never a person**: no member name, no contact, no telephone number. There is deliberately no endpoint that lists patrols, here or anywhere in this service: the curator types the number they already know, because a list read is what a scraper would ask for. An unknown number answers 404. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        number  path      string  true  "the number printed on the patrol's sign"
// @Success      200  {object}  adminPatrolResponse
// @Failure      400  {object}  map[string]string  "an unusable number"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "no patrol with that number this year"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the patrols are unavailable"
// @Router       /admin/patrols/{number} [get]
func (app *application) resolveAdminPatrolHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.PublicPatrols == nil {
		app.ServiceUnavailableResponse(w, r, "patruljerne er ikke tilgængelige lige nu")
		return
	}

	raw := httprouter.ParamsFromContext(r.Context()).ByName("number")
	// The same normalisation the public patrol page applies, reused rather than reimplemented: it drops leading
	// zeros so "042" and "42" are one patrol, which is what the number on the sign means.
	number, ok := normalizePatrolNumber(raw)
	if !ok {
		app.BadRequestResponse(w, r, errors.New("patruljenummeret skal v\u00e6re et tal"))
		return
	}

	p, found, err := app.models.PublicPatrols.ByNumber(app.config.eventYear, number)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		// A plain 404. Unlike the *public* patrol page, this need not be indistinguishable from a closed gate —
		// the caller is already behind the credential, and a curator who typed 42 instead of 24 deserves to be
		// told the number does not exist rather than left guessing.
		app.NotFoundResponse(w, r)
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, adminPatrolResponse{
		TeamID: p.TeamID,
		Number: p.Number,
		Name:   p.Name,
		Group:  p.GroupName,
		Korps:  p.KorpsLabel(),
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// tagAdminPhotosRequest tags a selection with one patrol.
type tagAdminPhotosRequest struct {
	PhotoIDs []string `json:"photoIds"`
	// Number is the patrol number the curator typed. Resolved server-side, for the same reason the checkpoint id
	// is: the client must not be the thing that decides which patrol a tag points at.
	Number string `json:"number"`
}

// tagAdminPhotosResponse reports the tag that was applied.
type tagAdminPhotosResponse struct {
	Tagged int                 `json:"tagged"`
	Patrol adminPatrolResponse `json:"patrol"`
	// Message is the sentence the action bar shows, written by the side that resolved the patrol.
	Message string `json:"message"`
}

// tagAdminPhotosHandler attributes a selection of photographs to one patrol.
//
// @Summary      Tag photographs with a patrol
// @Description  Attributes a whole selection of library photographs to one patrol, named by the number on its sign. The number is resolved server-side and the resolved `teamId` is what the tag stores — patrol numbers are **not unique per year** and are handed out late, so a tag keyed on the number alone could silently come to point at a different patrol. A photograph may carry several tags, because two patrols in one frame is ordinary. Re-tagging is a no-op rather than a duplicate. A tag names a patrol and **never a person**. In this version a tag has no public effect: it is curator metadata, so tagging can be used and corrected before it can put a photograph on the wrong family's page. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        request  body      tagAdminPhotosRequest  true  "the selection and the patrol number"
// @Success      200  {object}  tagAdminPhotosResponse
// @Failure      400  {object}  map[string]string  "no selection, or an unusable number"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "no patrol with that number this year"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the patrols or the event stream are unavailable"
// @Router       /admin/photos/tags [post]
func (app *application) tagAdminPhotosHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.PublicPatrols == nil {
		app.ServiceUnavailableResponse(w, r, "patruljerne er ikke tilgængelige lige nu")
		return
	}

	var in tagAdminPhotosRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	photoIDs, err := dedupeAdminIDs(in.PhotoIDs, "billeder")
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	number, ok := normalizePatrolNumber(in.Number)
	if !ok {
		app.BadRequestResponse(w, r, errors.New("patruljenummeret skal v\u00e6re et tal"))
		return
	}

	// Re-resolved here rather than trusting a teamId from the client. The confirmation step showed the curator a
	// name, but the request that follows must not be able to name a *different* patrol than the one confirmed —
	// and the only way to guarantee that is for the server to do the lookup both times.
	p, found, err := app.models.PublicPatrols.ByNumber(app.config.eventYear, number)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}
	if p.TeamID == "" {
		// Defensive: the fold refuses a tag with no team id, and a row with an empty one could never be matched,
		// untagged or joined. Refused here with a readable reason rather than as a dropped event in a log.
		app.ServerErrorResponse(w, r, fmt.Errorf("patrol %q resolved to no team id", number))
		return
	}

	now := time.Now().UTC()
	tagged := 0
	for _, photoID := range photoIDs {
		subject, serr := photo.Subject(app.config.eventYear, photoID, photo.VerbPatrolTagged)
		if serr != nil {
			app.BadRequestResponse(w, r, serr)
			return
		}
		if perr := app.commands.Publish(subject, photo.PatrolTagged{
			PhotoID:  photoID,
			Year:     app.config.eventYear,
			TeamID:   p.TeamID,
			Number:   p.Number,
			TaggedAt: now,
		}); perr != nil {
			app.Logger.Error("admin tag failed partway",
				"tagged", tagged, "photoId", photoID, "err", perr)
			app.writeAlbumPublishFailure(w, r, perr)
			return
		}
		tagged++
	}

	// The team id and the number, never the patrol's name: a log line is a durable record and there is no reason
	// for one to carry more than what identifies the tag.
	app.Logger.Info("admin tagged photographs with a patrol",
		"count", tagged, "teamId", p.TeamID, "number", p.Number, "ip", clientIP(r))

	billeder := "billeder"
	if tagged == 1 {
		billeder = "billede"
	}
	if err := app.WriteJSON(w, http.StatusOK, tagAdminPhotosResponse{
		Tagged: tagged,
		Patrol: adminPatrolResponse{
			TeamID: p.TeamID,
			Number: p.Number,
			Name:   p.Name,
			Group:  p.GroupName,
			Korps:  p.KorpsLabel(),
		},
		Message: fmt.Sprintf("%d %s tagget med patrulje %s.", tagged, billeder, p.Number),
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// untagAdminPhotoHandler removes one patrol attribution from one photograph.
//
// @Summary      Remove a patrol tag
// @Description  Removes exactly one patrol attribution from one photograph. Scoped by the year, the photograph and the team, so it cannot widen: without the photograph it would clear that patrol's tag on every photograph, and without the team it would clear every tag on that one. A soft removal, so a re-tag is expressible and an accidental untag is recoverable from the log. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        photoId  path      string  true  "library photograph id"
// @Param        teamId   path      string  true  "the patrol's team id, as returned when it was tagged"
// @Success      204  "untagged"
// @Failure      400  {object}  map[string]string  "a missing id"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the event stream is unavailable"
// @Router       /admin/photos/{photoId}/tags/{teamId} [delete]
func (app *application) untagAdminPhotoHandler(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	photoID := strings.TrimSpace(params.ByName("photoId"))
	teamID := strings.TrimSpace(params.ByName("teamId"))

	if photoID == "" || teamID == "" {
		app.BadRequestResponse(w, r, errors.New("både billede og patrulje skal angives"))
		return
	}

	subject, err := photo.Subject(app.config.eventYear, photoID, photo.VerbPatrolUntagged)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if perr := app.commands.Publish(subject, photo.PatrolUntagged{
		PhotoID:    photoID,
		Year:       app.config.eventYear,
		TeamID:     teamID,
		UntaggedAt: time.Now().UTC(),
	}); perr != nil {
		app.writeAlbumPublishFailure(w, r, perr)
		return
	}

	app.Logger.Info("admin removed a patrol tag",
		"photoId", photoID, "teamId", teamID, "ip", clientIP(r))

	w.WriteHeader(http.StatusNoContent)
}
