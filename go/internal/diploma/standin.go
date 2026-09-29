package diploma

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // the stand-in is a JPEG

	"nathejk.dk/internal/imaging"
)

// The stand-in photograph in the thumbnail (task 463).
//
// # What this is for
//
// The thumbnail on a patrol's public page is a picture *of* a diploma: one JPEG, the artwork scaled down, shared by
// every patrol and cached for an hour. The real document has a photograph in the middle of it and the thumbnail had
// an empty band there, so the preview did not look like the thing it previews.
//
// The maintainer asked for the gap to be filled: *"can we construct a start photo with no members that we can embed
// in the diploma thumbnails, then it would look more real"* — and then supplied the photograph, which is better than
// the answer that was being built. A first version drew the scene: a night gradient, a lit patch of ground, a
// vignette and some grain. It was deleted when this arrived, and the reason is worth keeping: a drawn night scene
// reads as a drawn night scene, and the thing that makes a preview look real is that it is real.
//
// # What the photograph is, and why it is safe to publish
//
// `assets/startphoto-standin.jpg` is the start line's backdrop with **nobody in front of it** — the banner, the
// moon, the grass. That is not a stylistic choice: a stand-in with people in it would be a photograph of children
// that no consent gate ever saw (PRD 011 §0b.2), served from an unauthenticated route, which is the thing this
// package's own comments spend two pages being careful about. This one names nobody and pictures nobody.
//
// # It must never reach the PDF
//
// The thumbnail is illustrative — one image for every patrol, saying "this is a diploma". The document is not. A
// patrol with no photograph either was not photographed or has a refusal recorded in hq's Fototilladelse, and
// printing the backdrop where their picture would go is inventing the photograph they declined. See `drawPhoto`,
// which says the same thing at the place somebody would change it.

// startPhotoStandIn is the empty backdrop, embedded.
//
// Supplied by the maintainer on 2026-09-28. Landscape, roughly 4:3, which is the photo box's own aspect — so it
// needs no crop today. `standInFor` crops anyway, because the next one might not be.
//
//go:embed assets/startphoto-standin.jpg
var startPhotoStandIn []byte

// StartPhotoStandIn returns the stand-in's bytes.
//
// Exported for the same reason `Background` is: one place to replace the asset, and a test can read what is shipped
// rather than a copy of it.
func StartPhotoStandIn() []byte { return startPhotoStandIn }

// standInFor decodes the stand-in and scales it to fill a box exactly.
//
// **Cover, not fit.** The box has a fixed aspect and a letterboxed photograph would put grey bars inside the
// artwork's own frame, which looks like a rendering fault rather than a picture. So the source is centre-cropped to
// the box's aspect first and scaled afterwards, which is what the CSS the public pages use for the same job does.
func standInFor(box image.Rectangle) (image.Image, error) {
	src, _, err := image.Decode(bytes.NewReader(startPhotoStandIn))
	if err != nil {
		return nil, fmt.Errorf("decoding the stand-in photograph: %w", err)
	}

	cropped := centreCrop(src, box.Dx(), box.Dy())
	// Scaled by the same area-average filter as every other image on the public surface, rather than a second one
	// of its own.
	scaled := imaging.Fit(cropped, max(box.Dx(), box.Dy()))

	// **`Fit` never enlarges**, so an asset smaller than the box comes back at its own size and would be drawn with
	// the artwork showing around it — which is how this failed the first time, with a photograph that looked nearly
	// right. A replacement asset that is too small is a wrong binary, so it says so here rather than rendering a
	// border nobody would attribute to the asset.
	if sb := scaled.Bounds(); sb.Dx() < box.Dx() || sb.Dy() < box.Dy() {
		return nil, fmt.Errorf(
			"the stand-in photograph is %dx%d after scaling and cannot fill the %dx%d box: it needs to be at "+
				"least that big in each direction", sb.Dx(), sb.Dy(), box.Dx(), box.Dy())
	}
	return scaled, nil
}

// centreCrop returns the largest centred rectangle of src with the given aspect ratio.
func centreCrop(src image.Image, aspectW, aspectH int) image.Image {
	b := src.Bounds()
	if aspectW <= 0 || aspectH <= 0 || b.Dx() <= 0 || b.Dy() <= 0 {
		return src
	}

	// Compare width·aspectH against height·aspectW to decide which dimension is the surplus one, in integers so
	// there is no rounding to reason about.
	var want image.Rectangle
	if b.Dx()*aspectH > b.Dy()*aspectW {
		w := b.Dy() * aspectW / aspectH
		x := b.Min.X + (b.Dx()-w)/2
		want = image.Rect(x, b.Min.Y, x+w, b.Max.Y)
	} else {
		h := b.Dx() * aspectH / aspectW
		y := b.Min.Y + (b.Dy()-h)/2
		want = image.Rect(b.Min.X, y, b.Max.X, y+h)
	}

	out := image.NewRGBA(image.Rect(0, 0, want.Dx(), want.Dy()))
	draw.Draw(out, out.Bounds(), src, want.Min, draw.Src)
	return out
}

// photoBoxIn is the photograph's rectangle inside an image of the whole page.
//
// Derived from the same millimetre constants the PDF places the photograph with, scaled to whatever resolution the
// artwork was rasterised at — so the two cannot drift.
func photoBoxIn(bounds image.Rectangle) image.Rectangle {
	w, h := float64(bounds.Dx()), float64(bounds.Dy())
	px := func(mm, pageMM, pixels float64) int { return int(mm / pageMM * pixels) }

	x0 := bounds.Min.X + px(photoXMM, pageWidthMM, w)
	y0 := bounds.Min.Y + px(photoYMM, pageHeightMM, h)
	return image.Rect(x0, y0,
		x0+px(photoWMM, pageWidthMM, w),
		y0+px(photoHMM, pageHeightMM, h))
}
