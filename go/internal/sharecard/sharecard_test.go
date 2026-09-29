package sharecard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"nathejk.dk/internal/diploma"
)

// The branded share card (PRD 026).
//
// # What these are really protecting
//
// The card is **lifted off the event poster** by colour: saturated yellow is the moon, flat black is the wordmark,
// everything else is parchment and is dropped. That works, and it rests on an assumption about one JPEG — so the
// thing worth testing is not "does it produce a PNG" but "does it still find the marks, and does it refuse rather
// than publish nonsense when it cannot".
//
// It matters because the failure is invisible from inside the process. A card with the wrong crop is a valid PNG of
// the right size; the only place anybody would notice is a Facebook post, days later, once Facebook has cached it.
// Two wrong guesses were caught exactly this way during the work: a hand-measured crop that took the top half of
// the letters and rendered "NATUEIK", and thresholds loose enough to turn the moon's drop shadow white.

func TestTheCardIsAPNGOfTheRequestedSize(t *testing.T) {
	out, err := Render(diploma.Background(), 1200, 630)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decoding what was produced: %v", err)
	}
	if format != "png" {
		t.Errorf("format = %q, want png", format)
	}
	if cfg.Width != 1200 || cfg.Height != 630 {
		t.Errorf("card is %dx%d, want 1200x630 — the size platforms render a large card at; under 600 wide they "+
			"show a thumbnail beside the text instead", cfg.Width, cfg.Height)
	}
}

// The three things that must be on it: black, the yellow moon, and white letters.
//
// Asserted by counting pixels rather than by comparing to a golden image. A golden PNG would fail on any harmless
// change — a threshold nudged, the fill fraction adjusted — and tell nobody whether the result was still right. These
// three proportions are what "a black card with the moon and the wordmark on it" actually means.
func TestTheCardCarriesTheMoonAndTheWordmarkOnBlack(t *testing.T) {
	img := renderAndDecode(t, 1200, 630)

	var black, yellow, white, other int
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl := rgb(img.At(x, y))
			switch {
			case luma(r, g, bl) < 24:
				black++
			case isMoon(r, g, bl):
				yellow++
			case r > 200 && g > 200 && bl > 200:
				white++
			default:
				other++
			}
		}
	}
	total := b.Dx() * b.Dy()

	// Mostly black, because the card is black with marks on it.
	if frac := float64(black) / float64(total); frac < 0.55 {
		t.Errorf("only %.0f%% of the card is black, want at least 55%%", frac*100)
	}
	// The moon is there and is a moon rather than a speck.
	if frac := float64(yellow) / float64(total); frac < 0.005 {
		t.Errorf("the moon covers %.2f%% of the card, want at least 0.5%% — either it was not found or something "+
			"tan was found instead", frac*100)
	}
	// And the wordmark: seven heavy capitals across most of the width.
	if frac := float64(white) / float64(total); frac < 0.04 {
		t.Errorf("the wordmark covers %.1f%% of the card, want at least 4%%", frac*100)
	}
	// Antialiasing produces greys at every edge, and nothing else should. A third of the card being neither is the
	// signature of the parchment having come along.
	if frac := float64(other) / float64(total); frac > 0.15 {
		t.Errorf("%.0f%% of the card is neither black, yellow nor white — the background came with it", frac*100)
	}
}

// The wordmark reaches across the card, which is what "taking up most of the space" was asked for.
//
// The specific failure this catches is the one that happened: a crop that clipped the letters produced marks that
// still filled the width but were half as tall, so width alone is not enough — the marks' *height* is what a wrong
// crop changes.
func TestTheMarksFillTheCard(t *testing.T) {
	img := renderAndDecode(t, 1200, 630)
	b := img.Bounds()

	minX, maxX, minY, maxY := b.Max.X, b.Min.X, b.Max.Y, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl := rgb(img.At(x, y))
			if luma(r, g, bl) < 40 {
				continue
			}
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}

	if w := maxX - minX; w < int(float64(b.Dx())*0.85) {
		t.Errorf("the marks span %d of %d pixels across, want at least 85%%", w, b.Dx())
	}
	// A band 6× wider than it is tall cannot fill a 1.91:1 card vertically, so this is deliberately modest — it is
	// here to catch a crop that lost half the letters, which is what "NATUEIK" looked like numerically.
	if h := maxY - minY; h < int(float64(b.Dy())*0.28) {
		t.Errorf("the marks span %d of %d pixels down, want at least 28%% — a clipped crop is the way this has "+
			"already gone wrong", h, b.Dy())
	}
}

// The thresholds are measured off the poster, and this is where the measurements live.
//
// If the artwork is replaced and these stop holding, the card is wrong — and this test is the place that says what
// the numbers were taken from, so the next person can re-measure rather than guess.
func TestTheThresholdsMatchThePoster(t *testing.T) {
	art, _, err := image.Decode(bytes.NewReader(diploma.Background()))
	if err != nil {
		t.Fatalf("decoding the poster: %v", err)
	}

	for _, want := range []struct {
		name  string
		x, y  int
		moon  bool
		ink   bool
		about string
	}{
		{"inside the moon", 280, 420, true, false, "saturated yellow, 255,255,1"},
		{"the moon's drop shadow", 380, 420, false, false,
			"luma 53 and warm: it must be neither, or it is repainted white and rings the moon"},
		{"inside a letter", 660, 450, false, true, "flat black, 0,0,0"},
		{"the parchment", 1500, 500, false, false, "luma 196"},
	} {
		r, g, b := rgb(art.At(want.x, want.y))
		if got := isMoon(r, g, b); got != want.moon {
			t.Errorf("%s (%d,%d) rgb=%d,%d,%d: isMoon = %v, want %v — %s",
				want.name, want.x, want.y, r, g, b, got, want.moon, want.about)
		}
		if got := isInk(r, g, b); got != want.ink {
			t.Errorf("%s (%d,%d) rgb=%d,%d,%d: isInk = %v, want %v — %s",
				want.name, want.x, want.y, r, g, b, got, want.ink, want.about)
		}
	}
}

// The band is found rather than assumed, and it is the wordmark's band.
func TestTheBandFoundIsTheWordmarks(t *testing.T) {
	art, _, err := image.Decode(bytes.NewReader(diploma.Background()))
	if err != nil {
		t.Fatalf("decoding the poster: %v", err)
	}

	band, ok := findBand(art)
	if !ok {
		t.Fatal("no band found in the poster's top")
	}
	page := art.Bounds()

	// It starts below the page edge and ends above "2026" — the year is a separate line and must not come along,
	// because the card is the site's rather than one event's.
	if top := float64(band.Min.Y) / float64(page.Dy()); top < 0.02 || top > 0.08 {
		t.Errorf("the band starts at %.3f of the page, want 0.02–0.08 — at 0.000 it has latched onto the dark "+
			"smudge along the poster's top edge, which is how this failed first", top)
	}
	if bottom := float64(band.Max.Y) / float64(page.Dy()); bottom < 0.16 || bottom > 0.23 {
		t.Errorf("the band ends at %.3f of the page, want 0.16–0.23: shorter clips the letters, longer reaches "+
			"the year", bottom)
	}
	// And it spans the width, since the moon and seven capitals do.
	if w := float64(band.Dx()) / float64(page.Dx()); w < 0.8 {
		t.Errorf("the band is %.2f of the page wide, want at least 0.80", w)
	}
}

// **A card that cannot be produced is an error, never a plausible-looking wrong card.**
//
// This is the guard that makes the whole colour-lifting approach acceptable: the assumption is about one image, and
// when it stops holding the answer has to be a 500 that somebody sees in a log rather than a black rectangle
// Facebook caches for a week.
func TestArtworkItCannotReadIsRefused(t *testing.T) {
	if _, err := Render([]byte("not an image"), 1200, 630); err == nil {
		t.Error("undecodable artwork must be an error")
	}

	// A uniform grey page: decodable, the right shape, and carrying no marks at all. The coverage check is what
	// catches it — there is no band to find, so nothing is lifted.
	grey := image.NewRGBA(image.Rect(0, 0, 2480, 3508))
	for y := 0; y < 3508; y++ {
		for x := 0; x < 2480; x++ {
			grey.SetRGBA(x, y, color.RGBA{R: 0xd0, G: 0xc0, B: 0xa0, A: 0xff})
		}
	}
	if _, err := Render(encodePNG(t, grey), 1200, 630); err == nil {
		t.Error("artwork with no wordmark in it must be an error, not a blank card")
	}

	// And a page that is *all* ink: the other end of the coverage check.
	blackPage := image.NewRGBA(image.Rect(0, 0, 2480, 3508))
	if _, err := Render(encodePNG(t, blackPage), 1200, 630); err == nil {
		t.Error("artwork that is entirely dark must be an error: the crop would be meaningless")
	}
}

func TestASillySizeIsRefused(t *testing.T) {
	for _, size := range [][2]int{{0, 630}, {1200, 0}, {-1, -1}} {
		if _, err := Render(diploma.Background(), size[0], size[1]); err == nil {
			t.Errorf("%dx%d must be refused", size[0], size[1])
		}
	}
}

func renderAndDecode(t *testing.T, w, h int) image.Image {
	t.Helper()
	out, err := Render(diploma.Background(), w, h)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decoding: %v", err)
	}
	return img
}

func rgb(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

// encodePNG renders a synthetic page for the failure cases above.
func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatalf("encoding a fixture: %v", err)
	}
	return out.Bytes()
}
