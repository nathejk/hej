package users

// Directory grouping (PRD 007 §6, task 153).
//
// Grouping is decided on the server and handed to the client as data. The client renders
// sections from ids and labels without knowing what a klan or a section is — which is the
// point: when *lok* arrives upstream as subsections, the bandit listing gains a tier by
// changing what this file emits, not by rewriting the pane.
//
// That is also why a group is returned as a **path** rather than a single group. Today
// every path has exactly one element. A two-level bandit grouping (lok → klan) is the
// expected near-future shape, and a path that is already a slice absorbs it; a single
// `group` field would have to be replaced everywhere it is read.

// Group is one level of grouping in the directory listing.
type Group struct {
	// ID is stable and opaque to the client. It is what own-group comparison and
	// expansion state are keyed on, so it must not change when a label is edited
	// upstream.
	ID string
	// Label is what the user reads: a klan name, "Gøglere", "Crew".
	Label string
	// IsOwn marks the caller's own group, which the pane expands by default. The
	// common case then needs no interaction at all.
	IsOwn bool
}

// Group ids for the populations that form a single flat list. Fixed strings rather than
// derived from anything, because there is exactly one of each and the client keys
// expansion state on them.
const (
	groupIDGoeglere = "goeglere"
	groupIDCrew     = "crew"
	// groupIDNoKlan collects banditter whose klan is missing from the data. A person
	// with no group must still be findable — dropping them from the listing would make
	// a data problem look like a missing colleague.
	groupIDNoKlan = "uden-klan"
)

// GroupPathFor returns where the subject is listed when viewed by the viewer, in the
// context of one population.
//
// The population matters because a person can be listed twice: a crew bandit appears
// among the banditter grouped by klan, and among the crew in the crew group (see
// PopulationsOf). Which of those a given caller sees is decided by intersecting the
// subject's populations with what the viewer may list, so this function is asked about one
// population at a time rather than guessing.
//
// Returns nil when the viewer may not list that population at all, so a caller that
// forgets to check permission still cannot render a group.
func GroupPathFor(viewer, subject User, pop Population) []Group {
	if !MayList(viewer.Role, pop) || !IsListedIn(subject, pop) {
		return nil
	}

	switch pop {
	case PopulationBandit:
		// Banditter are grouped by klan. PatrolID/PatrolName carry the klan for a
		// bandit (a patrulje for a spejder), which is why the field is not called
		// KlanID — one field, two vocabularies upstream.
		if subject.PatrolID == "" {
			return []Group{{
				ID:    groupIDNoKlan,
				Label: "Uden klan",
				// Never "own": a viewer with no klan should not have a group
				// expanded on the strength of two people both missing data.
				IsOwn: false,
			}}
		}
		return []Group{{
			ID:    subject.PatrolID,
			Label: subject.PatrolName,
			IsOwn: viewer.PatrolID != "" && viewer.PatrolID == subject.PatrolID,
		}}

	case PopulationGoegler:
		return []Group{{
			ID:    groupIDGoeglere,
			Label: "Gøglere",
			// A gøgler viewer is looking at their own population. Crew viewing the
			// gøgler list are not, so the list does not auto-expand for them.
			IsOwn: viewer.Role == RoleGoegler,
		}}

	case PopulationCrew:
		return []Group{{
			ID:    groupIDCrew,
			Label: "Crew",
			IsOwn: viewer.Role.IsCrew(),
		}}
	}

	// PopulationSpejder reaches here only if MayList ever allowed it, which it must
	// not: spejdere are never listed, only looked up one patrol at a time.
	return nil
}
