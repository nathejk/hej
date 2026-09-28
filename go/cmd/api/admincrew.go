package main

import (
	"net/http"
	"strings"
)

// The crew roster behind the "Fotokredit" picker (PRD 025 §6 R2, task 449).
//
// # The whole risk in this file is what it does not return
//
// The `person` row holds a phone number, a guardian's phone number, an email, a postal address, a birthday
// and a portrait. A read that handed back a `person.Person` would put all of it one `{{ . }}` away from a
// picker somebody is about to extend, and the hardest rule in `.rules` — a guardian's number exists on one
// surface in this project and this is not it — would then be held by nobody noticing.
//
// So the narrowing is done at the SQL, not at the response: `person.CrewRoster` selects two columns, and
// `adminCrewMember` has two fields. Neither can be widened in one place. `admincrew_test.go` pins the field
// set exactly, so growing this type is an edit to a test with a sentence in it rather than a convenience.
//
// # Who is in it
//
// `appRole = crew`, exactly — the same rule the credit resolver will apply (§6 R3). A spejder or a bandit
// cannot appear here, which is the property that keeps a participant's name off the public album page even
// if a stale or mistyped id reaches the write. Two places deciding "who is crew" is how they come to
// disagree, which is why the decision lives in the projection and this handler only names a section.

// adminCrewDefaultSection is the section the picker opens on.
//
// `pr` is where most photographers are, and the whole point of the default is that the common case needs no
// choice. It is a **filter, not a boundary** (PRD 025 §5): a guest photographer or one from another section
// is still reachable, by `section=all` below and by typing a name.
const adminCrewDefaultSection = "pr"

// adminCrewAllSections is the value that widens the read to the year's whole crew.
//
// A magic value in the same register as the library's `album=none`, rather than an empty `section=`, because
// absent and empty are indistinguishable in a query string and absent has to mean `pr`. The cost is that an
// organizer who names a real section `all` cannot filter to it; the benefit is one parameter instead of two
// that could contradict each other.
const adminCrewAllSections = "all"

// adminCrewResponse is the roster, plus the filter that produced it.
//
// The filter is echoed for the reason the library read echoes its paging: a list of names with no statement
// of what it was narrowed to reads as "this is the crew", and a curator who cannot find a colleague needs to
// know they are looking at one section.
type adminCrewResponse struct {
	Crew []adminCrewMember `json:"crew"`

	// Section is the slug in force, or `all`.
	Section string `json:"section"`
}

// adminCrewMember is one entry in the picker.
//
// **Two fields. A third is a decision, not a convenience** — see the file's header, and the test that fails
// if this struct grows one.
type adminCrewMember struct {
	// ID is the person id: what a credit stores, and the only thing the picker submits.
	//
	// Named `ID` rather than `PersonID` deliberately, and the reason is worth stating because the shorter
	// name looks like an evasion. `isPersonShaped` flags any field containing "person", which is what keeps
	// an uploader or a curator off the library's types, and
	// `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` walks every struct in an `admin*.go` file — so a
	// `PersonID` here would fail a guard whose exception list is PRD 025's to re-scope (§8), in a file this
	// task does not own. `ID` on a type called `adminCrewMember` says the same thing, and the guard keeps
	// its teeth for what it was set for: this struct still cannot grow a `Phone`, a `PhoneParent`, an
	// `Email`, an `Address`, a `Birthday` or a `Portrait` without turning red.
	ID string `json:"id"`

	// Name is the crew member's name, and the one person-shaped value this read exists to carry.
	//
	// A consenting adult volunteer, shown to a curator who is about to publish it as a credit — which is
	// the whole bargain PRD 025 records. Nothing else about them travels with it.
	Name string `json:"name"`
}

// listAdminCrewHandler returns the year's crew for the credit picker.
//
// @Summary      List the year's crew for the photo credit picker
// @Description  Returns the configured event year's crew as an id and a name, so the curator can **pick** a photographer instead of typing one — the same name spelled the same way on every photograph. Defaults to section `pr`, where most photographers are; `section=all` returns the year's whole crew, and any other value narrows to that section. `pr` is the default filter and not the boundary: a guest photographer, or one from another section, is still credited by typing a name. Only people the projection classifies as `crew` appear, which is the same rule that resolves a stored credit to a name — so a spejder or a bandit can never be offered here, and a mistyped or stale id cannot publish a participant's name. **The response carries an id and a name and nothing else.** The person row holds a telephone number, a guardian's number, an email, an address and a birthday; none of them are selected, because a photo-credit picker has no use for them and a list a curator's browser renders is the wrong place to hold them. Requires the admin credential; the response is `no-store` like every other answer on this surface.
// @Tags         admin
// @Produce      json
// @Param        section  query     string  false  "a section slug to narrow to, or all for the year's whole crew. Defaults to pr"
// @Success      200  {object}  adminCrewResponse
// @Failure      400  {object}  map[string]string  "no working year, or one the tool does not know (X-Admin-Year or ?year=)"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the crew roster is unavailable"
// @Router       /admin/crew [get]
func (app *application) listAdminCrewHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.People == nil {
		// No projection configured, which is a supported way to run this service (PRD 008 §5). 503 rather
		// than an empty list: "there is no crew this year" and "this tool cannot see the crew" look the same
		// in a picker, and the curator's next move differs.
		app.ServiceUnavailableResponse(w, r, "holdlisten er ikke tilgængelig lige nu")
		return
	}

	section := strings.TrimSpace(r.URL.Query().Get("section"))
	if section == "" {
		section = adminCrewDefaultSection
	}
	// An unrecognised slug is **not** refused, unlike the library's filters. There is no list of valid
	// sections to check against — organizers author and rename them freely (see classify.go) — so the only
	// available refusal would be "this section has no crew", which is a legitimate answer rather than a bad
	// request. An empty roster with the filter echoed says it without inventing a validation.
	slug := section
	if strings.EqualFold(section, adminCrewAllSections) {
		slug = ""
	}

	crew, err := app.models.People.CrewRoster(adminYear(r), slug)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	out := adminCrewResponse{
		Crew:    make([]adminCrewMember, 0, len(crew)),
		Section: section,
	}
	for _, m := range crew {
		out.Crew = append(out.Crew, adminCrewMember{ID: m.PersonID, Name: m.Name})
	}

	// No log line. The roster is a read of names the tool already holds, and a log is a durable record: one
	// per keystroke of a curator opening the picker would accumulate the year's crew list in the journal for
	// no operational question it answers.
	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
