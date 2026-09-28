package album

import (
	"strings"
	"testing"
)

// The album sort mode, in the fold (PRD 024 §6 R1/R9, task 442).

func TestCreatedWritesTheSortModeItCarries(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.created", Created{
		AlbumID: "al-1", Year: "2026", Slug: "s", Title: "T",
		SortMode: SortModeTimeAsc, CreatedAt: at,
	})
	if !strings.Contains(stmts[0], `sortMode="time-asc"`) {
		t.Errorf("want the mode written on insert\ngot: %s", stmts[0])
	}
}

// **The other half of R9.** A create that says nothing about the mode must not write the column at all, so the
// schema's `manual` default stands — which is how a log written before this field existed folds to an album
// nothing will ever re-sort. Writing `manual` explicitly here would look equivalent and is not: it would also
// overwrite a curator's chosen mode on the next replay of the create.
func TestCreatedWithNoSortModeLeavesTheColumnAlone(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.created", Created{
		AlbumID: "al-1", Year: "2026", Slug: "s", Title: "T", CreatedAt: at,
	})
	if strings.Contains(stmts[0], "sortMode") {
		t.Errorf("a create with no mode must not mention sortMode, so the column default applies\ngot: %s",
			stmts[0])
	}
}

// The mode is a curator's setting after creation, so it belongs out of the update clause for the same reason
// `published` and `deleted` are: a re-delivered create would otherwise re-sort an album somebody had switched
// to manual, and their arrangement is not recoverable from the log.
func TestCreatedDoesNotResetTheSortModeOnReplay(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.created", Created{
		AlbumID: "al-1", Year: "2026", Slug: "s", Title: "T",
		SortMode: SortModeFilenameAsc, CreatedAt: at,
	})
	clause := stmts[0][strings.Index(stmts[0], "ON DUPLICATE KEY UPDATE"):]
	if strings.Contains(clause, "sortMode") {
		t.Errorf("the update clause must not set sortMode\n%s", clause)
	}
}

func TestCreatedRefusesAnUnknownSortMode(t *testing.T) {
	for _, mode := range []string{"tid", "time", "TIME-ASC", "manual "} {
		err := foldErr(t, "NATHEJK.2026.album.al-1.created", Created{
			AlbumID: "al-1", Year: "2026", Slug: "s", Title: "T", SortMode: mode, CreatedAt: at,
		})
		if err == nil {
			t.Errorf("sortMode %q should be refused", mode)
		}
	}
}

func TestUpdatedSetsTheSortMode(t *testing.T) {
	for _, mode := range SortModes() {
		stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
			AlbumID: "al-1", Year: "2026", SortMode: str(mode), UpdatedAt: at,
		})
		if len(stmts) != 1 || !strings.Contains(stmts[0], `sortMode="`+mode+`"`) {
			t.Errorf("want sortMode %q written\ngot: %v", mode, stmts)
		}
	}
}

// Refused rather than stored, including the empty string: "not mentioned" is a nil pointer, so a blank mode is
// a malformed message. Stored, it would leave the album in a state nothing recomputes and nothing reports.
func TestUpdatedRefusesAnUnknownSortMode(t *testing.T) {
	for _, mode := range []string{"", "tid", "time", "TIME-ASC", "manual "} {
		err := foldErr(t, "NATHEJK.2026.album.al-1.updated", Updated{
			AlbumID: "al-1", Year: "2026", SortMode: str(mode), UpdatedAt: at,
		})
		if err == nil {
			t.Errorf("sortMode %q should be refused", mode)
		}
	}
}

// An update that mentions no mode must not write the column, like every other field on this delta.
func TestUpdatedWithoutASortModeDoesNotWriteIt(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", Title: str("Ny titel"), UpdatedAt: at,
	})
	if strings.Contains(stmts[0], "sortMode") {
		t.Errorf("an update that did not mention the mode must not write it\ngot: %s", stmts[0])
	}
}

// The validator is the one gate, and its list must be the offered list: a mode the tool offers but the
// validator rejects is a select that 400s, and one the validator accepts but nothing offers is a mode no
// recompute implements.
func TestValidSortModeAgreesWithTheOfferedList(t *testing.T) {
	modes := SortModes()
	if len(modes) != 5 {
		t.Fatalf("PRD 024 §6 R1 names five modes, SortModes() has %d: %v", len(modes), modes)
	}
	seen := map[string]bool{}
	for _, m := range modes {
		if !ValidSortMode(m) {
			t.Errorf("SortModes() offers %q but ValidSortMode rejects it", m)
		}
		if seen[m] {
			t.Errorf("SortModes() lists %q twice", m)
		}
		seen[m] = true
	}
	if modes[0] != SortModeManual {
		t.Errorf("manual is the inert value and the column default; it belongs first, got %q", modes[0])
	}
	for _, m := range []string{"", " ", "Manual", "time", "time_asc", "filename"} {
		if ValidSortMode(m) {
			t.Errorf("ValidSortMode must reject %q", m)
		}
	}
}
