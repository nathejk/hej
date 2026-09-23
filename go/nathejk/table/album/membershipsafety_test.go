package album

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A membership is addressed by photograph, never by position (task 386).
//
// # The bug this family guard exists for
//
// `album_item` uses the ordinal as part of its primary key, which makes it tempting to treat a position as
// an identity. It is not one. A position is a slot in one album; the photograph is what somebody chose, and
// the two stop agreeing the moment the album is reordered.
//
// Three separate bugs came from that one confusion, all found on the same afternoon, all in this file:
//
//  1. **`handleItemAdded` deadlettered on every boot.** An `itemadded` folded onto a table that already
//     reflected a later reorder named an ordinal one photograph holds and a photoId another holds, so the
//     upsert conflicted with two rows on two keys and MariaDB refused.
//  2. **`handleItemRemoved` removed the wrong photograph.** Keyed on the ordinal, it soft-deleted whoever had
//     since moved into that slot — silently, off a public album.
//  3. **`handleItemsReordered`'s vacate collided with itself**, because a removed row stranded in the offset
//     range by an earlier reorder still occupied the position the next vacate wanted.
//
// Each is a one-line fix and each would be re-introduced by somebody writing the obvious statement. So the
// rule is asserted rather than remembered.
//
// # Why this reads the source
//
// The same reason `querysafety_test.go` does, stated there at length: these properties live in SQL text, and
// `cqrs.Writer` is `Consume(string) error` with no way to execute a statement against a fake. It is blunt,
// and it is honest about what it checks — the presence of a clause, not the behaviour of a database. The
// behaviour was verified against the dev database; see the task log.

// foldBody returns the code of one consumer method, from its signature to the next top-level declaration.
//
// **Comment lines are stripped**, and that is not tidiness. These guards search for SQL fragments, and the
// explanations above each statement necessarily quote the very fragments being forbidden — the first draft of
// `TestTheAddDoesNotOverwriteWhateverHoldsThePosition` failed against its own comment explaining why
// `INSERT IGNORE` was rejected. A guard that reads prose is a guard that fires on the documentation.
func foldBody(t *testing.T, method string) string {
	t.Helper()

	src, err := os.ReadFile("consumer.go")
	if err != nil {
		t.Fatalf("reading consumer.go: %v", err)
	}
	text := string(src)

	start := strings.Index(text, "func (c consumer) "+method+"(")
	if start < 0 {
		t.Fatalf("consumer.go no longer has a %s method; this guard needs updating", method)
	}
	// Sliced to the next top-level `func`, not to the next comment heading. A guard that sliced to a heading
	// broke once when a function was inserted between the two — it then read somebody else's body and passed.
	end := strings.Index(text[start+1:], "\nfunc ")
	body := text[start:]
	if end >= 0 {
		body = text[start : start+1+end]
	}

	var code []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		code = append(code, line)
	}
	return strings.Join(code, "\n")
}

// whereClauses returns every WHERE clause in a statement, to the end of the statement text.
var whereClause = regexp.MustCompile(`WHERE [^"]*`)

// **The rule.** A membership write locates its row by `photoId`.
//
// Matched on `ordinal=` rather than on the word, because `handleItemsReordered`'s vacate legitimately says
// `ORDER BY ordinal DESC` while naming no row at all — it moves the whole album, which is the point of it.
// Its placement statements are covered here, and `reorder_test.go` asserts them separately.
func TestNoMembershipFoldAddressesARowByItsPositionAlone(t *testing.T) {
	for _, method := range []string{"handleItemAdded", "handleItemsReordered"} {
		body := foldBody(t, method)
		for _, clause := range whereClause.FindAllString(body, -1) {
			if !strings.Contains(clause, "albumId=") {
				// Not a membership lookup — a subquery fragment or an album-level statement.
				continue
			}
			if strings.Contains(clause, "ordinal=") && !strings.Contains(clause, "photoId") {
				t.Errorf("%s locates a membership by position alone:\n  %s\n\n"+
					"A position is a slot in one album; the photograph is what somebody chose, and the two "+
					"stop agreeing the moment the album is reordered. Match on photoId. See task 386.",
					method, clause)
			}
		}
	}
}

// `handleItemRemoved` is the one that may still name an ordinal — but only in the legacy branch, and the
// branch that has a photoId must not.
//
// Asserted by position in the function rather than by counting clauses: the photograph branch returns before
// the fallback, so the first `deleted=1` statement is the one that matters and it must be the photograph's.
func TestARemovalPrefersThePhotographAndOnlyFallsBackToThePosition(t *testing.T) {
	body := foldBody(t, "handleItemRemoved")

	byPhoto := strings.Index(body, "photoId=%s")
	byOrdinal := strings.Index(body, "ordinal=%d")
	if byPhoto < 0 {
		t.Fatal("a removal must be able to address the photograph, or a replay after a reorder removes the " +
			"wrong one — silently, off a public album (task 386)")
	}
	if byOrdinal < 0 {
		t.Fatal("the ordinal fallback must stay: events published before task 386 carry only an ordinal, and " +
			"skipping a removal would put a photograph somebody asked to have taken down back on a page")
	}
	if byPhoto > byOrdinal {
		t.Error("the photograph branch must come first; the ordinal is the fallback, not the rule")
	}

	// And the fallback must be reachable only when there is no photoId to use.
	if !strings.Contains(body, `if body.PhotoID != ""`) {
		t.Error("the ordinal fallback must be guarded on the absence of a photoId, or it would apply to " +
			"events that told us exactly which photograph to remove")
	}
}

// The reorder's vacate moves rows in descending order.
//
// Without it the statement collides with itself as soon as the album holds a row already in the offset range
// — which it does after any reorder of an album with a removed membership, because a removed photograph is
// not named in the order and nothing places it back down. Reachable with no replay at all: remove an item,
// then reorder twice.
func TestTheReorderVacatesInDescendingOrder(t *testing.T) {
	body := foldBody(t, "handleItemsReordered")

	vacate := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "ordinal = ordinal + %d") {
			vacate = line
			break
		}
	}
	if vacate == "" {
		t.Fatal("could not find the vacate statement; this guard needs updating")
	}
	if !strings.Contains(vacate, "ORDER BY ordinal DESC") {
		t.Errorf("the vacate must move rows highest-first, or it writes onto a position another row still "+
			"holds — the same reason a shift-right of an array walks backwards\ngot: %s", vacate)
	}
}

// The add is a conditional insert, not an upsert that names the photograph in its update list.
//
// `photoId=VALUES(photoId)` is what made the old statement die: it means "put this photograph at this
// position, whatever is there", which is an intention the writer never has — and when the two keys point at
// two different rows, updating one violates the other.
func TestTheAddDoesNotOverwriteWhateverHoldsThePosition(t *testing.T) {
	body := foldBody(t, "handleItemAdded")

	if strings.Contains(body, "photoId=VALUES(photoId)") {
		t.Error("an add must not claim a position from another photograph. That update list is what " +
			"deadlettered every boot: with the two keys pointing at two rows, writing photoId on one " +
			"violates the unique key on the other (task 386)")
	}
	if !strings.Contains(body, "WHERE NOT EXISTS") {
		t.Error("the insert must be conditional on the photograph not already being in the album")
	}
	// `INSERT IGNORE` would also pass a naive reading of the line above while being materially worse: it
	// swallows a primary key clash too, so a membership that genuinely failed to record would do so in
	// silence instead of as a dead letter.
	if strings.Contains(body, "INSERT IGNORE") {
		t.Error("INSERT IGNORE hides a real ordinal clash as well as the harmless one. A membership that " +
			"was not recorded must stay loud")
	}
}
