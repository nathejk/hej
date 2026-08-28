package imaging_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"nathejk.dk/internal/imaging"
)

// jpegWithOrientation builds a real JPEG carrying an EXIF orientation tag.
//
// Constructed byte by byte rather than committed as a fixture: the point of the parser is
// that it reads a *structure*, so the test has to be able to vary byte order, tag order
// and the value itself — which a binary fixture cannot.
func jpegWithOrientation(t *testing.T, img image.Image, orientation int, bigEndian bool) []byte {
	t.Helper()

	var body bytes.Buffer
	if err := jpeg.Encode(&body, img, nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	encoded := body.Bytes()
	if encoded[0] != 0xFF || encoded[1] != 0xD8 {
		t.Fatal("encoder did not produce an SOI")
	}

	var order binary.ByteOrder = binary.LittleEndian
	tiff := []byte{'I', 'I'}
	if bigEndian {
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

	tiff = put16(tiff, 42)
	tiff = put32(tiff, 8) // IFD0 starts right after the header
	tiff = put16(tiff, 1) // one entry
	tiff = put16(tiff, 0x0112)
	tiff = put16(tiff, 3) // SHORT
	tiff = put32(tiff, 1) // count
	tiff = put16(tiff, uint16(orientation))
	tiff = append(tiff, 0, 0) // remaining 2 bytes of the value field
	tiff = put32(tiff, 0)     // no next IFD

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1}
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(payload)+2))
	segment = append(segment, length...)
	segment = append(segment, payload...)

	// SOI, then our APP1, then the rest of the encoder's output.
	out := append([]byte{0xFF, 0xD8}, segment...)
	return append(out, encoded[2:]...)
}

// gradient is asymmetric in both axes, so any rotation or mirror is detectable by
// sampling corners — a symmetric test image would pass with the transform inverted.
func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(255 * x / max(w-1, 1)), G: uint8(255 * y / max(h-1, 1)), B: 0, A: 255})
		}
	}
	return img
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestReadOrientationBothByteOrders(t *testing.T) {
	for _, bigEndian := range []bool{false, true} {
		for want := 1; want <= 8; want++ {
			raw := jpegWithOrientation(t, gradient(8, 8), want, bigEndian)
			if got := imaging.ReadOrientation(raw); got != want {
				t.Errorf("bigEndian=%v orientation = %d, want %d", bigEndian, got, want)
			}
		}
	}
}

// Anything unexpected must read as "upright" rather than failing: a malformed tag is no
// reason to refuse somebody's photo.
func TestReadOrientationDefaultsToUpright(t *testing.T) {
	plain := func() []byte {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, gradient(4, 4), nil); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}()

	cases := map[string][]byte{
		"no exif segment": plain,
		"not a jpeg":      []byte("PNG or whatever"),
		"empty":           nil,
		"truncated":       plain[:8],
		// A truncated EXIF payload: correct marker, nonsense inside.
		"broken exif": append([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x0A}, []byte("Exif\x00\x00II")...),
		"bad value":   jpegWithOrientation(t, gradient(4, 4), 99, false),
	}
	for name, raw := range cases {
		if got := imaging.ReadOrientation(raw); got != 1 {
			t.Errorf("%s: orientation = %d, want 1", name, got)
		}
	}
}

// Orientation 6 is a photo taken with the phone rotated: stored landscape, meant to be
// displayed portrait. The stored result must be portrait — this is the case that produces
// sideways faces when it is missed, and the fallback `<input capture>` path hits it.
func TestPrepareRotatesAccordingToExif(t *testing.T) {
	// Landscape source: 40 wide, 20 high.
	raw := jpegWithOrientation(t, gradient(40, 20), 6, false)

	out, err := imaging.Prepare(raw, 1024, []int{256}, 85)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if out.Full.Width != 20 || out.Full.Height != 40 {
		t.Errorf("stored image is %dx%d, want it turned upright (20x40)", out.Full.Width, out.Full.Height)
	}
	// And the thumbnail must agree — every rendition comes from one correction, so a
	// disagreement would mean the orientation was applied twice or not at all on one path.
	thumb := out.Thumbs[0]
	if thumb.Width > thumb.Height {
		t.Errorf("thumbnail is %dx%d, want portrait like the full image", thumb.Width, thumb.Height)
	}
}

func TestPrepareLeavesUprightImagesAlone(t *testing.T) {
	raw := jpegWithOrientation(t, gradient(40, 20), 1, false)
	out, err := imaging.Prepare(raw, 1024, []int{256}, 85)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if out.Full.Width != 40 || out.Full.Height != 20 {
		t.Errorf("got %dx%d, want the original 40x20", out.Full.Width, out.Full.Height)
	}
}

// Every orientation must produce a decodable image of plausible dimensions. A cheap test,
// but it is the one that catches an index inversion in the transform for values nobody
// looks at by hand (5 and 7).
func TestPrepareHandlesAllOrientations(t *testing.T) {
	for orientation := 1; orientation <= 8; orientation++ {
		raw := jpegWithOrientation(t, gradient(30, 10), orientation, false)
		out, err := imaging.Prepare(raw, 1024, []int{256}, 85)
		if err != nil {
			t.Fatalf("orientation %d: %v", orientation, err)
		}
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(out.Full.Bytes))
		if err != nil {
			t.Fatalf("orientation %d: stored bytes not a JPEG: %v", orientation, err)
		}
		wantSwapped := orientation >= 5
		gotSwapped := cfg.Width < cfg.Height
		if wantSwapped != gotSwapped {
			t.Errorf("orientation %d: stored %dx%d, swapped=%v want swapped=%v",
				orientation, cfg.Width, cfg.Height, gotSwapped, wantSwapped)
		}
	}
}

func TestPrepareProducesBothSizes(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gradient(2000, 1000), nil); err != nil {
		t.Fatal(err)
	}

	out, err := imaging.Prepare(buf.Bytes(), 1024, []int{256}, 85)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if out.Full.Width != 1024 || out.Full.Height != 512 {
		t.Errorf("full = %dx%d, want 1024x512", out.Full.Width, out.Full.Height)
	}
	if len(out.Thumbs) != 1 {
		t.Fatalf("got %d thumbnails, want 1", len(out.Thumbs))
	}
	thumb := out.Thumbs[0]
	if thumb.Name != "thumb256" {
		t.Errorf("thumbnail name = %q, want thumb256", thumb.Name)
	}
	if thumb.Width != 256 || thumb.Height != 128 {
		t.Errorf("thumb = %dx%d, want 256x128", thumb.Width, thumb.Height)
	}
	// The thumbnail is the point of task 104: it must actually be small, or PRD 007's
	// offline sync gains nothing from it.
	if len(thumb.Bytes) >= len(out.Full.Bytes) {
		t.Errorf("thumb (%d B) is not smaller than full (%d B)", len(thumb.Bytes), len(out.Full.Bytes))
	}
	for name, data := range map[string][]byte{"full": out.Full.Bytes, "thumb": thumb.Bytes} {
		if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
			t.Errorf("%s is not a decodable JPEG: %v", name, err)
		}
	}
}

// Several sizes at once, which is what the list shape exists for. Each rendition must
// carry its own dimensions, and they must be genuinely different files.
func TestPrepareProducesEveryRequestedSize(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gradient(1200, 900), nil); err != nil {
		t.Fatal(err)
	}

	out, err := imaging.Prepare(buf.Bytes(), 1024, []int{512, 256, 96}, 85)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(out.Thumbs) != 3 {
		t.Fatalf("got %d thumbnails, want 3", len(out.Thumbs))
	}

	wantNames := []string{"thumb512", "thumb256", "thumb96"}
	wantEdges := []int{512, 256, 96}
	sizes := map[int]bool{}
	for i, thumb := range out.Thumbs {
		// Order is the order requested — a consumer indexing by position must not have to
		// guess.
		if thumb.Name != wantNames[i] {
			t.Errorf("thumb %d named %q, want %q", i, thumb.Name, wantNames[i])
		}
		if thumb.Width != wantEdges[i] {
			t.Errorf("%s is %dpx wide, want %d", thumb.Name, thumb.Width, wantEdges[i])
		}
		if thumb.Height == 0 || len(thumb.Bytes) == 0 {
			t.Errorf("%s is missing dimensions or bytes: %+v", thumb.Name, thumb)
		}
		if sizes[len(thumb.Bytes)] {
			t.Errorf("%s has the same byte length as another rendition", thumb.Name)
		}
		sizes[len(thumb.Bytes)] = true
	}

	// Smaller edge, smaller file — otherwise the size list buys nothing.
	if !(len(out.Thumbs[0].Bytes) > len(out.Thumbs[1].Bytes) &&
		len(out.Thumbs[1].Bytes) > len(out.Thumbs[2].Bytes)) {
		t.Errorf("renditions do not shrink with their edge: %d, %d, %d",
			len(out.Thumbs[0].Bytes), len(out.Thumbs[1].Bytes), len(out.Thumbs[2].Bytes))
	}
}

// No sizes requested is a legitimate configuration (thumbnails off), not an error.
func TestPrepareWithNoThumbnailSizes(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gradient(300, 300), nil); err != nil {
		t.Fatal(err)
	}
	out, err := imaging.Prepare(buf.Bytes(), 1024, nil, 85)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if len(out.Thumbs) != 0 {
		t.Errorf("got %d thumbnails, want none", len(out.Thumbs))
	}
	if len(out.Full.Bytes) == 0 {
		t.Error("the full image must still be produced")
	}
}

// Accepts PNG in, JPEG out — and nothing from the source file survives, which is how EXIF
// (including GPS) is guaranteed gone.
func TestPrepareAcceptsPngAndDropsTrailingBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, gradient(60, 60)); err != nil {
		t.Fatal(err)
	}
	tagged := append(buf.Bytes(), []byte("GPSLatitudeSecretMarker")...)

	out, err := imaging.Prepare(tagged, 1024, []int{256}, 85)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if bytes.Contains(out.Full.Bytes, []byte("SecretMarker")) ||
		bytes.Contains(out.Thumbs[0].Bytes, []byte("SecretMarker")) {
		t.Error("appended metadata survived re-encoding")
	}
}

func TestPrepareRejectsNonImages(t *testing.T) {
	if _, err := imaging.Prepare([]byte("MZ\x90\x00 not an image"), 1024, []int{256}, 85); err != imaging.ErrNotAnImage {
		t.Fatalf("err = %v, want ErrNotAnImage", err)
	}
}

func TestFitNeverUpscales(t *testing.T) {
	img := gradient(40, 30)
	got := imaging.Fit(img, 1024)
	if got.Bounds().Dx() != 40 || got.Bounds().Dy() != 30 {
		t.Errorf("got %v, want the original bounds", got.Bounds())
	}
}

// Area averaging is why this package does not use nearest-neighbour. On a source of
// alternating black and white columns, averaging must produce grey; nearest-neighbour
// would pick one column and produce pure black or white.
func TestFitAveragesRatherThanSamples(t *testing.T) {
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			shade := uint8(0)
			if x%2 == 0 {
				shade = 255
			}
			img.Set(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}

	small := imaging.Fit(img, size/8)
	r, _, _, _ := small.At(2, 2).RGBA()
	grey := r >> 8
	if grey < 100 || grey > 155 {
		t.Errorf("downscaled pixel = %d, want mid-grey — averaging is not happening", grey)
	}
}
