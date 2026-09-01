package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// Contacts pane (PRD 007).
//
// The directory half: a manifest the client caches and works from offline. The live
// patrol lookup is a separate surface with the opposite caching rules and lives apart
// from this file deliberately (task 157).
//
// # What this endpoint must not carry
//
// The response type below is an allow-list, not a projection of a row. Everything it
// carries is cached on other people's devices, so a field added here is a field
// distributed to a few hundred phones — that should be a visible decision, not a side
// effect of somebody widening a SELECT.
//
// In particular `phoneParent` is absent, and must stay absent. That is a `.rules`
// invariant rather than a preference: guardian numbers never enter the PWA except where a
// member approves their own. `person.Person` carries the field, so the protection here is
// that this struct simply has nowhere to put it, plus the test in task 159.

// contactsManifest is the cached directory as the client sees it.
type contactsManifest struct {
	// Version identifies this payload. It is the ETag value too, so a client can hold
	// one string and ask "still current?" — see the version endpoint (task 155), which
	// answers that question without sending the people again.
	Version string `json:"version"`

	// Entries is one row per (person, group) pair, ordered deterministically.
	Entries []contactEntry `json:"entries"`
}

// contactEntry is one person as listed in one group.
//
// A person can appear more than once, and that is correct rather than a bug to
// de-duplicate: a crew member out as a bandit belongs in their klan *and* among crew, and
// a crew viewer sees both lists. Both are answering "who is out as what". Favourites key
// on Id, so a duplicate entry does not become a duplicate favourite.
type contactEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// Population is which list this entry belongs to, so the client can render the
	// bandit, gøgler and crew sections without inferring anything.
	Population string `json:"population"`

	// Groups is the grouping path, outermost first. One level today (klan, or the single
	// gøgler/crew group); a slice because lok arrives upstream as subsections and should
	// add a tier rather than force a rewrite.
	Groups []contactGroup `json:"groups"`

	// Phone is the person's own number. Never a guardian's.
	//
	// Empty for a withdrawn member: their number is purged while their name and portrait
	// stay, so a colleague can still recognise them but nobody calls them (task 160).
	Phone string `json:"phone,omitempty"`

	// CrewFunction is the section label, for crew only. Display-only, and deliberately
	// the label rather than the slug — the slug decides placement, which is not the
	// client's business.
	CrewFunction string `json:"crewFunction,omitempty"`

	// StillInRace is false once a member has left the race, which the pane renders as a
	// status marking. One bit rather than the lifecycle: the directory only needs to know
	// whether to offer a phone number and show a marking. The live patrol lookup returns
	// the full status, because there it is worth the request (task 150).
	StillInRace bool `json:"stillInRace"`

	// PortraitVersion is the thumbnail's content hash, or empty when there is no
	// portrait. Doubles as the image's cache key: content-addressed, so a changed
	// portrait changes this string and an unchanged one never refetches.
	PortraitVersion string `json:"portraitVersion,omitempty"`
}

type contactGroup struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	IsOwn bool   `json:"isOwn"`
}

// listableRolesFor maps the populations a viewer may list to the app roles that can
// produce them, so the query fetches only rows the caller is allowed to see.
//
// Fetching by role and then placing by slug means a crew member out as a bandit is
// fetched under `crew` and *placed* in the bandit population — which is why this returns
// roles rather than populations, and why the entry-building loop below still has to run
// every row through PopulationsOf.
//
// Spejder is never included. Not by omission: by the time this is called, MayList has
// already refused that population for every role.
func listableRolesFor(viewer users.Role) []string {
	var roles []string
	for _, r := range users.AllRoles {
		if r == users.RoleSpejder {
			// Spejdere are not listable by anyone. The patrol lookup is the only path.
			continue
		}
		// A role is worth fetching if any population it can be placed in is listable.
		// Crew is the interesting case: a crew row may land in bandit, gøgler or crew.
		var wanted bool
		for _, pop := range users.AllPopulations {
			if users.MayList(viewer, pop) && canPlaceRoleIn(r, pop) {
				wanted = true
				break
			}
		}
		if wanted {
			roles = append(roles, string(r))
		}
	}
	return roles
}

// canPlaceRoleIn reports whether a person with the given app role could be listed in the
// given population. Coarser than PopulationsOf, which needs the actual person.
func canPlaceRoleIn(role users.Role, pop Population) bool {
	switch {
	case role == users.RoleBandit:
		return pop == users.PopulationBandit
	case role == users.RoleGoegler:
		return pop == users.PopulationGoegler
	case role.IsCrew():
		// Crew can be placed among banditter or gøglere by section slug, as well as
		// among crew.
		return pop == users.PopulationCrew || pop == users.PopulationBandit || pop == users.PopulationGoegler
	}
	return false
}

// Population is aliased locally so the helper signatures above read naturally.
type Population = users.Population

// @Summary      Contacts directory manifest
// @Description  The people the caller may list, with grouping, phone numbers, portrait versions and a still-in-race flag. This is the payload the client caches for offline use. Spejdere are never included — crew reach them only through the patrol lookup. Guardian numbers and postal addresses are never included. Supports `If-None-Match`; the version is the ETag.
// @Tags         contacts
// @Produce      json
// @Param        If-None-Match  header    string  false  "version held by the client"
// @Success      200  {object}  contactsManifest
// @Success      304  "unchanged"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "spejdere do not get the contacts pane"
// @Router       /contacts/manifest [get]
func (app *application) contactsManifestHandler(w http.ResponseWriter, r *http.Request) {
	viewer, ok := app.contactsViewer(w, r)
	if !ok {
		return
	}

	manifest, err := app.buildContactsManifest(viewer)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	// ETag before body: an unchanged directory is the common case once the event is
	// running, and the whole point of the version is that the poll costs nothing.
	etag := `"` + manifest.Version + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, manifest, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// contactsViewer resolves the caller and refuses anyone without the pane.
//
// Shared by every contacts handler so the spejder refusal is written once. A spejder gets
// 403 rather than an empty directory: an empty list would read as "nobody is here yet" and
// invite a bug report, and the honest answer is that this pane is not theirs.
func (app *application) contactsViewer(w http.ResponseWriter, r *http.Request) (users.User, bool) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return users.User{}, false
	}
	viewer, found := app.models.Users.Get(s.UserID)
	if !found {
		app.NotFoundResponse(w, r)
		return users.User{}, false
	}
	if !users.MayUseContacts(viewer.Role) {
		// 403, not 404: a spejder knows perfectly well the pane exists — it is in the app
		// their patrol-mates use — so there is nothing to conceal, and an empty directory
		// would read as "nobody is here yet" and invite a bug report. The patrol lookup is
		// the opposite case and answers 404 for a refusal, because there the existence of a
		// patrol is exactly what must not leak.
		app.ForbiddenResponse(w, r)
		return users.User{}, false
	}
	return viewer, true
}

// buildContactsManifest assembles the payload for one viewer.
func (app *application) buildContactsManifest(viewer users.User) (contactsManifest, error) {
	if app.models.People == nil {
		// No projection configured (a database-less run). An empty directory with a
		// stable version is the honest answer: the client shows "nothing synced yet"
		// rather than an error it cannot act on.
		return contactsManifest{Version: contactsVersion(nil), Entries: []contactEntry{}}, nil
	}

	people, err := app.models.People.ListByAppRoles(app.config.eventYear, listableRolesFor(viewer.Role))
	if err != nil {
		return contactsManifest{}, err
	}

	entries := make([]contactEntry, 0, len(people))
	for _, p := range people {
		subject, ok := contactSubject(p)
		if !ok {
			// An unrecognised app role is a data condition, not an error, and the
			// directory adapter already logs it on the login path. Skipping keeps one
			// bad row from emptying the pane.
			continue
		}

		for _, pop := range users.PopulationsOf(subject) {
			groups := users.GroupPathFor(viewer, subject, pop)
			if groups == nil {
				// Not permitted, or not placed there. GroupPathFor re-checks
				// permission, which is why there is no separate MayList call here.
				continue
			}
			entries = append(entries, newContactEntry(p, subject, pop, groups))
		}
	}

	// Deterministic order, because the version is a hash of the payload: without this a
	// map iteration or a changed query plan would look like a directory change and make
	// every device refetch.
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Population != b.Population {
			return a.Population < b.Population
		}
		ga, gb := groupKey(a.Groups), groupKey(b.Groups)
		if ga != gb {
			return ga < gb
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})

	return contactsManifest{Version: contactsVersion(people), Entries: entries}, nil
}

func newContactEntry(p person.Person, subject users.User, pop users.Population, groups []users.Group) contactEntry {
	out := contactEntry{
		ID:              p.PersonID,
		Name:            p.Name,
		Population:      string(pop),
		StillInRace:     stillInRace(p.MemberStatus),
		PortraitVersion: portraitVersion(p),
	}

	for _, g := range groups {
		out.Groups = append(out.Groups, contactGroup{ID: g.ID, Label: g.Label, IsOwn: g.IsOwn})
	}

	// A withdrawn member keeps their name and portrait. Their number goes only when they are
	// **released** — handed over to a guardian and gone from the area (maintainer direction,
	// 2026-09-01). Every other ending still leaves them somewhere around the event and worth
	// being able to reach: a `reunited` member is back with their own patrol, and `waiting`,
	// `transit` and `sheltered` are all in our care.
	//
	// Note this is deliberately *not* the same question as StillInRace, which drives the status
	// marking. A reunited member is out of the race and still contactable, so the row shows a
	// marking next to a working number — which reads correctly rather than contradictorily.
	if contactable(p.MemberStatus) {
		out.Phone = p.Phone
	}

	// Crew function is display-only context ("Samarit", "HQ"), and only for people
	// actually listed among crew.
	if pop == users.PopulationCrew {
		out.CrewFunction = p.SectionName
	}

	return out
}

// contactSubject converts a projection row into the user shape placement and grouping
// read. Mirrors personDirectory.toUser, minus every field the contacts pane may not have.
func contactSubject(p person.Person) (users.User, bool) {
	role := users.Role(p.AppRole)
	if !role.Valid() {
		return users.User{}, false
	}
	return users.User{
		ID:          p.PersonID,
		Role:        role,
		Name:        p.Name,
		PatrolID:    p.TeamID,
		PatrolName:  p.TeamName,
		Section:     p.SectionName,
		SectionSlug: p.SectionSlug,
	}, true
}

// contactable reports whether the member's phone number may still be shown.
//
// False for exactly one status: `released`. That is the only ending where the member has truly
// left — handed over to a guardian, off the site, no longer ours to ring. Everything else leaves
// them in or around the event area, where being able to reach them is the point:
//
//	waiting, transit, sheltered  — in our care, and the most likely people to need calling
//	reunited                     — back with their own patrol, still on site
//	finished                      — at the finish
//
// Deliberately separate from stillInRace, which answers a different question (are they racing?)
// and drives the status marking. Conflating the two is the bug this replaced: it purged the number
// of every withdrawn member, including people sitting in a car on the way to HQ.
//
// A released member's *guardian* is not an alternative to ring here either — guardian numbers
// never enter the PWA (`.rules`), and that is not softened by the member being unreachable.
func contactable(status string) bool {
	return types.MemberStatus(status) != types.MemberStatusReleased
}

// stillInRace reports whether the member is still taking part.
//
// Drives the status marking in the pane — not whether a number is shown, which is contactable's
// job above. `reunited` and `released` are both out of the race; only one of them is out of reach.
//
// INTERIM (task 175): derived here from the two ending statuses, in one place. It belongs in
// shared-go next to MemberStatus.InOurCare() and CanFinish(), so that `hej` and `hq` cannot
// disagree about what "left the race" means.
//
// Note `finished` is deliberately *not* a withdrawal: finishing means walking the route to the
// end, and marking a finisher as having left would quietly turn an achievement into a dropout.
// shared-go's own docs are careful about that distinction.
//
// The transitions now reach this projection: task 174 lifted hq's `spejderstatus` message bodies
// into `shared-go/messages/member.go`, and `person`'s consumer projects all eight of them, so
// this reads real statuses rather than the constant `true` it returned before that landed.
func stillInRace(status string) bool {
	switch types.MemberStatus(status) {
	case types.MemberStatusReunited, types.MemberStatusReleased:
		return false
	}
	return true
}

// portraitVersion returns the cache key for a person's thumbnail.
//
// The thumbnail hash, falling back to the full portrait's — matching how
// /me/photo?size=thumb degrades for portraits captured before thumbnails existed
// (task 104). Empty means no portrait, which the client renders as initials rather than a
// broken image.
func portraitVersion(p person.Person) string {
	if p.PortraitThumbRef != "" {
		return p.PortraitThumbRef
	}
	return p.PortraitRef
}

// @Summary      Contacts directory version
// @Description  A short opaque version for the caller's directory, used by the client's freshness poll: on foreground, on reconnect, and on an interval while the app is open. Changes whenever anything the caller may see changes. Deliberately tiny — this is called by every device with the pane open, and it is the first continuous during-race traffic this API takes. Push cannot be used for invalidation, because iOS requires every web push to show a notification.
// @Tags         contacts
// @Produce      json
// @Param        If-None-Match  header    string  false  "version held by the client"
// @Success      200  {object}  contactsVersionResponse
// @Success      304  "unchanged"
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "spejdere do not get the contacts pane"
// @Router       /contacts/version [get]
func (app *application) contactsVersionHandler(w http.ResponseWriter, r *http.Request) {
	viewer, ok := app.contactsViewer(w, r)
	if !ok {
		return
	}

	version, err := app.contactsVersionFor(viewer)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	etag := `"` + version + `"`
	w.Header().Set("ETag", etag)
	// A short max-age is a second line of defence against the poll's cost: if a client
	// misbehaves and polls far more often than the agreed interval, the browser's own cache
	// absorbs it before the request reaches us.
	w.Header().Set("Cache-Control", "private, max-age=10")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, contactsVersionResponse{Version: version}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

type contactsVersionResponse struct {
	Version string `json:"version"`
}

// contactsVersionFor returns the version for a viewer's permitted set, from a short-lived
// cache.
//
// The cache is the point of this endpoint. Computing a version means reading the viewer's
// permitted rows, and a few hundred devices polling every 60 s would turn that into
// continuous query load on the same BFF that takes PRD 002's position reports. Because the
// version hashes *data* rather than presentation, every viewer with the same permitted role
// set shares an answer — in practice three or four distinct sets for the whole event — so a
// few seconds of caching collapses the load to almost nothing.
//
// The TTL bounds staleness at a few seconds on top of the client's own interval, which is
// well inside "without too much delay".
func (app *application) contactsVersionFor(viewer users.User) (string, error) {
	if app.models.People == nil {
		return contactsVersion(nil), nil
	}

	roles := listableRolesFor(viewer.Role)
	key := strings.Join(roles, ",")

	if v, ok := app.contactsVersions.get(key); ok {
		return v, nil
	}

	people, err := app.models.People.ListByAppRoles(app.config.eventYear, roles)
	if err != nil {
		return "", err
	}
	version := contactsVersion(people)
	app.contactsVersions.put(key, version)
	return version, nil
}

// versionCache is a tiny TTL cache keyed by permitted role set.
//
// Hand-rolled rather than pulling in a dependency: it holds at most a handful of short
// strings, and the whole implementation is shorter than the configuration a library would
// need. Not an LRU because the key space is bounded by the number of role combinations,
// which is fixed by the access matrix.
type versionCache struct {
	ttl time.Duration

	mu      sync.Mutex
	entries map[string]versionCacheEntry
	// now is injectable so tests can expire entries without sleeping.
	now func() time.Time
}

type versionCacheEntry struct {
	version string
	expires time.Time
}

func newVersionCache(ttl time.Duration) *versionCache {
	return &versionCache{ttl: ttl, entries: map[string]versionCacheEntry{}, now: time.Now}
}

func (c *versionCache) get(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || c.now().After(e.expires) {
		return "", false
	}
	return e.version, true
}

func (c *versionCache) put(key, version string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = versionCacheEntry{version: version, expires: c.now().Add(c.ttl)}
}

// @Summary      A directory member's portrait
// @Description  Serves the thumbnail of someone the caller may list, authorized per request. `size` follows the same convention as `/me/photo`. Refusal and absence are deliberately indistinguishable: a distinguishable 403 would let a bandit discover which person ids are gøglere. Spejder portraits are never available here — crew reach them only through the patrol lookup, which is uncached.
// @Tags         contacts
// @Produce      jpeg
// @Param        personId  path      string  true   "person id"
// @Param        size      query     string  false  "thumb (default), full, or a rendition name such as thumb256"
// @Success      200  {file}    binary
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string  "spejdere do not get the contacts pane"
// @Failure      404  {object}  map[string]string  "no such portrait, or not visible to the caller"
// @Router       /contacts/people/{personId}/photo [get]
func (app *application) contactsPhotoHandler(w http.ResponseWriter, r *http.Request) {
	viewer, ok := app.contactsViewer(w, r)
	if !ok {
		return
	}
	if app.models.People == nil {
		app.ServiceUnavailableResponse(w, r, "billeder er ikke tilgængelige lige nu")
		return
	}

	personID := httprouter.ParamsFromContext(r.Context()).ByName("personId")

	subjectRow, found, err := app.models.People.Get(app.config.eventYear, personID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	// One refusal for every reason: no such person, a person the caller may not list, a
	// person with no portrait, or bytes that have gone missing. They must be
	// indistinguishable, because the alternative is an oracle — a bandit walking person ids
	// and learning which ones exist, or which are gøglere, from the difference between 403
	// and 404. This is also why the checks below do not log at different levels or return
	// early with different bodies.
	if !found {
		app.NotFoundResponse(w, r)
		return
	}
	subject, valid := contactSubject(subjectRow)
	if !valid || !app.mayListSubject(viewer, subject) {
		app.NotFoundResponse(w, r)
		return
	}
	if subjectRow.PortraitRef == "" {
		app.NotFoundResponse(w, r)
		return
	}

	// Thumbnails by default here, unlike /me/photo which defaults to the full image. The
	// directory only ever needs a face at list or dialog size, and defaulting to `full`
	// would mean a careless client shipping ~800 KB portraits of colleagues to every device.
	size := r.URL.Query().Get("size")
	if strings.TrimSpace(size) == "" {
		size = "thumb"
	}

	ref := blob.Ref(portraitRefForSize(subjectRow, size))
	if !ref.Valid() {
		app.Logger.Error("portrait ref in projection is not a content hash",
			"personId", personID, "ref", string(ref))
		app.NotFoundResponse(w, r)
		return
	}

	// Matching ETag lets the sync engine skip images it already holds. 304 before the blob
	// read, so an unchanged portrait costs no disk I/O.
	if r.Header.Get("If-None-Match") == `"`+string(ref)+`"` {
		w.Header().Set("ETag", `"`+string(ref)+`"`)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	app.streamPortrait(w, r, ref, "private, max-age=3600", personID)
}

// mayListSubject reports whether the viewer may see this person anywhere in the directory.
//
// True when any of the subject's populations is listable by the viewer — a crew bandit is
// visible to a bandit through the bandit listing even though they are also crew. Never true
// for a spejder, whatever the viewer's role.
func (app *application) mayListSubject(viewer, subject users.User) bool {
	for _, pop := range users.PopulationsOf(subject) {
		if users.MayList(viewer.Role, pop) {
			return true
		}
	}
	return false
}

// contactsVersion hashes the *data* behind a viewer's directory.
//
// Content-derived rather than a counter or a timestamp: there is no natural version column
// to read, and a hash makes "did anything I can see change?" answerable per permitted set,
// so one person's edit does not invalidate everybody's cache.
//
// Deliberately computed from the rows rather than from the rendered entries, which is a
// correction to the first cut of this code. Rendered entries carry `IsOwn`, a
// *presentation* flag that differs for every viewer — hashing it made the version unique
// per person, so the cheap poll in the version endpoint could never be cached and two
// members of the same klan holding identical data would disagree about the version.
// Hashing the data keeps the variants down to one per permitted role set.
func contactsVersion(people []person.Person) string {
	h := sha256.New()
	for _, p := range people {
		// Every field the payload can expose, in a fixed order. A field added to
		// contactEntry must be added here too, or a change to it would not invalidate
		// caches — which is why they sit next to each other in this file.
		io.WriteString(h, p.PersonID)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.Name)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.Phone)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.MemberStatus)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.TeamID)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.TeamName)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.SectionSlug)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.SectionName)
		io.WriteString(h, "\x00")
		io.WriteString(h, portraitVersion(p))
		io.WriteString(h, "\x1e")
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

func groupKey(groups []contactGroup) string {
	var key string
	for _, g := range groups {
		key += g.ID + "/"
	}
	return key
}
