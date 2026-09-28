package main

// The read side of the photo credit (PRD 025, task 452).
//
// # Why this is a handler's job and not a join
//
// A credit is either a name a curator typed or a reference to a crew member (PRD 025 §6 R1), and the reference
// has to become a name before any page sees it. That resolution could have been a SQL join in the album and
// library reads — and was not, deliberately.
//
// `photo` and `person` are projections folded from independent event streams. A join across them in a read's
// statement would put the credit's four bounds (crew only, name only, one year, absent-is-absent) into *every*
// statement that renders a credit, which is how they come to differ. `person.CreditNames` owns those bounds; the
// reads fetch references, this merges names in, and there is one function to audit rather than a pattern to look
// for.
//
// # Why an unresolvable reference renders nothing
//
// PRD 025 §6 R5, and it is the erasure path rather than an error path: deleting a crew member's person record is
// how they are removed from every photograph they took (§6 R4). So "no name came back" is the *expected* outcome
// of a deletion, not a failure — a placeholder, an empty label or a logged error would each turn somebody
// exercising that right into an incident.
//
// A photograph with no credit already renders without one, so absence needs no new handling downstream.

// creditNames resolves credit references to names, or returns an empty map.
//
// Nil-safe in the way this codebase's reads are: an unavailable person projection means no names, which lands on
// the same path as a deleted crew member. The alternative — failing the page — would take a public album offline
// because a *credit line* could not be resolved.
func (app *application) creditNames(year string, ids []string) map[string]string {
	if app.models.People == nil || len(ids) == 0 {
		return map[string]string{}
	}
	names, err := app.models.People.CreditNames(year, ids)
	if err != nil {
		// Logged and degraded, not surfaced. The page is the photographs; the credit is a line under them.
		app.Logger.Error("resolving photo credits", "year", year, "err", err)
		return map[string]string{}
	}
	return names
}

// resolvedCredit is the credit to render: the typed one, or the resolved crew member's name.
//
// One place, so the precedence is stated once. `creditCrewId` wins when it is set, and that is not a tie-break
// but a consequence: the writer clears one when it sets the other (task 450), in the handler *and* in the fold,
// so a photograph carrying both is a row that should not exist. Preferring the reference means such a row
// resolves to the name that can still be erased, which is the safer of the two readings.
func resolvedCredit(typed, crewID string, names map[string]string) string {
	if crewID != "" {
		return names[crewID]
	}
	return typed
}
