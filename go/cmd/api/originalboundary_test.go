package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Originals stay behind the admin credential (PRD 027 R10–R12, task 476).
//
// # What this guards, and why it is worth three tests
//
// PRD 027 keeps the photographer's file byte for byte — full resolution, **metadata and GPS included** — as
// `photo.originalRef`. That reverses what `cmd/api/albummedia.go`'s header states absolutely ("**Never** change the
// pipeline to preserve EXIF because this feature wants a coordinate"), and it is safe for exactly one reason:
//
//	an original is a **download**, behind `requireAdmin`. It is never a thing a reader is served.
//
// Every rendition — thumb, medium, the 1600px display image — is still decoded and re-encoded, which strips
// everything. `TestStoreAlbumImageReadsTheCoordinateAndStripsIt` pins that half and must never need editing.
//
// This file pins the other half. The prohibition PRD 027 relaxes was protecting two different things, and only one
// of them moved: *the archive* may now hold metadata, *no reader* may be handed it. Without a guard that second
// clause is a convention, and conventions lose to a plausible-looking diff.
//
// # Why the failure mode justifies a source walk
//
// Nothing goes red when a public route gains a `variant=original` branch. No page breaks, no test fails, no log
// line appears. What happens instead is that, some years later, a photograph of a child is served to the open web
// with the coordinates of where it was taken inside the file. That is not a bug anyone finds by using the product,
// which is precisely the kind of invariant this repo expresses as a test that reads its own source — see
// `publicprivacy_test.go` and `curatorboundary_test.go` for the same technique and the same reasoning.
//
// # Written before the data existed
//
// Task 476 landed **before** task 477 added the column, deliberately. A guard written after the fact is a guard
// written to pass against whatever was built; this one was written against the field's name while there was nothing
// to find, so it describes the intended rule rather than the achieved state. It passes in both worlds.

// originalRefNeedles are the spellings that mean "this code is reaching for the photographer's file".
//
// The projection column, the Go field, and the query/variant value a URL would carry. All three, because the leak
// could arrive as any one of them: a handler reading `p.OriginalRef`, a template writing `variant=original`, or a
// new query helper naming the column directly.
var originalRefNeedles = []string{"originalRef", "OriginalRef", "variant=original", `"original"`}

// adminFilesPermittedAnOriginal are the files allowed to resolve an original, each with the reason it is.
//
// A named list rather than a prefix match, for the reason `curatorboundary_test.go` gives about its own: adding a
// file has to be a deliberate edit somebody makes *here*, next to the justification, rather than a side effect of
// choosing a filename. **Every entry is the argument for that file being allowed to hand over EXIF.**
//
// Note what is not here and must never be: `publicsite.go`, `albumpage.go`, `viewer/`, anything under `vue/`.
var adminFilesPermittedAnOriginal = map[string]bool{
	// The library upload. Writes the original in the first place (PRD 027 R1) — the one file that must, since it is
	// the only place the uploaded bytes exist before they are re-encoded away.
	"albummedia.go": true,
	// The upload endpoint, which publishes the ref on `photo.Uploaded` (R2).
	"adminupload.go": true,
	// The video transcode worker (PRD 029, task 495). Reads a video's original to encode it, and never serves it:
	// what it publishes are the renditions, re-encoded with `-map_metadata -1`. Not behind `requireAdmin` because it
	// is not a route at all; nothing outside the process can reach it.
	"videoworker.go": true,
	// The admin media route, which serves `variant=original` as a deliberate single-photograph download (R6).
	// Behind `requireAdmin`, with no rendition-repair plan: an original cannot be rebuilt.
	"adminlibrary.go": true,
	// The album zip, whose `size=original` streams the stored originals in the album's order (R5).
	"adminalbumzip.go": true,
	// The library takedown. Must free the original's bytes too (R8) — a takedown that left an EXIF-bearing file
	// behind would be a takedown in name only, which is the one failure here that cannot be repaired afterwards.
	"admindelete.go": true,
}

// filesWithAnUnrelatedOriginal have their own `original`, belonging to a different feature.
//
// **This is not a permission list and must not be used as one.** These files are skipped because the word collides,
// not because they may hand over a library original — and the distinction is worth the separate map, because the
// moment the two lists merge, "may serve the photographer's file" and "happens to use the same identifier" become
// the same claim.
//
// Found by this guard on its first run, which is the argument for writing it before the data existed: the needle
// `originalRef` is not as specific as it looks.
var filesWithAnUnrelatedOriginal = map[string]bool{
	// The **portrait** original (task 111, PRD 003/007). A different feature with the opposite rule: a portrait
	// original is metadata-**stripped** (`imaging.Prepare(keepOriginal: true)`), because a portrait is of a person
	// and nothing unexamined is retained. It lives on `person.portraitOriginalRef` and has nothing to do with the
	// photograph library. PRD 027 deliberately did not touch it.
	"portrait.go": true,
}

// TestOnlyTheAdminSurfaceResolvesAnOriginal fails if a file outside the permitted list reaches for an original.
//
// The blunt instrument of the three, and the one that catches a helper nobody registered as a route.
func TestOnlyTheAdminSurfaceResolvesAnOriginal(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			// Tests construct and assert on these deliberately; the guard is about the served surface.
			continue
		}
		if adminFilesPermittedAnOriginal[name] || filesWithAnUnrelatedOriginal[name] {
			continue
		}

		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, needle := range originalRefNeedles {
			if strings.Contains(withoutComments(string(src)), needle) {
				t.Errorf("%s resolves an original (%s), but it is not a file permitted to.\n"+
					"A library original is the photographer's file with its metadata intact, including where the "+
					"photograph was taken (PRD 027). It may only be handed to somebody holding the admin "+
					"credential. If this file is part of that surface, add it to "+
					"adminFilesPermittedAnOriginal with a reason; if its `original` belongs to another feature "+
					"entirely, add it to filesWithAnUnrelatedOriginal instead — those two lists mean different "+
					"things; otherwise serve a rendition, which is stripped.", name, needle)
			}
		}
	}
}

// TestNoRouteOutsideTheAdminCredentialResolvesAnOriginal is the route-shaped half of R10.
//
// # Why this is not the same test as the one above
//
// The file list above asks "which files may do this". This asks "which **routes** reach code that does", by
// resolving every registration in `routes.go` to the file its handler is declared in. The two catch different
// mistakes, and the difference is the one that matters: a file could be on the permitted list for a good reason
// (it serves an admin download) and then grow a second handler that somebody registers *without* `requireAdmin`.
// The allowlist would pass that; this does not.
//
// Routes are enumerated by parsing `routes.go` rather than from a list here, which is the whole point —
// `publicprivacy_test.go` records why at length: a route added later must be covered without anybody remembering
// to come back and add it.
func TestNoRouteOutsideTheAdminCredentialResolvesAnOriginal(t *testing.T) {
	handlers := handlerDeclarationFiles(t)

	var checked int
	for _, route := range allRegisteredRoutes(t) {
		if route.admin || route.handler == "" {
			continue
		}
		file, ok := handlers[route.handler]
		if !ok {
			// The handler is declared somewhere this walk cannot see — another package, or a shape the parser
			// does not recognise. Reported rather than skipped: an unreadable route is invisible to this guard,
			// which is exactly what the guard exists to prevent.
			t.Errorf("routes.go:%d registers %s %s on handler %s, which this guard cannot locate. "+
				"Teach handlerDeclarationFiles about it — a route it cannot see is a route it cannot check.",
				route.line, route.method, route.path, route.handler)
			continue
		}
		if filesWithAnUnrelatedOriginal[file] {
			// The portrait upload is a real non-admin route in a file that says `originalRef` about something
			// else. Skipped for the same reason as above, and only here — never in the reader-surface guard,
			// which is about markup rather than identifiers.
			continue
		}
		checked++

		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		body := withoutComments(string(src))
		for _, needle := range originalRefNeedles {
			if strings.Contains(body, needle) {
				t.Errorf("%s %s is not behind requireAdmin, and its handler %s lives in %s, which resolves an "+
					"original (%s).\n"+
					"An original carries the camera's metadata including its GPS coordinate (PRD 027 R10). "+
					"Either put the route behind requireAdmin, or move the handler out of this file.",
					route.method, route.path, route.handler, file, needle)
			}
		}
	}

	// A walk that silently matched nothing would pass forever. This is the `len(out) == 0` check
	// `glimtopenapi_test.go` learned to make for the same reason.
	if checked == 0 {
		t.Fatal("checked no non-admin routes — has the registration style in routes.go changed?")
	}
}

// TestNoReaderSurfaceNamesAnOriginal is R11: nothing that renders an image may point at one.
//
// # Why R11 is separate from R10 at all
//
// Because a route can be correctly behind `requireAdmin` and still be named from markup that a non-admin surface
// shares. The photo viewer is exactly that case: `cmd/api/viewer/viewer.js` is deliberately **one** copy served to
// both the public album page and the admin tool (PRD 023 §9 says the feature has failed if a second appears), so a
// variant added to it for a curator's convenience would ship to the open web the same afternoon.
//
// And the cost is not only privacy. A 24 MP file in an `srcset` is a 10 MB page on a phone — the precise thing the
// 800px rendition was added to avoid (task 409).
func TestNoReaderSurfaceNamesAnOriginal(t *testing.T) {
	// Each of these renders photographs for somebody who may not hold the admin credential.
	for _, path := range []string{
		"viewer/viewer.js",
		"viewer/viewer.css",
		"publicsite.go",
		"albumpage.go",
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		body := withoutComments(string(src))
		for _, needle := range originalRefNeedles {
			if strings.Contains(body, needle) {
				t.Errorf("%s names an original (%s). This file renders photographs for readers who do not hold "+
					"the admin credential, and an original carries the camera's metadata (PRD 027 R11). "+
					"Serve a rendition: they are stripped, and they are the sizes a page should be asking for.",
					path, needle)
			}
		}
	}

	// The viewer's candidates, asserted as the whole statement rather than as a fragment.
	//
	// Fragments have burned this repo before — a needle matched the regex that *removed* the thing it was checking
	// for and passed over a broken retry. So this pins the complete assignment: two candidates, the renditions that
	// exist (task 409), at the widths they actually are.
	viewer, err := os.ReadFile("viewer/viewer.js")
	if err != nil {
		t.Fatalf("reading the viewer: %v", err)
	}
	const wantSrcset = `img.srcset = item.medium + ' 800w, ' + item.full + ' 1600w';`
	if !strings.Contains(string(viewer), wantSrcset) {
		t.Errorf("the viewer's srcset is no longer exactly:\n  %s\n"+
			"Those two widths are the renditions that exist (PRD 023 §7.9, task 409), and a third candidate is "+
			"how an original reaches a page. If the renditions changed, change this assertion deliberately.",
			wantSrcset)
	}

	// The PWA, which has no business knowing originals exist. Checked as a tree rather than file by file: the point
	// is that **nothing** under vue/src does this, and naming files would go stale on the first new component.
	pwa, err := filepath.Glob("../../../vue/src/**/*.ts")
	if err != nil {
		t.Fatalf("globbing the PWA: %v", err)
	}
	_ = pwa // see walkPWASources below; the glob is only a cheap existence check for the layout
	assertPWANamesNoOriginal(t)
}

// assertPWANamesNoOriginal walks vue/src and fails if anything there reaches for an original.
//
// A walk rather than a file list, so a view added next month is covered without anybody remembering. The PWA is
// authenticated but it is **not** the admin surface: a participant's session is not the curator's credential, and
// PRD 027 puts originals behind the latter.
func assertPWANamesNoOriginal(t *testing.T) {
	t.Helper()

	root := filepath.Join("..", "..", "..", "vue", "src")
	if _, err := os.Stat(root); err != nil {
		// The Go module is sometimes built without the frontend checked out beside it. Skipping is honest; failing
		// would make this guard depend on the layout of a directory it does not own.
		t.Logf("skipping the PWA walk: %v", err)
		return
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".ts", ".vue", ".js":
		default:
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, needle := range []string{"originalRef", "variant=original"} {
			if strings.Contains(string(src), needle) {
				t.Errorf("%s names an original (%s). The PWA is authenticated but it is not the admin surface — "+
					"a participant's session is not the curator's credential — and an original carries the "+
					"camera's metadata (PRD 027 R11).", path, needle)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the PWA: %v", err)
	}
}

// handlerDeclarationFiles maps `app` method names to the file each is declared in.
//
// A regex over the package's files rather than an AST walk, deliberately: `packageFiles` parses with a FileSet it
// does not return, so an AST walk cannot say which file a declaration came from without parsing the directory a
// second time. The registration style this has to match is one line in every case —
// `func (app *application) somethingHandler(` — and a declaration this misses is **reported** by the caller rather
// than skipped, so the blunt instrument cannot hide anything.
func handlerDeclarationFiles(t *testing.T) map[string]string {
	t.Helper()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}

	decl := regexp.MustCompile(`(?m)^func \(app \*application\) (\w+)\(`)
	out := map[string]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, rerr := os.ReadFile(name)
		if rerr != nil {
			t.Fatalf("reading %s: %v", name, rerr)
		}
		for _, m := range decl.FindAllStringSubmatch(string(src), -1) {
			out[m[1]] = name
		}
	}
	if len(out) == 0 {
		t.Fatal("found no methods on the application type — has the package layout changed?")
	}
	return out
}

// Comments are stripped before any of these guards searches, using `withoutComments` from viewer_test.go.
//
// **That is not tidiness, it is the difference between a working guard and one that fires on its own
// documentation.** Every needle below is quoted many times in the prose explaining why it is forbidden, and
// `nathejk/table/album/membershipsafety_test.go` records that exact mistake happening four times in one session.
//
// Reusing the viewer's helper rather than adding a third copy: the package already had two (`stripGoComments` for
// `//` only, `withoutComments` for both), and a guard file introducing a fourth spelling of "ignore the prose" is
// how they drift apart.
