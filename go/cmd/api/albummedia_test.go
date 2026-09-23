package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strings"
	"testing"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/imaging"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/photo"
)

// jpegWithTestGPS builds a JPEG carrying an EXIF GPS block.
//
// A second, smaller copy of the builder in `internal/imaging`'s tests, and deliberately not shared: that
// one is in an external test package, and exporting an EXIF *writer* from production code to serve two
// test suites would put a metadata writer in a binary whose entire job is to remove metadata. The
// duplication is ~40 lines and it buys the ingest an end-to-end check that the coordinate reaches the
// column and not the stored bytes.
func jpegWithTestGPS(t testing.TB, latDeg, latMin uint32, latSec float64, latRef byte,
	lngDeg, lngMin uint32, lngSec float64, lngRef byte) []byte {
	t.Helper()

	encoded := devFixtureImage(320, 240, 0, 0, "gps")
	if len(encoded) < 4 || encoded[0] != 0xFF || encoded[1] != 0xD8 {
		t.Fatal("the fixture image is not a JPEG")
	}

	order := binary.LittleEndian
	put16 := func(dst []byte, v uint16) []byte {
		b := make([]byte, 2)
		order.PutUint16(b, v)
		return append(dst, b...)
	}
	put32 := func(dst []byte, v uint32) []byte {
		b := make([]byte, 4)
		order.PutUint32(b, v)
		return append(dst, b...)
	}

	// IFD0 at 8 holds one entry (the GPS pointer); the GPS IFD follows at 26; its four entries and the
	// next-IFD offset take 52 bytes, so the rationals start at 78.
	const gpsIFD, rationals = 26, 78

	tiff := []byte{'I', 'I'}
	tiff = put16(tiff, 42)
	tiff = put32(tiff, 8)
	tiff = put16(tiff, 1)
	tiff = put16(tiff, 0x8825) // GPS IFD pointer
	tiff = put16(tiff, 4)      // LONG
	tiff = put32(tiff, 1)
	tiff = put32(tiff, gpsIFD)
	tiff = put32(tiff, 0) // no next IFD

	tiff = put16(tiff, 4) // four GPS entries
	for _, e := range []struct {
		tag    uint16
		typ    uint16
		count  uint32
		value  uint32
		inline []byte
	}{
		{tag: 0x0001, typ: 2, count: 2, inline: []byte{latRef, 0, 0, 0}},
		{tag: 0x0002, typ: 5, count: 3, value: rationals},
		{tag: 0x0003, typ: 2, count: 2, inline: []byte{lngRef, 0, 0, 0}},
		{tag: 0x0004, typ: 5, count: 3, value: rationals + 24},
	} {
		tiff = put16(tiff, e.tag)
		tiff = put16(tiff, e.typ)
		tiff = put32(tiff, e.count)
		if e.inline != nil {
			tiff = append(tiff, e.inline...)
		} else {
			tiff = put32(tiff, e.value)
		}
	}
	tiff = put32(tiff, 0) // no next IFD

	for _, c := range []struct {
		deg, min uint32
		sec      float64
	}{{latDeg, latMin, latSec}, {lngDeg, lngMin, lngSec}} {
		tiff = put32(tiff, c.deg)
		tiff = put32(tiff, 1)
		tiff = put32(tiff, c.min)
		tiff = put32(tiff, 1)
		tiff = put32(tiff, uint32(c.sec*100))
		tiff = put32(tiff, 100)
	}

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1}
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(payload)+2))
	segment = append(segment, length...)
	segment = append(segment, payload...)

	var out bytes.Buffer
	out.Write([]byte{0xFF, 0xD8})
	out.Write(segment)
	out.Write(encoded[2:])
	return out.Bytes()
}

// Album media ingest (task 333). The interesting half is the coordinate: that it is read before it is
// destroyed, that it is judged against the race area, and that a judgement we cannot make is not
// reported as a judgement against the photograph.

// stubRaceAreas answers the one read the bounds check needs.
type stubRaceAreas struct {
	area checkpoint.RaceArea
	ok   bool
	err  error
}

func (s stubRaceAreas) RaceArea(string) (checkpoint.RaceArea, bool, error) {
	return s.area, s.ok, s.err
}

// A box around the 2026 course, roughly what the 3 km buffered hull produces.
func testRaceArea() checkpoint.RaceArea {
	return checkpoint.RaceArea{
		SouthWest: checkpoint.Point{Lat: 55.60, Lng: 12.10},
		NorthEast: checkpoint.Point{Lat: 55.85, Lng: 12.45},
	}
}

func TestWithinRaceBounds(t *testing.T) {
	area := testRaceArea()

	inside := map[string][2]float64{
		"middle":          {55.73, 12.26},
		"on the SW edge":  {55.60, 12.10},
		"on the NE edge":  {55.85, 12.45},
		"near the corner": {55.6001, 12.4499},
	}
	for name, p := range inside {
		if !withinRaceBounds(area, p[0], p[1]) {
			t.Errorf("%s (%f,%f) should be inside", name, p[0], p[1])
		}
	}

	outside := map[string][2]float64{
		"just north":   {55.86, 12.26},
		"just south":   {55.59, 12.26},
		"just east":    {55.73, 12.46},
		"just west":    {55.73, 12.09},
		"Manhattan":    {40.7128, -74.0060},
		"null island":  {0, 0},
		"the antipode": {-55.73, -12.26},
	}
	for name, p := range outside {
		if withinRaceBounds(area, p[0], p[1]) {
			t.Errorf("%s (%f,%f) should be outside", name, p[0], p[1])
		}
	}
}

func TestAlbumBoundsVerdict(t *testing.T) {
	app := newTestApp(t)
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}

	if got := app.albumBoundsVerdict(55.73, 12.26); got != photo.BoundsInside {
		t.Errorf("a coordinate at the event should be inside, got %q", got)
	}
	if got := app.albumBoundsVerdict(40.7128, -74.0060); got != photo.BoundsOutside {
		t.Errorf("a coordinate in Manhattan should be outside, got %q", got)
	}
}

// **The distinction this feature needs and would be easy to lose.** No race area is a statement about
// *us*, not about the photograph — so it must not become `outside`, which would permanently condemn
// every photograph uploaded before the course was sited.
func TestAlbumBoundsVerdictIsUnknownWhenWeCannotJudge(t *testing.T) {
	cases := map[string]stubRaceAreas{
		"no positioned checkpoints yet": {ok: false},
		"the read failed":               {err: context.DeadlineExceeded},
	}
	for name, areas := range cases {
		app := newTestApp(t)
		app.models.RaceAreas = areas

		got := app.albumBoundsVerdict(55.73, 12.26)
		if got != photo.BoundsUnknown {
			t.Errorf("%s: want unknown, got %q", name, got)
		}
		if got == photo.BoundsOutside {
			t.Errorf("%s: must not blame the photograph for our own missing data", name)
		}
	}

	// And with no projection at all.
	app := newTestApp(t)
	app.models.RaceAreas = nil
	if got := app.albumBoundsVerdict(55.73, 12.26); got != photo.BoundsUnknown {
		t.Errorf("no race-area projection: want unknown, got %q", got)
	}
}

// `unknown` must never be plottable: we could not check it, and plotting an unchecked coordinate on a
// public page is the failure the whole verdict exists to prevent.
func TestUnknownIsNotPlottable(t *testing.T) {
	if photo.Plottable(photo.BoundsUnknown) {
		t.Fatal("an unchecked coordinate must not reach the map")
	}
}

// The end-to-end property: the coordinate reaches the column, and does not reach the stored bytes.
func TestStoreAlbumImageReadsTheCoordinateAndStripsIt(t *testing.T) {
	app := newTestApp(t)
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}

	raw := jpegWithTestGPS(t, 55, 43, 59.74, 'N', 12, 15, 53.35, 'E')
	if _, _, ok := imaging.ReadGPS(raw); !ok {
		t.Fatal("the fixture should carry a coordinate to begin with")
	}

	stored, err := app.storeAlbumImage(context.Background(), raw)
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}

	if stored.Location == nil {
		t.Fatal("want the coordinate carried out of the ingest")
	}
	if stored.Location.Lat < 55.73 || stored.Location.Lat > 55.74 {
		t.Errorf("latitude %f is not the fixture's", stored.Location.Lat)
	}
	if stored.Location.BoundsVerdict != photo.BoundsInside {
		t.Errorf("want the inside verdict, got %q", stored.Location.BoundsVerdict)
	}

	// The whole point: reading it did not mean keeping it.
	rc, err := app.blobs.Get(context.Background(), blob.Ref(stored.Ref))
	if err != nil {
		t.Fatalf("reading back the stored object: %v", err)
	}
	defer rc.Close()
	storedBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading the stored object: %v", err)
	}
	if _, _, ok := imaging.ReadGPS(storedBytes); ok {
		t.Fatal("the stored image still carries GPS: reading the coordinate must never mean keeping it")
	}
}

// The common real case: no fix in the file.
func TestStoreAlbumImageWithoutACoordinate(t *testing.T) {
	app := newTestApp(t)
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}

	stored, err := app.storeAlbumImage(context.Background(), devFixtureImage(400, 300, 0, 0, "test"))
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}
	// No location at all rather than a zero coordinate with a `none` verdict: after PRD 022 the absence is
	// expressed by the absence of the value, which is what makes "moved the point, forgot the verdict"
	// unsayable downstream.
	if stored.Location != nil {
		t.Errorf("a file with no GPS must yield no location, got %+v", *stored.Location)
	}
	if stored.Ref == "" || len(stored.Ref) != 64 {
		t.Errorf("want a content hash, got %q", stored.Ref)
	}
}

// A photograph taken at home, with a real coordinate that is not at the event. Kept, and not plottable.
func TestStoreAlbumImageKeepsAnOutOfBoundsCoordinate(t *testing.T) {
	app := newTestApp(t)
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}

	raw := jpegWithTestGPS(t, 40, 42, 46.0, 'N', 74, 0, 21.6, 'W')
	stored, err := app.storeAlbumImage(context.Background(), raw)
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}

	if stored.Location == nil {
		t.Fatal("the coordinate must be kept, so a curator can see what was rejected")
	}
	if stored.Location.BoundsVerdict != photo.BoundsOutside {
		t.Errorf("want the outside verdict, got %q", stored.Location.BoundsVerdict)
	}
	if photo.Plottable(stored.Location.BoundsVerdict) {
		t.Error("an out-of-bounds coordinate must not be plottable")
	}
}

func TestStoreAlbumImageRejectsNonImages(t *testing.T) {
	app := newTestApp(t)

	if _, err := app.storeAlbumImage(context.Background(),
		[]byte("this is not an image, it is a sentence")); err == nil {
		t.Fatal("want a rejection")
	}
}

// A file that is 0,0 — a camera with no fix. Must read as "no coordinate", not as the Atlantic.
func TestStoreAlbumImageTreatsNullIslandAsNoFix(t *testing.T) {
	app := newTestApp(t)
	app.models.RaceAreas = stubRaceAreas{area: testRaceArea(), ok: true}

	raw := jpegWithTestGPS(t, 0, 0, 0, 'N', 0, 0, 0, 'E')
	stored, err := app.storeAlbumImage(context.Background(), raw)
	if err != nil {
		t.Fatalf("storeAlbumImage: %v", err)
	}
	if stored.Location != nil {
		t.Errorf("0,0 must be treated as no fix rather than as a place, got %+v", *stored.Location)
	}
}

func TestAlbumQueriesOrNilIsAHonestNil(t *testing.T) {
	// The typed-nil trap: a nil *album.Table in an interface field is not == nil, so every
	// availability check would pass and then panic. One of those checks is in the glimt delete path.
	if q := albumQueriesOrNil(nil); q != nil {
		t.Fatal("a nil table must convert to a nil interface")
	}
}

// Album ids are used as subject tokens and as filenames-by-proxy; the fixture's must survive.
func TestDevAlbumFixtureSlugsAreUsableAsSubjectTokens(t *testing.T) {
	for _, spec := range devAlbumFixtures() {
		id := "dev-album-" + spec.slug
		if _, err := album.Subject("2026", id, album.VerbCreated); err != nil {
			t.Errorf("fixture album %q yields an unusable subject: %v", spec.slug, err)
		}
		if strings.ContainsAny(spec.slug, "./ *>") {
			t.Errorf("fixture slug %q is not URL-safe", spec.slug)
		}
	}
}

// The fixture's states are the reason it exists: one unpublished album, one out-of-bounds coordinate,
// one item with no caption and no coordinate, and mixed orientations in one album.
func TestDevAlbumFixtureCoversTheStatesWorthSeeing(t *testing.T) {
	specs := devAlbumFixtures()

	var unpublished, withCoordinate, plain, portrait, landscape int
	for _, spec := range specs {
		if !spec.published {
			unpublished++
		}
		for _, item := range spec.items {
			switch {
			case item.lat != nil:
				withCoordinate++
			case item.caption == "":
				plain++
			}
			if item.h > item.w {
				portrait++
			}
			if item.w > item.h {
				landscape++
			}
		}
	}

	if unpublished == 0 {
		t.Error("no unpublished album: the invisible-to-the-public state cannot be checked")
	}
	if withCoordinate == 0 {
		t.Error("no coordinates: the map cannot be looked at")
	}
	if plain == 0 {
		t.Error("no item without a caption: the card must not reserve space for one")
	}
	if portrait == 0 || landscape == 0 {
		t.Error("want both orientations in the fixture: that is what catches a grid bug")
	}
}
