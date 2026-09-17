package main

import (
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
)

// Glimt (PRD 019).
//
// This file holds the pieces every Glimt handler needs before any of them exist: who the
// caller is, and whether they moderate.

// isGlimtModerator reports whether this member currently has the Team section assigned, and
// may therefore read every glimt at every scope and hide anything (PRD 019 §0, task 300).
//
// # Why this is a database read and not a session claim
//
// It would be cheaper to put `section` in the signed session cookie and read it from there.
// That option is deliberately refused. Sessions here are stateless and signed with no
// server-side store, which is what lets them survive a restart — and also means there is no
// way to invalidate one. A moderation claim inside a cookie would keep working for the whole
// life of that cookie after the assignment was taken away, and the person who took it away
// would have no way to make it stop. One indexed read per moderation request is the right
// price for revocation that actually revokes.
//
// If someone later wants to cache this, the thing to preserve is not the read — it is the
// property that removing the assignment removes the power promptly. A cache with a TTL
// measured in seconds would be fine; a claim minted at login would not.
//
// # Fails closed
//
// No database, no person row, an empty slug, or any other section all return false. Every
// one of those is a routine state rather than an exception: the API is designed to run
// without a database (PRD 008 §5), and a member with no section assigned is normal upstream
// data. The safe answer to "can this person hide other people's photos?" when we cannot tell
// is no.
func (app *application) isGlimtModerator(personID string) bool {
	p, found := app.person(personID)
	if !found {
		return false
	}
	return users.MayModerateGlimt(p.SectionSlug)
}

// glimtQueriesOrNil narrows the projection to its read API, or nil.
//
// The same shape as peopleOrNil, and for the same reason: a typed nil `*glimt.Table` stored in an
// interface field is not `== nil`, so every handler's "is this available?" check would pass and then
// panic on the first call. Converting here is what makes the nil checks in handlers honest.
func glimtQueriesOrNil(t *glimt.Table) glimt.Queries {
	if t == nil {
		return nil
	}
	return t
}

// glimtViewer builds the visibility subject for a caller.
//
// Every read path goes through this rather than assembling a users.GlimtViewer inline, so
// that no handler can forget the moderator lookup and quietly answer a narrower question
// than the feed did — the failure mode PRD 019 §8 is written against.
func (app *application) glimtViewer(personID string, role users.Role) users.GlimtViewer {
	return users.GlimtViewer{
		PersonID:    personID,
		Role:        role,
		IsModerator: app.isGlimtModerator(personID),
	}
}

// glimtSubjectOf narrows a stored row to the fields a visibility decision needs.
//
// The conversion lives here rather than in the glimt package because that package may not import
// internal/... (it is bound for shared-go), and it lives in one function rather than at each call
// site because a handler that built the subject by hand could omit `Hidden` — which would make a
// taken-down glimt visible again, silently, in exactly one code path.
func glimtSubjectOf(g glimt.Glimt) users.GlimtSubject {
	return users.GlimtSubject{
		AuthorPersonID: g.AuthorPersonID,
		AuthorGroup:    users.GlimtGroup(g.AuthorGroup),
		Audience:       users.GlimtAudience(g.Audience),
		Hidden:         g.HiddenAt != nil,
	}
}
