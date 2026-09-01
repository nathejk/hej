package main

import (
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/users"
)

// The crew-only patrol lookup (PRD 007 §6, task 157).
//
// Deliberately in its own file, because its rules are the *opposite* of the directory's in
// the one dimension that matters: nothing here may ever be stored on a device.
//
// # Why this surface exists at all
//
// Spejdere are not in the contacts directory, in either direction. That excludes the largest
// privacy cost in the event — a browsable index of minors' faces — but it would also have
// excluded the scenario the feature started from: a samarit sent to "patrol 138, member
// Freja" who needs to know which face is Freja. This is the narrow answer: one patrol at a
// time, by exact number, live, audited, and never cached.
//
// # The invariants, and why each is load-bearing
//
//   - **Exact match on the full number.** A prefix search would turn one permitted question
//     into an enumeration tool.
//   - **403 and 404 indistinguishable.** Otherwise the endpoint reports which numbers exist,
//     which maps the patrol and klan numbering to anyone with a session.
//   - **`Cache-Control: no-store`.** This is what keeps ~557 spejder thumbnails off crew
//     devices. An earlier design cached them so the lookup worked offline; the decision to
//     leave it live is what removed that exposure, and headers are how the decision is
//     enforced rather than remembered.
//   - **Every call logged.** The one place in this feature where adults look at minors, on
//     data they did not have to be given in advance.
//
// The cost is accepted and stated in the PRD: with no signal, this does not work, and the
// fallback is the radio — which is how it works today.

// patrolLookupResponse is one patrol's members.
type patrolLookupResponse struct {
	// Number echoes what was looked up, so a client rendering several results in sequence
	// cannot mislabel them.
	Number  string               `json:"number"`
	Members []patrolLookupMember `json:"members"`
}

// patrolLookupMember is one member of the patrol.
//
// Carries more than a directory entry does — a full status and a phone number — because this
// is a live read for an operational purpose, and none of it is persisted. It still carries no
// guardian number: that is a `.rules` invariant with no operational exception, and these are
// exactly the records that have one.
type patrolLookupMember struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Status is the member's full lifecycle status (types.MemberStatus), not the
	// directory's single still-in-race bit. Fresh because this request is online.
	Status string `json:"status,omitempty"`
	Phone  string `json:"phone,omitempty"`
	// HasPortrait tells the client whether to request the image at all, so a member with no
	// photo costs no request and renders initials.
	HasPortrait bool `json:"hasPortrait"`
}

// @Summary      Look up one patrol
// @Description  Crew only. Returns the members of one patrol with their current status and phone number, found by the exact patrol number. Never cached: `no-store`, excluded from the service worker, and not a sync dataset — no spejder record is ever written to a device. Refusal and absence are indistinguishable, so the endpoint cannot be used to discover which patrol numbers exist. Every call is logged. Requires connectivity by design.
// @Tags         contacts
// @Produce      json
// @Param        number  path      string  true  "the full patrol number"
// @Success      200  {object}  patrolLookupResponse
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string  "no such patrol, or the caller may not look patrols up"
// @Router       /contacts/patrols/{number} [get]
func (app *application) patrolLookupHandler(w http.ResponseWriter, r *http.Request) {
	// no-store before anything else, including on every error path. Set first so an early
	// return cannot produce a cacheable refusal — and because a 404 that reveals nothing is
	// still a response we would rather not have sitting in a proxy.
	w.Header().Set("Cache-Control", "no-store")

	viewer, ok := app.contactsViewer(w, r)
	if !ok {
		return
	}

	number := strings.TrimSpace(httprouter.ParamsFromContext(r.Context()).ByName("number"))

	// A non-crew caller and a nonexistent patrol get the same answer. Note the refusal is
	// logged at info: a bandit probing this endpoint is worth seeing, and it is the only
	// signal we would have.
	if !users.MayLookUpPatrol(viewer.Role) {
		app.Logger.Info("patrol lookup refused",
			"viewerId", viewer.ID, "viewerRole", string(viewer.Role), "number", number)
		app.NotFoundResponse(w, r)
		return
	}
	if number == "" || app.models.People == nil {
		app.NotFoundResponse(w, r)
		return
	}

	people, err := app.models.People.ListPatrolByNumber(app.config.eventYear, number)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if len(people) == 0 {
		app.NotFoundResponse(w, r)
		return
	}

	// This is the audit log PRD 007 §11.7 asks for, and it is one line precisely because the
	// lookup is online: no client-side queue, no batch upload, no ingestion endpoint. It
	// records who looked at which patrol, which is the question worth being able to answer.
	app.Logger.Info("patrol lookup",
		"viewerId", viewer.ID,
		"viewerRole", string(viewer.Role),
		"number", number,
		"members", len(people),
	)

	out := patrolLookupResponse{Number: number, Members: make([]patrolLookupMember, 0, len(people))}
	for _, p := range people {
		member := patrolLookupMember{
			ID:          p.PersonID,
			Name:        p.Name,
			Status:      p.MemberStatus,
			HasPortrait: p.PortraitRef != "",
		}
		// Same rule as the directory (2026-09-01): the number goes only once the member is
		// `released`, because that is the one status where they have left the area and been
		// handed to a guardian. A member waiting by the trail, in a car or at HQ is exactly who
		// a samarit needs to ring, so purging their number here would break the case this
		// surface exists for.
		if contactable(p.MemberStatus) {
			member.Phone = p.Phone
		}
		out.Members = append(out.Members, member)
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// @Summary      A patrol member's portrait
// @Description  Crew only, and reachable only in the context of a patrol lookup: the person must be a member of the patrol named in the path. Never cached (`no-store`), which is what keeps spejder portraits off devices. Refusal and absence are indistinguishable.
// @Tags         contacts
// @Produce      jpeg
// @Param        number    path      string  true   "the full patrol number"
// @Param        personId  path      string  true   "person id, who must belong to that patrol"
// @Param        size      query     string  false  "thumb (default), full, or a rendition name"
// @Success      200  {file}    binary
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /contacts/patrols/{number}/photo/{personId} [get]
func (app *application) patrolPhotoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	viewer, ok := app.contactsViewer(w, r)
	if !ok {
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	number := strings.TrimSpace(params.ByName("number"))
	personID := params.ByName("personId")

	if !users.MayLookUpPatrol(viewer.Role) || number == "" || personID == "" || app.models.People == nil {
		app.NotFoundResponse(w, r)
		return
	}

	// The person must be in the patrol that was asked for. Scoping the image to a lookup
	// rather than to a person id is what stops this becoming a general "any spejder's face by
	// id" route — with a patrol number required, a caller must already know which patrol
	// somebody is in, which is the same thing the lookup itself requires.
	people, err := app.models.People.ListPatrolByNumber(app.config.eventYear, number)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	for _, p := range people {
		if p.PersonID != personID {
			continue
		}
		if p.PortraitRef == "" {
			app.NotFoundResponse(w, r)
			return
		}

		size := r.URL.Query().Get("size")
		if strings.TrimSpace(size) == "" {
			size = "thumb"
		}
		ref := blob.Ref(portraitRefForSize(p, size))
		if !ref.Valid() {
			app.NotFoundResponse(w, r)
			return
		}

		app.Logger.Info("patrol portrait viewed",
			"viewerId", viewer.ID, "viewerRole", string(viewer.Role),
			"number", number, "personId", personID)

		// no-store, not the directory's cacheable default: this is a minor's face, and the
		// device must not keep it. Passed explicitly at the call site so the difference
		// between the two portrait surfaces is visible here rather than buried in a helper.
		app.streamPortrait(w, r, ref, "no-store", personID)
		return
	}

	app.NotFoundResponse(w, r)
}
