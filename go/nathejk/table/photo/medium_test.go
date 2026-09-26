package photo

import (
	"os"
	"strings"
	"testing"
)

// The 800px rendition (task 409, PRD 023 §7.9).
//
// PRD 023's only schema change, and the task file is explicit about where it would bite: **the shared-blob
// delete walks must be taught about the new column.** A ref column those walks do not know about has two
// possible outcomes and both are bad — bytes orphaned on disk forever, or a live object deleted because
// nothing claimed it. Neither shows up in a test that only exercises uploads, which is why the second half
// of this file is about `RefsInUse` rather than about the fold.
//
// The second half reads source text rather than running SQL, for the reason `album/querysafety_test.go`
// records at length: `cqrs.Reader` hands back `*sql.Rows`, which nothing outside `database/sql` can
// construct, so a fake reader is not expressible without a real database or a mocking dependency — and the
// stubs in `cmd/api` bypass this package entirely, which is exactly why a bug here would not show up there.
// The property being guarded genuinely lives *in the statement*: a query that does not name a column cannot
// answer for it, whatever the Go around it does.

func TestUploadedWritesTheMediumRef(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026",
		Ref: ref("b"), ThumbRef: ref("c"), MediumRef: ref("d"),
		Width: 1600, Height: 1067, Bytes: 402_113,
		UploadedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `mediumRef="`+ref("d")+`"`) {
		t.Errorf("the medium ref was not written\n%s", stmts[0])
	}
	// On the upsert too, or a re-upload of a photograph that predates the rendition would keep its empty
	// column forever — the one case where re-dragging a folder genuinely should change something.
	if !strings.Contains(stmts[0], "mediumRef=VALUES(mediumRef)") {
		t.Errorf("the upsert does not update mediumRef, so a re-upload cannot add the rendition\n%s", stmts[0])
	}
}

// Same rule as the thumbnail's: a malformed rendition ref costs the rendition, not the photograph.
func TestUploadedBlanksAnInvalidMediumRef(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026",
		Ref: ref("b"), MediumRef: "../../etc/passwd",
		UploadedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if strings.Contains(stmts[0], "etc/passwd") {
		t.Errorf("a path-shaped medium ref reached the statement\n%s", stmts[0])
	}
	if !strings.Contains(stmts[0], `mediumRef=""`) {
		t.Errorf("an invalid medium ref must become empty, not vanish from the statement\n%s", stmts[0])
	}
}

// No backfill, and none needed: an upload that carries no medium rendition writes an empty column, which
// every reader treats as "serve the full image". This is the property that let task 409 ship without
// touching a single existing row.
func TestUploadedWithoutAMediumRefWritesEmpty(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
	})

	if !strings.Contains(stmts[0], `mediumRef=""`) {
		t.Errorf("want an empty mediumRef\n%s", stmts[0])
	}
}

// photoSource returns the text of one file in this package.
func photoSource(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(src)
}

// funcBody returns the text of one declaration, from its signature to the next top-level one.
func funcBody(t *testing.T, file, signature string) string {
	t.Helper()
	text := photoSource(t, file)

	start := strings.Index(text, signature)
	if start < 0 {
		t.Fatalf("%s no longer contains %q; this guard needs updating", file, signature)
	}
	// gofmt guarantees a top-level declaration starts at column zero, so the next one ends this.
	end := strings.Index(text[start+1:], "\nfunc ")
	if end < 0 {
		return text[start:]
	}
	return text[start : start+1+end]
}

// **The load-bearing test of this file**, and the break-test task 409's acceptance criteria ask for.
//
// `RefsInUse` decides whether a blob may be deleted, and it is asked from inside the delete paths. If it
// does not interrogate `mediumRef`, then a medium rendition **shared** with another live photograph — which
// content addressing makes the expected case rather than a corner one (PRD 011 §0b.2) — is reported unused
// and purged out from under that photograph.
//
// Both halves are checked, because they fail differently. A column missing from the `SELECT` means a
// matching row's own ref cannot be compared with what was asked about; a column missing from the `WHERE`
// means the row is never found at all.
func TestRefsInUseInterrogatesEveryRefColumn(t *testing.T) {
	body := funcBody(t, "querier.go", "func (q querier) RefsInUse(")

	for _, c := range []struct{ column, cost string }{
		{"blobRef", "a photograph's only copy deleted while another photograph still shows it"},
		{"thumbRef", "a thumbnail deleted from under a live album grid"},
		{"mediumRef", "an 800px rendition deleted from under a live viewer, or orphaned on disk forever"},
	} {
		if !strings.Contains(body, "SELECT photoId") || !strings.Contains(body, c.column) {
			t.Errorf("RefsInUse never mentions %s. Cost of the omission: %s (task 368, task 409)\n%s",
				c.column, c.cost, body)
			continue
		}
		if !strings.Contains(body, c.column+` IN (`) && !strings.Contains(body, `"`+c.column+`"`) {
			t.Errorf("RefsInUse does not filter on %s, so a photograph whose only match is that column is "+
				"never seen. Cost: %s\n%s", c.column, c.cost, body)
		}
	}
}

// The columns list and the placeholder arithmetic must be derived from one another.
//
// Getting this wrong is neither a compile error nor a wrong answer in a stubbed test — it is a MariaDB
// argument-count error raised inside a delete path. That fails closed, which is the right direction, but it
// fails only at runtime and only when somebody deletes a photograph.
func TestRefsInUseDerivesItsPlaceholdersFromItsColumns(t *testing.T) {
	body := funcBody(t, "querier.go", "func (q querier) RefsInUse(")

	if strings.Contains(body, "len(refs)*2") {
		t.Error("RefsInUse still sizes its arguments for two ref columns. There are three (task 409), and a " +
			"hand-counted multiplier is what breaks the next time one is added")
	}
	if !strings.Contains(body, "for range columns") && !strings.Contains(body, "range columns") {
		t.Error("RefsInUse should build its argument list by iterating the column list, so the clause, the " +
			"placeholders and the bound arguments cannot disagree")
	}
}

// The single-photograph read has to select every ref column too: `admindelete.go` builds its purge list from
// this row, so a column absent here is a rendition the purge never even considers.
func TestGetSelectsEveryRefColumn(t *testing.T) {
	body := funcBody(t, "querier.go", "func (q querier) Get(")

	for _, col := range []string{"blobRef", "thumbRef", "mediumRef"} {
		if !strings.Contains(body, col) {
			t.Errorf("Get does not select %s, so a caller building a ref list from a Photo would miss it\n%s",
				col, body)
		}
	}
}

// The curator's read, for the same reason: the library delete path reads a LibraryPhoto.
func TestTheLibraryColumnsIncludeEveryRef(t *testing.T) {
	src := photoSource(t, "curator.go")

	start := strings.Index(src, "const libraryColumns")
	if start < 0 {
		t.Fatal("curator.go no longer has libraryColumns; this guard needs updating")
	}
	// The value is a raw string literal, so slice between its backticks rather than to the first newline —
	// which is where the opening backtick sits.
	open := strings.Index(src[start:], "`")
	if open < 0 {
		t.Fatal("libraryColumns is no longer a raw string literal; this guard needs updating")
	}
	close := strings.Index(src[start+open+1:], "`")
	if close < 0 {
		t.Fatal("libraryColumns' literal is unterminated")
	}
	list := src[start+open+1 : start+open+1+close]

	for _, col := range []string{"p.blobRef", "p.thumbRef", "p.mediumRef"} {
		if !strings.Contains(list, col) {
			t.Errorf("libraryColumns omits %s. The contact sheet would render from it and the library delete "+
				"path would never purge it\n%s", col, list)
		}
	}
}

// Every ref column needs its own index, because every one of them is interrogated by `RefsInUse` **inside a
// delete path**. An unindexed one turns a purge check into a scan of the year's photographs.
//
// Checked in table.sql and in table.go, because they do different jobs: table.sql builds a correct table on
// a fresh database, and `CREATE TABLE IF NOT EXISTS` does nothing to one that has already booted — which is
// the trap task 393 hit with the credit column and task 409's process note restates.
func TestEveryRefColumnIsIndexedOnBothPaths(t *testing.T) {
	schema := photoSource(t, "table.sql")
	for _, key := range []string{"KEY ref_lookup (blobRef)", "KEY thumb_lookup (thumbRef)", "KEY medium_lookup (mediumRef)"} {
		if !strings.Contains(schema, key) {
			t.Errorf("table.sql is missing %s", key)
		}
	}

	boot := photoSource(t, "table.go")
	if !strings.Contains(boot, `"mediumRef"`) {
		t.Error("table.go does not EnsureColumn mediumRef, so an existing database keeps the old table and " +
			"every read naming the column fails at its first query (task 409's process note)")
	}
	if !strings.Contains(boot, "medium_lookup") {
		t.Error("table.go does not ensure the medium_lookup index. EnsureColumn adds columns, not keys, so " +
			"on an existing database the purge check would scan the year's photographs")
	}
}
