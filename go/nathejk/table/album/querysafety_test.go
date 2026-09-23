package album

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Guards on the SQL the public reads execute (PRD 022 §8.4, task 365).
//
// # Why these read the source text instead of running a query
//
// The safety properties of this package live **in SQL**, not in Go: `Plottable`'s whole claim is that a
// handler "cannot plot an unchecked coordinate by forgetting a condition", which is true precisely because
// the conditions are in the statement rather than in a caller. So the thing worth testing is the statement.
//
// It cannot be tested by executing it. `cqrs.Reader` returns `*sql.Rows`, which nothing outside
// `database/sql` can construct, so a fake reader is not expressible without either a real database in the
// test suite or a mocking dependency — and the stubs in `cmd/api` bypass this file entirely, which is
// exactly why a bug here would not show up there.
//
// Reading the source is the same technique `cmd/api/glimtopenapi_test.go` and
// `cmd/api/publicprivacy_test.go` already use for invariants the type system cannot hold: parse what was
// written, and fail when a condition somebody relied on stops being there. It is blunt, and it is honest
// about what it checks — the presence of a clause, not the behaviour of a database.

// queryBody returns the text of one querier method, from its signature to its closing brace.
func queryBody(t *testing.T, method string) string {
	t.Helper()

	src, err := os.ReadFile("querier.go")
	if err != nil {
		t.Fatalf("reading querier.go: %v", err)
	}
	text := string(src)

	start := strings.Index(text, "func (q querier) "+method+"(")
	if start < 0 {
		t.Fatalf("querier.go no longer has a %s method; this guard needs updating", method)
	}
	// The next top-level func is the end of this one. Sufficient because gofmt guarantees a top-level
	// declaration starts at column zero.
	end := strings.Index(text[start+1:], "\nfunc ")
	if end < 0 {
		return text[start:]
	}
	return text[start : start+1+end]
}

// **The load-bearing test of this package.** Only `inside` may be plotted, and only from an album the
// curator published, and only while both the item and the photograph are live.
//
// Five conditions, and every one of them is a thing somebody could plausibly delete while "simplifying the
// join". Each is listed with what its absence would put on a public map.
func TestPlottableFiltersEverythingItMust(t *testing.T) {
	body := queryBody(t, "Plottable")

	for _, c := range []struct {
		clause string
		cost   string
	}{
		{"p.boundsVerdict = ?", "coordinates that were never checked, or judged out of bounds, on the public map"},
		{"a.published = 1", "photographs from a draft album the curator has not shown anybody"},
		{"a.deleted = 0", "photographs from an album that was taken down"},
		{"i.deleted = 0", "a photograph removed from this album still pinned to it"},
		{"p.deleted = 0", "a photograph the curator deleted still on the map"},
		{"p.latitude IS NOT NULL", "a marker at the equator for a photograph with no coordinate"},
		{"p.longitude IS NOT NULL", "a marker at the equator for a photograph with no coordinate"},
	} {
		if !strings.Contains(body, c.clause) {
			t.Errorf("Plottable no longer filters %q.\nWithout it the public map shows: %s", c.clause, c.cost)
		}
	}

	// And the verdict placeholder must be fed `BoundsInside` specifically. The clause above only proves
	// *a* verdict is filtered on; this proves it is the plottable one.
	if !strings.Contains(body, "BoundsInside") {
		t.Error("Plottable must filter on BoundsInside; any other verdict is not plottable by definition")
	}
	for _, wrong := range []string{"BoundsUnknown", "BoundsOutside"} {
		if strings.Contains(body, wrong) {
			t.Errorf("Plottable must not mention %s: only inside is plottable", wrong)
		}
	}
}

// The coordinate belongs to the photograph now. A read that took it from `album_item` would be reading a
// column that no longer exists — but a read that *wrote* one there would recreate the divergence PRD 022
// §8.3 removed, so the absence is asserted rather than left to the schema.
func TestNoReadTakesACoordinateFromTheMembership(t *testing.T) {
	src, err := os.ReadFile("querier.go")
	if err != nil {
		t.Fatalf("reading querier.go: %v", err)
	}

	// `i.` is the membership alias throughout this file; `p.` is the photograph.
	for _, forbidden := range []string{
		"i.latitude", "i.longitude", "i.boundsVerdict", "i.blobRef", "i.thumbRef", "i.caption",
	} {
		if strings.Contains(string(src), forbidden) {
			t.Errorf("%s: a photograph's facts come from the photo table, not from the membership row",
				forbidden)
		}
	}
}

// Every read of the photograph must require it to be live, or a curator's deletion would take effect on
// some surfaces and not others — the album page but not the cover, or the count but not the map.
//
// Checked across the whole file rather than per method, because the failure mode is one join being added
// later without the filter.
func TestEveryPhotoJoinRequiresThePhotographToBeLive(t *testing.T) {
	src, err := os.ReadFile("querier.go")
	if err != nil {
		t.Fatalf("reading querier.go: %v", err)
	}
	text := string(src)

	joins := regexp.MustCompile(`JOIN\s+photo\s+p\s+ON`).FindAllStringIndex(text, -1)
	if len(joins) == 0 {
		t.Fatal("no join to photo found; after PRD 022 every album read needs one")
	}

	// Each join must have a `p.deleted = 0` within the statement it belongs to. Statements here are raw
	// string literals, so the backtick that closes one bounds the search.
	for _, j := range joins {
		rest := text[j[0]:]
		if end := strings.Index(rest, "`"); end >= 0 {
			rest = rest[:end]
		}
		if !strings.Contains(rest, "p.deleted = 0") {
			t.Errorf("a join to photo at offset %d does not require p.deleted = 0:\n%s", j[0], rest)
		}
	}
}

// The item count and the cover must agree with the album page about which photographs exist. They are
// separate subqueries, so "the album says 12 billeder and shows 11" is a bug that ships easily.
func TestTheCountAndTheCoverSeeTheSamePhotographsAsThePage(t *testing.T) {
	published := queryBody(t, "Published")

	// Both subqueries join the photograph and require it live. Asserted by counting, because there are
	// exactly two and one of them being right is the likely failure.
	if got := strings.Count(published, "JOIN photo p ON p.photoId = i.photoId"); got != 2 {
		t.Errorf("want both the count and the cover subquery to join photo, found %d joins", got)
	}
	if got := strings.Count(published, "p.deleted = 0"); got != 2 {
		t.Errorf("want both subqueries to require a live photograph, found %d", got)
	}
}
