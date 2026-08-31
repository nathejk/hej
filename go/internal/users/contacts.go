package users

// Contacts authorization (PRD 007).
//
// This file is the *only* place that decides who may see whom in the contacts pane.
// The manifest, the photo handler, the person profile and the patrol lookup all route
// through these three functions. That is deliberate: if this logic exists in two
// places it will diverge, and the failure mode is either a privacy breach or a broken
// game — a spejder whose face reaches a bandit damages the event itself, not just the
// person.
//
// The rules, from PRD 007 §6:
//
//	| viewer | spejder            | bandit | gøgler | crew |
//	|--------|--------------------|--------|--------|------|
//	| spejder| — no pane at all — |        |        |      |
//	| bandit | never              | all    | never  | all  |
//	| gøgler | never              | never  | all    | all  |
//	| crew   | patrol lookup only | all    | all    | all  |
//
// Three things about that table are easy to get wrong later:
//
//   - **Spejdere are never listable, by anyone.** Crew reach them only through the
//     patrol lookup, which is a live, uncached, exact-match, audited request for one
//     patrol. That is why "may list" and "may look up" are separate questions here
//     rather than one permission with a flag.
//   - **Crew is one role, not three.** samarit, guide and postmandskab are all crew
//     (PRD 007 §6, 2026-08-31), and so is the unclassified RoleCrew fallback. The
//     patrol lookup goes to all of them — a deliberate decision, not an accident of
//     the fallback, since in the real data every crew member currently lands on the
//     fallback (task 078).
//   - **Placement is not permission.** Which population a person is *listed in* comes
//     from their role and section slug (PopulationsOf below) — a crew member out as a
//     bandit is listed among banditter while still viewing as crew. Permission and
//     placement are answered by different functions on purpose.

// Population is the group a person is listed under in the contacts directory.
//
// It is not the same thing as a Role, and the difference is the point: a crew member
// whose section slug is `bandit` has Role crew and is listed among banditter. Keeping
// the two vocabularies apart is what stops a placement change from being mistaken for a
// permission change.
type Population string

const (
	PopulationSpejder Population = "spejder"
	PopulationBandit  Population = "bandit"
	PopulationGoegler Population = "gøgler"
	PopulationCrew    Population = "crew"
)

// AllPopulations is every population, in a stable order, so tests and audits can
// enumerate them without keeping their own list.
var AllPopulations = []Population{
	PopulationSpejder,
	PopulationBandit,
	PopulationGoegler,
	PopulationCrew,
}

// MayUseContacts reports whether the viewer gets the contacts pane at all.
//
// Everyone except a spejder. This gates the nav entry, the routes and every endpoint;
// the nav gating alone is only cosmetic, since a hidden menu item is not access
// control.
func MayUseContacts(viewer Role) bool {
	return viewer.Valid() && viewer != RoleSpejder
}

// MayList reports whether the viewer may see the given population in the directory
// listing — which includes search results, group listings, favourites and profiles.
//
// No caller may list spejdere. That is checked first and unconditionally, because it
// is the property most worth making impossible to lose in a refactor: a spejder
// appearing in a bandit's list is a broken race, and a spejder appearing in anyone's
// list is a minor's face in a cached payload we promised not to build.
func MayList(viewer Role, subject Population) bool {
	if !MayUseContacts(viewer) {
		return false
	}
	if subject == PopulationSpejder {
		return false
	}

	// Crew are visible to everyone who has the pane: knowing what the samarit coming
	// to help you looks like is useful, and crew are adults acting in an official
	// capacity.
	if subject == PopulationCrew {
		return true
	}

	if viewer.IsCrew() {
		// Crew see the whole adult directory.
		return true
	}

	// Participants see their own population and nothing across the divide. The
	// bandit/gøgler separation is not a privacy rule but a game one; it is symmetric
	// and has no exceptions.
	switch viewer {
	case RoleBandit:
		return subject == PopulationBandit
	case RoleGoegler:
		return subject == PopulationGoegler
	}
	return false
}

// MayLookUpPatrol reports whether the viewer may use the patrol lookup — the single
// path by which a spejder's details are reachable in this app.
//
// Crew only, all crew included. Deliberately not extended to banditter or gøglere: the
// lookup exists for a safety task, not a game one, and giving banditter any path to
// spejder faces would undo the game-integrity property the directory's structure
// provides for free.
func MayLookUpPatrol(viewer Role) bool {
	return MayUseContacts(viewer) && viewer.IsCrew()
}
