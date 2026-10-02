package photo

import (
	"strings"
	"testing"
)

// The photographer's original on the upload fold (PRD 027 R2/R3, task 477).
//
// # Why this is its own file
//
// Because the rule it pins is not "a column gets written". It is **"a later event must not be able to destroy the
// only copy of somebody's file"**, and that deserves to be findable by name rather than buried among the fold's
// other thirty cases.
//
// The hazard is specific and not obvious. `photoId` is the hash of the stored **display** rendition, not of the
// upload — so the same id legitimately arrives from two different files: the same photograph with its EXIF
// stripped, or re-saved by an editor, produces the same 1600px re-encode and therefore the same id. Re-uploading is
// the documented recovery procedure when a batch half-failed (task 372), so this is a routine path, not a corner
// case.
//
// Under a plain `VALUES(...)` upsert, re-uploading a stripped copy of a photograph would blank the original already
// held. Every other column in this fold recovers from that by being re-derivable, re-typed by a curator, or simply
// re-sent. The original does not: there is no second copy anywhere in the system, by design.

// originalColumns are the five that must move together.
var originalColumns = []string{
	"originalRef", "originalContentType", "originalBytes", "originalWidth", "originalHeight",
}

func TestUploadedWritesTheOriginal(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026",
		Ref: ref("b"), ThumbRef: ref("c"), MediumRef: ref("d"),
		Width: 1600, Height: 1067, Bytes: 402_113,
		Original: &Original{
			Ref: ref("e"), ContentType: "image/jpeg", Bytes: 8_412_907,
			// Deliberately not 1600-proportional, and deliberately portrait where the display image is
			// landscape: these describe the stored bytes **before** rotation, so a photograph taken sideways has
			// them swapped relative to width/height. A test using proportional numbers would pass against code
			// that confused the two.
			Width: 4000, Height: 6000,
		},
		UploadedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{
		`originalRef="` + ref("e") + `"`,
		`originalContentType="image/jpeg"`,
		"originalBytes=8412907",
		"originalWidth=4000",
		"originalHeight=6000",
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("want %s\ngot: %s", want, stmts[0])
		}
	}
}

// No original is a normal event, not a degraded one.
//
// It is the state of every photograph uploaded before PRD 027 shipped, and of any upload where storing the file was
// declined. The row says "" / 0 and readers fall back to the display image.
func TestUploadedWithoutAnOriginalWritesEmptyColumns(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{`originalRef=""`, `originalContentType=""`, "originalBytes=0",
		"originalWidth=0", "originalHeight=0"} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("want %s\ngot: %s", want, stmts[0])
		}
	}
}

// **The one that matters: a re-upload may add an original and may never take one away.**
//
// Asserted on the `ON DUPLICATE KEY UPDATE` clause rather than by running two statements, because that is where the
// rule lives — `cqrs.Writer` is `Consume(string) error` with no way to execute against a fake, which is the same
// constraint `querysafety_test.go` and `membershipsafety_test.go` work under and explain at length.
//
// A plain `originalRef=VALUES(originalRef)` here would be the bug, and it is the kind that reads as correct in a
// diff: every other rendition in this clause is written exactly that way, precisely because losing one is survivable.
func TestUploadedNeverBlanksAnOriginalItAlreadyHas(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
		Original: &Original{Ref: ref("e"), Width: 4000, Height: 6000},
	})
	clause := upsertClause(t, stmts[0])

	for _, col := range originalColumns {
		// Every one of the five is guarded on **originalRef**, not on its own value. That is what keeps the group
		// consistent: a later event either replaces the whole file or touches none of it. Guarding each column on
		// itself would let a new ref land beside the previous file's dimensions — a row describing a photograph
		// that does not exist, with nothing to check it against.
		want := col + `=IF(VALUES(originalRef)="", ` + col + `, VALUES(` + col + `))`
		if !strings.Contains(clause, want) {
			t.Errorf("the upsert must keep %s when the new event carries no original.\nwant: %s\ngot: %s",
				col, want, clause)
		}
	}

	// And the mistake stated directly, so the failure names it. `photoId` is the hash of the display rendition, so
	// a re-upload of a stripped copy of the same photograph arrives with the same id and no original.
	for _, forbidden := range originalColumns {
		if strings.Contains(clause, forbidden+"=VALUES("+forbidden+")") {
			t.Errorf("%s is written as a plain VALUES(): a re-upload carrying no original would blank it, and "+
				"the original is the one thing in this projection that cannot be produced again. See task 372 — "+
				"re-uploading is the documented recovery procedure, and the id is the hash of the *display* "+
				"rendition, so the same id legitimately arrives from a stripped copy of the same photograph.",
				forbidden)
		}
	}
}

// An unusable ref costs the original, not the photograph — but the whole group goes with it.
//
// The same rule the renditions follow, with one asymmetry worth stating: losing a rendition costs bandwidth, while
// losing an original costs the only copy of the file. So the **writer** fails the upload rather than publishing a bad
// ref (PRD 027 §5); this blanking is the fold's last line of defence against a malformed message, not a path anybody
// is meant to take.
func TestUploadedBlanksAnInvalidOriginalRef(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
		Original: &Original{Ref: "../../etc/passwd", ContentType: "image/jpeg", Bytes: 99, Width: 4000, Height: 6000},
	})

	if strings.Contains(stmts[0], "passwd") {
		t.Fatalf("an unusable ref reached the statement: a ref is the one string here that becomes a "+
			"filesystem path\ngot: %s", stmts[0])
	}
	// Not just the ref: the metadata goes too. Keeping the dimensions of a file we did not store would describe a
	// photograph that is not there, and a reader asking "is an original held?" would get the wrong answer from any
	// column but the ref.
	for _, want := range []string{`originalRef=""`, `originalContentType=""`, "originalBytes=0",
		"originalWidth=0", "originalHeight=0"} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("want %s — a rejected original must leave no trace of itself\ngot: %s", want, stmts[0])
		}
	}
}

// The original is not written by the metadata events.
//
// `MediumAdded` exists because a rendition can be produced later (task 433). An original cannot: there is no source
// to produce it from, which is the whole of PRD 027's "no backfill is possible". So no event other than `Uploaded`
// may set these columns — a second writer would be a second answer to "what is the photographer's file".
func TestOnlyTheUploadFoldWritesTheOriginal(t *testing.T) {
	stmts := fold(t, "PHOTO.2026.photo."+hash("a")+".mediumadded", MediumAdded{
		PhotoID: hash("a"), Year: "2026", MediumRef: ref("d"), AddedAt: at,
	})

	for _, stmt := range stmts {
		for _, col := range originalColumns {
			if strings.Contains(stmt, col) {
				t.Errorf("mediumadded writes %s. An original cannot be produced after the fact — there is "+
					"nothing to produce it from — so only the upload fold may set these.\ngot: %s", col, stmt)
			}
		}
	}
}

// upsertClause returns the part of an INSERT after ON DUPLICATE KEY UPDATE.
//
// Sliced rather than searched whole, because the insert half and the update half contain the same column names and a
// needle matching either would make these guards meaningless: `originalRef="…"` appears in both, and the rule being
// tested is about the update half alone.
func upsertClause(t *testing.T, stmt string) string {
	t.Helper()

	const marker = "ON DUPLICATE KEY UPDATE "
	i := strings.Index(stmt, marker)
	if i < 0 {
		t.Fatalf("the upload fold no longer upserts; this guard needs updating\ngot: %s", stmt)
	}
	return stmt[i+len(marker):]
}
