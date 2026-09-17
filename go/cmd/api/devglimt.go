package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
)

// The Glimt dev fixture (task 327).
//
// # Why this exists
//
// A Glimt feed cannot be *reviewed* without glimt in it, and nothing in a dev environment ever posts
// one. The carousel, the grid, the aspect-ratio handling and the audience chips are all invisible
// against an empty projection — which is how a feature gets built, tested, and shipped looking wrong.
//
// # Why an endpoint and not a `cmd/seed` case
//
// `cmd/seed` publishes member events and cannot write blobs, and a glimt is a row that *references*
// media. Going through a handler means the fixture takes the real path — decode-validate, EXIF strip,
// re-encode, content-address, publish — which is exactly what `cmd/seed`'s own header argues for:
// seeded data should travel the same code as production data, including the parts most likely to be
// wrong.
//
// # Why it is safe
//
// Registered only under `ENV=development` (see dev.go): outside development the route does not exist,
// so there is no handler to reach by misconfiguration. It also publishes under whatever `EVENT_YEAR`
// the instance reads, unlike `cmd/seed`'s sentinel year — which it has to, because the point is to see
// the fixture in the running app. That is the same trade `cmd/simscan` makes, and the same mitigation
// applies: the glimt are obviously synthetic, and a purge or a fresh event store removes them.
//
// # Why the images are generated
//
// No fixture photographs are committed. A picture of a real person as test data is precisely the thing
// this feature is careful about, and a generated pattern is enough to judge a grid, a carousel, a
// thumbnail and an aspect ratio. Each is visibly synthetic, numbered, and captioned in the image
// itself so a screenshot is self-describing.

// devGlimtFixtureResponse reports what was created.
type devGlimtFixtureResponse struct {
	Created []string `json:"created"`
	// Note is a plain-language reminder that this is append-only and shared.
	Note string `json:"note"`
}

// devFixtureSpec is one glimt to create.
type devFixtureSpec struct {
	teamNumber string
	teamName   string
	// group decides who sees a `group`-scoped glimt. Left empty to mean "the caller's own group",
	// which is what makes the fixture visible to whoever ran it.
	group    string
	audience string
	caption  string
	// sizes are the media to generate, as width×height. Length is the item count.
	sizes []devFixtureSize
	// own makes the caller the author, so `Slet` and "Dit hold" appear on that card.
	own bool
	// report has the caller report it immediately after creating it, so the "Skjult" badge and the
	// moderation queue have a subject. Published as a real report rather than by setting a column,
	// because the fold is what sets `hiddenAt` and the fixture should exercise it.
	report bool
}

type devFixtureSize struct {
	w, h int
}

// devGlimtFixtures is the set, chosen to cover what the views actually have to render rather than to
// look like a lot of data.
func devGlimtFixtures() []devFixtureSpec {
	return []devFixtureSpec{
		{
			teamNumber: "42", teamName: "TEST Ørnene",
			audience: glimt.AudienceGroup,
			caption:  "Ved posten i skoven. Det regner.",
			// Four items, mixed orientation: this is the card that exercises the carousel, its
			// dots, and whether the strip copes with a portrait next to a landscape.
			sizes: []devFixtureSize{{1200, 1600}, {1600, 1200}, {1400, 1400}, {1600, 900}},
			own:   true,
		},
		{
			teamNumber: "42", teamName: "TEST Ørnene",
			audience: glimt.AudienceGroup,
			// No caption: the card must not reserve space for one.
			sizes: []devFixtureSize{{1600, 1200}},
			own:   true,
		},
		{
			teamNumber: "43", teamName: "TEST Ulvene",
			audience: glimt.AudienceNathejk,
			// A long caption, to see wrapping and whether the card grows sensibly.
			caption: "Vi gik forbi den store sø lige før midnat, og der stod en gøgler i " +
				"fuldt kostume midt på stien og spillede på trompet. Ingen af os forstod " +
				"hvorfor, men det var det bedste indtil videre.",
			sizes: []devFixtureSize{{1600, 1200}, {1200, 1600}},
		},
		{
			teamNumber: "44", teamName: "TEST Bjørnene",
			audience: glimt.AudiencePublic,
			caption:  "Morgenmad!",
			sizes:    []devFixtureSize{{1400, 1400}},
		},
		{
			teamNumber: "45", teamName: "TEST Ravnene",
			audience: glimt.AudiencePublic,
			caption:  "Klar til at gå.",
			// Two items so the public page has a multi-item entry too.
			sizes: []devFixtureSize{{1600, 900}, {900, 1600}},
		},
		{
			teamNumber: "46", teamName: "TEST Grævlingene",
			audience: glimt.AudienceGroup,
			caption:  "Dette glimt er anmeldt, så det skal være skjult.",
			sizes:    []devFixtureSize{{1600, 1200}},
			report:   true,
		},
	}
}

// devGlimtFixtureHandler creates the fixture set as the calling member.
//
// @Summary      Create Glimt fixture data (development only)
// @Description  Creates a set of synthetic glimt so the feed, the hold grid, the carousel and the moderation view can be looked at. Images are generated in-process and are obviously fake. Attributed to invented holds — which works because attribution is frozen at creation — but authored by the caller, so the caller can see the group-scoped ones. Registered only when ENV=development; outside development this route does not exist. Publishes to the configured EVENT_YEAR on a shared, append-only broker: the glimt cannot be un-published, only purged.
// @Tags         dev
// @Produce      json
// @Success      201  {object}  devGlimtFixtureResponse
// @Failure      400  {object}  map[string]string  "the caller's role maps to no group"
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string  "the caller has no directory record"
// @Failure      503  {object}  map[string]string  "event stream unavailable"
// @Router       /dev/glimt-fixture [post]
func (app *application) devGlimtFixtureHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		// Authenticated even here: the fixture is authored *by* the caller, so that the
		// group-scoped glimt are visible to whoever asked for them. A fixture nobody can see
		// would be worse than none.
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	viewer, found := app.models.Users.Get(s.UserID)
	if !found {
		app.NotFoundResponse(w, r)
		return
	}
	group, ok := users.GlimtGroupFor(viewer.Role)
	if !ok {
		app.BadRequestResponse(w, r, errors.New("din rolle kan ikke afgøre en gruppe"))
		return
	}

	out := devGlimtFixtureResponse{
		Created: []string{},
		Note: "Synthetic glimt on a shared append-only broker. They cannot be un-published, " +
			"only purged with the event store.",
	}

	// Spread the timestamps backwards over the last few hours, newest last, so the feed's
	// ordering and the relative-time formatting have something to show. Without this every
	// fixture glimt reads "Lige nu" and the whole time column is untested by eye.
	base := time.Now().UTC().Add(-4 * time.Hour)

	for i, spec := range devGlimtFixtures() {
		createdAt := base.Add(time.Duration(i) * 37 * time.Minute)

		media := make([]glimt.Media, 0, len(spec.sizes))
		for ordinal, size := range spec.sizes {
			img := devFixtureImage(size.w, size.h, i, ordinal, spec.teamName)
			stored, err := app.storeGlimtImage(r, img)
			if err != nil {
				app.ServerErrorResponse(w, r, fmt.Errorf("fixture media: %w", err))
				return
			}
			media = append(media, glimt.Media{
				Ordinal:     ordinal,
				Ref:         stored.Ref,
				ThumbRef:    stored.ThumbRef,
				Kind:        stored.Kind,
				ContentType: stored.ContentType,
				Bytes:       stored.Bytes,
				Width:       stored.Width,
				Height:      stored.Height,
			})
		}

		glimtID := uuid.NewString()
		author := s.UserID
		if !spec.own {
			// A synthetic author, so the card shows the hold attribution and offers `Anmeld`
			// rather than `Slet`. Ownership is the only thing this id controls, and nothing
			// resolves it to a person — which is the whole point of task 302's projection.
			author = "dev-fixture-" + glimtID
		}
		authorGroup := spec.group
		if authorGroup == "" {
			authorGroup = string(group)
		}

		subject, err := glimt.Subject(app.config.eventYear, glimtID, glimt.VerbCreated)
		if err != nil {
			app.ServerErrorResponse(w, r, err)
			return
		}
		if err := app.commands.Publish(subject, glimt.Created{
			GlimtID:        glimtID,
			Year:           app.config.eventYear,
			AuthorPersonID: author,
			AuthorGroup:    authorGroup,
			TeamNumber:     spec.teamNumber,
			TeamName:       spec.teamName,
			Audience:       spec.audience,
			Caption:        spec.caption,
			Media:          media,
			CreatedAt:      createdAt,
		}); err != nil {
			app.ServiceUnavailableResponse(w, r, "kunne ikke udgive fixture, er broker'en oppe?")
			return
		}
		out.Created = append(out.Created, glimtID)

		if spec.report {
			// A real report, so the projection's fold is what hides it — the fixture should
			// exercise the mechanism rather than fake its result.
			reportSubject, rerr := glimt.Subject(app.config.eventYear, glimtID, glimt.VerbReported)
			if rerr != nil {
				app.ServerErrorResponse(w, r, rerr)
				return
			}
			if perr := app.commands.Publish(reportSubject, glimt.Reported{
				GlimtID:          glimtID,
				Year:             app.config.eventYear,
				ReporterPersonID: s.UserID,
				Reason:           "fixture",
				ReportedAt:       createdAt.Add(time.Minute),
			}); perr != nil {
				app.ServiceUnavailableResponse(w, r, "kunne ikke udgive fixture-anmeldelse")
				return
			}
		}
	}

	if err := app.WriteJSON(w, http.StatusCreated, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// devFixtureImage draws an obviously synthetic JPEG of the requested size.
//
// Deliberately not a photograph and deliberately not noise. It needs three things to be useful:
//
//   - **a distinct colour per glimt**, so a card in a feed and a tile in a grid can be matched by eye
//   - **visible orientation**, so a portrait cropped as a landscape is obvious — hence the corner
//     markers, which are the first thing to disappear under a bad `object-fit`
//   - **a legible index**, so "the third photo in the second glimt" is a thing you can say about a
//     screenshot
//
// Drawn with `image/draw` alone: no font rendering, because a font dependency for a dev fixture is not
// worth it, and blocks are legible enough at thumbnail size — arguably more so.
func devFixtureImage(w, h, glimtIndex, ordinal int, label string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	base := devFixturePalette[glimtIndex%len(devFixturePalette)]
	// A diagonal gradient rather than a flat fill, so JPEG compression has something to do and a
	// thumbnail is visibly a scaled version of the same picture rather than a different colour.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := float64(x+y) / float64(w+h)
			img.Set(x, y, color.RGBA{
				R: scaleChannel(base.R, t),
				G: scaleChannel(base.G, t),
				B: scaleChannel(base.B, t),
				A: 255,
			})
		}
	}

	// Corner markers. These are what make a wrong crop obvious: `object-cover` on a portrait in a
	// landscape box eats the top and bottom pair first.
	//
	// **Opaque white, and `color.RGBA` must be opaque here.** `color.RGBA` is
	// *alpha-premultiplied*, so `{255, 255, 255, 220}` is out of gamut (R > A) and Go renders it
	// almost black — which is what the first version of this did, on a dark purple background,
	// making the markers invisible and defeating their whole purpose. Found by looking at the
	// output rather than by any test. If a translucent marker is ever wanted, use `color.NRGBA`.
	markerSize := maxOf(16, minOf(w, h)/12)
	white := image.NewUniform(color.RGBA{255, 255, 255, 255})
	for _, corner := range []image.Rectangle{
		image.Rect(0, 0, markerSize, markerSize),
		image.Rect(w-markerSize, 0, w, markerSize),
		image.Rect(0, h-markerSize, markerSize, h),
		image.Rect(w-markerSize, h-markerSize, w, h),
	} {
		draw.Draw(img, corner, white, image.Point{}, draw.Over)
	}

	// A row of blocks in the middle: one block per item ordinal, so the third photo of a glimt has
	// three blocks. Countable at thumbnail size, which a numeral would not be.
	//
	// Sized generously for the same reason as the markers: a 320px thumbnail is the size these are
	// actually read at, and the first version's blocks were a few pixels across there.
	blockH := maxOf(14, minOf(w, h)/9)
	blockW := blockH
	gap := blockH / 2
	total := (ordinal + 1)
	startX := (w - (total*blockW + (total-1)*gap)) / 2
	y0 := (h - blockH) / 2
	for b := 0; b < total; b++ {
		x0 := startX + b*(blockW+gap)
		draw.Draw(img, image.Rect(x0, y0, x0+blockW, y0+blockH), white, image.Point{}, draw.Over)
	}

	// A bar whose width encodes the glimt index, under the blocks. Between the colour and this, two
	// screenshots can be told apart without reading anything.
	barW := minOf(w-2*markerSize, (glimtIndex+1)*(w/12))
	barY := y0 + blockH + blockH/2
	if barY+blockH/3 < h {
		draw.Draw(img, image.Rect(markerSize, barY, markerSize+barW, barY+blockH/3),
			white, image.Point{}, draw.Over)
	}

	_ = label // kept in the signature so a future version can render it if a font is ever worth it

	var buf bytes.Buffer
	// Quality 80: a real photograph's order of magnitude, so the stored sizes are plausible when
	// somebody is looking at how much space a feed costs.
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		// Cannot happen for an RGBA image of positive size, and there is nothing useful to do
		// about it in a fixture. An empty slice fails validation upstream, loudly.
		return nil
	}
	return buf.Bytes()
}

// devFixturePalette is a set of hues that stay distinguishable at thumbnail size and in the dark.
var devFixturePalette = []color.RGBA{
	{28, 78, 140, 255},  // blue
	{140, 46, 46, 255},  // red
	{40, 110, 70, 255},  // green
	{120, 84, 24, 255},  // amber
	{80, 48, 120, 255},  // violet
	{30, 100, 110, 255}, // teal
}

// scaleChannel darkens a channel across the gradient, floored so nothing becomes pure black.
func scaleChannel(c uint8, t float64) uint8 {
	f := 0.55 + 0.45*(1-t)
	v := math.Round(float64(c) * f)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func maxOf(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minOf(a, b int) int {
	if a < b {
		return a
	}
	return b
}
