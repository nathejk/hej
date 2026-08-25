package person

import (
	"fmt"
	"strings"
)

// The cqrs.Writer contract takes a finished SQL statement, not a statement plus
// arguments (see cqrs.Writer's doc: "A Writer applies a single statement"). So
// projectors build literals, and every shared-go entity does the same with
// fmt.Sprintf and %q.
//
// That means escaping is this file's responsibility rather than the driver's. These
// helpers exist so no call site formats a value by hand — the values here come from
// event bodies produced by another service, which is not the same as trusted input.

// quote renders a Go string as a SQL string literal.
//
// %q produces a double-quoted, backslash-escaped literal, which MariaDB accepts as a
// string unless ANSI_QUOTES is enabled (it is not, in either the dev or prod config).
// This matches what shared-go's projectors do, deliberately: a second convention
// here would be one more thing to get subtly wrong.
func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

// nullableQuote renders a value that may be genuinely absent.
//
// The distinction is the whole reason this helper exists: an empty string and SQL
// NULL mean different things in this table. NULL on phoneParent means "this
// population has no guardian number"; "" would mean "should have one and it is
// missing". PRD 003 renders those differently and PRD 005 branches on it.
func nullableQuote(s string) string {
	if s == "" {
		return "NULL"
	}
	return quote(s)
}

// boolInt renders a bool as MariaDB's TINYINT(1).
func boolInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// upsert builds an idempotent INSERT ... ON DUPLICATE KEY UPDATE for the person
// table.
//
// Idempotency is not optional: projections are rebuilt by replaying the stream from
// sequence zero on every boot, so every one of these statements runs again on each
// start, and a message may be redelivered within a single run (cqrs.Consumer's
// contract). A plain INSERT would fail the whole replay on the second boot.
//
// Only the columns a given event actually carries are written. That is what lets
// several projectors cooperate on one row without clobbering each other: the
// spejder handler owns the person's own details, the patrulje handler owns the team
// name, and neither resets the other's columns to a zero value.
func upsert(cols map[string]string) string {
	if len(cols) == 0 {
		return ""
	}

	// Sorted for a deterministic statement. Not cosmetic: identical input must
	// produce identical SQL, or a dead-lettered statement is impossible to match
	// against the event that produced it.
	names := make([]string, 0, len(cols))
	for name := range cols {
		names = append(names, name)
	}
	sortStrings(names)

	values := make([]string, 0, len(names))
	updates := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, cols[name])
		// personId and year are the primary key; updating them to themselves is
		// harmless but pointless, and listing them invites confusion about whether
		// the key can change.
		if name == "personId" || name == "year" {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", name, name))
	}

	if len(updates) == 0 {
		// A key-only upsert: insert the row if absent, otherwise leave it alone.
		return fmt.Sprintf(
			"INSERT IGNORE INTO person (%s) VALUES (%s)",
			strings.Join(names, ", "), strings.Join(values, ", "),
		)
	}

	return fmt.Sprintf(
		"INSERT INTO person (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		strings.Join(names, ", "),
		strings.Join(values, ", "),
		strings.Join(updates, ", "),
	)
}

// upsertKeepingRole is upsert, except that appRole is written on insert only.
//
// The distinction exists for exactly one column and for a concrete failure. A crew
// member's app role is derived from their *section assignment*, which arrives on a
// different event from their personal details — and their details are re-published
// whenever an organizer edits them, and re-delivered on every replay. With a plain
// upsert, `crewmember.updated` would set appRole back to the generic "crew" fallback
// that handleCrewMemberUpdated has to supply for a person whose section it does not
// know. For a samarit that silently removes their SOS page: no error, no dead letter,
// just a member who quietly loses a capability some time after someone corrected a
// typo in their email.
//
// Insert-only is safe in the other direction because the assignment handler writes
// appRole with an unconditional UPDATE, so it always wins regardless of which event
// lands first.
func upsertKeepingRole(cols map[string]string) string {
	statement := upsert(cols)
	if statement == "" {
		return ""
	}
	// Rewrite rather than parameterise upsert: this keeps the common path — the one
	// every other handler uses — free of a flag that would need explaining at each
	// call site.
	const marker = " ON DUPLICATE KEY UPDATE "
	idx := strings.Index(statement, marker)
	if idx < 0 {
		// An INSERT IGNORE (key-only upsert) already writes nothing on conflict.
		return statement
	}

	kept := make([]string, 0)
	for _, assignment := range strings.Split(statement[idx+len(marker):], ", ") {
		if strings.HasPrefix(assignment, "appRole=") {
			continue
		}
		kept = append(kept, assignment)
	}
	if len(kept) == 0 {
		// Nothing left to update: degrade to insert-if-absent rather than emit a
		// syntactically invalid empty update list.
		return strings.Replace(statement[:idx], "INSERT INTO", "INSERT IGNORE INTO", 1)
	}
	return statement[:idx] + marker + strings.Join(kept, ", ")
}

// sortStrings is a tiny insertion sort, to keep this file free of imports beyond
// fmt/strings — the package is bound for shared-go and every dependency it carries is
// one the move has to justify.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
