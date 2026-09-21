package trackpoint

import (
	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the merge.
//
// # One read, and it takes the people it is allowed to see
//
// `ByPeople` is bounded by the ids the caller names, following the discipline `checkpoint.Queries`
// established: there is no "every point in the year" to ask for. A handler cannot widen the question to
// somebody else's route even by mistake, because the projection will not answer it.
//
// That matters more here than for checkpoints. A checkpoint position is a secret about the *course*; a
// position track is a record of where a named child was, minute by minute, on one night. The read being
// bounded is what keeps "which people?" a decision made in one place (the patrol's membership) rather than
// a parameter forty call sites could get wrong.
type Queries interface {
	// ByPeople returns the recorded points for the named people, grouped by person, each group in time
	// order.
	//
	// The grouping is the reason this does not return a flat slice: the merge is a **union of per-person
	// segments**, and a flat time-ordered list is precisely the shape that produces a zig-zag between two
	// members walking ten metres apart. Handing back the grouping makes the correct merge the easy one.
	//
	// Empty in, empty out, and no query — the rule the checkpoint projection's bounded reads follow:
	// treating an empty filter as "everything" is how a narrow read becomes a table scan.
	//
	// An empty result is the **common case**, not an error. Task 082 measured 2% coverage, and plenty of
	// members never grant location at all.
	ByPeople(year string, personIDs []string) ([][]Point, error)
}

// Point is one recorded position.
//
// Carries no person id. The grouping in `ByPeople`'s result is what keeps one person's points together,
// and once merged there is deliberately no way to ask which member a point came from — that is PRD 011
// §0b.1's requirement, enforced by the type rather than by the handler remembering.
type Point struct {
	// TS is epoch milliseconds.
	TS int64

	Lat, Lng float64

	// Accuracy is the reported radius in metres, or 0 when unknown.
	Accuracy float64
}

type querier struct {
	db cqrs.Reader
}

// ByPeople returns each named person's points, in time order.
//
// One query rather than one per person, because a patrol has up to ~8 members and eight round trips to
// serve one page is eight chances to be slow on a public route. The rows come back ordered by person then
// time, so the grouping is a single pass with no map and no re-sort.
func (q querier) ByPeople(year string, personIDs []string) ([][]Point, error) {
	if len(personIDs) == 0 {
		return nil, nil
	}

	args := make([]any, 0, len(personIDs)+1)
	args = append(args, year)
	for _, id := range personIDs {
		args = append(args, id)
	}

	rows, err := q.db.Query(`
		SELECT personId, ts, lat, lng, accuracy
		FROM track_point
		WHERE year = ? AND personId IN (`+placeholders(len(personIDs))+`)
		ORDER BY personId ASC, ts ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups [][]Point
	var current []Point
	lastPerson := ""

	for rows.Next() {
		var personID string
		var p Point
		if err := rows.Scan(&personID, &p.TS, &p.Lat, &p.Lng, &p.Accuracy); err != nil {
			return nil, err
		}
		if personID != lastPerson {
			if len(current) > 0 {
				groups = append(groups, current)
			}
			current = nil
			lastPerson = personID
		}
		current = append(current, p)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups, rows.Err()
}

// placeholders renders n comma-separated `?` marks.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, '?')
	}
	return string(out)
}

var _ Queries = querier{}
