// Package sharecard renders the neutral branded card that Facebook, Messenger and iMessage show when somebody
// shares the public site's frontpage (PRD 026).
//
// # Why the card is rendered rather than stored
//
// There is no 1200×630 artwork in this repo and the maintainer has none to hand:
//
//	"I don't have a branded artwork at hand - create a black image, with logo moon and title (NATHEJK) taking up
//	 most of the space. Then i might come with a better solution later on, but this is it for now"
//
// So this is explicitly a placeholder, and it is built from **the event's own poster** rather than drawn: the
// poster already carries the crescent and the wordmark in the right letterforms, and lifting them means the card
// cannot drift from the year's design. Replace `Render` with a decoded asset when there is one — the handler
// depends on the function, not on how it gets its pixels.
//
// # Why the glyphs are extracted instead of typeset
//
// Drawing "NATHEJK" needs either a font rasteriser — `golang.org/x/image`, which `internal/imaging` refuses for
// reasons that apply here too — or seven hand-built letterforms that would look hand-built. And the obvious font is
// the one already embedded next door, `impact.ttf`, whose licence permits **embedding a subset in a document** and
// says nothing friendly about anything else (see `diploma.impactFont`). Rasterising it into a PNG served over HTTP
// is at best an argument.
//
// Lifting the marks off the poster avoids all of that: they are pixels we already ship, in the exact shapes the
// event uses, with no font involved.
//
// # How the lift works, and how it fails safely
//
// The poster's top band is a yellow crescent and near-black letters on light parchment, which separates on colour
// alone:
//
//   - **saturated yellow** — the moon, kept as it is;
//   - **very dark** — the wordmark, repainted white so it reads on black;
//   - **everything else** — parchment, dropped.
//
// The threshold for "very dark" is deliberately low. The moon carries a soft drop shadow and the parchment has
// cracks and speckles in the 80–150 range; both would otherwise become white marks on the card, and the shadow
// would ring the moon. At 70 the solid letters survive and the rest does not.
//
// That is an assumption about one image, so `Render` **checks its own output**: a band that yields implausibly
// little or implausibly much ink means the artwork moved, and it returns an error instead of publishing a card with
// the wrong thing on it. `TestTheCardFindsTheMoonAndTheWordmark` holds the same numbers, so next year's poster
// fails in CI rather than on Facebook.
package sharecard

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // the poster is a JPEG
	"image/png"
)

// Where to look for the marks, and how to recognise the band once found.
//
// **The band is located, not hardcoded.** The first version of this pinned it at fractions of the page measured by
// eye, and the numbers were wrong in the way eyeballing is wrong: the crop took the top half of the letters, so the
// card read NATUEIK. A projection profile finds the same band without anybody measuring, and it keeps working when
// the artwork shifts — which it does every year.
const (
	// Only the top of the page is considered. The wordmark is up there and so is everything else that is large and
	// dark, which is what the profile has to tell apart.
	searchBottom = 0.34

	// A row counts as part of the marks when this fraction of it is moon or ink.
	//
	// 5%, and the number had to be raised: at 0.4% the parchment's own cracks and the dark smudge along the
	// poster's top edge tripped it, the band began at y=0, and the crop came back 0% ink — a failure the guard
	// caught, which is the only reason this was not shipped. A letter row is 40–60% ink and the moon alone is 14%
	// of the width, so 5% separates the marks from the texture with room on both sides.
	rowInkFraction = 0.05

	// Rows this far apart are two different things. The poster sets NATHEJK on one line and the year beneath it, so
	// scanning stops at the first real gap — which is how "2026" stays off a card that is not about one event.
	// Expressed against the page so it scales with the raster.
	bandGapFraction = 0.012

	// A column counts only if this fraction of the band's height is marked in it. Without it, a single speck of
	// dark texture at the page edge extended the crop to x=0 and the card carried three grey dots in the corner.
	colInkFraction = 0.02

	// A little air around what was found, so an antialiased edge is not clipped.
	bandPadFraction = 0.004
)

// The classification thresholds. See the package comment for why dark is this dark.
// The thresholds, **measured off the poster** rather than guessed (see `TestTheThresholdsMatchThePoster`):
//
//	the wordmark          0,0,0            luma 0
//	the moon              255,255,1        luma 226
//	the moon's shadow     58,52,52         luma 53
//	                      70,64,54         luma 64
//	the parchment         214,196,152      luma 196
//	its darker specks     ~180,150,80      luma ~150, and warm
//
// Two guesses were wrong before these were taken, and both were visible only in the rendered card: ink at luma ≤ 70
// swallowed the moon's shadow and repainted it white, ringing the moon with a bright arc; and a moon test loose
// enough to accept tan accepted the parchment's own specks, which arrived as brown dots in the corner.
const (
	// Ink is the wordmark, which is flat black with an enormous margin to the shadow at 53.
	inkMaxLuma = 30

	// And neutral. Kept even though the luma threshold now excludes the shadow on its own: a future artwork could
	// have a dark warm element here, and a wordmark that is not neutral is a redesign worth failing on.
	inkMaxChroma = 40

	// The moon is saturated yellow and nothing else on the page is. These bounds sit close around 255,255,1 so
	// that the parchment's warm specks — which are what a looser test let through — cannot pass.
	moonMinRed    = 230
	moonMinGreen  = 200
	moonMaxBlue   = 60
	moonMinSpread = 150 // how much more red than blue a pixel needs to count as the yellow
)

// Ink coverage that means the lift worked. Below the floor the letters were not found; above the ceiling something
// dark and large is in the band that should not be — a redesign, or a crop that slipped onto the artwork's own
// dark blocks.
const (
	minInkFraction = 0.04
	maxInkFraction = 0.45
)

// The card's colours. Black, as asked for; the letters in the same near-white the site's dark bars use, and the
// moon in the poster's own yellow, taken from the poster rather than named here.
var (
	cardBackground = color.RGBA{A: 0xff}
	cardInk        = color.RGBA{R: 0xfa, G: 0xfa, B: 0xfa, A: 0xff}
)

// Render draws the card at w×h from the event poster's bytes, as PNG.
//
// The marks are scaled to `fill` of the width and centred, which is what "taking up most of the space" comes to for
// a band that is roughly six times wider than it is tall: a wordmark cannot fill a 1.91:1 card vertically without
// being cropped or restacked, and either would be a redesign rather than a placeholder.
func Render(poster []byte, w, h int) ([]byte, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("sharecard: a %dx%d card makes no sense", w, h)
	}

	art, _, err := image.Decode(bytes.NewReader(poster))
	if err != nil {
		return nil, fmt.Errorf("sharecard: decoding the poster: %w", err)
	}

	marks, ink, err := lift(art)
	if err != nil {
		return nil, err
	}

	card := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(card, card.Bounds(), &image.Uniform{C: cardBackground}, image.Point{}, draw.Src)

	const fill = 0.92
	targetW := int(float64(w) * fill)
	targetH := marks.Bounds().Dy() * targetW / marks.Bounds().Dx()
	// A band taller than the card would mean the crop is wrong, not that the card should be cropped.
	if targetH > h {
		return nil, fmt.Errorf("sharecard: the marks are %dx%d and do not fit a %dx%d card", targetW, targetH, w, h)
	}
	at := image.Rect((w-targetW)/2, (h-targetH)/2, (w-targetW)/2+targetW, (h-targetH)/2+targetH)
	drawScaled(card, at, marks)

	_ = ink
	var out bytes.Buffer
	if err := png.Encode(&out, card); err != nil {
		return nil, fmt.Errorf("sharecard: encoding: %w", err)
	}
	return out.Bytes(), nil
}

// lift returns the moon and the wordmark on transparent pixels, plus the ink coverage it found.
func lift(art image.Image) (*image.RGBA, float64, error) {
	b := art.Bounds()
	band, ok := findBand(art)
	if !ok {
		return nil, 0, fmt.Errorf("sharecard: no wordmark found in the top %.0f%% of the poster",
			searchBottom*100)
	}
	_ = b

	out := image.NewRGBA(image.Rect(0, 0, band.Dx(), band.Dy()))
	inked := 0
	for y := 0; y < band.Dy(); y++ {
		for x := 0; x < band.Dx(); x++ {
			r16, g16, b16, _ := art.At(band.Min.X+x, band.Min.Y+y).RGBA()
			r, g, bl := uint8(r16>>8), uint8(g16>>8), uint8(b16>>8)

			switch {
			case isMoon(r, g, bl):
				out.SetRGBA(x, y, color.RGBA{R: r, G: g, B: bl, A: 0xff})
			case isInk(r, g, bl):
				out.SetRGBA(x, y, cardInk)
				inked++
			}
		}
	}

	coverage := float64(inked) / float64(band.Dx()*band.Dy())
	if coverage < minInkFraction || coverage > maxInkFraction {
		return nil, coverage, fmt.Errorf(
			"sharecard: the poster's top band is %.1f%% ink, outside %.0f–%.0f%% — the artwork has moved, so the "+
				"card would not say what it is supposed to say",
			coverage*100, minInkFraction*100, maxInkFraction*100)
	}
	return out, coverage, nil
}

// findBand locates the first horizontal band of marks in the top of the poster.
//
// A projection profile: count the moon-or-ink pixels in each row, take the first run of rows that are busy enough,
// stop at the first real gap, then do the same across the columns of that run. This is the standard way to find a
// line of text in a scan, and it is worth the twenty lines because the alternative — numbers somebody measured —
// is what put the wrong half of the letters on the card.
func findBand(art image.Image) (image.Rectangle, bool) {
	b := art.Bounds()
	limit := b.Min.Y + int(float64(b.Dy())*searchBottom)
	if limit > b.Max.Y {
		limit = b.Max.Y
	}

	rowNeeds := int(float64(b.Dx()) * rowInkFraction)
	gap := int(float64(b.Dy()) * bandGapFraction)
	if gap < 1 {
		gap = 1
	}

	marked := func(x, y int) bool {
		r16, g16, b16, _ := art.At(x, y).RGBA()
		r, g, bl := uint8(r16>>8), uint8(g16>>8), uint8(b16>>8)
		return isMoon(r, g, bl) || isInk(r, g, bl)
	}

	rowBusy := func(y int) bool {
		n := 0
		for x := b.Min.X; x < b.Max.X; x++ {
			if marked(x, y) {
				n++
				if n > rowNeeds {
					return true
				}
			}
		}
		return false
	}

	top := -1
	for y := b.Min.Y; y < limit; y++ {
		if rowBusy(y) {
			top = y
			break
		}
	}
	if top < 0 {
		return image.Rectangle{}, false
	}

	// Extend down, tolerating gaps smaller than a line break — the letters have internal counters and the moon has
	// a thin waist, so a row or two of nothing is normal inside one band.
	bottom, quiet := top, 0
	for y := top; y < limit; y++ {
		if rowBusy(y) {
			bottom, quiet = y, 0
			continue
		}
		if quiet++; quiet > gap {
			break
		}
	}

	colNeeds := int(float64(bottom-top+1) * colInkFraction)
	left, right := -1, -1
	for x := b.Min.X; x < b.Max.X; x++ {
		n := 0
		for y := top; y <= bottom; y++ {
			if marked(x, y) {
				n++
			}
		}
		if n <= colNeeds {
			continue
		}
		if left < 0 {
			left = x
		}
		right = x
	}
	if left < 0 {
		return image.Rectangle{}, false
	}

	pad := int(float64(b.Dy()) * bandPadFraction)
	return image.Rect(left-pad, top-pad, right+pad+1, bottom+pad+1).Intersect(b), true
}

// isInk is the wordmark: dark, and neutral rather than a shadow of something coloured.
func isInk(r, g, b uint8) bool {
	if luma(r, g, b) > inkMaxLuma {
		return false
	}
	lo, hi := r, r
	for _, v := range []uint8{g, b} {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return int(hi)-int(lo) <= inkMaxChroma
}

func isMoon(r, g, b uint8) bool {
	return r >= moonMinRed && g >= moonMinGreen && b <= moonMaxBlue && int(r)-int(b) >= moonMinSpread
}

// luma is the usual perceptual weighting, in 0–255.
func luma(r, g, b uint8) int {
	return (299*int(r) + 587*int(g) + 114*int(b)) / 1000
}

// drawScaled copies src into dst's rectangle, averaging the source pixels that fall in each destination pixel.
//
// Area-average rather than nearest-neighbour, because the marks are being reduced by a factor of two or more and
// nearest-neighbour on a hard-edged wordmark is visibly ragged. The same choice `internal/imaging.Fit` makes, and
// not shared with it because that one fits a whole image to an edge while this places one into a given rectangle —
// and it has to carry the alpha, which is how the parchment stays out of the card.
func drawScaled(dst *image.RGBA, at image.Rectangle, src *image.RGBA) {
	sb := src.Bounds()
	for y := at.Min.Y; y < at.Max.Y; y++ {
		sy0 := sb.Min.Y + (y-at.Min.Y)*sb.Dy()/at.Dy()
		sy1 := sb.Min.Y + (y-at.Min.Y+1)*sb.Dy()/at.Dy()
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := at.Min.X; x < at.Max.X; x++ {
			sx0 := sb.Min.X + (x-at.Min.X)*sb.Dx()/at.Dx()
			sx1 := sb.Min.X + (x-at.Min.X+1)*sb.Dx()/at.Dx()
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}

			var rs, gs, bs, as, n int
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					c := src.RGBAAt(sx, sy)
					// Premultiplied by the alpha, so a transparent source pixel contributes nothing to the colour
					// rather than contributing black — which is what left a grey halo round every letter when this
					// averaged the channels flat.
					a := int(c.A)
					rs += int(c.R) * a / 255
					gs += int(c.G) * a / 255
					bs += int(c.B) * a / 255
					as += a
					n++
				}
			}
			if n == 0 || as == 0 {
				continue
			}
			// Back out of the premultiplication, then blend onto whatever is already there.
			a := as / n
			over(dst, x, y, color.RGBA{
				R: uint8(rs * 255 / as),
				G: uint8(gs * 255 / as),
				B: uint8(bs * 255 / as),
				A: uint8(a),
			})
		}
	}
}

// over blends one colour onto the destination pixel.
func over(dst *image.RGBA, x, y int, c color.RGBA) {
	d := dst.RGBAAt(x, y)
	a := int(c.A)
	blend := func(s, d uint8) uint8 { return uint8((int(s)*a + int(d)*(255-a)) / 255) }
	dst.SetRGBA(x, y, color.RGBA{R: blend(c.R, d.R), G: blend(c.G, d.G), B: blend(c.B, d.B), A: 0xff})
}
