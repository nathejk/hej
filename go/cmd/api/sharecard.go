package main

import (
	"net/http"
	"sync"

	"nathejk.dk/internal/diploma"
	"nathejk.dk/internal/sharecard"
)

// The branded card a social platform shows when somebody shares the public site (PRD 026).
//
// # Why there is an endpoint for it at all
//
// `og:image` must be an absolute URL a stranger's server can fetch — Facebook's scraper, not the visitor's browser,
// is what loads it. So the card has to be a route, and it has to answer to an unauthenticated request that carries
// no cookie and no session.
//
// # Why it is rendered rather than served from a file
//
// There is no 1200×630 artwork in the repo, and the maintainer had none to hand (2026-09-28): *"create a black
// image, with logo moon and title (NATHEJK) taking up most of the space. Then i might come with a better solution
// later on, but this is it for now"*. `internal/sharecard` lifts the crescent and the wordmark off the event poster,
// which is the one place those marks already exist as pixels we ship. When a designed card arrives this becomes a
// decoded asset and the route does not change.

// shareCardWidth and shareCardHeight are the size every platform renders a large card at.
//
// 1200×630 is Facebook's recommendation and close enough to 1.91:1 that Twitter, Messenger, Slack and iMessage all
// use it without cropping. A card smaller than 600 wide is shown as a thumbnail beside the text instead, which is
// the failure mode worth avoiding — it makes a shared album look like a link to a document.
const (
	shareCardWidth  = 1200
	shareCardHeight = 630
)

// shareCardCacheControl is the diploma thumbnail's window, for the diploma thumbnail's reason.
//
// The bytes are generated from artwork embedded in the binary, so they change only on deploy — but they are not
// content-addressed, so `immutable` would be a lie the day the poster is replaced. An hour means a burst is served
// from caches and a new deploy's card appears the same afternoon.
//
// Facebook caches what it scrapes for days regardless, which is a separate cache and not one this header reaches.
const shareCardCacheControl = "public, max-age=3600"

// shareCardHandler serves the neutral branded card.
//
// @Summary      The public site's share card (PNG)
// @Description  The image social platforms show when somebody shares the public site: a black card carrying the event's crescent and wordmark, at 1200×630. Rendered from the event poster embedded in the binary rather than stored as its own asset, so it cannot drift from the year's artwork — see internal/sharecard. This is the card for the **frontpage**; an album's preview is its own cover photograph and a patrol's is its diploma, so neither uses this route. Cached for an hour, like the diploma thumbnail and for the same reason: the bytes are generated rather than content-addressed, so `immutable` would be a lie the day the artwork is replaced. Carries `X-Robots-Tag: noindex` like every other image on this surface — which does not affect share previews, since a scraper fetching og:image ignores it. Unauthenticated; ignores the session cookie.
// @Tags         public-site
// @Produce      png
// @Success      200  {file}    binary  "the card"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      500  {object}  map[string]string  "the artwork could not be rendered"
// @Router       /{year}/share-card.png [get]
func (app *application) shareCardHandler(w http.ResponseWriter, r *http.Request) {
	// The media budget, not the page one: this is an image, and the thing fetching it is usually a scraper rather
	// than a reader. Same call the diploma thumbnail makes (task 347).
	if !app.allowPublicMediaRead(w, r) {
		return
	}

	bytes, err := app.shareCard()
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", shareCardCacheControl)
	w.Header().Set("X-Robots-Tag", "noindex")
	if _, err := w.Write(bytes); err != nil {
		app.Logger.Error("writing the share card", "err", err)
	}
}

// shareCard renders the card once and remembers the outcome, including a failure.
//
// The same shape as `diplomaThumbnail`, for the same reasons: a test gets a fresh one because it hangs off the
// application, and a broken asset is not re-attempted on every request of a burst.
func (app *application) shareCard() ([]byte, error) {
	app.shareCardOnce.Do(func() {
		app.shareCardBytes, app.shareCardErr = sharecard.Render(
			diploma.Background(), shareCardWidth, shareCardHeight)
	})
	return app.shareCardBytes, app.shareCardErr
}

// shareCardState is the cached render, mixed into `application`.
//
// Its own type so the three fields travel together and the zero value is a valid "not rendered yet" — the same
// reason `sync.Once` is usable without initialisation.
type shareCardState struct {
	shareCardOnce  sync.Once
	shareCardBytes []byte
	shareCardErr   error
}
