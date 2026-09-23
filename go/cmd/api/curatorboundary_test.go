package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The curator/public read boundary (PRD 022 §8.8, task 366).
//
// # What this guards and why a test has to
//
// `album.Queries` is publication-filtered in SQL; `album.CuratorQueries` is not. The whole safety argument
// of that split is that a public handler **cannot** reach a draft album because it is never handed the
// interface that returns one.
//
// Nothing in the type system enforces the second half of that. `app.models` is one struct and every handler
// in this package can reach every field on it, so "public handlers do not touch the curator reads" is a
// property of which files read which fields — exactly the kind of invariant that holds on the day it is
// written and quietly stops holding when somebody needs an item count on the frontpage and reaches for the
// nearest read that has one.
//
// So this walks the source, in the manner of `glimtopenapi_test.go` and `publicprivacy_test.go`.

// adminOwnedFiles are the files permitted to read the curator interfaces.
//
// A list rather than a prefix match, so adding a file to it is a deliberate edit somebody makes here, next
// to the reasoning, rather than a side effect of a filename. Empty today because the admin handlers arrive
// with tasks 372–379; the guard is in place first, which is the point — it will fail the moment a curator
// read is used from anywhere else, including from a file somebody *meant* to be admin-only but did not list.
var adminOwnedFiles = map[string]bool{
	// e.g. "admin.go": true, "adminlibrary.go": true — added as tasks 372–379 land.
}

// curatorReads are the model fields that return draft-visible data.
var curatorReads = []string{"models.AlbumCurator", "models.PhotoCurator"}

// TestOnlyTheAdminSurfaceReadsTheCuratorInterfaces fails if a curator read is used outside the admin files.
func TestOnlyTheAdminSurfaceReadsTheCuratorInterfaces(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			// Tests construct and stub these deliberately; the guard is about the served surface.
			continue
		}
		// main.go wires them onto the models in the first place, which is the one place that must.
		if name == "main.go" || name == "albummedia.go" {
			continue
		}
		if adminOwnedFiles[name] {
			continue
		}

		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, read := range curatorReads {
			if strings.Contains(string(src), read) {
				t.Errorf("%s reads %s, but it is not an admin-owned file.\n"+
					"A curator read returns unpublished albums and deleted photographs. If this file is "+
					"part of the admin surface, add it to adminOwnedFiles with a reason; if it is not, it "+
					"must use models.Albums or models.Photos instead.", name, read)
			}
		}
	}
}

// The complement, and the more important half: the **public** interfaces must not grow a way to ask for a
// draft. A parameter is how that would arrive — `Published(year, includeDrafts bool)` looks harmless in a
// diff and defeats the entire split.
//
// Asserted against the interface declarations rather than against usage, because a widened signature is
// wrong even before anybody passes `true`.
func TestThePublicAlbumInterfaceHasNoDraftSwitch(t *testing.T) {
	src, err := os.ReadFile("../../nathejk/table/album/querier.go")
	if err != nil {
		t.Fatalf("reading album/querier.go: %v", err)
	}
	text := string(src)

	start := strings.Index(text, "type Queries interface {")
	if start < 0 {
		t.Fatal("album.Queries no longer exists; this guard needs updating")
	}
	end := strings.Index(text[start:], "\n}")
	if end < 0 {
		t.Fatal("could not find the end of album.Queries")
	}
	decl := text[start : start+end]

	for _, smell := range []string{
		"includeUnpublished", "includeDrafts", "includeDeleted", "unpublished bool",
		"drafts bool", "all bool",
	} {
		if strings.Contains(decl, smell) {
			t.Errorf("album.Queries mentions %q: the public read must have no way to ask for a draft. "+
				"Use album.CuratorQueries instead (PRD 022 §8.8).", smell)
		}
	}

	// And it must still have exactly the three public reads. A fourth is not forbidden, but one that
	// returns a draft would be — so a change in the count is worth a human looking.
	methods := regexp.MustCompile(`(?m)^\t([A-Z]\w*)\(`).FindAllStringSubmatch(decl, -1)
	if len(methods) != 3 {
		var names []string
		for _, m := range methods {
			names = append(names, m[1])
		}
		t.Errorf("album.Queries has %d methods (%v); it had 3 (Published, BySlug, Plottable). "+
			"If a read was added, confirm it cannot return an unpublished album.", len(methods), names)
	}
}

// The same for the library's public read. It has no publication filter to defeat — a photograph is never
// public in its own right — but it must not become the way a draft's *contents* are reached either.
func TestThePublicPhotoInterfaceStaysNarrow(t *testing.T) {
	src, err := os.ReadFile("../../nathejk/table/photo/querier.go")
	if err != nil {
		t.Fatalf("reading photo/querier.go: %v", err)
	}
	text := string(src)

	start := strings.Index(text, "type Queries interface {")
	if start < 0 {
		t.Fatal("photo.Queries no longer exists; this guard needs updating")
	}
	end := strings.Index(text[start:], "\n}")
	decl := text[start : start+end]

	methods := regexp.MustCompile(`(?m)^\t([A-Z]\w*)\(`).FindAllStringSubmatch(decl, -1)
	if len(methods) != 2 {
		var names []string
		for _, m := range methods {
			names = append(names, m[1])
		}
		t.Errorf("photo.Queries has %d methods (%v); it had 2 (Get, RefsInUse). The curator's reads belong "+
			"on photo.CuratorQueries (PRD 022 §8.8).", len(methods), names)
	}
	// `Library` and `Counts` are the curator's; finding either here means the split has been undone.
	for _, curatorOnly := range []string{"Library(", "Counts(", "Photo("} {
		if strings.Contains(decl, curatorOnly) {
			t.Errorf("photo.Queries has gained %s, which is a curator read", curatorOnly)
		}
	}
}
