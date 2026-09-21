package year

import (
	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the application.
//
// One read. There is no "list every year": the only caller wants the event it is configured for, and a list
// read would be the beginning of a year-switcher nobody has asked for.
type Queries interface {
	// Route returns the event's two cities, and whether there is a route worth printing.
	//
	// ok is false for an unknown year **and** for a year whose cities are not both filled in. The two are one
	// answer on purpose: every caller's question is "is there a route line to render", and a caller handed
	// `("Lundby", "")` has to invent the rule for itself. A diploma reading "fra Lundby til " is worse than
	// one with no route line at all — and the line is omitted, never guessed (see internal/diploma).
	Route(slug string) (from, to string, ok bool, err error)
}

type querier struct {
	db cqrs.Reader
}

func (q querier) Route(slug string) (string, string, bool, error) {
	if slug == "" {
		return "", "", false, nil
	}

	rows, err := q.db.Query(`SELECT cityDeparture, cityDestination FROM event_year WHERE slug = ?`, slug)
	if err != nil {
		return "", "", false, err
	}
	defer rows.Close()

	if !rows.Next() {
		// No row for this year. Normal before anybody has edited the year in hq, and on a fresh replay of a
		// stream whose year events predate retention.
		return "", "", false, rows.Err()
	}
	var from, to string
	if err := rows.Scan(&from, &to); err != nil {
		return "", "", false, err
	}
	if from == "" || to == "" {
		return "", "", false, rows.Err()
	}
	return from, to, true, rows.Err()
}

var _ Queries = querier{}
