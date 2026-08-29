package imaging_test

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"nathejk.dk/internal/imaging"
)

// Fuzzing, because these two functions are the only place in the service that walks
// attacker-supplied binary structure by hand.
//
// Everything else about an upload goes through Go's own decoders, which are hardened and
// not ours to second-guess. `ReadOrientation` and `StripMetadata` instead do their own
// segment/chunk arithmetic on bytes a member can choose freely — index arithmetic on
// untrusted lengths being the classic way to produce an out-of-range panic.
//
// A panic here would not take the API down (net/http recovers per connection and closes
// it), but it would turn one member's photo into an unexplained broken upload with only a
// stack trace in the log. Cheap to rule out; expensive to diagnose in the field.
//
// Run longer than the default when touching either parser:
//
//	go test ./internal/imaging -run=xxx -fuzz=FuzzStripMetadata -fuzztime=2m

func fuzzSeeds(f *testing.F) {
	f.Helper()

	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, gradient(24, 16), nil); err != nil {
		f.Fatal(err)
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, gradient(24, 16)); err != nil {
		f.Fatal(err)
	}

	f.Add(jpg.Bytes())
	f.Add(pngBuf.Bytes())
	f.Add(jpegWithOrientation(f, gradient(24, 16), 6, false))
	f.Add(jpegWithOrientation(f, gradient(24, 16), 3, true))
	// Structurally interesting nonsense: a bare SOI, a segment claiming a huge length,
	// and a PNG signature with no chunks.
	f.Add([]byte{0xFF, 0xD8})
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0xFF, 0xFF, 'E', 'x', 'i', 'f', 0, 0})
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Add([]byte{})
}

// FuzzReadOrientation: never panics, and never returns a value outside the EXIF range.
//
// The range matters as much as the absence of a panic: the value indexes a transform in
// applyOrientation, so an out-of-range answer would rotate a portrait into nonsense.
func FuzzReadOrientation(f *testing.F) {
	fuzzSeeds(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		got := imaging.ReadOrientation(data)
		if got < 1 || got > 8 {
			t.Fatalf("orientation = %d, want 1-8", got)
		}
	})
}

// FuzzStripMetadata: never panics, and anything it accepts must still be a decodable
// image with the same dimensions as the input.
//
// That second property is the one worth fuzzing for. "No panic" only says the parser
// survived; this says the parser did not quietly corrupt a member's photo while removing
// metadata — which is the failure that would be discovered months later, by which time
// the upload is gone.
func FuzzStripMetadata(f *testing.F) {
	fuzzSeeds(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, format := range []string{"jpeg", "png", "gif", ""} {
			stripped, ok := imaging.StripMetadata(data, format)
			if !ok {
				continue
			}

			before, beforeFormat, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				// The scrubber accepted bytes Go cannot decode. Not a contradiction:
				// StripMetadata validates container structure, not pixel data, and the
				// caller only ever reaches it after its own successful decode.
				continue
			}
			if beforeFormat != format {
				continue
			}

			after, _, err := image.Decode(bytes.NewReader(stripped))
			if err != nil {
				t.Fatalf("%s: stripped output no longer decodes: %v", format, err)
			}
			if before.Bounds() != after.Bounds() {
				t.Fatalf("%s: bounds changed: %v -> %v", format, before.Bounds(), after.Bounds())
			}
		}
	})
}
