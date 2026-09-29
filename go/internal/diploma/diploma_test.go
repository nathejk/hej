package diploma

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The diploma renderer (task 345).
//
// Most of what matters here is wording and what the type *cannot* carry, so most of these tests avoid the PDF
// entirely — `sentences` exists so the Danish can be checked without reading bytes back out of a compressed
// page stream.

func finishedAt(h, m int) *time.Time {
	t := time.Date(2026, 9, 20, h, m, 0, 0, time.UTC)
	return &t
}

// **A photograph that cannot be drawn must not cost the diploma** (task 361).
//
// The bytes travel from a camera app through foto through a projection through an HTTP fetch, and any of those can
// deliver something unusable on a given day. fpdf makes this dangerous in a specific way: it **latches** an error
// and then refuses every later operation, so an unreadable image would silently take the name and the sentences
// with it and produce a blank page. Each case below asserts the document still contains its text.
func TestABrokenPhotographStillRendersTheDiploma(t *testing.T) {
	at := time.Date(2026, 9, 20, 3, 42, 0, 0, time.UTC)
	base := Diploma{Number: "42", Name: "Ørnene", Title: "Nathejk 2026", FinishedAt: &at}

	cases := []struct {
		name   string
		mutate func(d *Diploma)
	}{
		{"no photograph at all", func(d *Diploma) {}},
		{"a content type fpdf cannot place", func(d *Diploma) {
			d.Photo, d.PhotoContentType = []byte("\x00\x01binary"), "image/heic"
		}},
		{"an empty content type", func(d *Diploma) {
			d.Photo, d.PhotoContentType = []byte("\xff\xd8\xff"), ""
		}},
		{"bytes that are not the image they claim", func(d *Diploma) {
			d.Photo, d.PhotoContentType = []byte("this is not a JPEG at all"), "image/jpeg"
		}},
		{"a truncated JPEG", func(d *Diploma) {
			// A real JPEG header and nothing after it, which is what a cut-off fetch produces.
			d.Photo, d.PhotoContentType = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, "image/jpeg"
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := base
			tc.mutate(&d)

			var buf bytes.Buffer
			if err := PDF(d, &buf); err != nil {
				t.Fatalf("PDF: %v", err)
			}
			if buf.Len() < 300_000 {
				t.Errorf("the PDF is %d bytes — the artwork or the text is missing", buf.Len())
			}
			if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
				t.Error("not a PDF")
			}
		})
	}
}

// And a good photograph is actually embedded, rather than silently skipped — the failure the test above cannot
// see, since every one of its cases renders "fine".
func TestAGoodPhotographIsEmbedded(t *testing.T) {
	photo, err := os.ReadFile(filepath.Join("testdata", "patrol.jpg"))
	if err != nil {
		t.Skipf("no sample photograph: %v", err)
	}

	at := time.Date(2026, 9, 20, 3, 42, 0, 0, time.UTC)
	d := Diploma{Number: "42", Name: "Ørnene", Title: "Nathejk 2026", FinishedAt: &at}

	var without, with bytes.Buffer
	if err := PDF(d, &without); err != nil {
		t.Fatalf("PDF without: %v", err)
	}
	d.Photo, d.PhotoContentType = photo, "image/jpeg"
	if err := PDF(d, &with); err != nil {
		t.Fatalf("PDF with: %v", err)
	}

	// The photograph is ~400 kB; anything less than a clear margin means it did not go in.
	if with.Len()-without.Len() < 100_000 {
		t.Errorf("with a photograph the PDF grew by only %d bytes — it was skipped", with.Len()-without.Len())
	}
}

// The content-type gate is an allow-list, and a HEIC is the case it exists for: that is what an *original* from a
// phone is, and originals are never served (see nathejk/table/patrolphoto). This is the second fence.
func TestTheContentTypeGateAcceptsOnlyWhatFpdfCanPlace(t *testing.T) {
	for contentType, want := range map[string]string{
		"image/jpeg":                 "JPG",
		"image/jpg":                  "JPG",
		"IMAGE/JPEG":                 "JPG",
		"image/jpeg; charset=binary": "JPG",
		"image/png":                  "PNG",
		"image/gif":                  "GIF",
		"image/heic":                 "",
		"image/webp":                 "",
		"application/pdf":            "",
		"":                           "",
	} {
		if got := fpdfImageType(contentType); got != want {
			t.Errorf("fpdfImageType(%q) = %q, want %q", contentType, got, want)
		}
	}
}

// **The type carries a photograph, and that was a decision rather than a drift.**
//
// This test used to fail the moment a photograph field appeared, on the grounds recorded below. On 2026-09-21 the
// maintainer instructed the opposite — *"the diploma should carry start photo, like it did in the diplom-repo"*,
// and *"include cover photo in own blob store"* — so the field exists and this test's job changed.
//
// The old reasoning, kept because it is the thing a future reader needs to weigh: a diploma is reachable at an
// unauthenticated URL addressed by a patrol number, so a photograph on it publishes eight children's faces, which
// is a stronger identifier than any of the names task 337 is careful about (PRD 011 §0b.2).
//
// What makes it acceptable now, and it is worth being precise: the picture is the patrol's **cover**, which an
// organizer selects in hq when there is more than one. That selection is a human curation step, which is exactly
// the consent gate §0b.2 asked for — and `patrolphoto` returns no photograph at all for a patrol whose
// Fototilladelse records a refusal. The gate is upstream, in the tool where somebody can see the picture, rather
// than here where nothing can.
//
// **What this test still guards is a person.** No name, no phone number, no email, no uploader, no photographer
// — the fields that would make this document about an individual rather than about a patrol. That rule did not
// move, and the photograph arriving is the reason to state it again.
func TestADiplomaCannotCarryAPerson(t *testing.T) {
	allowed := map[string]bool{
		"Number": true, "Name": true, "Title": true, "Route": true, "FinishedAt": true,
		// The patrol's photograph, as bytes and a content type. Bytes rather than a ref or a URL so this package
		// still renders without a store or a network — see the field's doc.
		"Photo": true, "PhotoContentType": true,
	}

	typ := reflect.TypeOf(Diploma{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if !allowed[name] {
			t.Errorf("Diploma has a new field %q. If it names a person — a name, a phone number, an email, a "+
				"photographer — read the package doc before adding it: this surface is unauthenticated.", name)
		}
	}

	// And the name field is the *patrol's*, which is the distinction the whole surface rests on. Asserted here so
	// the allow-list above cannot quietly be read as permission for a person's name.
	if _, ok := typ.FieldByName("PersonName"); ok {
		t.Error("a diploma must not name a person")
	}
}

func TestTheFinishedWordingNamesTheEventAndTheMinute(t *testing.T) {
	got := sentences(Diploma{
		Name:       "Ørnene",
		Title:      "Nathejk 2026",
		FinishedAt: finishedAt(3, 42),
	})

	joined := strings.Join(got, " | ")
	for _, want := range []string{"har gennemført Nathejk 2026", "og gik i mål lørdag nat kl. 03:42"} {
		if !strings.Contains(joined, want) {
			t.Errorf("want %q in the diploma, got %q", want, joined)
		}
	}
	// The minute is zero-padded: "kl. 3:42" on a certificate looks like a typo.
	if strings.Contains(joined, "kl. 3:42") {
		t.Errorf("the clock must be zero-padded, got %q", joined)
	}
}

// The route line is printed **only** when configured. `diplom` hardcoded 2024's start and destination, which is
// how a diploma ends up naming last year's villages.
func TestTheRouteLineIsOmittedWhenUnset(t *testing.T) {
	without := strings.Join(sentences(Diploma{Title: "Nathejk 2026", FinishedAt: finishedAt(2, 5)}), " | ")
	if strings.Contains(without, "fra") {
		t.Errorf("an unset route must print no route line, got %q", without)
	}

	with := strings.Join(sentences(Diploma{
		Title: "Nathejk 2026", Route: "fra Lundby til Glumsø", FinishedAt: finishedAt(2, 5),
	}), " | ")
	if !strings.Contains(with, "fra Lundby til Glumsø") {
		t.Errorf("a configured route must be printed, got %q", with)
	}
}

// A diploma with no finish says "deltog i", not "har gennemført" — one of the two wordings the maintainer named
// on 2026-09-21, and since task 360 the one a backstop-opened patrol actually receives. It was a defensive test
// when the handler withheld such diplomas; it is now a test of shipped behaviour.
func TestNoFinishMeansParticipationWording(t *testing.T) {
	got := strings.Join(sentences(Diploma{Title: "Nathejk 2026"}), " | ")

	if strings.Contains(got, "gennemført") {
		t.Errorf("a patrol with no finish must not be told it completed the event: %q", got)
	}
	if !strings.Contains(got, "deltog i Nathejk 2026") {
		t.Errorf("want the participation wording, got %q", got)
	}
}

// An empty title still produces a sentence. A diploma reading "har gennemført !" would be worse than one
// reading "Nathejk".
func TestAMissingTitleFallsBackRatherThanRenderingAGap(t *testing.T) {
	got := strings.Join(sentences(Diploma{FinishedAt: finishedAt(1, 0)}), " | ")
	if !strings.Contains(got, "har gennemført Nathejk") {
		t.Errorf("want a fallback title, got %q", got)
	}
}

func TestPDFRendersAnA4Document(t *testing.T) {
	var buf bytes.Buffer
	err := PDF(Diploma{
		Number: "42", Name: "Ørnene", Title: "Nathejk 2026", FinishedAt: finishedAt(3, 42),
	}, &buf)
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}

	out := buf.Bytes()
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("not a PDF: % x", out[:min(8, len(out))])
	}
	// A4 portrait in points, which is what fpdf writes into the MediaBox for "P"/"A4".
	if !bytes.Contains(out, []byte("595.28 841.89")) {
		t.Error("want an A4 portrait MediaBox")
	}
	// The background is a ~1.4 MB JPEG, so anything much smaller means it did not make it in.
	if len(out) < 700_000 {
		t.Errorf("the PDF is %d bytes; the artwork appears to be missing", len(out))
	}
}

// The embedded artwork must actually be decodable, at the aspect ratio the page is drawn at. A corrupt or
// replaced asset would otherwise surface as a blank diploma at the worst possible moment.
func TestTheBackgroundIsADecodableA4Image(t *testing.T) {
	img, format, err := image.Decode(bytes.NewReader(Background()))
	if err != nil {
		t.Fatalf("decoding the background: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("format = %q, want jpeg", format)
	}

	b := img.Bounds()
	ratio := float64(b.Dy()) / float64(b.Dx())
	// A4 is 1:1.414. Allow a little slack for an artwork that is trimmed differently.
	if ratio < 1.35 || ratio > 1.48 {
		t.Errorf("the artwork is %dx%d (ratio %.3f); A4 portrait is ~1.414", b.Dx(), b.Dy(), ratio)
	}

	// **And it has to be print resolution.** A diploma is a thing people print and put on a fridge, and the one
	// way to get that wrong invisibly is to drop in a screen-sized image: it renders fine on a phone and comes
	// out of a printer soft. 3000 px on the long edge is ~260 dpi on A4; 2026's artwork is 3508, i.e. 300 dpi
	// exactly (see go/scripts/render-diploma-background.sh). The mock this replaced was 150 dpi and would have
	// failed this.
	if b.Dy() < 3000 {
		t.Errorf("the artwork is %dx%d; want at least 3000 px on the long edge for print", b.Dx(), b.Dy())
	}
}

func TestThumbnailIsAJPEGOfTheRequestedSize(t *testing.T) {
	out, err := Thumbnail(600)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decoding the thumbnail: %v", err)
	}
	if got := img.Bounds().Dy(); got != 600 {
		t.Errorf("longest side = %d, want 600", got)
	}
	// Small enough to be worth serving to a phone: the point of a thumbnail.
	if len(out) > 200_000 {
		t.Errorf("thumbnail is %d bytes, which is not a thumbnail", len(out))
	}
}

// The thumbnail carries the stand-in photograph, in the box the PDF puts a photograph in (task 463).
//
// # Why this is worth asserting
//
// The thumbnail was a scaled copy of the artwork, which left the photograph's band empty — the maintainer's report
// was that it did not look like a diploma. The fix is a composite, and a composite has two ways to go wrong
// silently: the picture can be absent (a decode that failed, an embed that was dropped) and it can be in the wrong
// place (the geometry drifting from the PDF's). Both produce a perfectly valid JPEG.
//
// So this compares the thumbnail against the **bare artwork**: inside the photograph's box they must differ, and
// well outside it they must not.
func TestTheThumbnailCarriesTheStandInWhereThePDFPutsThePhotograph(t *testing.T) {
	out, err := Thumbnail(600)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	thumb, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decoding the thumbnail: %v", err)
	}

	bare, _, err := image.Decode(bytes.NewReader(Background()))
	if err != nil {
		t.Fatalf("decoding the artwork: %v", err)
	}

	tb := thumb.Bounds()
	box := photoBoxIn(tb)
	if box.Dx() < 20 || box.Dy() < 15 {
		t.Fatalf("the photo box is %v in a %v thumbnail, which cannot be right", box, tb)
	}

	// Scale a point in the thumbnail to the same relative point in the full-size artwork.
	sample := func(x, y int) (uint32, uint32, uint32) {
		bb := bare.Bounds()
		r, g, b, _ := bare.At(
			bb.Min.X+(x-tb.Min.X)*bb.Dx()/tb.Dx(),
			bb.Min.Y+(y-tb.Min.Y)*bb.Dy()/tb.Dy()).RGBA()
		return r >> 8, g >> 8, b >> 8
	}
	differs := func(x, y int) bool {
		tr, tg, tbl, _ := thumb.At(x, y).RGBA()
		ar, ag, abl := sample(x, y)
		d := abs(int(tr>>8)-int(ar)) + abs(int(tg>>8)-int(ag)) + abs(int(tbl>>8)-int(abl))
		// Generous: both images are JPEGs and the thumbnail has been resampled, so equal pixels are never bitwise
		// equal. A composited photograph differs by far more than this.
		return d > 60
	}

	// The middle of the box is the stand-in.
	cx, cy := (box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2
	if !differs(cx, cy) {
		t.Error("the middle of the photograph's box is unchanged from the artwork: the stand-in was not drawn")
	}

	// A margin inside the box, to catch a composite that landed a few pixels off rather than in the box.
	for _, p := range [][2]int{
		{box.Min.X + box.Dx()/8, box.Min.Y + box.Dy()/8},
		{box.Max.X - box.Dx()/8, box.Max.Y - box.Dy()/8},
	} {
		if !differs(p[0], p[1]) {
			t.Errorf("%v is inside the box and unchanged: the stand-in is not filling it", p)
		}
	}

	// And the artwork's own headline, well above the box, is untouched — a composite drawn at the wrong offset
	// would show up here.
	if differs(tb.Min.X+tb.Dx()/2, tb.Min.Y+tb.Dy()/20) {
		t.Error("the top of the artwork changed: the stand-in is being drawn in the wrong place")
	}
}

// The box the thumbnail fills is the box the PDF places a photograph in.
//
// One set of millimetres, two renderers. Asserted as fractions of the page rather than in pixels, because that is
// the form the two share — and a drift here is the failure a reader would only notice by holding the thumbnail next
// to the certificate.
func TestTheThumbnailsPhotoBoxMatchesThePDFs(t *testing.T) {
	// A page-shaped rectangle at an arbitrary resolution, to check the mapping rather than one raster.
	box := photoBoxIn(image.Rect(0, 0, 2100, 2970))

	for _, want := range []struct {
		name         string
		got          float64
		wantFraction float64
	}{
		{"left", float64(box.Min.X) / 2100, photoXMM / pageWidthMM},
		{"top", float64(box.Min.Y) / 2970, photoYMM / pageHeightMM},
		{"width", float64(box.Dx()) / 2100, photoWMM / pageWidthMM},
		{"height", float64(box.Dy()) / 2970, photoHMM / pageHeightMM},
	} {
		if diff := want.got - want.wantFraction; diff > 0.002 || diff < -0.002 {
			t.Errorf("%s = %.4f of the page, want %.4f (%.0fmm of %.0fmm)",
				want.name, want.got, want.wantFraction, photoWMM, pageWidthMM)
		}
	}
}

// **The stand-in never reaches a PDF.**
//
// The rule, and the reason it is a test rather than a comment: a patrol with no photograph either was not
// photographed or has a refusal recorded in hq's Fototilladelse, and filling their certificate's empty box with the
// backdrop would be inventing the photograph they declined. It is also the obvious "improvement" somebody would
// make after seeing that the thumbnail has one.
func TestTheStandInNeverReachesThePDF(t *testing.T) {
	var out bytes.Buffer
	if err := PDF(Diploma{Number: "42", Name: "Ørnene", Title: "Nathejk 2026"}, &out); err != nil {
		t.Fatalf("PDF: %v", err)
	}

	// A JPEG is embedded in a PDF as its own bytes (DCTDecode passes them through), so a chunk from the middle of
	// the asset is findable if it was placed. Taken from the middle rather than the start: JPEG headers are not
	// distinctive enough to prove which image it is.
	asset := StartPhotoStandIn()
	if len(asset) < 4096 {
		t.Fatalf("the stand-in asset is %d bytes, which is not the photograph", len(asset))
	}
	needle := asset[len(asset)/2 : len(asset)/2+512]
	if bytes.Contains(out.Bytes(), needle) {
		t.Error("the stand-in photograph is embedded in a diploma with no photograph — a certificate must not " +
			"carry a picture the patrol did not have taken, or declined")
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
