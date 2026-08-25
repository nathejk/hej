package users_test

import (
	"testing"

	"nathejk.dk/internal/users"
)

// A shared number is a recognized number: it must still resolve through LookupAll,
// or request-pin would refuse to send an SMS to exactly the people who need the
// disambiguation flow.
func TestLookupAllReturnsEveryOwnerOfASharedNumber(t *testing.T) {
	dir := users.NewMockDirectory()

	got := dir.LookupAll(users.MockSharedPhone)
	if len(got) != 2 {
		t.Fatalf("want 2 owners of the shared number, got %d", len(got))
	}
	if got[0].ID == got[1].ID {
		t.Fatal("the two owners must be distinct people")
	}
}

// The safety rule: Lookup collapses "ambiguous" into "not found", so a caller that
// has not thought about collisions gets a refused login rather than logging someone
// in as their sibling.
func TestLookupRefusesASharedNumber(t *testing.T) {
	dir := users.NewMockDirectory()

	if _, ok := dir.Lookup(users.MockSharedPhone); ok {
		t.Fatal("Lookup must report not-found for a shared number; returning one of the owners would be a guess")
	}
}

func TestLookupStillResolvesAUniqueNumber(t *testing.T) {
	dir := users.NewMockDirectory()

	u, ok := dir.Lookup("+4530000001")
	if !ok {
		t.Fatal("a uniquely held number must still resolve")
	}
	if u.Role != users.RoleSpejder {
		t.Fatalf("role = %q, want spejder", u.Role)
	}
}

// Order must be stable, or a disambiguation prompt reshuffles between requests and
// the user cannot tell which entry they picked last time.
func TestLookupAllOrderIsStable(t *testing.T) {
	dir := users.NewMockDirectory()

	first := dir.LookupAll(users.MockSharedPhone)
	for range 5 {
		next := dir.LookupAll(users.MockSharedPhone)
		if len(next) != len(first) {
			t.Fatalf("result length changed: %d then %d", len(first), len(next))
		}
		for i := range first {
			if next[i].ID != first[i].ID {
				t.Fatalf("order changed at %d: %q then %q", i, first[i].ID, next[i].ID)
			}
		}
	}
}

// An unknown number must be indistinguishable from a known one at this level: the
// empty slice carries no more information than "nothing here".
func TestLookupAllUnknownNumberReturnsEmpty(t *testing.T) {
	dir := users.NewMockDirectory()

	if got := dir.LookupAll("+4599999999"); len(got) != 0 {
		t.Fatalf("want no matches for an unknown number, got %d", len(got))
	}
}

// Both owners of a shared number must still be resolvable by id, since that is how a
// session is restored once one of them has been chosen.
func TestBothSharedOwnersResolveByID(t *testing.T) {
	dir := users.NewMockDirectory()

	for _, u := range dir.LookupAll(users.MockSharedPhone) {
		got, ok := dir.Get(u.ID)
		if !ok {
			t.Fatalf("%q does not resolve by id", u.ID)
		}
		if got.ID != u.ID {
			t.Fatalf("Get(%q) returned %q", u.ID, got.ID)
		}
	}
}
