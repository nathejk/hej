package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/photo"
)

// The curator's bulk position (PRD 022 §6, task 376).
//
// # Bulk is the requirement, not a convenience
//
// PRD 022 §3: "these forty are from Post 3" is the true shape of the work, and doing it forty times is how it
// does not get done. The checkpoint picker exists for the same reason — a curator knows the post, not the
// coordinate, and making them read a number off a map is asking them to do a conversion the service can do.
//
// # The bounds check is re-run on every set, and that is the point
//
// A curator-placed point is **not** exempt from it. Nothing reaches the public map unverified, which is the
// whole claim the four verdicts exist to support (PRD 011 §6). Two consequences worth stating because both look
// like bugs from the outside:
//
//   - A curator can place a point and be told it is `outside`. The coordinate is **kept** and shown as rejected,
//     because a curator has to be able to see that something was refused rather than wonder why a pin is
//     missing.
//   - Early in the season, with no checkpoint yet sited, there is no race area to judge against and the verdict
//     is `unknown` — not `outside`. `unknown` is a statement about *us*, and blaming the photograph for our
//     missing data would condemn every early upload permanently.

// patchAdminPhotosRequest sets or clears fields across a selection.
//
// # Why one endpoint with a mode rather than two
//
// Setting and clearing are separate *events* (PRD 022 §8.7) but the same *action* from the curator's side: they
// pick a selection and say where it is, or that they no longer know. Splitting the HTTP surface would mean the
// UI holds the same selection against two endpoints and has to decide which — and the decision is already
// expressed by whether a location was given.
type patchAdminPhotosRequest struct {
	PhotoIDs []string `json:"photoIds"`

	// Location is the point to set. Mutually exclusive with ClearLocation.
	//
	// A pointer, so "not mentioned" and "set" are distinguishable — the same reason `photo.Updated`'s fields are
	// pointers. A request that mentions neither is refused rather than treated as a no-op, because a UI that
	// sent one is broken and silence would hide it.
	Location *patchAdminLocation `json:"location,omitempty"`

	// CheckpointID sets the location from one of the event's sited checkpoints.
	//
	// The alternative to Location, and the one a curator actually uses. Resolved server-side: the client sends
	// the post, not the coordinate, so a stale or wrong coordinate in a browser cannot become a pin on a public
	// map.
	CheckpointID string `json:"checkpointId,omitempty"`

	// Caption sets the curator's words on the selection.
	//
	// On the **photograph**, not on an album item (PRD 022 §8.3): one place to edit, shared by every album it
	// appears in. A per-album override is imaginable and deliberately deferred (§11 Q3) rather than built
	// speculatively — the cost of guessing wrong is the exact bug the library split removed, two copies of one
	// fact drifting apart.
	//
	// A pointer, so clearing a caption and not mentioning one are different requests.
	Caption *string `json:"caption,omitempty"`

	// ClearLocation removes the coordinate.
	ClearLocation bool `json:"clearLocation,omitempty"`

	// Reason is the curator's note on a clear, for the log. Never shown publicly.
	Reason string `json:"reason,omitempty"`
}

type patchAdminLocation struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// patchAdminPhotosResponse reports what was set, and what the bounds check made of it.
type patchAdminPhotosResponse struct {
	// Updated is how many photographs were changed.
	Updated int `json:"updated"`

	// BoundsVerdict is the verdict the coordinate received, or empty on a clear.
	//
	// One verdict for the whole selection, because one coordinate was applied to all of them — which is also why
	// this is worth returning rather than making the client re-read: a curator who has just placed forty
	// photographs outside the race area needs telling now, not on the next reload.
	BoundsVerdict string `json:"boundsVerdict,omitempty"`

	// Plottable is whether that verdict reaches the public map. Derived server-side so the client cannot
	// disagree with `photo.Plottable` about the one rule that matters here.
	Plottable bool `json:"plottable"`

	// Message is the sentence the action bar shows, written by the side that knows which verdict happened.
	Message string `json:"message"`
}

// patchAdminPhotosHandler sets or clears a location across a selection.
//
// @Summary      Set or clear a location on many photographs
// @Description  Sets one coordinate on a whole selection of library photographs, or clears theirs. The point may be given directly as `location`, or — preferably — as a `checkpointId`, which the server resolves to that checkpoint's coordinate so a stale coordinate in a browser cannot become a pin on a public map. **The race-area bounds check is re-run for every set**, and the resulting verdict is stored: a curator-placed point is not exempt, and only an `inside` verdict ever reaches the public map. A point judged `outside` is kept and reported as rejected rather than discarded, so the curator can see what was refused. With no checkpoint yet sited there is no area to judge against and the verdict is `unknown`, which is a statement about us rather than about the photograph. Clearing publishes a distinct event, so the log records the intent. Requires the admin credential.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        request  body      patchAdminPhotosRequest  true  "the selection, and either a location, a checkpointId or clearLocation"
// @Success      200  {object}  patchAdminPhotosResponse
// @Failure      400  {object}  map[string]string  "no selection, no action, or contradictory actions"
// @Failure      401  {object}  map[string]string  "missing or wrong admin credential"
// @Failure      404  {object}  map[string]string  "an unknown checkpoint"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the library or the event stream are unavailable"
// @Router       /admin/photos [patch]
func (app *application) patchAdminPhotosHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return
	}

	var in patchAdminPhotosRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	photoIDs, err := dedupeAdminIDs(in.PhotoIDs, "billeder")
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if len(photoIDs) > maxAdminSelection {
		app.BadRequestResponse(w, r, errors.New("for mange billeder på én gang"))
		return
	}

	// Exactly one action. Two would be contradictory and zero would be a broken client — both refused rather
	// than resolved by precedence, because a precedence rule here is a silent decision about forty photographs.
	given := 0
	if in.Location != nil {
		given++
	}
	if in.CheckpointID != "" {
		given++
	}
	if in.ClearLocation {
		given++
	}
	if in.Caption != nil {
		given++
	}
	if given != 1 {
		app.BadRequestResponse(w, r,
			errors.New("angiv præcis én ting: en position, et postnummer, en billedtekst, "+
				"eller at positionen skal fjernes"))
		return
	}

	if in.Caption != nil {
		app.setAdminPhotoCaptions(w, r, photoIDs, *in.Caption)
		return
	}

	if in.ClearLocation {
		app.clearAdminPhotoLocations(w, r, photoIDs, in.Reason)
		return
	}

	lat, lng, ok := app.resolveAdminLocation(w, r, in)
	if !ok {
		return
	}

	// **The bounds check, re-run.** Not carried from the client, not cached from the upload: the race area grows
	// as organizers site checkpoints, so this is judged now, against what is known now.
	verdict := app.albumBoundsVerdict(lat, lng)
	location := &photo.Location{Lat: lat, Lng: lng, BoundsVerdict: verdict}

	now := time.Now().UTC()
	updated := 0
	for _, photoID := range photoIDs {
		subject, serr := photo.Subject(app.config.eventYear, photoID, photo.VerbUpdated)
		if serr != nil {
			app.BadRequestResponse(w, r, serr)
			return
		}
		// Only the location is mentioned. `photo.Updated`'s fields are pointers precisely so a bulk position
		// cannot blank forty captions somebody spent an evening writing (task 363).
		if perr := app.commands.Publish(subject, photo.Updated{
			PhotoID:   photoID,
			Year:      app.config.eventYear,
			Location:  location,
			UpdatedAt: now,
		}); perr != nil {
			app.Logger.Error("admin bulk position failed partway",
				"updated", updated, "photoId", photoID, "err", perr)
			app.writeAlbumPublishFailure(w, r, perr)
			return
		}
		updated++
	}

	app.Logger.Info("admin set a position on a selection",
		"count", updated, "lat", lat, "lng", lng, "verdict", verdict,
		"checkpoint", in.CheckpointID, "ip", clientIP(r))

	if err := app.WriteJSON(w, http.StatusOK, patchAdminPhotosResponse{
		Updated:       updated,
		BoundsVerdict: verdict,
		Plottable:     photo.Plottable(verdict),
		Message:       adminVerdictMessage(updated, verdict),
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// resolveAdminLocation turns the request into a coordinate.
//
// A checkpoint id is resolved **here**, server-side, rather than the client sending the coordinate it has on
// screen. That is not ceremony: the client's copy may be stale, and this is the one path where a wrong number
// becomes a pin on a public map. It also means the picker can offer a post without trusting the browser to
// remember where it is.
func (app *application) resolveAdminLocation(w http.ResponseWriter, r *http.Request, in patchAdminPhotosRequest) (lat, lng float64, ok bool) {
	if in.CheckpointID != "" {
		if app.models.CheckpointCurator == nil {
			app.ServiceUnavailableResponse(w, r, "posterne er ikke tilgængelige lige nu")
			return 0, 0, false
		}
		points, err := app.models.CheckpointCurator.Positioned(app.config.eventYear)
		if err != nil {
			app.ServerErrorResponse(w, r, err)
			return 0, 0, false
		}
		for _, c := range points {
			if string(c.ID) == in.CheckpointID {
				return c.Lat, c.Lng, true
			}
		}
		// An unsited or unknown post. 404 rather than a silent fallback, because the alternative is a selection
		// that quietly did not move.
		app.NotFoundResponse(w, r)
		return 0, 0, false
	}

	lat, lng = in.Location.Lat, in.Location.Lng

	// 0,0 is refused, for the reason `imaging.ReadGPS` refuses it: it is a real place in the Atlantic and also
	// what an empty form submits. A curator who means "no position" uses `clearLocation`, which is a different
	// fact and a different event.
	if lat == 0 && lng == 0 {
		app.BadRequestResponse(w, r,
			errors.New("0,0 er ikke en position — brug “fjern position” hvis positionen skal væk"))
		return 0, 0, false
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		app.BadRequestResponse(w, r, errors.New("positionen er ikke en gyldig koordinat"))
		return 0, 0, false
	}
	return lat, lng, true
}

// clearAdminPhotoLocations removes the coordinate from a selection.
//
// Publishes `locationcleared`, **not** an `updated` carrying nulls. PRD 022 §8.7: a coordinate somebody
// deliberately removed is not the same fact as one that was changed, and the log should record which — that is
// the question anyone auditing a takedown or a correction will actually ask.
//
// It is also why `photo.Updated` cannot express a clear at all: `Location: nil` there means "not mentioned", so
// overloading it would make the caption-blanking bug possible for positions too.
func (app *application) clearAdminPhotoLocations(w http.ResponseWriter, r *http.Request, photoIDs []string, reason string) {
	now := time.Now().UTC()
	cleared := 0

	for _, photoID := range photoIDs {
		subject, serr := photo.Subject(app.config.eventYear, photoID, photo.VerbLocationCleared)
		if serr != nil {
			app.BadRequestResponse(w, r, serr)
			return
		}
		if perr := app.commands.Publish(subject, photo.LocationCleared{
			PhotoID:   photoID,
			Year:      app.config.eventYear,
			Reason:    reason,
			ClearedAt: now,
		}); perr != nil {
			app.Logger.Error("admin clear position failed partway",
				"cleared", cleared, "photoId", photoID, "err", perr)
			app.writeAlbumPublishFailure(w, r, perr)
			return
		}
		cleared++
	}

	app.Logger.Info("admin cleared a position on a selection",
		"count", cleared, "reason", reason, "ip", clientIP(r))

	billeder := "billeder"
	if cleared == 1 {
		billeder = "billede"
	}
	if err := app.WriteJSON(w, http.StatusOK, patchAdminPhotosResponse{
		Updated:   cleared,
		Plottable: false,
		Message:   fmt.Sprintf("Position fjernet fra %d %s.", cleared, billeder),
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// setAdminPhotoCaptions writes a caption across a selection.
//
// # One caption, every album
//
// The caption is the photograph's, so this changes it everywhere the photograph appears (PRD 022 §8.3). That is
// the point of the library split rather than a side effect: before it, the same photograph in two albums had two
// captions, and a curator who fixed one had created a discrepancy invisible from both.
//
// Only the caption is mentioned on the event. `photo.Updated`'s pointer fields mean this cannot disturb a
// coordinate — the mirror of the rule that stops a bulk position blanking captions.
func (app *application) setAdminPhotoCaptions(w http.ResponseWriter, r *http.Request, photoIDs []string, caption string) {
	if len([]rune(caption)) > maxAdminCaption {
		app.BadRequestResponse(w, r, errors.New("billedteksten er for lang"))
		return
	}

	now := time.Now().UTC()
	updated := 0
	for _, photoID := range photoIDs {
		subject, serr := photo.Subject(app.config.eventYear, photoID, photo.VerbUpdated)
		if serr != nil {
			app.BadRequestResponse(w, r, serr)
			return
		}
		if perr := app.commands.Publish(subject, photo.Updated{
			PhotoID:   photoID,
			Year:      app.config.eventYear,
			Caption:   &caption,
			UpdatedAt: now,
		}); perr != nil {
			app.Logger.Error("admin caption failed partway",
				"updated", updated, "photoId", photoID, "err", perr)
			app.writeAlbumPublishFailure(w, r, perr)
			return
		}
		updated++
	}

	// The caption text is **not** logged. It is curator prose rather than an identifier, the log is a durable
	// record, and there is nothing an operator would use it for.
	app.Logger.Info("admin set a caption on a selection",
		"count", updated, "cleared", caption == "", "ip", clientIP(r))

	billeder := "billeder"
	if updated == 1 {
		billeder = "billede"
	}
	message := fmt.Sprintf("Billedtekst sat på %d %s.", updated, billeder)
	if caption == "" {
		message = fmt.Sprintf("Billedtekst fjernet fra %d %s.", updated, billeder)
	}

	if err := app.WriteJSON(w, http.StatusOK, patchAdminPhotosResponse{
		Updated: updated,
		Message: message,
	}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// maxAdminCaption bounds a caption.
//
// Generous: a caption is a sentence or two on a public page, and a curator who wants three should not be stopped
// by an arbitrary number. Present because the column is TEXT and an unbounded field on a write path is how a
// projection row becomes a megabyte.
const maxAdminCaption = 1000

// adminVerdictMessage writes the sentence for a set, and the three verdicts read differently on purpose.
//
// PRD 022 §7 asks for this copy to be written carefully rather than generated, and the reason is that two of the
// three outcomes are refusals a curator must not mistake for a fault at their end:
//
//   - `outside` is a statement about the photograph: it is not at the event, and it will not be plotted.
//   - `unknown` is a statement about **us**: we had no race area to judge against, which happens before the
//     checkpoints are sited. Saying "outside" here would blame the curator for our missing data.
func adminVerdictMessage(n int, verdict string) string {
	billeder := "billeder"
	if n == 1 {
		billeder = "billede"
	}

	switch verdict {
	case photo.BoundsInside:
		return fmt.Sprintf("Position sat på %d %s. De vises på kortet.", n, billeder)
	case photo.BoundsOutside:
		return fmt.Sprintf("Position sat på %d %s, men den ligger uden for løbsområdet, "+
			"så de vises ikke på kortet.", n, billeder)
	case photo.BoundsUnknown:
		return fmt.Sprintf("Position sat på %d %s. Den kunne ikke vurderes, fordi ingen poster "+
			"har en placering endnu — så de vises ikke på kortet.", n, billeder)
	default:
		return fmt.Sprintf("Position sat på %d %s.", n, billeder)
	}
}

// listAdminCheckpointsResponse is the picker's list.
type listAdminCheckpointsResponse struct {
	Checkpoints []adminCheckpointView `json:"checkpoints"`
}

// adminCheckpointView is one post in the picker.
type adminCheckpointView struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

// listAdminCheckpointsHandler returns the year's sited checkpoints for the picker.
//
// @Summary      List the event's sited checkpoints
// @Description  Returns the configured event year's checkpoints that have a position, in route order, so a curator can set a photograph's location by naming the post rather than by reading a coordinate. Unsited checkpoints are absent rather than an error — organizers add posts before siting them — and an empty list is a normal early-season answer. This read is **not** available to the app's patrol-scoped handlers: `checkpoint.Queries` deliberately has no way to ask for all checkpoints, because the event area is not fully known to participants (PRD 002), so this lives on a separate curator interface behind the admin credential. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Success      200  {object}  listAdminCheckpointsResponse
// @Failure      401  {object}  map[string]string  "missing or wrong admin credential"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the checkpoints are unavailable"
// @Router       /admin/checkpoints [get]
func (app *application) listAdminCheckpointsHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.CheckpointCurator == nil {
		app.ServiceUnavailableResponse(w, r, "posterne er ikke tilgængelige lige nu")
		return
	}

	points, err := app.models.CheckpointCurator.Positioned(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	out := listAdminCheckpointsResponse{Checkpoints: make([]adminCheckpointView, 0, len(points))}
	for _, c := range points {
		out.Checkpoints = append(out.Checkpoints, adminCheckpointView{
			ID:   string(c.ID),
			Name: c.Name,
			Lat:  c.Lat,
			Lng:  c.Lng,
		})
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// checkpointCuratorOrNil narrows the projection to the curator read API, or nil.
//
// The same typed-nil guard as the others, and kept separate from `checkpoint.Queries`' wiring for the reason
// curator.go records: this interface can enumerate every sited position in the event, and the one that
// patrol-scoped handlers hold deliberately cannot.
func checkpointCuratorOrNil(t *checkpoint.Table) checkpoint.CuratorQueries {
	if t == nil {
		return nil
	}
	return t
}
