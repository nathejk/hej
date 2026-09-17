package users

// Glimt visibility (PRD 019 §6, §8, task 299).
//
// This file is the *only* place that decides who may see a glimt. The feed query, the hold
// collection, the single-glimt read and the media handler all route through MaySeeGlimt.
// That matters more here than it usually does: a glimt's media lives at a
// content-addressed URL, so **a blob URL is a capability**. If the media handler answers a
// looser question than the feed did, every "min gruppe" photo becomes readable by anyone
// who has the hash — and the hash is in the payload of everyone allowed to see it.
//
// The rules, from PRD 019 §6:
//
//	| audience  | who sees it                                   |
//	|-----------|-----------------------------------------------|
//	| `group`   | members whose role maps to the author's group  |
//	| `nathejk` | every authenticated member, any role           |
//	| `public`  | anyone, including people with no session       |
//
// Plus two overrides that are part of the same question rather than exceptions to it:
// the author always sees their own glimt, and the Team section sees everything (below).
//
// Anonymous callers are **not** modelled here. The public page and its media handler
// (task 323) query for `audience = public AND NOT hidden` directly and ignore the session
// cookie entirely, so there is no viewer to pass in and no risk of a zero-valued viewer
// accidentally satisfying a rule. This function answers only "may this *member* see it".

// GlimtGroup is the audience bucket a member belongs to for the purpose of a `group`-scoped
// glimt: spejder, bandit, or crew.
//
// It is a third vocabulary next to Role and Population, and it is coarser than both on
// purpose. PRD 019 asked for "same user group (spejder/bandit/crew)", which means all five
// crew-ish roles — postmandskab, guide, samarit, gøgler and the unclassified fallback —
// share one bucket. Note that this puts gøglere with crew, where Population deliberately
// keeps them apart: the directory separates them because listing a gøgler among crew would
// misrepresent who someone is, while a glimt shared "with my group" by a gøgler is meant
// for the people staffing the event alongside them. Same words, different question.
type GlimtGroup string

const (
	GlimtGroupSpejder GlimtGroup = "spejder"
	GlimtGroupBandit  GlimtGroup = "bandit"
	GlimtGroupCrew    GlimtGroup = "crew"
)

// AllGlimtGroups is every group, in a stable order, so tests and audits do not keep their
// own list.
var AllGlimtGroups = []GlimtGroup{
	GlimtGroupSpejder,
	GlimtGroupBandit,
	GlimtGroupCrew,
}

// Valid reports whether g is a group this version of the code knows. Values reaching this
// from the database are untrusted like any other stored string.
func (g GlimtGroup) Valid() bool {
	for _, known := range AllGlimtGroups {
		if g == known {
			return true
		}
	}
	return false
}

// GlimtGroupFor maps an app role to its glimt group.
//
// Returns false for a role this code does not know, so an unrecognised role cannot land in
// a group by accident. That is the safe direction: the caller then fails the `group` check
// rather than joining whichever bucket a zero value would have named.
func GlimtGroupFor(r Role) (GlimtGroup, bool) {
	switch r {
	case RoleSpejder:
		return GlimtGroupSpejder, true
	case RoleBandit:
		return GlimtGroupBandit, true
	case RolePostmandskab, RoleGuide, RoleSamarit, RoleGoegler, RoleCrew:
		return GlimtGroupCrew, true
	}
	return "", false
}

// GlimtAudience is who a glimt was shared with, chosen once by its author.
//
// Immutable after creation (PRD 019 §6): widening it later would retroactively expose a
// photo that was shared under a narrower promise, so there is no update path and this type
// is only ever read.
type GlimtAudience string

const (
	// GlimtAudienceGroup is the default and the narrowest — the author's own group.
	GlimtAudienceGroup GlimtAudience = "group"
	// GlimtAudienceNathejk is every authenticated member, any role.
	GlimtAudienceNathejk GlimtAudience = "nathejk"
	// GlimtAudiencePublic is the open web, via /offentligt/glimt.
	GlimtAudiencePublic GlimtAudience = "public"
)

// AllGlimtAudiences is every audience, narrowest first. The order is meaningful: it is the
// order the composer offers them in, so the narrowest is the default and widening is a
// deliberate act.
var AllGlimtAudiences = []GlimtAudience{
	GlimtAudienceGroup,
	GlimtAudienceNathejk,
	GlimtAudiencePublic,
}

// Valid reports whether a is an audience this version of the code knows.
func (a GlimtAudience) Valid() bool {
	for _, known := range AllGlimtAudiences {
		if a == known {
			return true
		}
	}
	return false
}

// GlimtViewer is the caller a visibility question is asked about.
type GlimtViewer struct {
	// PersonID identifies the caller, so their own glimt stay visible to them whatever
	// the audience and whatever has been hidden.
	PersonID string
	Role     Role
	// IsModerator is true when the caller currently has the Team section assigned.
	//
	// It is passed in rather than derived here because it is not a property of a role:
	// it comes from a per-request lookup of the caller's section (task 300), precisely
	// so that revoking the assignment revokes the power. This package must not learn to
	// answer it from a Role or a session claim.
	IsModerator bool
}

// GlimtSubject is the glimt being asked about — only the fields the decision needs.
type GlimtSubject struct {
	AuthorPersonID string
	AuthorGroup    GlimtGroup
	Audience       GlimtAudience
	// Hidden is true once a report or a moderator has taken it down. Hiding is
	// reversible and is not a delete, so the row and its media still exist.
	Hidden bool
}

// MaySeeGlimt reports whether the viewer may see the subject glimt, in the feed or as
// media bytes.
//
// The Team-section override is **inside** this function, not a branch callers add around
// it. That is the whole design: PRD 019 §8 requires one definition of visibility, and a
// caller that could skip the check for moderators would be a caller that could skip it.
// Same predicate, one extra branch, tested both ways.
//
// Fails closed on anything it does not recognise — an unknown role, an unknown audience, a
// group that is not in AllGlimtGroups. All three are stored strings, and a glimt whose
// audience failed to parse is exactly the one not to show to a stranger.
func MaySeeGlimt(viewer GlimtViewer, g GlimtSubject) bool {
	if !viewer.Role.Valid() || !g.Audience.Valid() {
		return false
	}

	// The Team section sees every scope, including group-scoped glimt from groups they
	// are not in, and including what has been hidden — they cannot review what they
	// cannot see. PRD 019 §6 requires this reach to be disclosed in the composer and on
	// the privacy page rather than discovered; that is a UI obligation, but it is
	// recorded here because this line is what makes it true.
	if viewer.IsModerator {
		return true
	}

	// An empty PersonID must never match an empty AuthorPersonID: a glimt with no author
	// recorded is a data fault, and treating an unidentified caller as its owner would
	// hand them a hidden photo.
	own := viewer.PersonID != "" && viewer.PersonID == g.AuthorPersonID

	// Hidden glimt are visible to their author and to moderators, and to nobody else.
	// Checked before the audience, because hiding has to beat every audience — the
	// public scope publishes with no approval queue, so a report taking something down
	// is the only fast mechanism there is (PRD 019 §0).
	if g.Hidden {
		return own
	}
	if own {
		return true
	}

	switch g.Audience {
	case GlimtAudiencePublic, GlimtAudienceNathejk:
		// A member sees both alike. The difference between them is not who inside
		// Nathejk may read it but whether it also leaves the app, which is decided
		// by the public handlers, not here.
		return true
	case GlimtAudienceGroup:
		vg, ok := GlimtGroupFor(viewer.Role)
		return ok && g.AuthorGroup.Valid() && vg == g.AuthorGroup
	}
	return false
}

// GlimtFeedFilter describes, in terms a SQL query can use, which rows a viewer may be
// shown. It exists because a feed cannot call MaySeeGlimt before it has rows, and fetching
// every glimt in the event to filter three of them in Go is not an option after the race.
//
// It is a **narrowing of** MaySeeGlimt, not a second opinion on it. The querier builds its
// WHERE clause from this and the rows that come back are still passed through MaySeeGlimt
// on the way out, so a disagreement between the two can only be the filter being too
// generous — which the predicate then catches — never too strict in a way that leaks. A
// test asserts the two agree across every combination.
type GlimtFeedFilter struct {
	// Denied is true when the viewer may see nothing at all — an unrecognised role.
	//
	// It is a distinct field rather than "a filter with no group and no person id",
	// because those two are also what a legitimately group-less viewer looks like, and
	// the difference decides whether the query returns their own rows or nothing. The
	// agreement test in glimt_test.go found exactly this: without Denied, an unknown role
	// still matched its own and every public row while MaySeeGlimt refused them.
	Denied bool
	// Unrestricted is true for a moderator: every row, every audience, including hidden.
	Unrestricted bool
	// PersonID is the viewer, whose own rows are always included whatever the audience
	// and whatever is hidden. Empty means "no own rows", not "all rows".
	PersonID string
	// Group limits `group`-scoped rows to this one group. Empty means the viewer's role
	// mapped to no group, so they get no group-scoped rows at all.
	Group GlimtGroup
	// IncludeHidden is true only for a moderator. Hidden rows belonging to the viewer are
	// covered by PersonID instead, so a normal caller never needs this.
	IncludeHidden bool
}

// GlimtFeedFilterFor derives the query filter for a viewer.
//
// An invalid role yields a Denied filter that matches nothing — not even the viewer's own
// rows — which is the same fail-closed direction MaySeeGlimt takes. A role we cannot
// recognise is a session or a stored row we cannot trust, and the cost of refusing it is
// an empty feed rather than a leak.
func GlimtFeedFilterFor(viewer GlimtViewer) GlimtFeedFilter {
	if viewer.IsModerator {
		return GlimtFeedFilter{Unrestricted: true, IncludeHidden: true, PersonID: viewer.PersonID}
	}
	if !viewer.Role.Valid() {
		return GlimtFeedFilter{Denied: true}
	}
	f := GlimtFeedFilter{PersonID: viewer.PersonID}
	if g, ok := GlimtGroupFor(viewer.Role); ok {
		f.Group = g
	}
	return f
}

// Matches reports whether a row satisfies the filter. It is the Go expression of the same
// condition the SQL WHERE clause encodes, which is what lets a test assert that the two
// agree with each other and with MaySeeGlimt.
func (f GlimtFeedFilter) Matches(g GlimtSubject) bool {
	if f.Denied {
		return false
	}
	if f.Unrestricted {
		return true
	}
	// An audience we do not recognise is refused here as it is in MaySeeGlimt: the switch
	// below would fall through to false anyway, but saying it once at the top keeps the
	// two functions readable side by side.
	if !g.Audience.Valid() {
		return false
	}
	own := f.PersonID != "" && f.PersonID == g.AuthorPersonID
	if g.Hidden && !f.IncludeHidden {
		return own
	}
	if own {
		return true
	}
	switch g.Audience {
	case GlimtAudiencePublic, GlimtAudienceNathejk:
		return true
	case GlimtAudienceGroup:
		return f.Group != "" && f.Group == g.AuthorGroup
	}
	return false
}
