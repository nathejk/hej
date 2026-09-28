package imaging_test

import (
	"bytes"
	"encoding/binary"
	"image/jpeg"
	"testing"
	"time"

	"nathejk.dk/internal/imaging"
)

// ReadShotAt (task 440, PRD 024 §6 R3): the capture time an album sorted "by time" sorts on.
//
// # Why the fixtures are built byte by byte
//
// The same reason the GPS ones are, and it is the reason this package has no EXIF dependency at all (see
// the package comment): an EXIF block can be constructed exactly, including the malformed shapes that are
// the interesting half of the behaviour. A checked-in photograph from somebody's phone would test one
// camera's idea of EXIF and could not be made to hold a broken sub-IFD on purpose.

// shotOptions lets a test break one thing at a time.
type shotOptions struct {
	bigEndian bool
	// noExifPointer leaves IFD0 without the 0x8769 tag, which is where the timestamp lives.
	noExifPointer bool
	// omitTag builds the Exif sub-IFD with no DateTimeOriginal in it.
	omitTag bool
	// typeShort writes the timestamp as SHORT instead of ASCII.
	typeShort bool
	// countInline claims a value short enough to sit in the entry rather than at an offset.
	countInline bool
	// offsetBeyondEnd points the value past the end of the block.
	offsetBeyondEnd bool
	// exifPointerBeyondEnd points the Exif sub-IFD past the end of the block.
	exifPointerBeyondEnd bool
}

// jpegWithShotAt builds a JPEG carrying an EXIF Exif sub-IFD with DateTimeOriginal.
//
// Layout, fixed so the offsets below are readable rather than computed:
//
//	8    TIFF header
//	18   IFD0: one entry (the Exif sub-IFD pointer) + next-IFD offset
//	18   Exif sub-IFD: one entry + next-IFD offset
//	...  the ASCII the timestamp entry points at
func jpegWithShotAt(t testing.TB, stamp string, opts shotOptions) []byte {
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

	// IFD0 at 8: count(2) + one 12-byte entry + next(4) = 18 bytes, so the Exif sub-IFD starts at 26, and
	// its own 18 bytes put the ASCII value at 44.
	const ifd0 = 8
	const exifIFD = 26
	const valueAt = 44

	exifPointer := uint32(exifIFD)
	if opts.exifPointerBeyondEnd {
		exifPointer = 1 << 20
	}

	tiff = put16(tiff, 42)
	tiff = put32(tiff, ifd0)

	// IFD0: the Exif sub-IFD pointer.
	if opts.noExifPointer {
		tiff = put16(tiff, 0)
	} else {
		tiff = put16(tiff, 1)
		tiff = put16(tiff, 0x8769) // Exif IFD pointer
		tiff = put16(tiff, 4)      // LONG
		tiff = put32(tiff, 1)
		tiff = put32(tiff, exifPointer)
	}
	tiff = put32(tiff, 0) // no next IFD

	// The Exif sub-IFD: DateTimeOriginal.
	value := append([]byte(stamp), 0)
	if opts.omitTag {
		tiff = put16(tiff, 0)
	} else {
		typ := uint16(2) // ASCII
		if opts.typeShort {
			typ = 3
		}
		count := uint32(len(value))
		if opts.countInline {
			count = 4
		}
		at := uint32(valueAt)
		if opts.offsetBeyondEnd {
			at = 1 << 20
		}
		tiff = put16(tiff, 1)
		tiff = put16(tiff, 0x9003)
		tiff = put16(tiff, typ)
		tiff = put32(tiff, count)
		tiff = put32(tiff, at)
	}
	tiff = put32(tiff, 0) // no next IFD

	// The offsets above are written as constants rather than computed, so the fixture has to say when it no
	// longer matches itself — a silently wrong offset would make a test pass for the wrong reason. Only
	// checked for the shapes that have both entries: omitting one is *meant* to shorten the block.
	if !opts.noExifPointer && !opts.omitTag && len(tiff) != valueAt {
		t.Fatalf("fixture layout drifted: value should start at %d, block is %d bytes", valueAt, len(tiff))
	}
	tiff = append(tiff, value...)

	// APP1: "Exif\0\0" then the TIFF block.
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1}
	segment = binary.BigEndian.AppendUint16(segment, uint16(len(payload)+2))
	segment = append(segment, payload...)

	// After SOI, before the rest, which is where a real writer puts it.
	out := append([]byte{}, encoded[:2]...)
	out = append(out, segment...)
	return append(out, encoded[2:]...)
}

func copenhagen(t testing.TB) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Copenhagen")
	if err != nil {
		t.Skipf("no zoneinfo for Europe/Copenhagen: %v", err)
	}
	return loc
}

func TestReadShotAtReadsTheCaptureTime(t *testing.T) {
	loc := copenhagen(t)
	raw := jpegWithShotAt(t, "2026:09:12 23:41:07", shotOptions{})

	got, ok := imaging.ReadShotAt(raw, loc)
	if !ok {
		t.Fatal("want a capture time")
	}
	want := time.Date(2026, 9, 12, 23, 41, 7, 0, loc)
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

// The timestamp carries no timezone, so the location is the caller's to supply — and it must actually be
// applied. Reading 23:41 on a September night as UTC would place it two hours earlier, which is a
// different night once it is near midnight. That is precisely when Nathejk photographs are taken.
func TestReadShotAtAppliesTheLocation(t *testing.T) {
	loc := copenhagen(t)
	raw := jpegWithShotAt(t, "2026:09:12 23:41:07", shotOptions{})

	local, ok := imaging.ReadShotAt(raw, loc)
	if !ok {
		t.Fatal("want a capture time")
	}
	utc, ok := imaging.ReadShotAt(raw, time.UTC)
	if !ok {
		t.Fatal("want a capture time in UTC too")
	}
	if local.Equal(utc) {
		t.Error("the location was ignored: a September timestamp is CEST, two hours off UTC")
	}
	if _, offset := local.Zone(); offset != 2*3600 {
		t.Errorf("want CEST (+2h) in September, got offset %d", offset)
	}
	// nil is documented as UTC rather than as an error, so a caller that has not loaded a zone still gets
	// a usable answer instead of no capture time at all.
	if nilLoc, ok := imaging.ReadShotAt(raw, nil); !ok || !nilLoc.Equal(utc) {
		t.Errorf("nil location should mean UTC, got %s (ok=%v)", nilLoc, ok)
	}
}

func TestReadShotAtHandlesBothByteOrders(t *testing.T) {
	loc := copenhagen(t)
	want := time.Date(2026, 9, 12, 23, 41, 7, 0, loc)

	for _, big := range []bool{false, true} {
		raw := jpegWithShotAt(t, "2026:09:12 23:41:07", shotOptions{bigEndian: big})
		got, ok := imaging.ReadShotAt(raw, loc)
		if !ok || !got.Equal(want) {
			t.Errorf("bigEndian=%v: got %s (ok=%v), want %s", big, got, ok, want)
		}
	}
}

// A camera whose clock was never set, and one whose battery died. Both are structurally valid EXIF and
// semantically rubbish, and both turn up at an event where cameras are borrowed. "No capture time" is the
// right answer: the caller falls back to the upload time and knows it is doing so.
func TestReadShotAtRefusesAnUnsetClock(t *testing.T) {
	loc := copenhagen(t)

	for name, stamp := range map[string]string{
		"all zero":         "0000:00:00 00:00:00",
		"all spaces":       "    :  :     :  :  ",
		"dead battery":     "1980:01:01 00:00:00",
		"month 13":         "2026:13:01 10:00:00",
		"day 32":           "2026:09:32 10:00:00",
		"hour 25":          "2026:09:12 25:00:00",
		"far future":       "2999:09:12 10:00:00",
		"dashes":           "2026-09-12 23:41:07",
		"too short":        "2026:09:12",
		"trailing rubbish": "2026:09:12 23:41:07 CEST",
		"empty":            "",
	} {
		if got, ok := imaging.ReadShotAt(jpegWithShotAt(t, stamp, shotOptions{}), loc); ok {
			t.Errorf("%s (%q): want no capture time, got %s", name, stamp, got)
		}
	}
}

// Every structural break is "the file did not say", following ReadOrientation and ReadGPS. These are the
// shapes an attacker chooses, as opposed to the ones above, which a camera writes.
func TestReadShotAtRefusesBrokenBlocks(t *testing.T) {
	loc := copenhagen(t)

	for name, opts := range map[string]shotOptions{
		"no Exif sub-IFD pointer":  {noExifPointer: true},
		"pointer past the end":     {exifPointerBeyondEnd: true},
		"no DateTimeOriginal":      {omitTag: true},
		"wrong type":               {typeShort: true},
		"claims an inline value":   {countInline: true},
		"value offset past theend": {offsetBeyondEnd: true},
	} {
		if got, ok := imaging.ReadShotAt(jpegWithShotAt(t, "2026:09:12 23:41:07", opts), loc); ok {
			t.Errorf("%s: want no capture time, got %s", name, got)
		}
	}
}

func TestReadShotAtSurvivesRubbish(t *testing.T) {
	loc := copenhagen(t)

	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, gradient(8, 8), nil); err != nil {
		t.Fatal(err)
	}

	for name, raw := range map[string][]byte{
		"empty":                  {},
		"a plain JPEG":           jpg.Bytes(),
		"SOI only":               {0xFF, 0xD8},
		"a lying segment length": {0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E', 'x', 'i', 'f', 0, 0},
		"Exif header, no TIFF":   append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0, 8}, []byte("Exif\x00\x00")...),
	} {
		if _, ok := imaging.ReadShotAt(raw, loc); ok {
			t.Errorf("%s: want no capture time", name)
		}
	}
}

// The three readers share the header walk, so a file carrying all three must yield all three. They have
// interfered before in this package's history — see TestOrientationAndGPSDoNotInterfere, which is this
// assertion for the other pair.
func TestShotAtDoesNotDisturbOrientation(t *testing.T) {
	loc := copenhagen(t)
	raw := jpegWithShotAt(t, "2026:09:12 23:41:07", shotOptions{})

	// The fixture has no Orientation tag, so upright is the correct answer — what matters is that walking
	// to the Exif sub-IFD did not leave the orientation reader following a stale offset.
	if got := imaging.ReadOrientation(raw); got != 1 {
		t.Errorf("orientation = %d, want 1 for a file with no orientation tag", got)
	}
	if _, ok := imaging.ReadShotAt(raw, loc); !ok {
		t.Error("the capture time should still be readable")
	}
	// And a file with an orientation and no Exif sub-IFD has no capture time rather than a confused one.
	if _, ok := imaging.ReadShotAt(jpegWithOrientation(t, gradient(8, 8), 6, false), loc); ok {
		t.Error("a file with only an orientation tag has no capture time")
	}
}

// Prepare re-encodes from pixels, so it destroys the capture time exactly as it destroys the GPS fix. That
// is the property that makes reading it *before* storage the only place this can be done — and the reason
// the value has to go into a column rather than be re-read later.
func TestPrepareStripsTheCaptureTime(t *testing.T) {
	loc := copenhagen(t)
	raw := jpegWithShotAt(t, "2026:09:12 23:41:07", shotOptions{})

	if _, ok := imaging.ReadShotAt(raw, loc); !ok {
		t.Fatal("the fixture should carry a capture time to begin with")
	}
	out, err := imaging.Prepare(raw, 2048, []int{320}, 82, false)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	for name, b := range map[string][]byte{
		"display image": out.Full.Bytes,
		"thumbnail":     out.Thumbs[0].Bytes,
	} {
		if _, ok := imaging.ReadShotAt(b, loc); ok {
			t.Errorf("the stored %s still carries a capture time", name)
		}
	}
}
