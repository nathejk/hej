package main

import (
	"time"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/person"
)

// The Glimt wire shapes (PRD 019 §6, task 302).
//
// # The rule this file exists to enforce
//
// A glimt is **owned by a person and attributed to their hold**. The person is never disclosed —
// not their name, not their portrait, not their id — with exactly one exception, the Team-section
// moderation queue, which cannot review what it cannot attribute.
//
// That is enforced *structurally* rather than by discipline: `glimtResponse` has no field capable
// of carrying an author, so a handler cannot leak one by forgetting to strip it, and a future
// change that wants to add one has to add a field and explain itself. This is the same approach
// `.rules` demands for `phoneParent`, and for the same reason — the failure mode is silent, and it
// is a child's identity attached to a photograph.
//
// The author's *own* feed is not an exception. Their card reads "Din patrulje" plus a delete
// action, which is enough to convey ownership; sending their name so the client can decide to show
// "you" would put the one payload most likely to be cached on disk in the one place we said it
// would not be.

// glimtResponse is one glimt as the app sees it.
//
// Note what is absent and must stay absent: `authorPersonId`, any name, any portrait reference, any
// phone number. `Own` conveys everything a client needs about authorship.
type glimtResponse struct {
	ID string `json:"id"`

	// Hold is the attribution — number, name and group — frozen when the glimt was created.
	Hold holdAttribution `json:"hold"`

	// Own is true when the caller is the author. It is a **boolean rather than an id
	// comparison the client performs**, which is the point: the client never receives an
	// author id to compare against.
	Own bool `json:"own"`

	Audience string `json:"audience"`
	Caption  string `json:"caption"`

	CreatedAt time.Time `json:"created_at"`

	Media []glimtMediaResponse `json:"media"`

	// Hidden is true when this glimt has been taken down. It only ever reaches its author or
	// a moderator — anyone else simply does not see the glimt — so it is safe to send, and
	// the author is entitled to know their post was hidden rather than silently discovering
	// nobody can see it.
	Hidden bool `json:"hidden,omitempty"`
}

// holdAttribution is who posted, as far as anyone is told.
//
// Number, name and group at every scope including public: a parent must be able to recognise their
// own child's patrulje, which is the only thing that makes the public feed worth publishing
// (PRD 019 §0).
type holdAttribution struct {
	// Number is the hold's number ("42"). Empty for crew, who have a section rather than a
	// numbered hold — a normal state, and the client renders the name alone.
	Number string `json:"number,omitempty"`
	// Name is the patrulje or klan name, or the section name for crew.
	Name string `json:"name,omitempty"`
	// Group is spejder | bandit | crew, so a reader can tell a Patrulje 42 from a Klan 42.
	Group string `json:"group"`
}

// glimtMediaResponse is one media item, addressed by ordinal rather than by content hash.
//
// **The blob ref is deliberately not here.** The client fetches
// `/api/glimt/{id}/media/{ordinal}`, which re-checks visibility. Sending the hash instead would
// hand out a capability: a content-addressed URL that anyone holding the string could try, which is
// exactly how a group-scoped photo becomes public to whoever it was forwarded to. Ordinals are
// meaningless without the glimt id, and the glimt id is access-controlled.
type glimtMediaResponse struct {
	Ordinal int    `json:"ordinal"`
	Kind    string `json:"kind"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	// DurationMs is omitted for stills.
	DurationMs int `json:"duration_ms,omitempty"`
	// HasThumb tells the client whether to request `?variant=thumb` or fall back to the full
	// item. Without it the grid would either 404 on items whose thumbnail failed, or fetch
	// full-size media for every tile — and the post-race browse cannot afford the second.
	HasThumb bool `json:"has_thumb"`
}

// newGlimtResponse projects a stored glimt into the shape a member may see.
//
// The one place `glimt.Glimt.AuthorPersonID` is read on a read path, and it is read only to
// compute `Own` — it is never copied into the result, which has nowhere to put it.
func newGlimtResponse(g glimt.Glimt, viewerPersonID string) glimtResponse {
	out := glimtResponse{
		ID: g.GlimtID,
		Hold: holdAttribution{
			Number: g.TeamNumber,
			Name:   g.TeamName,
			Group:  g.AuthorGroup,
		},
		Own:       viewerPersonID != "" && viewerPersonID == g.AuthorPersonID,
		Audience:  g.Audience,
		Caption:   g.Caption,
		CreatedAt: g.CreatedAt,
		Hidden:    g.HiddenAt != nil,
		Media:     make([]glimtMediaResponse, 0, len(g.Media)),
	}
	for _, m := range g.Media {
		out.Media = append(out.Media, glimtMediaResponse{
			Ordinal:    m.Ordinal,
			Kind:       m.Kind,
			Width:      m.Width,
			Height:     m.Height,
			DurationMs: m.DurationMs,
			HasThumb:   m.ThumbRef != "",
		})
	}
	return out
}

// newGlimtResponses projects a page.
func newGlimtResponses(gs []glimt.Glimt, viewerPersonID string) []glimtResponse {
	out := make([]glimtResponse, 0, len(gs))
	for _, g := range gs {
		out = append(out, newGlimtResponse(g, viewerPersonID))
	}
	return out
}

// publicGlimtResponse is what the open web sees.
//
// A separate type from glimtResponse rather than the same one with fields omitted. Two reasons, and
// the second is the important one: `Own` and `Hidden` are meaningless without a caller, and — more
// to the point — the public payload is the one whose contents somebody will eventually have to
// certify. A distinct struct can be read top to bottom and understood completely, which a shared
// struct with conditional population cannot.
type publicGlimtResponse struct {
	ID        string               `json:"id"`
	Hold      holdAttribution      `json:"hold"`
	Caption   string               `json:"caption"`
	CreatedAt time.Time            `json:"created_at"`
	Media     []glimtMediaResponse `json:"media"`
}

func newPublicGlimtResponse(g glimt.Glimt) publicGlimtResponse {
	out := publicGlimtResponse{
		ID: g.GlimtID,
		Hold: holdAttribution{
			Number: g.TeamNumber,
			Name:   g.TeamName,
			Group:  g.AuthorGroup,
		},
		Caption:   g.Caption,
		CreatedAt: g.CreatedAt,
		Media:     make([]glimtMediaResponse, 0, len(g.Media)),
	}
	for _, m := range g.Media {
		out.Media = append(out.Media, glimtMediaResponse{
			Ordinal:    m.Ordinal,
			Kind:       m.Kind,
			Width:      m.Width,
			Height:     m.Height,
			DurationMs: m.DurationMs,
			HasThumb:   m.ThumbRef != "",
		})
	}
	return out
}

// moderationGlimtResponse is the Team section's view, and the **only** response carrying an author.
//
// It exists because moderation cannot work against an anonymous author: answering a report means
// knowing whose glimt it is, and a takedown that cannot name anyone is not reviewable. Kept as its
// own type so that "which responses disclose the author?" is answerable by grepping for this struct
// — one type, one handler, one route, all requiring the Team-section assignment.
type moderationGlimtResponse struct {
	glimtResponse

	// AuthorPersonID is the owner. The one disclosure in the feature.
	AuthorPersonID string `json:"author_person_id"`
	// AuthorName is resolved for the queue, because a person id is not something a human
	// moderating at 03:00 can act on.
	AuthorName string `json:"author_name,omitempty"`

	ReportCount int    `json:"report_count"`
	HiddenBy    string `json:"hidden_by,omitempty"`
}

// attributionFor derives the frozen attribution for a new glimt from the author's current record.
//
// **Called once, at creation.** The values are then stored on the glimt row and never re-derived
// (PRD 019 §6): a glimt keeps saying what it said even if the hold is renamed, even if the author
// moves to another patrulje, and even if their role changes. A moment that re-labels itself a year
// later is not a record of anything.
//
// Crew get their section name and no number, which is not missing data — they have a section rather
// than a numbered hold.
func attributionFor(p person.Person, role users.Role) (group, number, name string) {
	g, ok := users.GlimtGroupFor(role)
	if !ok {
		// An unrecognised role cannot be attributed to a group, and a group is what decides
		// who a `group`-scoped glimt reaches. The caller refuses the post rather than
		// guessing — see createGlimtHandler.
		return "", "", ""
	}
	group = string(g)

	if g == users.GlimtGroupCrew {
		// Section rather than team: a crew member's `teamName` is empty upstream, and their
		// affiliation is the section they staff.
		return group, "", p.SectionName
	}
	return group, p.TeamNumber, p.TeamName
}
