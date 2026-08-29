// Package imaging prepares uploaded portraits for storage: EXIF orientation, area-average
// downscaling, and JPEG re-encoding (PRD 003, tasks 105/104).
//
// # Why this is not a dependency
//
// Two small, well-understood operations are needed — read one EXIF tag, and downscale —
// and the repo has no imaging dependency today. Both are implemented here rather than
// pulling in `golang.org/x/image` plus an EXIF library, because what they must do is
// narrow and, crucially, *testable against constructed inputs*: an EXIF header can be
// built byte by byte in a test, and a resampler can be checked against a known gradient.
// If a future need arrives that is genuinely a library's job (colour management, HEIC),
// that is the moment to add one.
//
// # Why it is separate from the handler
//
// Everything here is a pure function of bytes. That is worth testing without a request,
// a session or a broker — the same reasoning that put `internal/track` in its own
// package.
package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"

	// Decoders only. Uploads may be PNG or GIF; everything is re-encoded to JPEG.
	_ "image/gif"
	_ "image/png"
)

// ErrNotAnImage means the bytes could not be decoded as an image.
//
// It is the *only* validation the upload path needs: a decode either succeeds on real
// pixels or it does not, so no content-type header or magic-byte table has to be trusted
// or maintained.
var ErrNotAnImage = errors.New("not a decodable image")

// Rendition is one encoded image produced from an upload.
//
// Name is what the event and the projection key it by, and what a client asks for. It is
// derived from the size (`thumb256`) rather than being a label like "small": a label needs
// a table somewhere to say what it means, and that table is what drifts from the pixels.
type Rendition struct {
	Name   string
	Bytes  []byte
	Width  int
	Height int
}

// Portrait is a prepared portrait: the display image, every thumbnail asked for, and
// optionally the original.
type Portrait struct {
	// Full is the display image, bounded by the caller's edge limit. Its Name is empty:
	// it is the portrait itself, not a variant of it.
	Full Rendition

	// Thumbs are the smaller renditions, in the order requested.
	//
	// A slice rather than one thumbnail because more sizes are expected (a grid
	// thumbnail is a different size from an avatar), and retrofitting a list onto a
	// single field means changing an event shape that is already on an append-only log.
	// Generated from the same decode as Full, so no rendition can disagree with another
	// about orientation.
	Thumbs []Rendition

	// Original is the uploaded image at its own resolution, with **all metadata
	// segments removed** and its pixels untouched.
	//
	// Kept because renditions are produced at upload and cannot be produced again from
	// a 1024px re-encode: without the original, adding a thumbnail size (or wanting more
	// detail on a face at 03:00) can only apply to portraits taken *after* the change.
	// The original is what makes a backfill possible at all.
	//
	// Empty when: the format has no metadata scrubber (see StripMetadata), or the upload
	// is **no larger than the display image**, in which case storing it would double the
	// backup for zero extra pixels (see Prepare).
	//
	// It is NOT the uploaded file. Stripping metadata is not optional — see the
	// package doc's note on originals — and it is why Orientation below exists.
	Original Rendition

	// Orientation is the EXIF orientation the upload declared (1–8).
	//
	// Load-bearing, and easy to overlook: Full and Thumbs have the rotation *applied*,
	// while Original holds the sensor's pixels with the tag stripped out. Without this
	// number recorded outside the file, a future re-render from the original would not
	// know which way up it goes — which is the one way keeping the original could still
	// lose information.
	Orientation int

	// Format is the decoded format of the upload ("jpeg", "png", "gif"). It describes
	// Original's bytes; every other rendition is JPEG.
	Format string
}

// Prepare decodes raw, corrects its orientation, and encodes the display image plus one
// thumbnail per entry in thumbEdges. When keepOriginal is set it also returns the
// metadata-stripped original.
//
// edge bounds the longest side of the display image; each thumbEdge does the same for one
// thumbnail. Nothing is ever upscaled: a small upload stays small rather than being blown
// up into a blurry "large" image — which also means a thumbnail can legitimately come back
// the same size as the full image when someone uploads a tiny picture.
func Prepare(raw []byte, edge int, thumbEdges []int, quality int, keepOriginal bool) (Portrait, error) {
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Portrait{}, ErrNotAnImage
	}

	orientation := 1
	if format == "jpeg" {
		// Only JPEG carries the tag. Read before anything else, because both the applied
		// rotation below and the recorded Orientation come from it.
		orientation = ReadOrientation(raw)
		img = applyOrientation(img, orientation)
	}

	out := Portrait{Orientation: orientation, Format: format}

	out.Full, err = render(img, "", edge, quality)
	if err != nil {
		return Portrait{}, err
	}

	out.Thumbs = make([]Rendition, 0, len(thumbEdges))
	for _, thumbEdge := range thumbEdges {
		thumb, terr := render(img, ThumbName(thumbEdge), thumbEdge, quality)
		if terr != nil {
			return Portrait{}, terr
		}
		out.Thumbs = append(out.Thumbs, thumb)
	}

	if keepOriginal {
		stripped, ok := StripMetadata(raw, format)
		if ok {
			// Dimensions are the *decoded* image's, i.e. before rotation is applied — they
			// describe the bytes being stored, not how they should be displayed. Taking
			// them from `img` after applyOrientation would describe neither.
			b, _, derr := image.DecodeConfig(bytes.NewReader(stripped))
			if derr != nil {
				// The scrubber produced something undecodable. Refusing to store it is the
				// only safe answer: an "original" that cannot be read is worse than none,
				// because it looks like a backfill is possible when it is not.
				return Portrait{}, fmt.Errorf("stripped original is not decodable: %w", derr)
			}

			// Only keep it if it actually holds more pixels than the display image.
			//
			// MEASURED IN PRODUCTION, 2026-08-29: without this, an upload the client had
			// already downscaled to 1024px was stored twice — 141,970 bytes of "original"
			// beside a 74,403-byte display image of **identical dimensions**. 1.9× the
			// storage for no additional information, on the one directory that must be
			// backed up.
			//
			// Compared on the longest edge rather than byte length: pixels are the
			// information, and a same-size original that merely encodes at higher quality
			// buys only a marginal reduction in generational loss when a future thumbnail
			// is rendered — not worth doubling the backup for.
			//
			// So this makes "keep the original" self-limiting: it costs storage exactly
			// when it buys something.
			if maxInt(b.Width, b.Height) > maxInt(out.Full.Width, out.Full.Height) {
				out.Original = Rendition{
					Name:   "original",
					Bytes:  stripped,
					Width:  b.Width,
					Height: b.Height,
				}
			}
		}
	}

	return out, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// StripMetadata removes every metadata block from an encoded image, leaving the compressed
// pixel data byte-for-byte intact. ok is false for a format with no scrubber here.
//
// # Why not just store the uploaded file
//
// PRD 003 §6 requires EXIF — notably GPS — to be gone. For the display renditions that
// comes free, because they are re-encoded from pixels. An original cannot be re-encoded
// (that would defeat the point of keeping it), so the metadata has to be removed from the
// container instead. A phone photograph routinely carries the location it was taken,
// which for a photograph of a child at a scouting event is exactly the field that must not
// be retained.
//
// This is lossless: it drops whole segments/chunks and copies the entropy-coded data
// unchanged, so the stored original decodes to the identical pixels.
//
// The orientation tag goes with everything else, which is why Prepare records it in the
// event instead — metadata we control and can audit, rather than metadata riding along
// inside a file.
func StripMetadata(raw []byte, format string) ([]byte, bool) {
	switch format {
	case "jpeg":
		return stripJPEGSegments(raw)
	case "png":
		return stripPNGChunks(raw)
	default:
		// GIF and anything else: no scrubber, so no original is kept. Deliberately a
		// refusal rather than "store it as-is" — the whole point is that nothing
		// unexamined is retained. In practice every camera path produces JPEG.
		return nil, false
	}
}

// stripJPEGSegments copies a JPEG without its APPn (EXIF, XMP, ICC, JFIF thumbnails) and
// COM segments.
func stripJPEGSegments(raw []byte) ([]byte, bool) {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return nil, false
	}

	out := make([]byte, 0, len(raw))
	out = append(out, 0xFF, 0xD8)

	i := 2
	for i+4 <= len(raw) {
		if raw[i] != 0xFF {
			return nil, false
		}
		marker := raw[i+1]

		// Start of scan: everything from here is compressed pixel data (plus the EOI, and
		// possibly trailing junk). Copied verbatim — this is the part that must not be
		// touched.
		if marker == 0xDA {
			out = append(out, raw[i:]...)
			return out, true
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD9) {
			out = append(out, raw[i], raw[i+1])
			i += 2
			continue
		}

		length := int(binary.BigEndian.Uint16(raw[i+2 : i+4]))
		if length < 2 || i+2+length > len(raw) {
			return nil, false
		}
		// APPn (0xE0–0xEF) is where EXIF, XMP and ICC live; COM (0xFE) is a free-text
		// comment. Everything else — quantisation tables, Huffman tables, frame headers —
		// is needed to decode the image.
		drop := (marker >= 0xE0 && marker <= 0xEF) || marker == 0xFE
		if !drop {
			out = append(out, raw[i:i+2+length]...)
		}
		i += 2 + length
	}

	// Ran out of bytes without reaching a scan: not a usable JPEG.
	return nil, false
}

// pngKeptChunks is the allowlist: critical chunks plus the ancillary ones that affect how
// the pixels are interpreted.
//
// An allowlist rather than a blocklist, deliberately. A blocklist has to enumerate every
// metadata chunk that exists now and every one added later; this way an unknown chunk is
// dropped, which is the safe default for data whose whole purpose is not being retained.
var pngKeptChunks = map[string]bool{
	// Critical.
	"IHDR": true, "PLTE": true, "IDAT": true, "IEND": true,
	// Affect rendering, carry nothing about the photographer or the place.
	"tRNS": true, "gAMA": true, "cHRM": true, "sRGB": true, "iCCP": true, "sBIT": true,
	"pHYs": true,
}

// stripPNGChunks copies a PNG keeping only the chunks in pngKeptChunks.
//
// Also removes anything appended after IEND, which is a favourite place to hide a payload
// and is ignored by decoders — so a file can be a valid image and carry something else
// entirely.
func stripPNGChunks(raw []byte) ([]byte, bool) {
	const signatureLen = 8
	if len(raw) < signatureLen+12 {
		return nil, false
	}

	out := make([]byte, 0, len(raw))
	out = append(out, raw[:signatureLen]...)

	i := signatureLen
	for i+8 <= len(raw) {
		length := int(binary.BigEndian.Uint32(raw[i : i+4]))
		if length < 0 {
			return nil, false
		}
		end := i + 12 + length // length + type + data + crc
		if end > len(raw) {
			return nil, false
		}
		chunkType := string(raw[i+4 : i+8])

		if pngKeptChunks[chunkType] {
			out = append(out, raw[i:end]...)
		}
		if chunkType == "IEND" {
			// Stop here rather than continuing: whatever follows IEND is not part of the
			// image.
			return out, true
		}
		i = end
	}

	// No IEND found.
	return nil, false
}

// ThumbName is the canonical name for a thumbnail of the given longest edge.
//
// One function so the producer and every consumer agree without a shared constant per
// size — adding a size should not require editing a name table.
func ThumbName(edge int) string {
	return fmt.Sprintf("thumb%d", edge)
}

func render(img image.Image, name string, edge, quality int) (Rendition, error) {
	scaled := Fit(img, edge)
	encoded, err := encode(scaled, quality)
	if err != nil {
		return Rendition{}, err
	}
	b := scaled.Bounds()
	return Rendition{Name: name, Bytes: encoded, Width: b.Dx(), Height: b.Dy()}, nil
}

func encode(img image.Image, quality int) ([]byte, error) {
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Fit scales img down so its longest edge is at most edge, preserving aspect ratio.
// Images already within the limit are returned untouched.
//
// # Why area averaging rather than nearest-neighbour
//
// The thumbnail is roughly a quarter of the source's edge, and nearest-neighbour at that
// ratio throws away 15 of every 16 pixels: it aliases badly, and on a face it eats
// exactly the fine detail — eyes, hairline — that makes the thumbnail worth having.
// Averaging the source pixels that cover each destination pixel is the correct filter for
// minification, it is a dozen lines, and it needs no dependency.
//
// Note this is a *box* filter, not Lanczos. For pure minification the difference is
// slight; for enlargement it would matter, and this function never enlarges.
func Fit(img image.Image, edge int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return img
	}
	if w <= edge && h <= edge {
		return img
	}

	newW, newH := w, h
	if w >= h {
		newW = edge
		newH = h * edge / w
	} else {
		newH = edge
		newW = w * edge / h
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	// Work through an RGBA copy so pixel access is direct rather than going through the
	// generic At() of whatever concrete type decoded: this loop touches every source
	// pixel exactly once, and At() on a YCbCr image converts colour space per call.
	src := toRGBA(img)
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	for y := 0; y < newH; y++ {
		// The source rows covered by this destination row.
		y0 := y * h / newH
		y1 := (y + 1) * h / newH
		if y1 <= y0 {
			y1 = y0 + 1
		}

		for x := 0; x < newW; x++ {
			x0 := x * w / newW
			x1 := (x + 1) * w / newW
			if x1 <= x0 {
				x1 = x0 + 1
			}

			var r, g, b, a, n uint32
			for sy := y0; sy < y1; sy++ {
				offset := src.PixOffset(x0, sy)
				for sx := x0; sx < x1; sx++ {
					p := src.Pix[offset : offset+4]
					r += uint32(p[0])
					g += uint32(p[1])
					b += uint32(p[2])
					a += uint32(p[3])
					n++
					offset += 4
				}
			}
			if n == 0 {
				continue
			}
			o := dst.PixOffset(x, y)
			dst.Pix[o+0] = uint8(r / n)
			dst.Pix[o+1] = uint8(g / n)
			dst.Pix[o+2] = uint8(b / n)
			dst.Pix[o+3] = uint8(a / n)
		}
	}
	return dst
}

// toRGBA returns img as an *image.RGBA anchored at (0,0).
//
// The re-anchoring matters: a decoded image can have a non-zero Min, and every pixel
// index in this package assumes zero-based bounds.
func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect.Min == (image.Point{}) {
		return rgba
	}
	bounds := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(out, out.Bounds(), img, bounds.Min, draw.Src)
	return out
}

// ReadOrientation returns the EXIF orientation of a JPEG (1–8), or 1 when there is no
// tag, the file is not a JPEG, or anything about the structure is unexpected.
//
// # Why the server reads this at all
//
// Because the server re-encodes from pixels, which *drops* the tag. A phone that stores a
// photo rotated and describes the rotation in EXIF — the norm on iOS — would otherwise
// have its portrait stored sideways. The app's own capture path avoids this by drawing
// the already-rendered video frame, but the `<input capture>` fallback hands over an
// untouched camera file, and that is a path real people will use.
//
// Every failure returns 1 rather than an error: a missing or malformed tag is not a
// reason to refuse somebody's photo, it just means "assume upright".
func ReadOrientation(raw []byte) int {
	const upright = 1

	exif := findExifSegment(raw)
	if len(exif) < 8 {
		return upright
	}

	// TIFF header: byte order, magic 42, offset of the first IFD.
	var order binary.ByteOrder
	switch {
	case exif[0] == 'I' && exif[1] == 'I':
		order = binary.LittleEndian
	case exif[0] == 'M' && exif[1] == 'M':
		order = binary.BigEndian
	default:
		return upright
	}
	if order.Uint16(exif[2:4]) != 42 {
		return upright
	}

	ifd := int(order.Uint32(exif[4:8]))
	if ifd < 8 || ifd+2 > len(exif) {
		return upright
	}

	count := int(order.Uint16(exif[ifd : ifd+2]))
	entries := exif[ifd+2:]
	const entrySize = 12
	for i := 0; i < count; i++ {
		off := i * entrySize
		if off+entrySize > len(entries) {
			return upright
		}
		entry := entries[off : off+entrySize]
		if order.Uint16(entry[0:2]) != 0x0112 { // Orientation
			continue
		}
		// Type 3 is SHORT, and a SHORT value sits inline in the first two bytes of the
		// value field rather than at an offset.
		if order.Uint16(entry[2:4]) != 3 {
			return upright
		}
		value := int(order.Uint16(entry[8:10]))
		if value < 1 || value > 8 {
			return upright
		}
		return value
	}
	return upright
}

// findExifSegment returns the TIFF block inside the JPEG's APP1/Exif segment, or nil.
func findExifSegment(raw []byte) []byte {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 { // SOI
		return nil
	}

	i := 2
	for i+4 <= len(raw) {
		if raw[i] != 0xFF {
			// Not at a marker, so the structure is not what we expect — and guessing a
			// way forward through arbitrary bytes is how a parser becomes a hazard.
			return nil
		}
		marker := raw[i+1]
		// Start of scan: image data begins, so there is no metadata left to find.
		if marker == 0xDA {
			return nil
		}
		// Standalone markers carry no length.
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD9) {
			i += 2
			continue
		}
		length := int(binary.BigEndian.Uint16(raw[i+2 : i+4]))
		if length < 2 || i+2+length > len(raw) {
			return nil
		}
		payload := raw[i+4 : i+2+length]
		if marker == 0xE1 && len(payload) >= 6 && bytes.Equal(payload[:6], []byte("Exif\x00\x00")) {
			return payload[6:]
		}
		i += 2 + length
	}
	return nil
}

// applyOrientation returns img transformed so it is upright.
//
// The eight EXIF values cover every combination of a quarter turn and a mirror. They are
// applied through one coordinate mapping rather than eight bespoke loops, because the
// bespoke version is where 6 and 8 get swapped — the classic "everyone's photos are
// upside down" bug.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}

	src := toRGBA(img)
	w, h := src.Rect.Dx(), src.Rect.Dy()

	// Values 5–8 involve a quarter turn, so the result's axes are swapped.
	outW, outH := w, h
	if orientation >= 5 {
		outW, outH = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))

	for y := 0; y < outH; y++ {
		for x := 0; x < outW; x++ {
			var sx, sy int
			switch orientation {
			case 2: // mirrored horizontally
				sx, sy = w-1-x, y
			case 3: // rotated 180°
				sx, sy = w-1-x, h-1-y
			case 4: // mirrored vertically
				sx, sy = x, h-1-y
			case 5: // transposed
				sx, sy = y, x
			case 6: // rotated 90° clockwise
				sx, sy = y, h-1-x
			case 7: // transversed
				sx, sy = w-1-y, h-1-x
			case 8: // rotated 90° counter-clockwise
				sx, sy = w-1-y, x
			default:
				sx, sy = x, y
			}
			so := src.PixOffset(sx, sy)
			do := dst.PixOffset(x, y)
			copy(dst.Pix[do:do+4], src.Pix[so:so+4])
		}
	}
	return dst
}
