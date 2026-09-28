package photo

import (
	"strings"
	"testing"
)

// The `ids` filter (task 438): which of these photographs the projection can see.
//
// # Why this is tested at the SQL level and not only through the endpoint
//
// The filter exists to answer a question about *timing* — has the consumer folded the upload yet — and the caller
// treats an empty answer as "not yet, ask again". A filter that quietly matched nothing, or everything, would turn
// that into either a wait that never ends or a reload that fires too early: both look like the bug this replaced.
// So the condition itself is worth pinning, including that the ids are **bound** rather than spliced.

func TestTheIDFilterAsksForExactlyThoseIDs(t *testing.T) {
	where, args := Filter{PhotoIDs: []string{"a", "b", "c"}}.where("2026")

	if !strings.Contains(where, "p.photoId IN (?, ?, ?)") {
		t.Errorf("want one placeholder per id, got %q", where)
	}
	// The year first and the live-only condition still there: a presence read must not be a way around either.
	if !strings.HasPrefix(where, "p.year = ?") || !strings.Contains(where, "p.deleted = 0") {
		t.Errorf("the id filter must compose with the year and the live-only default, got %q", where)
	}
	want := []any{"2026", "a", "b", "c"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("arg %d = %v, want %v", i, args[i], want[i])
		}
	}
}

// An id is never spliced into the statement, which is the reason the HTTP layer can accept it from a query string
// at all. Asserted with a value that would be a syntax error if it were.
func TestTheIDFilterBindsRatherThanSplices(t *testing.T) {
	nasty := `x') OR 1=1 --`
	where, args := Filter{PhotoIDs: []string{nasty}}.where("2026")

	if strings.Contains(where, nasty) {
		t.Errorf("an id reached the SQL text: %q", where)
	}
	if len(args) != 2 || args[1] != nasty {
		t.Errorf("the id must arrive as an argument, got %v", args)
	}
}

// No ids means no condition — the ordinary contact sheet read must not acquire an `IN ()` that matches nothing.
func TestNoIDFilterAddsNoCondition(t *testing.T) {
	where, _ := Filter{}.where("2026")
	if strings.Contains(where, "photoId IN") {
		t.Errorf("an empty id list must add nothing, got %q", where)
	}
}
