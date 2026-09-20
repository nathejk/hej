package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"nathejk.dk/internal/commands"
	"nathejk.dk/nathejk/table/album"
)

// The album dev fixture (PRD 011, task 333).
//
// # Why this exists
//
// The same argument devglimt.go makes, and it applies harder here. An album feature cannot be
// *reviewed* without albums: the frontpage's covers, the album page's grid, the mixed-orientation
// layout, the empty state and — the part with no other way to see it — the **bounds verdict** are all
// invisible against an empty projection. That is how a feature gets built, tested and shipped looking
// wrong.
//
// And unlike glimt, nothing will ever create an album by accident. There is no participant-facing path
// that produces one, so without this route the only way to see an album before the curation tool exists
// (PRD 011 §11 Q5) is to hand-write events onto a broker.
//
// # Why it takes the coordinate as data rather than as EXIF
//
// The real ingest reads GPS out of the uploaded file and then destroys it, which is the whole design
// (see albummedia.go). A fixture cannot exercise that half honestly, because writing an EXIF GPS block
// would mean shipping an EXIF *writer* in production code to serve a development convenience.
//
// So the fixture sets the coordinate explicitly and takes the same bounds check the real path takes.
// What that covers is the part with states worth looking at — inside, outside and none render
// differently — and what it deliberately does not cover is the EXIF read, which is tested against
// constructed files in `internal/imaging` instead. Stated plainly so nobody mistakes a green fixture
// for a tested parser.
//
// # It is append-only and shared
//
// Same trade as `devglimt` and `cmd/simscan`: this publishes to the configured EVENT_YEAR on a real
// broker, so the fixtures cannot be un-published — only purged with the event store. They are
// obviously synthetic, which is the mitigation.

// devAlbumFixtureResponse reports what was created.
type devAlbumFixtureResponse struct {
	Created []string `json:"created"`
	// Note is a plain-language reminder that this is append-only and shared.
	Note string `json:"note"`
}

// devAlbumSpec is one album to create.
type devAlbumSpec struct {
	slug        string
	title       string
	description string
	sortOrder   int
	// published is false for one album on purpose: an unpublished album must be invisible to every
	// public read, and that is only checkable if one exists.
	published bool
	items     []devAlbumItemSpec
}

// devAlbumItemSpec is one photograph.
type devAlbumItemSpec struct {
	w, h    int
	caption string
	// lat/lng are nil for "no coordinate", which is the common real case.
	lat, lng *float64
}

// devAlbumFixtures is the set, chosen to cover what the pages have to render rather than to look like a
// lot of data.
func devAlbumFixtures() []devAlbumSpec {
	// Inside the 2026 race area, so the bounds check says `inside` and they appear on the map.
	near := func(lat, lng float64) (*float64, *float64) { return &lat, &lng }
	insideLat, insideLng := near(55.7332, 12.2648)
	inside2Lat, inside2Lng := near(55.7401, 12.2812)
	// Manhattan. A real coordinate that is emphatically not at the event, so the `outside` verdict has
	// something to be about.
	outsideLat, outsideLng := near(40.7128, -74.0060)

	return []devAlbumSpec{
		{
			slug: "loerdag-morgen", title: "Lørdag morgen",
			description: "Da solen kom, og alle havde gået hele natten.",
			sortOrder:   10, published: true,
			items: []devAlbumItemSpec{
				// Mixed orientation in one album: this is what catches a grid that assumes landscape.
				{w: 1600, h: 1200, caption: "Ved målstregen", lat: insideLat, lng: insideLng},
				{w: 1200, h: 1600, caption: "Morgenmad"},
				{w: 1400, h: 1400, caption: "", lat: inside2Lat, lng: inside2Lng},
				// No caption AND no coordinate: the plainest possible item.
				{w: 1600, h: 900},
			},
		},
		{
			slug: "natten", title: "Natten",
			description: "", // No description: the frontpage card must not reserve space for one.
			sortOrder:   20, published: true,
			items: []devAlbumItemSpec{
				{w: 1600, h: 1200, caption: "Post 4A, lidt over midnat", lat: insideLat, lng: insideLng},
				// The out-of-bounds case: stored, visible to a curator, never on the map.
				{w: 1600, h: 1200, caption: "Med en forkert GPS-position", lat: outsideLat, lng: outsideLng},
			},
		},
		{
			// Deliberately unpublished. Nothing public may show this, which is the point of having it.
			slug: "kladde", title: "Kladde — må ikke vises",
			description: "Denne er ikke udgivet og må ikke kunne ses offentligt.",
			sortOrder:   30, published: false,
			items: []devAlbumItemSpec{
				{w: 1600, h: 1200, caption: "Hemmelig", lat: insideLat, lng: insideLng},
			},
		},
	}
}

// devAlbumFixtureHandler creates the fixture albums.
//
// @Summary      Create album fixture data (development only)
// @Description  Creates a set of synthetic curated albums so the frontpage covers, the album pages, the mixed-orientation grid and the bounds verdicts can be looked at. Images are generated in-process and are obviously fake. One album is left unpublished and one photograph is given a coordinate outside the race area, because those two states are otherwise impossible to see. Coordinates are set as data rather than written into EXIF — the real ingest reads them out of the uploaded file, which is tested in internal/imaging. Registered only when ENV=development; outside development this route does not exist. Publishes to the configured EVENT_YEAR on a shared, append-only broker: the albums cannot be un-published, only purged.
// @Tags         dev
// @Produce      json
// @Success      201  {object}  devAlbumFixtureResponse
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "event stream unavailable"
// @Router       /dev/album-fixture [post]
func (app *application) devAlbumFixtureHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := contextGetSession(r); !ok {
		// Authenticated, unlike the albums it creates. Not because the fixture needs an author — an
		// album has none, deliberately — but because an unauthenticated write endpoint is not a thing
		// to add even in development, where the dev PIN route makes a session cheap.
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	out := devAlbumFixtureResponse{
		Created: []string{},
		Note: "Synthetic albums on a shared append-only broker. They cannot be un-published, " +
			"only purged with the event store.",
	}

	// Spread backwards over a few hours so `createdAt` differs between albums; nothing orders by it
	// today (sortOrder does that), but a column where every row is identical hides an ordering bug.
	base := time.Now().UTC().Add(-3 * time.Hour)

	for i, spec := range devAlbumFixtures() {
		albumID := fmt.Sprintf("dev-album-%s", spec.slug)
		createdAt := base.Add(time.Duration(i) * 41 * time.Minute)

		if err := app.publishAlbum(album.VerbCreated, albumID, album.Created{
			AlbumID:     albumID,
			Year:        app.config.eventYear,
			Slug:        spec.slug,
			Title:       spec.title,
			Description: spec.description,
			SortOrder:   spec.sortOrder,
			CreatedAt:   createdAt,
		}); err != nil {
			app.writeAlbumPublishFailure(w, r, err)
			return
		}

		for ordinal, item := range spec.items {
			img := devFixtureImage(item.w, item.h, i, ordinal, spec.title)
			stored, err := app.storeAlbumImage(r.Context(), img)
			if err != nil {
				app.ServerErrorResponse(w, r, fmt.Errorf("fixture media: %w", err))
				return
			}

			// The fixture's own coordinate overrides whatever the generated image carried — which is
			// nothing, since `devFixtureImage` encodes from pixels. Taking the same bounds check the
			// real path takes, rather than asserting a verdict, so a fixture cannot claim a photograph
			// is plottable when the rule says otherwise.
			verdict := album.BoundsNone
			if item.lat != nil && item.lng != nil {
				verdict = app.albumBoundsVerdict(*item.lat, *item.lng)
			}

			if err := app.publishAlbum(album.VerbItemAdded, albumID, album.ItemAdded{
				AlbumID:       albumID,
				Year:          app.config.eventYear,
				Ordinal:       ordinal,
				Ref:           stored.Ref,
				ThumbRef:      stored.ThumbRef,
				Caption:       item.caption,
				Width:         stored.Width,
				Height:        stored.Height,
				Bytes:         stored.Bytes,
				Lat:           item.lat,
				Lng:           item.lng,
				BoundsVerdict: verdict,
				AddedAt:       createdAt.Add(time.Duration(ordinal) * time.Minute),
			}); err != nil {
				app.writeAlbumPublishFailure(w, r, err)
				return
			}
		}

		if spec.published {
			// A separate event, after the items, which is how a real curator works: assemble, then
			// publish. Doing it in the create event would put a half-finished album on the open web.
			published := true
			if err := app.publishAlbum(album.VerbUpdated, albumID, album.Updated{
				AlbumID:   albumID,
				Year:      app.config.eventYear,
				Published: &published,
				UpdatedAt: createdAt,
			}); err != nil {
				app.writeAlbumPublishFailure(w, r, err)
				return
			}
		}

		out.Created = append(out.Created, fmt.Sprintf("%s (%d billeder, published=%v)",
			spec.slug, len(spec.items), spec.published))
	}

	if err := app.WriteJSON(w, http.StatusCreated, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// publishAlbum publishes one album event.
//
// One helper so the subject is built in one place: `album.Subject` validates the tokens, and an id with
// a dot in it would publish successfully while quietly never matching the per-album patterns again.
func (app *application) publishAlbum(verb, albumID string, body any) error {
	subject, err := album.Subject(app.config.eventYear, albumID, verb)
	if err != nil {
		return err
	}
	return app.commands.Publish(subject, body)
}

// writeAlbumPublishFailure answers a publish error.
//
// A missing broker is a 503 rather than a 500: it is a configuration state, not a fault, and the
// distinction is what tells a developer to start the broker rather than read a stack trace.
func (app *application) writeAlbumPublishFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, commands.ErrNoPublisher) {
		app.ServiceUnavailableResponse(w, r, "eventstrømmen er ikke tilgængelig")
		return
	}
	app.ServerErrorResponse(w, r, err)
}
