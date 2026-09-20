package imaging_test

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"math"

	"nathejk.dk/internal/imaging"
	"testing"
)

// EXIF GPS reading (PRD 011, task 333).
//
// The headers are built byte by byte, which is the reason the package doc gives for not taking an EXIF
// dependency: a GPS block is a few dozen bytes of TIFF and every malformed shape that matters can be
// constructed here deliberately. Most of these cases are the malformed ones, because a half-parsed
// coordinate is a pin in the wrong place on a public page — worse than no pin.

// gpsCoord is one latitude or longitude as EXIF stores it: three rationals plus a hemisphere.
type gpsCoord struct {
	deg, min uint32
	// secNum/secDen are kept separate so a test can build the zero-denominator case real cameras emit.
	secNum, secDen uint32
	ref            byte
}

func coord(deg, min uint32, sec float64, ref byte) gpsCoord {
	return gpsCoord{deg: deg, min: min, secNum: uint32(sec * 100), secDen: 100, ref: ref}
}

// gpsOptions lets a test break one thing at a time.
type gpsOptions struct {
	bigEndian bool
	// omitLat and friends drop a tag entirely.
	omitLat, omitLng, omitLatRef, omitLngRef bool
	// noGPSPointer leaves IFD0 without the 0x8825 tag.
	noGPSPointer bool
	// latTypeShort writes latitude as SHORT instead of RATIONAL.
	latTypeShort bool
	// latCount overrides latitude's component count (3 is correct).
	latCount uint32
	// gpsPointerBeyondEnd points the GPS IFD past the end of the block.
	gpsPointerBeyondEnd bool
	// latOffsetBeyondEnd points latitude's rationals past the end of the block.
	latOffsetBeyondEnd bool
}

// jpegWithGPS builds a JPEG carrying an EXIF GPS sub-IFD.
//
// Layout, fixed so the offsets below are readable rather than computed:
//
//	8    TIFF header
//	8    IFD0: one entry (the GPS pointer) + next-IFD offset
//	...  GPS IFD: four entries + next-IFD offset
//	...  the rationals latitude and longitude point at
func jpegWithGPS(t testing.TB, lat, lng gpsCoord, opts gpsOptions) []byte {
	t.Helper()

	var body bytes.Buffer
	if err := jpeg.Encode(&body, gradient(8, 8), nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	encoded := body.Bytes()

	var order binary.ByteOrder = binary.LittleEndian
	tiff := []byte{'I', 'I'}
	if opts.bigEndian {
		order = binary.BigEndian
		tiff = []byte{'M', 'M'}
	}

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

	// IFD0 at 8: count(2) + one 12-byte entry + next(4) = 18 bytes, so the GPS IFD starts at 26.
	const ifd0 = 8
	const gpsIFD = 26

	gpsPointer := uint32(gpsIFD)
	if opts.gpsPointerBeyondEnd {
		gpsPointer = 1 << 20
	}

	tiff = put16(tiff, 42)
	tiff = put32(tiff, ifd0)

	entryCount := uint16(1)
	if opts.noGPSPointer {
		entryCount = 0
	}
	tiff = put16(tiff, entryCount)
	if !opts.noGPSPointer {
		tiff = put16(tiff, 0x8825) // GPS IFD pointer
		tiff = put16(tiff, 4)      // LONG
		tiff = put32(tiff, 1)
		tiff = put32(tiff, gpsPointer)
	}
	tiff = put32(tiff, 0) // no next IFD

	// The GPS IFD's own entries. Count them first so the rationals' offset can be computed.
	type entry struct {
		tag    uint16
		typ    uint16
		count  uint32
		inline []byte // 4 bytes, when the value fits
		atLat  bool   // needs the latitude rationals' offset
		atLng  bool
	}
	var entries []entry

	if !opts.omitLatRef {
		entries = append(entries, entry{tag: 0x0001, typ: 2, count: 2,
			inline: []byte{lat.ref, 0, 0, 0}})
	}
	if !opts.omitLat {
		typ := uint16(5)
		if opts.latTypeShort {
			typ = 3
		}
		count := uint32(3)
		if opts.latCount != 0 {
			count = opts.latCount
		}
		entries = append(entries, entry{tag: 0x0002, typ: typ, count: count, atLat: true})
	}
	if !opts.omitLngRef {
		entries = append(entries, entry{tag: 0x0003, typ: 2, count: 2,
			inline: []byte{lng.ref, 0, 0, 0}})
	}
	if !opts.omitLng {
		entries = append(entries, entry{tag: 0x0004, typ: 5, count: 3, atLng: true})
	}

	// Rationals live after the GPS IFD: count(2) + n*12 + next(4).
	rationals := gpsIFD + 2 + len(entries)*12 + 4
	latAt := uint32(rationals)
	lngAt := uint32(rationals + 24)
	if opts.latOffsetBeyondEnd {
		latAt = 1 << 20
	}

	tiff = put16(tiff, uint16(len(entries)))
	for _, e := range entries {
		tiff = put16(tiff, e.tag)
		tiff = put16(tiff, e.typ)
		tiff = put32(tiff, e.count)
		switch {
		case e.atLat:
			tiff = put32(tiff, latAt)
		case e.atLng:
			tiff = put32(tiff, lngAt)
		default:
			tiff = append(tiff, e.inline...)
		}
	}
	tiff = put32(tiff, 0) // no next IFD

	for _, c := range []gpsCoord{lat, lng} {
		tiff = put32(tiff, c.deg)
		tiff = put32(tiff, 1)
		tiff = put32(tiff, c.min)
		tiff = put32(tiff, 1)
		tiff = put32(tiff, c.secNum)
		tiff = put32(tiff, c.secDen)
	}

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1}
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(payload)+2))
	segment = append(segment, length...)
	segment = append(segment, payload...)

	out := append([]byte{0xFF, 0xD8}, segment...)
	return append(out, encoded[2:]...)
}

func closeTo(got, want float64) bool { return math.Abs(got-want) < 0.0001 }

// A coordinate in the race area, written the way a phone writes it.
func TestReadGPSReadsADanishCoordinate(t *testing.T) {
	// 55°43'59.74"N 12°15'53.35"E — near the 2026 checkpoints.
	raw := jpegWithGPS(t, coord(55, 43, 59.74, 'N'), coord(12, 15, 53.35, 'E'), gpsOptions{})

	lat, lng, ok := imaging.ReadGPS(raw)
	if !ok {
		t.Fatal("want a coordinate")
	}
	if !closeTo(lat, 55.7332611) || !closeTo(lng, 12.2648194) {
		t.Fatalf("got %f,%f", lat, lng)
	}
}

// Both byte orders, because getting this wrong presents as "GPS works on Android and not iPhone" —
// which is exactly the kind of bug that ships.
func TestReadGPSHandlesBothByteOrders(t *testing.T) {
	for name, bigEndian := range map[string]bool{"little endian": false, "big endian": true} {
		raw := jpegWithGPS(t, coord(55, 30, 0, 'N'), coord(12, 30, 0, 'E'),
			gpsOptions{bigEndian: bigEndian})
		lat, lng, ok := imaging.ReadGPS(raw)
		if !ok {
			t.Fatalf("%s: want a coordinate", name)
		}
		if !closeTo(lat, 55.5) || !closeTo(lng, 12.5) {
			t.Errorf("%s: got %f,%f, want 55.5,12.5", name, lat, lng)
		}
	}
}

func TestReadGPSAppliesHemisphere(t *testing.T) {
	cases := map[string]struct {
		latRef, lngRef byte
		wantLat        float64
		wantLng        float64
	}{
		"north east": {'N', 'E', 10.5, 20.5},
		"south east": {'S', 'E', -10.5, 20.5},
		"north west": {'N', 'W', 10.5, -20.5},
		"south west": {'S', 'W', -10.5, -20.5},
	}
	for name, c := range cases {
		raw := jpegWithGPS(t, coord(10, 30, 0, c.latRef), coord(20, 30, 0, c.lngRef), gpsOptions{})
		lat, lng, ok := imaging.ReadGPS(raw)
		if !ok {
			t.Fatalf("%s: want a coordinate", name)
		}
		if !closeTo(lat, c.wantLat) || !closeTo(lng, c.wantLng) {
			t.Errorf("%s: got %f,%f, want %f,%f", name, lat, lng, c.wantLat, c.wantLng)
		}
	}
}

// A hemisphere we do not recognise must not be silently treated as positive: that turns a southern
// coordinate into a northern one, which is a pin in the wrong hemisphere rather than a missing pin.
func TestReadGPSRejectsAnUnknownHemisphere(t *testing.T) {
	for _, ref := range []byte{'X', 0, ' ', 'n'} {
		raw := jpegWithGPS(t, gpsCoord{deg: 10, min: 30, secNum: 0, secDen: 1, ref: ref},
			coord(20, 30, 0, 'E'), gpsOptions{})
		if _, _, ok := imaging.ReadGPS(raw); ok {
			t.Errorf("latitude ref %q should be rejected", ref)
		}
	}
}

// Every way the block can be absent or partial. All of them mean "no coordinate".
func TestReadGPSRejectsIncompleteBlocks(t *testing.T) {
	cases := map[string]gpsOptions{
		"no GPS pointer in IFD0":   {noGPSPointer: true},
		"no latitude":              {omitLat: true},
		"no longitude":             {omitLng: true},
		"no latitude hemisphere":   {omitLatRef: true},
		"no longitude hemisphere":  {omitLngRef: true},
		"latitude is not RATIONAL": {latTypeShort: true},
		"latitude has two parts":   {latCount: 2},
		"GPS pointer out of range": {gpsPointerBeyondEnd: true},
		"latitude out of range":    {latOffsetBeyondEnd: true},
	}
	for name, opts := range cases {
		raw := jpegWithGPS(t, coord(55, 30, 0, 'N'), coord(12, 30, 0, 'E'), opts)
		if lat, lng, ok := imaging.ReadGPS(raw); ok {
			t.Errorf("%s: want no coordinate, got %f,%f", name, lat, lng)
		}
	}
}

// Real cameras emit a zero denominator for seconds. Refused rather than read as ".0" — a writer
// producing a malformed rational is not one whose degrees and minutes should be trusted either.
func TestReadGPSRejectsAZeroDenominator(t *testing.T) {
	lat := gpsCoord{deg: 55, min: 30, secNum: 0, secDen: 0, ref: 'N'}
	raw := jpegWithGPS(t, lat, coord(12, 30, 0, 'E'), gpsOptions{})
	if _, _, ok := imaging.ReadGPS(raw); ok {
		t.Fatal("a zero denominator must yield no coordinate")
	}
}

// Null Island: a legal coordinate in the Atlantic, and also what a camera with no fix writes. The
// second is overwhelmingly more likely for a Nathejk photograph.
func TestReadGPSRejectsZeroZero(t *testing.T) {
	raw := jpegWithGPS(t, coord(0, 0, 0, 'N'), coord(0, 0, 0, 'E'), gpsOptions{})
	if _, _, ok := imaging.ReadGPS(raw); ok {
		t.Fatal("0,0 must be treated as no fix")
	}
}

func TestReadGPSRejectsOutOfRangeDegrees(t *testing.T) {
	// 91°N is not a place.
	raw := jpegWithGPS(t, coord(91, 0, 0, 'N'), coord(12, 0, 0, 'E'), gpsOptions{})
	if _, _, ok := imaging.ReadGPS(raw); ok {
		t.Fatal("a latitude past the pole must be rejected")
	}
	raw = jpegWithGPS(t, coord(55, 0, 0, 'N'), coord(181, 0, 0, 'E'), gpsOptions{})
	if _, _, ok := imaging.ReadGPS(raw); ok {
		t.Fatal("a longitude past the antimeridian must be rejected")
	}
}

// Nothing here may panic on arbitrary bytes: this walks an uploaded file.
func TestReadGPSSurvivesRubbish(t *testing.T) {
	var body bytes.Buffer
	if err := jpeg.Encode(&body, gradient(4, 4), nil); err != nil {
		t.Fatalf("encode: %v", err)
	}

	inputs := map[string][]byte{
		"empty":            {},
		"one byte":         {0xFF},
		"SOI only":         {0xFF, 0xD8},
		"not a JPEG":       []byte("hello, this is not an image at all"),
		"plain JPEG":       body.Bytes(),
		"truncated APP1":   {0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x20, 'E', 'x', 'i', 'f', 0, 0},
		"lying length":     {0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E', 'x', 'i', 'f', 0, 0, 'I', 'I'},
		"bad byte order":   {0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x0E, 'E', 'x', 'i', 'f', 0, 0, 'Z', 'Z', 0, 0, 0, 0},
		"zero IFD offset":  {0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x10, 'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 0, 0, 0, 0},
		"truncated after ": append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x0A}, 'E', 'x', 'i', 'f'),
	}
	for name, raw := range inputs {
		if _, _, ok := imaging.ReadGPS(raw); ok {
			t.Errorf("%s: should not yield a coordinate", name)
		}
	}
}

// A file carrying only orientation must not yield a coordinate, and a file carrying only GPS must
// still report upright. The two readers share a header parser, so this guards the seam.
func TestOrientationAndGPSDoNotInterfere(t *testing.T) {
	withOrientation := jpegWithOrientation(t, gradient(8, 4), 6, false)
	if _, _, ok := imaging.ReadGPS(withOrientation); ok {
		t.Error("an orientation-only file must not yield a coordinate")
	}
	if got := imaging.ReadOrientation(withOrientation); got != 6 {
		t.Errorf("orientation still reads as %d after the shared-header refactor, want 6", got)
	}

	withGPS := jpegWithGPS(t, coord(55, 30, 0, 'N'), coord(12, 30, 0, 'E'), gpsOptions{})
	if got := imaging.ReadOrientation(withGPS); got != 1 {
		t.Errorf("a GPS-only file should read as upright, got %d", got)
	}
}

// The property that must never regress: reading the coordinate does not put it in the stored bytes.
func TestPrepareStillStripsGPS(t *testing.T) {
	raw := jpegWithGPS(t, coord(55, 43, 59.74, 'N'), coord(12, 15, 53.35, 'E'), gpsOptions{})

	if _, _, ok := imaging.ReadGPS(raw); !ok {
		t.Fatal("the fixture should carry a coordinate to begin with")
	}

	prepared, err := imaging.Prepare(raw, 1600, []int{320}, 85, true)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	for name, bytes := range map[string][]byte{
		"display image": prepared.Full.Bytes,
		"thumbnail":     prepared.Thumbs[0].Bytes,
		"original":      prepared.Original.Bytes,
	} {
		if _, _, ok := imaging.ReadGPS(bytes); ok {
			t.Errorf("%s still carries a GPS coordinate: reading it must never mean keeping it", name)
		}
	}
}
