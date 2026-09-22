// Package diploma renders a patrol's diploma (PRD 011 §6, §11 Q6; task 345).
//
// # Why this lives in hej at all
//
// There is a sibling service, `diplom`, that has produced these since 2024, and PRD 011 §8 originally leaned
// on linking a visitor's browser to it — a link is not a service-to-service call, so the org's architecture
// rule held. The maintainer decided otherwise on 2026-09-21: *"the diploma generation logic should be moved
// here"*. So this is §11 Q6 option (c), and the reasoning behind the choice is worth keeping: the diploma is
// now one element on a page this repo already renders, gated by a verdict this repo already computes, from a
// finish time this repo already has. Linking out would have meant a second origin, a second deployment and a
// year hardcoded in another codebase — for a PDF that is a background image and four lines of text.
//
// # The photograph, and the decision that put it back
//
// `diplom`'s version places a photograph of the patrol in the middle of the page, and this package refused to —
// at length, because the reasoning is not obvious and the field is the obvious "improvement" somebody would add.
// The maintainer reversed it on 2026-09-21: *"the diploma should carry start photo, like it did in the
// diplom-repo"*, with the bytes held in this app's own blob store because the same pictures will feed a public
// gallery shortly.
//
// The refusal is kept here, because a future reader weighing a third change needs both halves:
//
//   - This surface is **unauthenticated**. `diplom`'s diplomas were reached by people who knew the link; these
//     sit behind a patrol number anybody can type (PRD 011 §11 Q2).
//   - PRD 011 §0b.2 settled that photographs are publishable only where **consent was obtained upstream**, by a
//     curator, at the point a photograph enters an album.
//   - The public surface **names no person** (§0b.1, task 337), and a photograph of eight children's faces is a
//     stronger identifier than any name we are careful about elsewhere.
//
// What answers the second point is *which* photograph this is. It is the patrol's **cover**, which an organizer
// selects in hq when a patrol has more than one — a human curation step, in a tool where somebody can see the
// picture. Where nobody has chosen, `nathejk/table/patrolphoto` picks the newest start photograph and skips
// anything the crew flagged for review. So the consent gate is upstream, where it can be exercised, rather than
// here where nothing can see what it is deciding about.
//
// The first and third points are accepted rather than solved, and that is the maintainer's call to make. What
// this package still refuses is a **person**: no name, no phone, no email, no photographer credit. A test walks
// the type and fails on anything person-shaped.
//
// The middle of the page was left empty for this while the artwork was a mock, so the layout needed no change.
//
// # The background
//
// `assets/background-2026.jpg` is **2026's artwork**, supplied by the maintainer on 2026-09-21 as a
// print-ready A4 PDF and rasterised to 300 dpi by `go/scripts/render-diploma-background.sh`. The vector source
// is kept beside it in `assets/source/`, because it is the thing that gets edited; the JPEG is a derivative and
// the script says how to remake it.
//
// It replaced the 2024 poster that stood in as a visible mock until then. Note what it is: **the event poster**,
// which carries the wordmark, the year and "Vi ses i mørket!" — it is not a diploma-specific design and the word
// *Diplom* appears nowhere on it. The rendered text therefore has to say what the page is, and the empty band
// it prints into runs roughly 128-244 mm down the page, between the artwork's own two blocks.
//
// See ReplaceBeforeLaunch for what is still outstanding.
//
// # Why a package rather than a handler
//
// Rendering is a pure function of a patrol's facts plus an image, which is worth testing without a database,
// a request or a broker — the same reasoning that put `internal/distance` and `internal/imaging` here.
package diploma

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"

	"nathejk.dk/internal/imaging"

	_ "embed"
)

// ReplaceBeforeLaunch is what still has to happen before this is shown to a family for real.
//
// A constant rather than a comment so it appears in `go doc` and in a grep for "launch". **The artwork is no
// longer on the list** — 2026's design landed on 2026-09-21 — and what is left is deliberately small:
//
//   - **The route line.** `diplom` hardcodes "fra Lundby til Glumsø" for 2024. This renders it only when
//     configured, and nothing configures it yet, so the line is omitted rather than inventing places (see
//     Diploma.Route). 2026's start and finish are not written down anywhere this package can reach.
//   - **A check that the text belongs on this artwork.** The background is the event *poster*: it says NATHEJK
//     2026 and "Vi ses i mørket!", not "Diplom". The name and the sentence block print into the empty band
//     between those two, which fits — measured on a rendered sample, not assumed — but whether a poster is what
//     a diploma should look like is the maintainer's call, not this package's.
//
// The headline font question is **closed**: `diplom` embeds `impact.ttf`, whose redistribution is restricted, and
// this renders in fpdf's built-in Helvetica. The artwork carries its own headline as outlines, so nothing here
// needs Impact.
const ReplaceBeforeLaunch = "route line, and whether a poster is the right diploma"

//go:embed assets/background-2026.jpg
var background []byte

// impactFont is the headline face the diplomas have used since 2024.
//
// # The licence question, answered (2026-09-21)
//
// This was on ReplaceBeforeLaunch, because Impact ships with Windows and Microsoft's licence restricts
// *redistributing the font*. The maintainer settled it:
//
//	"Microsoft explicitly treats embedding a Windows font in a document — such as a PDF — as a permitted
//	 special case, and says there are generally no special restrictions on distributing those documents."
//
// That distinction is the whole answer, and it is worth keeping because it is easy to get backwards: what is
// restricted is shipping `impact.ttf` as a font for others to install. **Embedding a subset in a document** is
// the permitted case, and a diploma is a document. Note what this therefore does *not* license: serving this
// file over HTTP, or using it as a webfont on the public site — the pages use the `font-nathejk` stack for that
// reason, naming Impact as a *local* family rather than delivering it.
//
// fpdf embeds only the glyphs used, so a diploma carries a few dozen characters of it.
//
//go:embed assets/impact.ttf
var impactFont []byte

// Diploma is everything a diploma says.
//
// Note what it cannot hold: no person, no photograph, no phone number. The type is the enforcement, the same
// way `patroltrack.Segment` has nowhere to put a person id.
type Diploma struct {
	// Number is the patrol's number, as the public page addresses it.
	Number string

	// Name is the patrol's own name — "Ørnene". A *patrol's* name, never a person's.
	Name string

	// Title is the event as it is written on the page: "Nathejk 2026".
	Title string

	// Route is "fra Lundby til Glumsø", or empty to omit the line.
	//
	// Empty by default and omitted rather than guessed: `diplom` hardcoded 2024's start and destination, and
	// a diploma naming the wrong places is worse than one naming none.
	Route string

	// FinishedAt is when the patrol crossed the line, in the event's own timezone, or nil.
	//
	// Nil renders the participation wording instead of the finish wording — `diplom` does the same, and since
	// task 360 so does this app: a patrol whose page is open but who never reached the finish gets a diploma
	// saying "deltog i". So **both branches are reachable from the public page**, which they were not when the
	// handler withheld the document instead (task 346).
	//
	// Nil is precisely "nobody from this patrol reached the finish": the gate reads the patrol's scan at the
	// last checkgroup, and a checkpoint scans the patrol rather than its members.
	FinishedAt *time.Time

	// Photo is the patrol's photograph, as image bytes, or nil for none.
	//
	// # Bytes rather than a ref or a URL
	//
	// This package renders; it does not fetch. A ref would make it reach into a blob store, and a URL would make
	// it make an HTTP request while drawing a PDF — both turn a pure function of a patrol's facts into something
	// that needs a network and a filesystem to test. The handler resolves the photograph (task 361) and hands the
	// bytes over.
	//
	// Nil is an ordinary state: not every patrol is photographed, foto may be unreachable, and the diploma is
	// worth printing either way — so nothing here treats a missing picture as an error.
	Photo []byte

	// PhotoContentType is the format of Photo, e.g. "image/jpeg".
	//
	// Carried rather than sniffed because fpdf needs to be told, and the projection already recorded what foto
	// re-encoded to. An unsupported value means the photograph is skipped, not that the render fails.
	PhotoContentType string
}

// Background returns the image the diploma is drawn on.
//
// A function rather than a field so a caller cannot forget it, and so replacing the artwork is a change in one
// place — which is exactly how 2026's design replaced the mock: one embed line and one asset.
func Background() []byte { return background }

// PDF renders the diploma as an A4 portrait PDF.
//
// # Millimetres, and why the numbers look arbitrary
//
// They belong to the **artwork**, not to any layout logic: the background is a full-page bleed and everything
// else has to sit in the gap the image leaves — which on both the 2024 and 2026 posters is the band from roughly
// y=128 to y=250, between the wordmark block and "Vi ses i mørket!".
//
// The geometry is `diplom`'s, restored (task 361). While the page carried no photograph the text sat higher, in
// the middle of the empty band; adding the photograph back at `diplom`'s coordinates put the box *underneath*
// text that had moved up into it, and the sample showed three lines printed across a patrol's faces. So the
// numbers are one set again: photograph, then name, then sentences, in the order they are drawn.
//
// **One geometry whether or not there is a photograph**, as `diplom` had it. A patrol nobody photographed gets
// whitespace where the picture would be rather than a second layout to maintain — and a certificate with room
// above the name reads as a certificate, not as a mistake.
func PDF(d Diploma, w io.Writer) error {
	const (
		pageWidthMM  = 210.0
		pageHeightMM = 297.0

		// The patrol's name, centred across the page, below the photograph's box.
		nameY        = 210.0
		nameFontSize = 28.0

		// The sentence block beneath it. 224 leaves the name's 28pt line room to breathe and still ends the
		// third sentence above the artwork's bottom band — measured on a sample, not calculated.
		textY        = 224.0
		textFontSize = 14.0
		lineHeight   = 7.0
		sideMarginMM = 30.0
	)

	// headlineFont is the family name registered with fpdf, not a file name.
	const headlineFont = "impact"

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(fmt.Sprintf("Nathejk diplom \u2014 patrulje %s", d.Number), true)
	// No author or creator beyond this: a PDF's metadata is a place personal data hides, and the default would
	// name the library rather than a person — but being explicit costs nothing and means nobody has to check.
	pdf.SetAuthor("Nathejk", true)
	pdf.AddPage()

	// The background, as a registered image rather than a file path: `diplom` reads from disk
	// (`/app/assets/...`), which makes the binary depend on a mount being present. Embedded bytes cannot go
	// missing in a deploy.
	pdf.RegisterImageOptionsReader("background", fpdf.ImageOptions{ImageType: "JPG"},
		bytes.NewReader(Background()))
	pdf.ImageOptions("background", 0, 0, pageWidthMM, pageHeightMM, false,
		fpdf.ImageOptions{ImageType: "JPG"}, 0, "")

	// The patrol's photograph, above the name.
	//
	// `diplom`'s geometry, kept deliberately: 100×75 mm at (55, 130) — centred, 4:3 landscape, sitting in the
	// clear middle of the artwork with the name and the sentences beneath it. Reusing the numbers means a family
	// comparing this year's diploma with 2024's sees the same document rather than a redesign.
	//
	// **Drawn before the text**, so if a photograph is ever taller than its box the text is on top of it rather
	// than under it. fpdf scales to fit the given rectangle, so a portrait photograph is letterboxed inside it
	// rather than overflowing — which is why the box is fixed and the image is not measured here.
	drawPhoto(pdf, d)

	// **Impact, embedded** — the face these diplomas have used since 2024, and the same one the public pages'
	// `font-nathejk` names. See impactFont for why embedding it is licensed where redistributing it is not.
	//
	// A UTF-8 font, so this text is **not** latin1-encoded: fpdf writes the glyphs it needs into the PDF and a
	// patrol called Ørnene renders from the string as it is. The body text below stays on a core font and still
	// needs the conversion, which is why both paths exist — mixing them up prints either mojibake or boxes, and
	// only a rendered sample shows which.
	pdf.AddUTF8FontFromBytes(headlineFont, "", impactFont)
	pdf.SetFont(headlineFont, "", nameFontSize)
	pdf.SetXY(sideMarginMM, nameY)
	pdf.MultiCell(pageWidthMM-2*sideMarginMM, 12, headlineSafe(d.Name), "", "C", false)

	pdf.SetFont("Helvetica", "", textFontSize)
	pdf.SetY(textY)
	for _, line := range sentences(d) {
		pdf.SetX(sideMarginMM)
		pdf.MultiCell(pageWidthMM-2*sideMarginMM, lineHeight, latin1(line), "", "C", false)
	}

	return pdf.Output(w)
}

// headlineSafe folds a string to characters the embedded headline font can draw.
//
// # Why this exists, and what it prevents
//
// fpdf does not degrade on a glyph a UTF-8 font lacks — it **fails the render**: `character outside the
// supported range: 🦅`. Patrol names are free text typed by teenagers, and an emoji in one is not exotic. So
// without this, one patrol's choice of name means one patrol gets no diploma at all, with a 500 on a public
// page as the only symptom.
//
// Found by a test that existed for exactly this reason on the Helvetica path (`latin1` degrades), and which
// started failing the moment the font changed. It is the second time this file's error handling has been caught
// by a test written about a *different* rendering path.
//
// The fold is Latin-1's repertoire, which Impact covers completely and which contains every character Danish
// needs. Anything else becomes a question mark — visible, so somebody can fix the name, and not a box.
func headlineSafe(s string) string {
	s = typography.Replace(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// Latin-1 is exactly U+0020..U+00FF here: the repertoire Impact covers, minus the control range, which
		// a PDF viewer would draw as a box even when the font has a slot for it.
		if r >= 0x20 && r <= 0xFF {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('?')
	}
	return b.String()
}

// drawPhoto places the patrol's photograph, or draws nothing.
//
// # Why a missing or broken photograph must not fail the render
//
// The bytes come from another service, through a projection, from a camera app. Every one of those can be absent
// or wrong on a given day, and none of it is a reason to deny a patrol its certificate — the photograph is the
// one element of this document that is decoration. So every failure here is a silent skip:
//
//   - no bytes at all (not photographed, or foto unreachable),
//   - a content type fpdf cannot place,
//   - bytes that are not the image they claim to be, which `RegisterImageOptionsReader` reports through the
//     PDF's error state rather than a return value.
//
// That last one is why the error state is cleared afterwards: fpdf latches an error and refuses every later
// operation, so an unreadable photograph would otherwise take the whole diploma — text and all — with it. Found
// by feeding it a JPEG-labelled string of nonsense, which is exactly what a truncated fetch produces.
//
// Clearing it means `ClearError`. See the note at the call site: `SetError(nil)` is silently a no-op, and using it
// here made this function look like it handled the case while doing nothing at all.
func drawPhoto(pdf *fpdf.Fpdf, d Diploma) {
	const (
		photoX = 55.0
		photoY = 130.0
		photoW = 100.0
		photoH = 75.0
	)

	if len(d.Photo) == 0 {
		return
	}
	imageType := fpdfImageType(d.PhotoContentType)
	if imageType == "" {
		return
	}

	opts := fpdf.ImageOptions{ImageType: imageType}
	pdf.RegisterImageOptionsReader("patrolphoto", opts, bytes.NewReader(d.Photo))
	if pdf.Err() {
		// Unreadable bytes. **`ClearError`, not `SetError(nil)`** — fpdf's `SetError` only ever *sets*, and only
		// when no error is latched yet, so passing nil is a no-op. The first version of this function did exactly
		// that and looked correct: the check ran, the "clear" did nothing, and `Output` returned "invalid JPEG
		// format: missing SOI marker" — a 500 on a public route instead of a certificate. Caught by the test
		// feeding it bytes that are not the image they claim, which is what a truncated fetch produces.
		pdf.ClearError()
		return
	}

	pdf.ImageOptions("patrolphoto", photoX, photoY, photoW, photoH, false, opts, 0, "")
	if pdf.Err() {
		pdf.ClearError()
	}
}

// fpdfImageType maps a content type to what fpdf calls the format, or "" for one it cannot place.
//
// A short allow-list rather than a guess: fpdf supports JPEG, PNG and GIF, and foto re-encodes its display
// renditions to JPEG. An unknown type skips the photograph instead of handing fpdf something it will latch an
// error on — and a HEIC from a phone, which is what an *original* would be, falls out here rather than being
// attempted. Originals are never served anyway (see nathejk/table/patrolphoto), so this is a second fence.
func fpdfImageType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0])) {
	case "image/jpeg", "image/jpg":
		return "JPG"
	case "image/png":
		return "PNG"
	case "image/gif":
		return "GIF"
	default:
		return ""
	}
}

// latin1 re-encodes a Danish string for fpdf's built-in fonts.
//
// # Why this is still needed
//
// fpdf's core fonts (Helvetica and friends) are single-byte, so handing them UTF-8 prints mojibake: "Ørnene"
// arrives as "Ã˜rnene". `diplom` solved it the same way — its `utf8_decode` is this function — and dropping it
// was the first thing a rendered sample caught, which is why a sample gets looked at rather than trusted.
//
// Latin-1 covers every character Danish needs (æ, ø, å and their capitals). Anything outside it becomes a
// replacement character rather than failing the render: a patrol name we cannot spell is a blemish on one
// diploma, while an error is no diploma at all.
//
// **Only the body text goes through this now.** The headline is set in embedded Impact, which is a UTF-8 font
// and takes the string as it is — so the two paths must not be confused. Putting a latin1 string into the UTF-8
// font prints boxes; putting a UTF-8 string into Helvetica prints mojibake. The body could move to an embedded
// font too, but Helvetica is the right face for it and this function is five lines.
func latin1(s string) string {
	// Typographic punctuation first. Latin-1 has no em dash, curly quote or ellipsis, and the encoder's
	// substitute for an unmappable rune is 0x1A — a **control** character, which a PDF viewer draws as a box or
	// as nothing. A patrol called “Rævene” would have printed wearing two boxes. Folding to the ASCII cousin
	// keeps the text readable, which matters more here than the typography.
	s = typography.Replace(s)

	encoder := encoding.ReplaceUnsupported(charmap.ISO8859_1.NewEncoder())
	out, err := encoder.String(s)
	if err != nil {
		// Unreachable with ReplaceUnsupported, and if it ever is reached the original string is a better
		// answer than an empty one.
		return s
	}
	return out
}

// typography folds the punctuation a copy-pasted name tends to carry into what Latin-1 can express.
var typography = strings.NewReplacer(
	"\u2014", "-", // em dash
	"\u2013", "-", // en dash
	"\u2018", "'", "\u2019", "'", // curly single quotes
	"\u201c", `"`, "\u201d", `"`, // curly double quotes
	"\u2026", "...", // ellipsis
	"\u00a0", " ", // non-breaking space
)

// sentences is what the diploma says, in Danish.
//
// Separated from the drawing so the wording can be tested without producing a PDF and reading bytes back out
// of it — the same reason `distance.Label` is its own function.
//
// **No time on the page beyond the clock.** `diplom` prints "og gik i mål lørdag nat kl. 03:42", which is the
// nicest sentence in the whole feature: the *day* is fixed by the event and the minute is the patrol's own.
// Kept as it was, including "lørdag nat", because that is what a night race's finish is called and the date
// would be pedantic on a certificate.
func sentences(d Diploma) []string {
	title := d.Title
	if title == "" {
		title = "Nathejk"
	}

	if d.FinishedAt == nil {
		// The participation wording, and since task 360 a normal path rather than a defensive one: a patrol
		// whose page the backstop opened gets this sentence. It must never imply a finish.
		if d.Route != "" {
			return []string{fmt.Sprintf("deltog i %s %s!", title, d.Route)}
		}
		return []string{fmt.Sprintf("deltog i %s!", title)}
	}

	out := []string{fmt.Sprintf("har gennemført %s", title)}
	if d.Route != "" {
		out = append(out, d.Route)
	}
	out = append(out, fmt.Sprintf("og gik i mål lørdag nat kl. %02d:%02d",
		d.FinishedAt.Hour(), d.FinishedAt.Minute()))
	return out
}

// Thumbnail renders the artwork at `edge` pixels on its longest side, as JPEG.
//
// # Why the thumbnail carries no text
//
// It is a picture of the diploma, not a small diploma. The patrol's name at thumbnail size would be a few
// pixels tall and illegible, so drawing it would cost a text renderer and an embedded font — the licence
// question above — to produce something nobody can read. The page labels the slot, the click gives the real
// thing, and when the artwork is replaced the thumbnail follows with no code change.
//
// Quality 80 and area-average scaling, via `internal/imaging.Fit`, so this matches how every other image on
// the public surface is minified rather than introducing a second filter. The JPEG encode is done here rather
// than by exporting `imaging.encode`, because one caller is not a reason to widen that package's API.
func Thumbnail(edge int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(Background()))
	if err != nil {
		return nil, fmt.Errorf("decoding the diploma background: %w", err)
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, imaging.Fit(img, edge), &jpeg.Options{Quality: 80}); err != nil {
		return nil, fmt.Errorf("encoding the diploma thumbnail: %w", err)
	}
	return out.Bytes(), nil
}
