package main

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
)

// The year the curator is working in (task 392).
//
// # Why this is on the request and never read from the configuration
//
// Until task 392 every admin read and write was bound to `app.config.eventYear`, the deploy-time `EVENT_YEAR`.
// PRD 022 §5 made that deliberate: the year is the one thing a curator cannot undo by editing, because a
// photograph uploaded into the wrong year is in the wrong event. Making the year selectable raises that risk, so
// it is carried in exactly one way and refused rather than guessed:
//
//   - **Pages** are registered once per year, under that year (`/2025/photos`), and the route states it.
//   - **Fragments and the JSON API** have no year in their path. The page sends `X-Admin-Year` on every htmx and
//     `fetch` request (see main.js), and `<img>` thumbnails, which cannot carry a header, send `?year=`.
//   - **A missing or unknown year is a 400.** Never a default: defaulting to `EVENT_YEAR` is precisely the
//     silent wrong-year write this exists to prevent.
//
// Either way it ends up on the request context, and handlers read it with `adminYear(r)`.
// `TestNoAdminHandlerReadsTheConfiguredYear` holds that none reads the configuration instead.

// adminYearHeader is the header the tool's pages stamp on every request they make.
const adminYearHeader = "X-Admin-Year"

type adminYearKey struct{}

// adminYear returns the year the request works in. Every admin handler is wrapped by atAdminYear or
// requireAdminYear, so it is always set; "" would mean a route registered without either, which
// `TestEveryAdminRouteCarriesAYear` refuses.
func adminYear(r *http.Request) string {
	y, _ := r.Context().Value(adminYearKey{}).(string)
	return y
}

func withAdminYear(r *http.Request, year string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), adminYearKey{}, year))
}

// adminYears are the years the tool can work in, newest first: every year `event_year` knows, plus `EVENT_YEAR`
// always — so a database that is down at boot still leaves the current year working.
//
// Read once, when the routes are built, because the pages are registered per year. A new year therefore needs a
// restart, which is also when `EVENT_YEAR` would change.
func (app *application) adminYears() []string {
	years := []string{app.config.eventYear}
	if app.models.Years != nil {
		known, err := app.models.Years.Years()
		if err != nil {
			app.Logger.Error("reading the event years for the admin tool; only EVENT_YEAR is workable", "err", err)
		}
		for _, y := range known {
			if !slices.Contains(years, y) {
				years = append(years, y)
			}
		}
	}
	slices.SortFunc(years, func(a, b string) int { return strings.Compare(b, a) })
	return years
}

// currentEventYear is `EVENT_YEAR`, for the one purpose an admin page may know it: to say when the curator is
// working in some other year. Every read and write uses adminYear(r) instead.
func (app *application) currentEventYear() string { return app.config.eventYear }

// adminRoots are adminYears as path prefixes, `/2026`, for registering the pages.
func (app *application) adminRoots() []string {
	years := app.workableYears
	out := make([]string, len(years))
	for i, y := range years {
		out[i] = "/" + y
	}
	return out
}

// atAdminYear wraps a page registered under one year's prefix: the route is the year.
func (app *application) atAdminYear(root string, next http.HandlerFunc) http.HandlerFunc {
	year := strings.TrimPrefix(root, "/")
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, withAdminYear(r, year))
	}
}

// requireAdminYear wraps a fragment or API route: the year comes from the request, and must be a workable one.
//
// Checked against `app.workableYears`, read once when the routes are built rather than per request.
func (app *application) requireAdminYear(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		year := r.Header.Get(adminYearHeader)
		if year == "" {
			year = r.URL.Query().Get("year")
		}
		if year == "" {
			app.BadRequestResponse(w, r, errors.New("der er ikke angivet noget år"))
			return
		}
		if !slices.Contains(app.workableYears, year) {
			app.BadRequestResponse(w, r, errors.New("ukendt år"))
			return
		}
		next(w, withAdminYear(r, year))
	}
}
