package main

import (
	"archive/zip"
	"bytes"
	"context"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
)

// Downloading an album as one zip file (adminalbumzip.go).
//
// The property that matters is the **order**, and it is worth stating why it needs a test rather than a reading of
// the handler: the album's arrangement lives in the projection's ordinals, a zip reader is free to list entries in
// any order it likes, and most sort by name. So "the entries come out in the album's order" is only true if two
// separate things hold — the write order and the name prefix — and either one alone looks correct in a diff.

// zipItem is one album position backed by a real JPEG in the test app's blob store.
//
// Returns the position **and** the library row for it, because the download reads both models: the album decides
// the order and the names, the library decides which stored rendition answers for a requested size. A fixture
// satisfying only one of them would be exercising a fallback rather than the feature.
func zipItem(t *testing.T, app *application, ordinal int, id, fileName string, w, h int) (album.CuratorItem, photo.LibraryPhoto) {
	t.Helper()

	ref := seedSource(t, app, w, h)
	return album.CuratorItem{Ordinal: ordinal, PhotoID: id, Ref: ref, SortFileName: fileName},
		photo.LibraryPhoto{ID: id, Ref: ref, Width: w, Height: h, BoundsVerdict: photo.BoundsNone}
}

// zipApp wires an app around one album, with a library holding exactly its photographs.
func zipApp(t *testing.T, a album.CuratorAlbum, items []album.CuratorItem, rows []photo.LibraryPhoto) *httptest.Server {
	t.Helper()

	app, srv, _ := albumWriteApp(t, newAlbumCurator())
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{a: a, items: items})
	app.models.PhotoCurator = &libraryCurator{rows: rows}
	return srv
}

// readZip returns the archive's entry names in the order they were written, with their contents.
func readZip(t *testing.T, resp *http.Response) ([]string, map[string][]byte) {
	t.Helper()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("the download is not a readable zip (%d bytes): %v", len(raw), err)
	}
	names := []string{}
	bodies := map[string][]byte{}
	for _, f := range zr.File {
		names = append(names, f.Name)
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		bodies[f.Name] = content
	}
	return names, bodies
}

// zipEntryWidth decodes one entry's width, which is how a size assertion is made: the edge is the feature.
func zipEntryWidth(t *testing.T, body []byte) int {
	t.Helper()

	cfg, err := jpeg.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("the entry is not a decodable JPEG: %v", err)
	}
	return cfg.Width
}

// downloadZip asks for an album's archive and insists on a 200.
func downloadZip(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()

	resp := getAdmin(t, srv, path, testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: want 200, got %d", path, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", ct)
	}
	return resp
}

// The archive is the album, in the album's order, with the order in the names.
func TestTheAlbumZipKeepsTheAlbumsOrder(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())

	// The file names are deliberately in the **reverse** of the album's arrangement: a handler that leaned on the
	// photographer's names, or an archive read alphabetically, would come out backwards — which is exactly the
	// mistake the numeric prefix prevents.
	i1, r1 := zipItem(t, app, 0, photoID("a"), "IMG_0009.JPG", 400, 300)
	i2, r2 := zipItem(t, app, 1, photoID("b"), "IMG_0005.JPG", 420, 300)
	i3, r3 := zipItem(t, app, 2, photoID("c"), "IMG_0001.JPG", 440, 300)
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 3},
		items: []album.CuratorItem{i1, i2, i3},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{r1, r2, r3}}

	names, bodies := readZip(t, downloadZip(t, srv, "/api/admin/albums/al-1/zip"))

	want := []string{"001-IMG_0009.JPG", "002-IMG_0005.JPG", "003-IMG_0001.JPG"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("entries = %v, want %v — the album's order, carried in the names because a zip reader is free "+
			"to list them alphabetically", names, want)
	}
	// Each entry holds the photograph that position names. The widths differ by 20px per item precisely so that
	// "the names and the bytes drifted apart" is a failure rather than three indistinguishable images.
	for name, width := range map[string]int{
		"001-IMG_0009.JPG": 400, "002-IMG_0005.JPG": 420, "003-IMG_0001.JPG": 440,
	} {
		if got := zipEntryWidth(t, bodies[name]); got != width {
			t.Errorf("%s is %dpx wide, want %d: the names and the bytes have drifted apart", name, got, width)
		}
	}
}

// The three sizes, which are the point of the submenu.
//
// # Why each case is here
//
// The sizes are not three calls to one resizer: `medium` reads a rendition the library already holds, `large` is
// rendered at download time because nothing stores a 1200px copy, and `original` streams what is stored untouched.
// Three different code paths, so three cases — and the two that avoid re-encoding are the ones worth protecting,
// because a regression there is invisible (the file is still the right size) and costs a decode of the whole album
// plus a generation of JPEG loss.
func TestTheAlbumZipServesTheRequestedSize(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())

	// One photograph with a stored 800px rendition, as every upload since task 409 has.
	item, row := zipItem(t, app, 0, photoID("a"), "IMG_0001.JPG", 1600, 1200)
	mediumRef := seedSource(t, app, mediumEdge, 600)
	row.MediumRef = mediumRef

	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 1},
		items: []album.CuratorItem{item},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{row}}

	stored := readStoredBlob(t, app, row.Ref)
	storedMedium := readStoredBlob(t, app, mediumRef)

	for _, tc := range []struct {
		size      string
		wantWidth int
		// sameAs is the stored object the entry must be byte-for-byte, when the size is answered without
		// re-encoding. Nil when the bytes are rendered and only their width is a promise.
		sameAs []byte
		why    string
	}{
		{size: "medium", wantWidth: mediumEdge, sameAs: storedMedium,
			why: "the 800px rendition already exists, so medium must read it rather than render a second copy"},
		{size: "large", wantWidth: largeEdge,
			why: "nothing stores a 1200px copy, so large is rendered from the display image"},
		{size: "xlarge", wantWidth: 1600, sameAs: stored,
			why: "xlarge is the stored display image, streamed untouched — and it is 1600px, not an original"},
		// No `size` at all is the largest, not the smallest: a link somebody shares or bookmarks without the
		// parameter should not quietly hand over the least useful file.
		{size: "", wantWidth: 1600, sameAs: stored, why: "no size means xlarge"},
	} {
		path := "/api/admin/albums/al-1/zip"
		if tc.size != "" {
			path += "?size=" + tc.size
		}
		_, bodies := readZip(t, downloadZip(t, srv, path))
		body, ok := bodies["001-IMG_0001.JPG"]
		if !ok {
			t.Fatalf("size %q: the entry is missing", tc.size)
		}
		if got := zipEntryWidth(t, body); got != tc.wantWidth {
			t.Errorf("size %q is %dpx wide, want %d: %s", tc.size, got, tc.wantWidth, tc.why)
		}
		if tc.sameAs != nil && !bytes.Equal(body, tc.sameAs) {
			t.Errorf("size %q was re-encoded: %s", tc.size, tc.why)
		}
	}
}

// A photograph already smaller than the requested edge is sent as stored.
//
// `imaging.Fit` does not upscale, so rendering it would re-encode the same pixels — the one case where doing the
// work is both slower and visibly worse. The album is full of these: a crop, a screenshot, an older upload.
func TestTheAlbumZipDoesNotReEncodeASmallPhotograph(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())

	item, row := zipItem(t, app, 0, photoID("a"), "IMG_0001.JPG", 900, 700)
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 1},
		items: []album.CuratorItem{item},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{row}}
	stored := readStoredBlob(t, app, row.Ref)

	_, bodies := readZip(t, downloadZip(t, srv, "/api/admin/albums/al-1/zip?size=large"))
	if !bytes.Equal(bodies["001-IMG_0001.JPG"], stored) {
		t.Error("a 900px photograph asked for at 1200px must be sent as stored, not re-encoded at the same size")
	}
}

// An unrecognised size is refused rather than defaulted.
//
// A typo'd `size` silently answering with originals is a curator wondering why their e-mail attachment is 400 MB.
func TestTheAlbumZipRefusesAnUnknownSize(t *testing.T) {
	srv := zipApp(t,
		album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 1},
		nil, nil)

	for _, bad := range []string{"xl", "1200", "thumb", "Medium"} {
		resp := getAdmin(t, srv, "/api/admin/albums/al-1/zip?size="+bad, testAdminUser, testAdminPass)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("size=%q: want 400, got %d", bad, resp.StatusCode)
		}
	}
}

// The download is named for the album and the day it was taken.
func TestTheAlbumZipIsNamedForTheAlbumAndTheDay(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())
	item, row := zipItem(t, app, 0, photoID("a"), "IMG_0001.JPG", 400, 300)
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1", Slug: "loerdag-morgen", Title: "Lørdag morgen", ItemCount: 1},
		items: []album.CuratorItem{item},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{row}}

	resp := downloadZip(t, srv, "/api/admin/albums/al-1/zip")

	// The date is today's, so the assertion is on the shape and on the slug rather than on a fixed string.
	want := `attachment; filename="` + adminAlbumZipFileName("loerdag-morgen", "al-1", time.Now()) + `"`
	if got := resp.Header.Get("Content-Disposition"); got != want {
		t.Errorf("Content-Disposition = %q, want %q", got, want)
	}
	// The slug and not the title: a `Content-Disposition` carrying `Lørdag morgen` needs a second, differently
	// escaped parameter to survive, and the browser's fallback when it disagrees is mojibake in a file name.
	if strings.Contains(resp.Header.Get("Content-Disposition"), "Lørdag") {
		t.Error("the file name must come from the slug, not the Danish title")
	}
}

// A removed position and a deleted photograph are not in the archive, and the numbering closes over the gap.
//
// The numbering matters as much as the exclusion: a sequence with a hole in it reads as a download that lost a
// file, which is a support question about a working feature.
func TestTheAlbumZipSkipsRemovedAndDeletedWithoutLeavingAGap(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())

	first, r1 := zipItem(t, app, 0, photoID("a"), "IMG_0001.JPG", 400, 300)
	removed, r2 := zipItem(t, app, 1, photoID("b"), "IMG_0002.JPG", 400, 300)
	removed.Removed = true
	gone, r3 := zipItem(t, app, 2, photoID("c"), "IMG_0003.JPG", 400, 300)
	gone.PhotoDeleted = true
	last, r4 := zipItem(t, app, 3, photoID("d"), "IMG_0004.JPG", 400, 300)

	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 2},
		items: []album.CuratorItem{first, removed, gone, last},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{r1, r2, r3, r4}}

	names, _ := readZip(t, downloadZip(t, srv, "/api/admin/albums/al-1/zip"))
	if want := "001-IMG_0001.JPG 002-IMG_0004.JPG"; strings.Join(names, " ") != want {
		t.Errorf("entries = %v, want %q", names, want)
	}
}

// An upload that carried no file name still gets a usable, unique entry name.
func TestTheAlbumZipNamesAFileThatHadNoName(t *testing.T) {
	for _, tc := range []struct{ fileName, want string }{
		{"", photoID("a") + ".jpg"},
		// Only the last path element survives, because the name came from a file somebody chose and
		// `../../etc/passwd` is a real class of bug in whatever unpacks the archive.
		{"../../etc/passwd", "passwd"},
		{`C:\Users\foto\IMG_0001.JPG`, "IMG_0001.JPG"},
	} {
		if got := adminZipEntryName(tc.fileName, photoID("a")); got != tc.want {
			t.Errorf("adminZipEntryName(%q) = %q, want %q", tc.fileName, got, tc.want)
		}
	}
}

// An album with nothing in it is refused rather than answered with an empty archive, which is a file that looks
// broken. 409 and not 404: the album is there, and a curator told "not found" goes looking for a deleted one.
func TestTheAlbumZipRefusesAnEmptyAlbum(t *testing.T) {
	srv := zipApp(t, album.CuratorAlbum{ID: "al-1", Slug: "tom", Title: "Tomt album"}, nil, nil)

	if got := getAdmin(t, srv, "/api/admin/albums/al-1/zip", testAdminUser, testAdminPass).StatusCode; got != http.StatusConflict {
		t.Errorf("want 409 for an album with no photographs, got %d", got)
	}
	if got := getAdmin(t, srv, "/api/admin/albums/al-nope/zip", testAdminUser, testAdminPass).StatusCode; got != http.StatusNotFound {
		t.Errorf("want 404 for an unknown album, got %d", got)
	}
}

// The download is behind the admin credential, like everything else on this surface.
func TestTheAlbumZipRequiresTheAdminCredential(t *testing.T) {
	srv := zipApp(t, album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 1}, nil, nil)

	if got := getAdmin(t, srv, "/api/admin/albums/al-1/zip", "", "").StatusCode; got != http.StatusUnauthorized {
		t.Errorf("want 401 without the credential, got %d", got)
	}
}

// The album list offers the download in its three sizes, and offers it only where it can work.
//
// A rendered-HTML assertion rather than a source grep, which is the whole reason the list is a fragment (task
// 395): the conditions on this menu item — not deleted, has photographs, carries the year — are only observable
// in a response.
func TestTheAlbumListOffersTheZipDownload(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(
		&curatedAlbum{a: album.CuratorAlbum{
			ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 4, OriginalCount: 4,
		}},
		&curatedAlbum{a: album.CuratorAlbum{ID: "al-2", Slug: "tom", Title: "Tomt album"}},
	))

	body := albumListFragment(t, srv)

	// The year rides in the query string because an anchor cannot carry `X-Admin-Year`.
	for _, want := range []string{
		`href="/api/admin/albums/al-1/zip?size=medium&amp;year=2026"`,
		`href="/api/admin/albums/al-1/zip?size=large&amp;year=2026"`,
		`href="/api/admin/albums/al-1/zip?size=xlarge&amp;year=2026"`,
		`href="/api/admin/albums/al-1/zip?size=original&amp;year=2026"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the menu is missing %s\n%s", want, body)
		}
	}
	// Each scale says its pixels: «stor» and «lille» are opinions, 1200px is a fact a curator can act on. 1600px is
	// labelled `XLarge` rather than `Original`, because it is the largest *rendition* — calling it an original for
	// years is what trained everybody to expect the wrong thing from it.
	//
	// `Original` now appears too, and means the photographer's file (PRD 027). Until task 479 shipped, this test
	// asserted the word was **absent**; the inversion is the point rather than a loosened guard, so it is recorded
	// here instead of being quietly dropped — the name is now earned.
	for _, want := range []string{"Medium (800px)", "Stor (1200px)", "XLarge (1600px)", ">Original<"} {
		if !strings.Contains(body, want) {
			t.Errorf("the size menu should say %q\n%s", want, body)
		}
	}
	// And it is the last entry, below the three scales: it is the heaviest and the most specialised, so reaching it
	// by accident while meaning "a copy for the newspaper" should take a deliberate move downwards.
	if strings.Index(body, "size=original") < strings.Index(body, "size=xlarge") {
		t.Error("the original is offered above the scales; it is the heaviest download and belongs last")
	}
	if strings.Contains(body, "/api/admin/albums/al-2/zip") {
		t.Error("an empty album must not offer a download that can only fail")
	}
}

// The photographer's file, through the zip and through the media route (PRD 027 R5/R6, tasks 480 and 481).
//
// # Why both surfaces are tested in one place
//
// Because they are the only two ways an original can leave this service, and the rule they share is the one that makes
// PRD 027's EXIF decision safe: the bytes are sent **as stored**, and nothing derives, resizes or re-encodes them. A
// re-encode would strip the metadata, which sounds harmless until you notice it would make the download silently not
// an original while still being called one — the exact confusion the `XLarge` rename exists to end.
func TestTheOriginalIsDownloadedAsStored(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())

	// A real photograph with real EXIF, so "the metadata survived" is a question the fixture can answer.
	raw := jpegWithTestGPS(t, 55, 43, 59.74, 'N', 12, 15, 53.35, 'E')
	originalRef, err := app.blobs.Put(context.Background(), raw)
	if err != nil {
		t.Fatalf("seeding the original: %v", err)
	}
	displayRef := seedSource(t, app, 1600, 1200)

	row := photo.LibraryPhoto{
		ID: photoID("a"), Ref: displayRef, Width: 1600, Height: 1200,
		OriginalRef: originalRef.String(), OriginalContentType: "image/jpeg", OriginalBytes: len(raw),
		OriginalWidth: 4000, OriginalHeight: 3000, BoundsVerdict: photo.BoundsNone,
	}
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 1},
		items: []album.CuratorItem{{
			Ordinal: 0, PhotoID: photoID("a"), Ref: displayRef, SortFileName: "IMG_0001.JPG",
		}},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{row}}

	// The zip.
	_, bodies := readZip(t, downloadZip(t, srv, "/api/admin/albums/al-1/zip?size=original"))
	entry := bodies["001-IMG_0001.JPG"]
	if !bytes.Equal(entry, raw) {
		t.Errorf("the zip's original entry is not the stored file (%d bytes vs %d). It must be streamed as "+
			"stored: an original that has been through imaging is not an original.", len(entry), len(raw))
	}
	if _, _, ok := imaging.ReadGPS(entry); !ok {
		t.Error("the zip's original entry carries no GPS, so it was re-encoded somewhere. That would make the " +
			"download silently not an original while still being called one.")
	}

	// And the media route, which is the single-photograph download (R6).
	resp := getAdmin(t, srv, "/api/admin/photos/"+photoID("a")+"/media?variant=original",
		testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("variant=original: want 200, got %d", resp.StatusCode)
	}
	served, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the response: %v", err)
	}
	if !bytes.Equal(served, raw) {
		t.Errorf("variant=original served %d bytes, want the stored file's %d", len(served), len(raw))
	}
}

// A photograph with no original falls back to the display image, on both surfaces.
//
// This is **most of the library** and permanently so: everything uploaded before PRD 027 shipped has no original and
// no backfill can produce one. The fallback is the honest answer rather than a substitute — 1600px is that
// photograph's most original surviving form — but a curator has to be told which entries are which, which is what
// task 482 is for.
func TestTheOriginalFallsBackToTheDisplayImage(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())

	item, row := zipItem(t, app, 0, photoID("a"), "IMG_0001.JPG", 1600, 1200)
	// No OriginalRef: a photograph from before PRD 027.
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 1},
		items: []album.CuratorItem{item},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{row}}
	stored := readStoredBlob(t, app, row.Ref)

	_, bodies := readZip(t, downloadZip(t, srv, "/api/admin/albums/al-1/zip?size=original"))
	if !bytes.Equal(bodies["001-IMG_0001.JPG"], stored) {
		t.Error("a photograph with no original should fall back to the display image, as stored and unmodified")
	}

	resp := getAdmin(t, srv, "/api/admin/photos/"+photoID("a")+"/media?variant=original",
		testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("variant=original on a photograph with no original must fall back, not 404; got %d",
			resp.StatusCode)
	}
}

// An original is never given a rendition-repair plan.
//
// # Why this is worth its own test
//
// The repair (task 430) rebuilds a missing rendition by re-rendering it from a source, and it works precisely because
// a rendition is derivable. An original is not derivable from anything — that is its definition. A plan pointed at one
// would attempt to write a re-encode over the only copy of somebody's file.
//
// `blob.PutAs` refuses to overwrite an original, so the attempt would fail rather than corrupt. That backstop is
// exactly why this needs testing: the bug would be invisible, a logged error on a path nobody watches, rather than a
// broken image somebody reports.
//
// Source-shaped because a repair plan is only *built* here — whether it was built is not observable in a response.
func TestTheOriginalVariantGetsNoRepairPlan(t *testing.T) {
	src := withoutComments(readRepoFile(t, "adminlibrary.go"))

	case_ := src[strings.Index(src, `case "original":`):]
	if end := strings.Index(case_, "\n\t}"); end > 0 {
		case_ = case_[:end]
	}
	if case_ == "" {
		t.Fatal("the original variant is gone from the admin media route; this guard needs updating")
	}
	// `edge` staying zero is what leaves the plan empty: the plan is only built `if edge > 0`.
	if strings.Contains(case_, "edge") {
		t.Errorf("the original variant sets an edge, which builds a rendition-repair plan for it. An original "+
			"cannot be re-rendered from anything, and a plan would try to write a re-encode over the only copy "+
			"of the photographer's file (PRD 027 R6).\ngot: %s", case_)
	}
}

// Every size the menu offers is a size the endpoint accepts, and the other way round.
//
// Two lists in two languages that have to agree, which is the shape that drifts: a link to a size the endpoint
// rejects is a 400 a curator meets after choosing, and a size the endpoint serves but nothing offers is dead code
// nobody will remember to remove.
func TestTheMenuAndTheEndpointAgreeOnTheSizes(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 2, OriginalCount: 2},
	}))
	body := albumListFragment(t, srv)

	for size := range adminZipSizes {
		if !strings.Contains(body, "size="+size+"&amp;") {
			t.Errorf("the endpoint accepts size=%q but the menu does not offer it", size)
		}
	}
	for _, offered := range regexp.MustCompile(`size=(\w+)&amp;`).FindAllStringSubmatch(body, -1) {
		if _, ok := adminZipSizes[offered[1]]; !ok {
			t.Errorf("the menu offers size=%q, which the endpoint refuses with a 400", offered[1])
		}
	}
}

// The filter that answers "which of these cannot be printed" (PRD 027 R7, task 482).
//
// Offered as a filter rather than as a mark on every cell, and that is a design decision worth a test because it will
// look like an omission to somebody expecting a badge. The contact sheet's marks follow one rule — the ordinary case
// gets none — and "has an original" is rare now and universal later, so no badge for it stays quiet at both ends. The
// filter is the mechanism this tool already uses for "which ones": uden album, uden billedtekst, uden fotokredit.
func TestTheLibraryCanBeFilteredByWhetherAnOriginalIsHeld(t *testing.T) {
	curator := &libraryCurator{}
	_, srv := libraryApp(t, curator)

	for _, tc := range []struct {
		query string
		want  *bool
	}{
		{"?original=no", boolPtr(false)},
		{"?original=yes", boolPtr(true)},
		{"", nil},
	} {
		curator.filters = nil
		if got := getAdmin(t, srv, "/api/admin/photos"+tc.query, testAdminUser, testAdminPass).StatusCode; got != http.StatusOK {
			t.Fatalf("%q: want 200, got %d", tc.query, got)
		}
		if len(curator.filters) != 1 {
			t.Fatalf("%q: want one read, got %d", tc.query, len(curator.filters))
		}
		got := curator.filters[0].HasOriginal
		switch {
		case tc.want == nil && got != nil:
			t.Errorf("%q narrowed the read to HasOriginal=%v; with no parameter it must mean either", tc.query, *got)
		case tc.want != nil && got == nil:
			t.Errorf("%q did not reach the read at all", tc.query)
		case tc.want != nil && *got != *tc.want:
			t.Errorf("%q gave HasOriginal=%v, want %v", tc.query, *got, *tc.want)
		}
	}

	// An unusable value is refused rather than ignored. A silently-dropped filter shows a curator the whole library
	// while the page says they are looking at a subset, and the action bar then acts on what they can see.
	if got := getAdmin(t, srv, "/api/admin/photos?original=maybe", testAdminUser, testAdminPass).StatusCode; got != http.StatusBadRequest {
		t.Errorf("want 400 for an unrecognised original filter, got %d", got)
	}
}

// The preset is on the filter row, on both views.
//
// Both, because an album is where the question is actually asked: a curator about to promise a printer "the Natten
// album" needs to know which of those 180 photographs can supply a file, and until task 460 that view had no row at
// all. The album view inherits the same row by construction (`adminFiltersIn`), and this asserts the inheritance
// rather than assuming it.
func TestTheFilterRowOffersUdenOriginal(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}

	for _, path := range []string{"/2026/photos", "/2026/photos?original=no"} {
		body := adminBody(t, getAdmin(t, srv, path, testAdminUser, testAdminPass))
		if !strings.Contains(body, "Uden original") {
			t.Errorf("%s: the filter row should offer «Uden original»\n%s", path, body)
		}
	}

	// And the label is "uden", not "mangler". Nothing is missing that anybody can supply — no backfill can produce an
	// original for a photograph uploaded before PRD 027 — so a label implying an outstanding task would send a
	// curator looking for a button that cannot exist.
	body := adminBody(t, getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass))
	if strings.Contains(body, "Mangler original") {
		t.Error("the label must not imply an outstanding task: no backfill can produce an original (PRD 027)")
	}
}

func boolPtr(b bool) *bool { return &b }

// # Why the count is the feature and not a decoration
//
// No backfill can produce an original for a photograph uploaded before PRD 027, so an album assembled across two eras
// is **permanently** mixed. The zip falls back to the 1600px rendition for those entries, which is honest — that is the
// photograph's most original surviving form — but a fallback nobody was told about is indistinguishable from a bug,
// and the person who discovers it is the one who has already sent the files to a printer.
//
// Three states, each saying something different, and all three asserted because the interesting one is the middle.
func TestTheDownloadMenuSaysHowManyOriginalsAnAlbumHas(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(
		// Fully covered: no count, because a number that always reads "180 af 180" is noise.
		&curatedAlbum{a: album.CuratorAlbum{
			ID: "al-all", Slug: "alle", Title: "Alle originaler", ItemCount: 180, OriginalCount: 180,
		}},
		// Partly covered: the count, and a sentence saying what the rest will be.
		&curatedAlbum{a: album.CuratorAlbum{
			ID: "al-some", Slug: "nogle", Title: "Blandet album", ItemCount: 180, OriginalCount: 34,
		}},
		// Not covered at all: no link. A download that would silently be entirely renditions is worse than an
		// absence — and the absence is explained rather than left as a gap.
		&curatedAlbum{a: album.CuratorAlbum{
			ID: "al-none", Slug: "ingen", Title: "Gammelt album", ItemCount: 42, OriginalCount: 0,
		}},
	))

	body := albumListFragment(t, srv)

	if !strings.Contains(body, `zip?size=original&amp;year=2026">Original<`) {
		t.Errorf("a fully covered album should offer a bare «Original» with no count\n%s", body)
	}
	if !strings.Contains(body, "Original (34 af 180)") {
		t.Errorf("a partly covered album should say how many originals it has\n%s", body)
	}
	if !strings.Contains(body, "De \u00f8vrige hentes i 1600px") {
		t.Errorf("a partly covered album should say what the remaining entries will be\n%s", body)
	}
	if strings.Contains(body, `/api/admin/albums/al-none/zip?size=original`) {
		t.Errorf("an album with no originals must not offer the download\n%s", body)
	}
	if !strings.Contains(body, "Ingen originaler i dette album") {
		t.Errorf("an album with no originals should say why the download is absent\n%s", body)
	}
	// The three scales are offered for all three albums regardless: they are renditions, and every photograph has
	// those. Only the original can be missing.
	for _, id := range []string{"al-all", "al-some", "al-none"} {
		if !strings.Contains(body, "/api/admin/albums/"+id+"/zip?size=xlarge") {
			t.Errorf("%s should still offer the scales; only the original can be absent", id)
		}
	}
}

// The library's header counts the permanent gap, and says nothing when there is none.
func TestTheHeaderCountsThePhotographsWithNoOriginal(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{counts: photo.Counts{Total: 180, WithoutOriginal: 146}}

	resp := getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if body := adminBody(t, resp); !strings.Contains(body, `<span class="n">146</span><span class="k">uden original`) {
		t.Errorf("the header should count the photographs with no original\n%s", body)
	}

	// And absent at zero, rather than a line reading "0 uden original" forever on a library filled after PRD 027.
	// The number marks pixels that can never be recovered, not a queue somebody can work through, so a zero is not
	// an encouraging progress report — it is noise.
	app2, srv2 := adminApp(t)
	app2.models.PhotoCurator = &libraryCurator{counts: photo.Counts{Total: 180}}
	if body := adminBody(t, getAdmin(t, srv2, "/2026/photos", testAdminUser, testAdminPass)); strings.Contains(body, "uden original") {
		t.Error("with every photograph covered, the header should not carry an «uden original» line at all")
	}
}

// Source-shaped, because what is being checked is the card's structure: the cover is the card's full width, the
// actions are in one `.menu`, and the open state is Alpine's — which is what keeps the list readable at a year's
// worth of albums.
func TestTheAlbumCardPutsItsActionsBehindTheCog(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 4, Published: true},
	}))

	body := albumListFragment(t, srv)

	for _, want := range []string{
		`x-data="{menu:false, sizes:false}"`,
		`class="cogbtn"`,
		`class="menu" x-show="menu" x-cloak`,
		`class="sizes" x-show="sizes" x-cloak`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the card needs %s\n%s", want, body)
		}
	}
	// `@click.outside` belongs on the wrapper, not on the menu: the cog is "outside" the menu, so a handler there
	// would close it on the way down and the cog's own toggle would reopen it — a cog that cannot be pressed twice.
	if !strings.Contains(body, `<div class="cog" @click.outside="menu=false; sizes=false">`) {
		t.Errorf("the outside-click handler belongs on the cog wrapper\n%s", body)
	}
	// Closing the menu collapses the size submenu with it. Otherwise the next open starts expanded, which is a menu
	// that remembers something nobody asked it to remember.
	if !strings.Contains(body, `@click="menu = !menu; sizes = false"`) {
		t.Errorf("the cog must collapse the size submenu as it closes\n%s", body)
	}
	// The old layout's button row is gone. It is named here because that is the thing being replaced, and a card
	// carrying both would be the half-finished state nobody notices.
	if strings.Contains(body, `class="acts"`) {
		t.Error("the actions row was replaced by the cog menu; two of them is worse than either")
	}
}

// Videos in the album zip (PRD 029, task 502): the original for "Original", the 720p MP4 for every other size.
func TestTheAlbumZipCarriesVideos(t *testing.T) {
	app, srv, _ := albumWriteApp(t, newAlbumCurator())
	put := func(b string) string {
		ref, err := app.blobs.Put(context.Background(), []byte(b))
		if err != nil {
			t.Fatal(err)
		}
		return ref.String()
	}
	hd, orig, pending := put("the 720p mp4"), put("the phone's mov"), put("a mov still processing")
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Slug: "film", Title: "Film", ItemCount: 2},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("v"), SortFileName: "IMG_0001.MOV"},
			{Ordinal: 1, PhotoID: photoID("p"), SortFileName: "IMG_0002.MOV"},
		},
	})
	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: photoID("v"), Ref: hd, OriginalRef: orig, Kind: "video", Status: "ready"},
		{ID: photoID("p"), OriginalRef: pending, Kind: "video", Status: "processing"},
	}}

	_, bodies := readZip(t, downloadZip(t, srv, "/api/admin/albums/al-1/zip?size=large"))
	if string(bodies["001-IMG_0001.mp4"]) != "the 720p mp4" {
		t.Errorf("large: want the 720p MP4 named .mp4, got entries %v", keys(bodies))
	}
	if string(bodies["002-IMG_0002.MOV"]) != "a mov still processing" {
		t.Errorf("a processing video should fall back to its original, got entries %v", keys(bodies))
	}
	_, bodies = readZip(t, downloadZip(t, srv, "/api/admin/albums/al-1/zip?size=original"))
	if string(bodies["001-IMG_0001.MOV"]) != "the phone's mov" {
		t.Errorf("original: want the phone's file, got entries %v", keys(bodies))
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
