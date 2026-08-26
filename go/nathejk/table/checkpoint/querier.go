package checkpoint

import (
	"database/sql"

	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the application.
//
// It exposes the **area**, not the checkpoints. That is the security boundary: the event area
// is deliberately not fully known to participants (PRD 002), so an interface that returned
// positions would put the decision "should this reach the client?" at every call site, and one
// of them would eventually get it wrong. The hull is the most that may leave the server, so
// the hull is all this offers.
type Queries interface {
	// RaceArea returns the buffered hull of the year's positioned checkpoints.
	//
	// ok is false when there is nothing to derive an area from — no checkpoints yet, none
	// with positions, or a result too large to be plausible. Callers must fall back rather
	// than substituting a default region.
	RaceArea(year string) (RaceArea, bool, error)
}

type querier struct {
	db cqrs.Reader
	// positionless reports how many checkpoints lacked a position, in aggregate. See
	// ReportPositionless.
	positionless func(year string, positionless, total int)
}

// RaceArea derives the area from the projection.
//
// Computed on demand rather than stored. It is a handful of rows and a hull over at most a few
// dozen points, so caching it would add an invalidation problem — the checkpoint set changes
// up to the event — in exchange for microseconds.
func (q querier) RaceArea(year string) (RaceArea, bool, error) {
	rows, err := q.db.Query(`
		SELECT latitude, longitude
		FROM checkpoint
		WHERE year = ? AND deleted = 0
		ORDER BY checkpointId`, year)
	if err != nil {
		return RaceArea{}, false, err
	}
	defer rows.Close()

	var points []Point
	total := 0
	for rows.Next() {
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&lat, &lng); err != nil {
			return RaceArea{}, false, err
		}
		total++
		if !lat.Valid || !lng.Valid {
			continue
		}
		points = append(points, Point{Lat: lat.Float64, Lng: lng.Float64})
	}
	if err := rows.Err(); err != nil {
		return RaceArea{}, false, err
	}

	// Reported here rather than per event: individual gaps are expected, the systematic case
	// is what matters, and only the aggregate shows it (see ReportPositionless).
	if q.positionless != nil && total > 0 {
		q.positionless(year, total-len(points), total)
	}

	area, ok := ComputeRaceArea(points, total)
	return area, ok, nil
}

var _ Queries = querier{}
