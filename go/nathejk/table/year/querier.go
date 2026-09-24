package year

import (
	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the application.
//
// Two reads. `Years` arrived with the curator's year selector (task 392), which the maintainer asked for: until
// then there was deliberately no list, because the only caller wanted the configured event.
type Queries interface {
	// Years returns every event year, newest first — four-digit slugs only, so the stray `null` row hq has
	// produced (and anything else that is not a year) never becomes something a curator can file photographs
	// into.
	Years() ([]string, error)

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

func (q querier) Years() ([]string, error) {
	rows, err := q.db.Query(`SELECT slug FROM event_year WHERE slug REGEXP '^[0-9]{4}$' ORDER BY slug DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		out = append(out, slug)
	}
	return out, rows.Err()
}
