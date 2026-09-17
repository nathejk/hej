package main

import (
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
)

// Browsing by hold (PRD 019 §0a.1, task 319).
//
// # This is the post-race surface, and the reason it exists is not symmetry
//
// During the race, newest-first is right: a member opens Glimt to see what is happening now. Afterwards
// the question changes completely — "show me *our* photos, then our friends' patrulje, then everything"
// — and a chronological feed answers it badly, because a hold's twelve photos are scattered through a
// thousand.
//
// So this is a first-class view rather than a filter in a menu, and two details follow from what it is
// for:
//
//   - **Oldest first.** A race reads forward in time. Somebody reliving an evening wants it in the
//     order it happened, which is the opposite of what the feed wants (task 301's querier test asserts
//     the asymmetry deliberately, so a later tidy-up does not unify them).
//   - **Thumbnails only.** This is the load peak of the whole feature (PRD 019 §0a.3): a thousand
//     people at the finish line on the worst network of the weekend. The grid asks for `?variant=thumb`
//     and full media is fetched only when an item is opened.

// glimtHoldsResponse is the index: which holds have posted something the caller may see.
type glimtHoldsResponse struct {
	Holds []glimtHoldSummary `json:"holds"`

	// OwnNumber is the caller's own hold number, so the client can offer a one-tap shortcut to
	// "din patrulje" — the main way anyone enters the post-race browse (PRD 019 §0a.1).
	//
	// Served here rather than left for the client to derive, because the profile payload carries
	// the hold's *name* and not its number, and a client matching on name would break on two
	// holds sharing one. Empty for crew, who have a section rather than a numbered hold — the
	// client then omits the shortcut instead of linking to a collection that cannot exist.
	OwnNumber string `json:"own_number,omitempty"`
}

type glimtHoldSummary struct {
	// Number and Name are the hold's, frozen on each glimt at creation. The index reports the
	// most recent spelling it finds, which is the best available answer if a hold was renamed
	// mid-event.
	Number string `json:"number"`
	Name   string `json:"name"`
	Group  string `json:"group"`
	// Count is how many glimt from this hold the caller may see — not how many exist. A count
	// including invisible ones would tell a spejder that the bandits posted eleven things.
	Count int `json:"count"`
	// LatestAt orders the index by recent activity, which during the race is what makes it
	// useful and afterwards is simply a stable order.
	LatestAt time.Time `json:"latest_at"`
}

// listGlimtHoldsHandler serves the index of holds.
//
// @Summary      Holds that have shared something
// @Description  The index for browsing by hold: every patrulje, klan or crew section with at least one glimt the caller may see, ordered by most recent activity. Counts reflect what the caller may see, not what exists — a count including invisible glimt would tell a spejder how much another group had posted. Holds whose only glimt are invisible to the caller do not appear at all.
// @Tags         glimt
// @Produce      json
// @Success      200  {object}  glimtHoldsResponse
// @Failure      401  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /glimt/hold [get]
func (app *application) listGlimtHoldsHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, "glimt er ikke tilgængelige lige nu")
		return
	}
	viewer, found := app.models.Users.Get(s.UserID)
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	holds, err := app.models.Glimt.Holds(app.config.eventYear, app.glimtFilter(s.UserID, viewer.Role))
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	out := glimtHoldsResponse{
		Holds:     make([]glimtHoldSummary, 0, len(holds)),
		OwnNumber: app.ownHoldNumber(s.UserID),
	}
	for _, h := range holds {
		out.Holds = append(out.Holds, glimtHoldSummary{
			Number:   h.TeamNumber,
			Name:     h.TeamName,
			Group:    h.Group,
			Count:    h.Count,
			LatestAt: h.LatestAt,
		})
	}
	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// listGlimtByHoldHandler serves one hold's collection.
//
// @Summary      One hold's glimt
// @Description  Every glimt from one hold that the caller may see, **oldest first** — a race reads forward in time, so somebody going through the night afterwards wants it in the order it happened. This is deliberately the opposite order from the feed. Paginated with `limit` (default 20, max 200) and `offset`. Visibility is filtered exactly as the feed is, so asking for another group's hold returns only what was shared beyond that group.
// @Tags         glimt
// @Produce      json
// @Param        number  path      string  true   "hold number"
// @Param        limit   query     int     false  "page size (default 20, max 200)"
// @Param        offset  query     int     false  "rows to skip"
// @Success      200  {object}  glimtFeedResponse
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string  "no hold number given"
// @Failure      503  {object}  map[string]string
// @Router       /glimt/hold/{number} [get]
func (app *application) listGlimtByHoldHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, "glimt er ikke tilgængelige lige nu")
		return
	}
	viewer, found := app.models.Users.Get(s.UserID)
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	number := httprouter.ParamsFromContext(r.Context()).ByName("number")
	if number == "" {
		// An empty number would match the column default, which every crew glimt carries —
		// returning them all as though they were one hold's collection. The querier guards
		// this too; refusing here makes it a 404 rather than a silently empty page.
		app.NotFoundResponse(w, r)
		return
	}

	limit, offset := pageParams(r)
	rows, err := app.models.Glimt.ByHold(app.config.eventYear, number,
		app.glimtFilter(s.UserID, viewer.Role), limit, offset)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	// Re-checked against the predicate on the way out, exactly as the feed is: the SQL narrowed
	// the query, and the predicate is the authority (PRD 019 §8). An empty result for a hold that
	// exists is a normal answer — it means nothing they posted was shared this far.
	out := glimtFeedResponse{
		Glimt: newGlimtResponses(app.visibleGlimt(rows, s.UserID, viewer.Role), s.UserID),
	}
	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// ownHoldNumber returns the caller's own hold number, for the feed's "din patrulje" shortcut.
//
// Empty for crew, who have a section rather than a numbered hold, and empty when the person record
// cannot be read — both mean "offer no shortcut", which is a missing convenience rather than a fault.
func (app *application) ownHoldNumber(personID string) string {
	p, found := app.person(personID)
	if !found {
		return ""
	}
	return p.TeamNumber
}
