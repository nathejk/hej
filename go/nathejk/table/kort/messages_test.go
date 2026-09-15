package kort

import (
	"encoding/json"
	"testing"

	"github.com/nathejk/shared-go/types"
)

// The bodies below are copied verbatim from this repo's vendored copy of the contract,
// roadmap/api/kort-events.md. That is the point of the tests: they pin our types to the
// documented shapes, so a drift between the two is a test failure rather than a silently empty
// feature during a race.

func TestDecodeCreated(t *testing.T) {
	const body = `{ "kortId": "kort-1", "kortsaetId": "kortsaet-1", "name": "Kort 1" }`

	var got Created
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.KortID != "kort-1" || got.KortsaetID != "kortsaet-1" || got.Name != "Kort 1" {
		t.Fatalf("got %+v", got)
	}
}

// A patch that names only the sheet's name must leave every other field nil, so the projection
// can tell "unchanged" from "cleared". This is the failure that would erase a checkpoint list on
// a rename, so it gets its own test.
func TestDecodeUpdatedIsAPatch(t *testing.T) {
	const body = `{ "kortId": "kort-1", "name": "Kort 1 — Start til Post 2" }`

	var got Updated
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name == nil || *got.Name != "Kort 1 — Start til Post 2" {
		t.Fatalf("name not decoded: %+v", got.Name)
	}
	if got.CheckpointIDs != nil {
		t.Error("checkpointIds must be nil (unchanged), not an empty slice")
	}
	if got.Extents != nil {
		t.Error("extents must be nil (unchanged)")
	}
	if got.Format != nil || got.Note != nil || got.SortOrder != nil || got.KortsaetID != nil {
		t.Errorf("absent fields must stay nil: %+v", got)
	}
	if got.HandoutCheckgroupID != nil {
		t.Error("handoutCheckgroupId must be nil when absent — absent is not the QR rule")
	}
}

func TestDecodeUpdatedCheckpointIDs(t *testing.T) {
	const body = `{ "kortId": "kort-1", "checkpointIds": ["cp-1", "cp-2"] }`

	var got Updated
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.CheckpointIDs == nil {
		t.Fatal("checkpointIds not decoded")
	}
	if want := []types.CheckpointID{"cp-1", "cp-2"}; len(*got.CheckpointIDs) != len(want) {
		t.Fatalf("got %v, want %v", *got.CheckpointIDs, want)
	}
}

// An explicitly empty array is an edit — "this sheet now has no checkpoints" — and must be
// distinguishable from an absent field. A plain []T could not express it, which is why these are
// pointers to slices.
func TestDecodeUpdatedEmptyArrayIsAnEdit(t *testing.T) {
	const body = `{ "kortId": "kort-1", "checkpointIds": [], "extents": [] }`

	var got Updated
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.CheckpointIDs == nil {
		t.Fatal("empty checkpointIds must decode to a non-nil pointer to an empty slice")
	}
	if len(*got.CheckpointIDs) != 0 {
		t.Errorf("want empty, got %v", *got.CheckpointIDs)
	}
	if got.Extents == nil {
		t.Fatal("empty extents must decode to a non-nil pointer to an empty slice")
	}
	if len(*got.Extents) != 0 {
		t.Errorf("want empty, got %v", *got.Extents)
	}
}

// The empty string is a meaningful value for handoutCheckgroupId — it is the QR rule, and it is
// how an organizer switches a sheet back from a post. Absence, by contrast, means unchanged. Both
// halves are asserted, because a single pointer carrying two meanings is exactly where a consumer
// goes wrong.
func TestDecodeUpdatedHandoutCheckgroupEmptyStringIsAValue(t *testing.T) {
	t.Run("explicit empty means the QR rule", func(t *testing.T) {
		const body = `{ "kortId": "kort-1", "handoutCheckgroupId": "" }`

		var got Updated
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.HandoutCheckgroupID == nil {
			t.Fatal("explicit \"\" must decode to a non-nil pointer: it is the QR rule, not an absence")
		}
		if *got.HandoutCheckgroupID != "" {
			t.Errorf("want empty id, got %q", *got.HandoutCheckgroupID)
		}
	})

	t.Run("a checkgroup id means handout at that post", func(t *testing.T) {
		const body = `{ "kortId": "kort-1", "handoutCheckgroupId": "cg-4" }`

		var got Updated
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.HandoutCheckgroupID == nil || *got.HandoutCheckgroupID != "cg-4" {
			t.Fatalf("got %v", got.HandoutCheckgroupID)
		}
	})
}

func TestDecodeExtents(t *testing.T) {
	const body = `{ "kortId": "kort-1", "extents": [
		{ "northWest": { "latitude": 56, "longitude": 9 },
		  "southEast": { "latitude": 55, "longitude": 9.4 } } ] }`

	var got Updated
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Extents == nil || len(*got.Extents) != 1 {
		t.Fatalf("got %v", got.Extents)
	}
	e := (*got.Extents)[0]
	if e.NorthWest.Latitude != 56 || e.NorthWest.Longitude != 9 {
		t.Errorf("north-west: %+v", e.NorthWest)
	}
	if e.SouthEast.Latitude != 55 || e.SouthEast.Longitude != 9.4 {
		t.Errorf("south-east: %+v", e.SouthEast)
	}
}

func TestDecodeSorted(t *testing.T) {
	const body = `{ "kortIds": ["kort-a", "kort-b", "kort-c"] }`

	var got Sorted
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.KortIDs) != 3 || got.KortIDs[0] != "kort-a" || got.KortIDs[2] != "kort-c" {
		t.Fatalf("got %v", got.KortIDs)
	}
}

// A set's created/updated is a whole record, so an absent teamType means the set has none — not
// "unchanged". Both bodies from the contract are asserted together, because the pair is the whole
// point.
func TestDecodeSetIsAWholeRecord(t *testing.T) {
	t.Run("with a team type", func(t *testing.T) {
		const body = `{ "kortsaetId": "kortsaet-1", "name": "Patruljer", "teamType": "patrulje" }`

		var got SetCreated
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.TeamType == nil {
			t.Fatal("teamType not decoded")
		}
		if *got.TeamType != PatrolTeamType {
			t.Errorf("got %q, want %q", *got.TeamType, PatrolTeamType)
		}
	})

	t.Run("without one, meaning the set has none", func(t *testing.T) {
		const body = `{ "kortsaetId": "kortsaet-2", "name": "Crew" }`

		var got SetUpdated
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.TeamType != nil {
			t.Fatalf("absent teamType must be nil, meaning the set has none: got %q", *got.TeamType)
		}
		if got.Name != "Crew" {
			t.Errorf("got %q", got.Name)
		}
	})

	t.Run("explicit null also means none", func(t *testing.T) {
		const body = `{ "kortsaetId": "kortsaet-2", "name": "Crew", "teamType": null }`

		var got SetUpdated
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.TeamType != nil {
			t.Fatal("null teamType must be nil")
		}
	})
}

func TestDecodeSetsSorted(t *testing.T) {
	const body = `{ "kortsaetIds": ["kortsaet-a", "kortsaet-b"] }`

	var got SetsSorted
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.KortsaetIDs) != 2 {
		t.Fatalf("got %v", got.KortsaetIDs)
	}
}

// An additive upstream field must not break a consumer mid-event. This is not hypothetical: skan
// added `mapId` to qr.registered exactly this way, and hq's own contract document still says that
// field does not exist. So tolerance here is a requirement, not a nicety.
func TestDecodeIgnoresUnknownFields(t *testing.T) {
	const body = `{ "kortId": "kort-1", "name": "Kort 1",
	                "somethingHQAddedLater": { "nested": true }, "printedAt": 1234 }`

	var got Updated
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("an additive upstream field must not fail the decode: %v", err)
	}
	if got.Name == nil || *got.Name != "Kort 1" {
		t.Fatalf("known fields must still decode: %+v", got)
	}
}

// The patrol set is found by team type, and the value is `patrulje`. "spejder" is the domain's
// word for a person, HQ refuses it on write, and a filter written against it would match nothing
// and silently reveal nothing (PRD 016 §11.1). Pinned here so the mistake cannot be reintroduced
// quietly.
func TestPatrolTeamType(t *testing.T) {
	if PatrolTeamType != "patrulje" {
		t.Fatalf("PatrolTeamType is %q; the patrol map set is marked \"patrulje\"", PatrolTeamType)
	}
	if PatrolTeamType == "spejder" {
		t.Fatal("\"spejder\" is a person, not a team type, and HQ refuses it on write")
	}
}

func TestFormatValid(t *testing.T) {
	for _, f := range []Format{FormatA4, FormatA3, FormatSkitse, FormatAndet} {
		if !f.Valid() {
			t.Errorf("%q should be valid", f)
		}
	}
	for _, f := range []Format{"", "A4", "pdf"} {
		if f.Valid() {
			t.Errorf("%q should not be valid", f)
		}
	}
}
