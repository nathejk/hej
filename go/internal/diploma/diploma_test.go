package diploma

import (
	"bytes"
	"image"
	"image/jpeg"
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

// **The type has nowhere to put a photograph, and that is the point.**
//
// `diplom`'s version places a `natpas` portrait of the patrol in the middle of the page. On an unauthenticated
// surface that would publish eight children's faces through no consent gate (PRD 011 §0b.2), which is a
// stronger identifier than any of the names task 337 is careful about.
//
// So this test fails the moment somebody adds the field back — which is the only way a rule like this survives
// a year, since the obvious "improvement" to a diploma is a picture on it.
func TestADiplomaCannotCarryAPhotographOrAPerson(t *testing.T) {
	allowed := map[string]bool{
		"Number": true, "Name": true, "Title": true, "Route": true, "FinishedAt": true,
	}

	typ := reflect.TypeOf(Diploma{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if !allowed[name] {
			t.Errorf("Diploma has a new field %q. If it carries a photograph, a person's name or a phone "+
				"number, read the package doc before adding it: this surface is unauthenticated.", name)
		}
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
