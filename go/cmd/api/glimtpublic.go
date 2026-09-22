package main

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/eventtime"
	"nathejk.dk/nathejk/table/glimt"
)

// The public Glimt page and its API (PRD 019 §0, §7, §8, task 323).
//
// # Why this lives in this service and not in PRD 013
//
// The public feed is served on `hej.nathejk.dk` by the same binary that holds the glimt. An export
// into the marketing site would mean a second copy of the photographs, a second retention window to
// keep correct, and a hidden glimt that stays up somewhere else. One store, one takedown.
//
// # The session is not merely ignored — it cannot be read
//
// PRD 019 §8 names this as a trap: a logged-in member's browser *will* send `hej_session` to these
// handlers, and if any of them ever read it, the public page silently becomes a different page for
// members than for parents. "Is this public-safe?" then stops being a testable question.
//
// The defence is structural rather than disciplinary: `requireAuth` is the **only** place a session
// enters the request context (see middleware.go), so a handler registered without it has nothing to
// read. `contextGetSession` in this file would return `false` unconditionally. That is why these
// routes are registered bare, and `glimtpublic_test.go` asserts the resulting behaviour with a real
// authenticated cookie attached.
//
// # What is served
//
// `audience = public AND hiddenAt IS NULL AND createdAt >= cutoff`, and nothing else, regardless of
// who asks. The cutoff is `app.glimtPublicCutoff()` (task 310) and passing it is not optional: a zero
// time there serves the whole archive to the open web.
//
// No personal name, no portrait, no arm number, no phone number — ever. `publicGlimtResponse` has
// nowhere to put any of them, which is the same structural approach `glimtResponse` takes.

// publicGlimtPageLimit is how many glimt the HTML page shows.
//
// # Why there is no pagination
//
// A "load more" is either JavaScript or a link with an offset. The first is forbidden here, and the
// second is a page that grows unboundedly for a crawler to walk. So the page shows the most recent
// slice and says so. Somebody looking for a specific patrulje's photographs belongs in the app,
// where the hold collection exists (task 326) — this page's job is to let a parent see what the event
// looked like.
const publicGlimtPageLimit = 60

// maxPublicReportReason bounds the note an anonymous reporter leaves.
//
// Shorter than the authenticated one: this field is reachable without a session and lands in a
// moderator's queue, so it is the one free-text input in the feature with no accountable author.
const maxPublicReportReason = 300

// publicGlimtFeedResponse is the JSON the public API answers with.
type publicGlimtFeedResponse struct {
	Glimt []publicGlimtResponse `json:"glimt"`
}

// publicGlimt reads the page of public glimt, or reports that it cannot.
//
// Factored out because the HTML page and the JSON endpoint must show **exactly** the same set: two
// queries would be two chances for one of them to forget the cutoff or the hidden filter, and the
// HTML one is the copy nobody would think to test for a leak.
func (app *application) publicGlimt(limit, offset int) ([]glimt.Glimt, error) {
	if app.models.Glimt == nil {
		return nil, errGlimtUnavailable
	}
	// The cutoff is the whole of task 310's public retention window (PRD 019 §6). Passing
	// `time.Time{}` here would serve the entire archive to the open web, which is why it comes from
	// a named method rather than being assembled at the call site.
	return app.models.Glimt.PublicFeed(app.config.eventYear, app.glimtPublicCutoff(), limit, offset)
}

var errGlimtUnavailable = errors.New("glimt er ikke tilgængelige lige nu")

// listPublicGlimtHandler serves the public feed as JSON.
//
// @Summary      The public glimt feed
// @Description  Every glimt shared publicly, newest first, that is not hidden and not older than the public retention window. **Unauthenticated, and it ignores the session cookie entirely** — a signed-in member's browser will send one, and this endpoint gets exactly the same response as an anonymous visitor. That is structural rather than a convention: the session is only ever placed in the request context by the auth middleware, which this route does not use. Carries no author, no personal name, no portrait and no phone number; a glimt is attributed to its hold. Media are fetched from /public/glimt/{id}/media/{ordinal}.
// @Tags         glimt-public
// @Produce      json
// @Param        limit   query     int  false  "page size (default 20, max 200)"
// @Param        offset  query     int  false  "rows to skip"
// @Success      200  {object}  publicGlimtFeedResponse
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      503  {object}  map[string]string
// @Router       /public/glimt [get]
func (app *application) listPublicGlimtHandler(w http.ResponseWriter, r *http.Request) {
	// Keyed by IP, unlike every other Glimt limiter, because there is no member to key on. That is
	// a worse key — a school behind one NAT shares a budget — which is exactly why the public read
	// limit must be generous.
	if !app.allowPublicGlimtRead(w, r) {
		return
	}

	limit, offset := pageParams(r)
	rows, err := app.publicGlimt(limit, offset)
	if err != nil {
		if errors.Is(err, errGlimtUnavailable) {
			app.ServiceUnavailableResponse(w, r, err.Error())
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}

	out := publicGlimtFeedResponse{Glimt: make([]publicGlimtResponse, 0, len(rows))}
	for _, g := range rows {
		out.Glimt = append(out.Glimt, newPublicGlimtResponse(g))
	}

	// Shareable, unlike the authenticated feed's `no-store`: this response is identical for every
	// caller by construction, so a shared cache is safe and is most of the answer to a link going
	// round a parents' group chat. Short, because a takedown must take effect quickly — the whole
	// point of the report endpoint below is that a bad photograph comes down in minutes.
	w.Header().Set("Cache-Control", "public, max-age=60")

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// showPublicGlimtMediaHandler serves one media item to the open web.
//
// @Summary      One public glimt media item
// @Description  Serves the stored bytes for item `ordinal` of a **public, non-hidden** glimt inside the public retention window. `variant=thumb` serves the 320px thumbnail. Unauthenticated and it ignores the session cookie: the glimt's audience is re-checked here, so this route cannot be used to reach a group-scoped item even with a valid session. Cached `public` and `immutable`, unlike the authenticated media route which must be `private` — that difference is the reason this exists as a separate route.
// @Tags         glimt-public
// @Produce      jpeg
// @Param        glimtId  path      string  true   "glimt id"
// @Param        ordinal  path      int     true   "media position within the glimt"
// @Param        variant  query     string  false  "full (default) or thumb"
// @Success      200  {file}    binary
// @Failure      304  "not modified"
// @Failure      404  {object}  map[string]string  "not public, hidden, expired, or gone"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      503  {object}  map[string]string
// @Router       /public/glimt/{glimtId}/media/{ordinal} [get]
func (app *application) showPublicGlimtMediaHandler(w http.ResponseWriter, r *http.Request) {
	// The media budget rather than the page one (task 347). Same reasoning as the album media route: a
	// page of thumbnails is dozens of these requests, and they must not spend the allowance the pages
	// need. Keyed by IP either way — there is still no member here.
	if !app.allowPublicMediaRead(w, r) {
		return
	}
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, errGlimtUnavailable.Error())
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	glimtID := params.ByName("glimtId")
	ordinal, err := strconv.Atoi(params.ByName("ordinal"))
	if err != nil {
		app.NotFoundResponse(w, r)
		return
	}

	g, found, err := app.models.Glimt.Get(app.config.eventYear, glimtID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	// **404, not 403.** The authenticated media route answers 403 deliberately, because there the
	// caller is a known member and an honest refusal tells them the photo was not shared with them.
	// Here the caller is anonymous and a 403 would confirm that a glimt with this id exists and is
	// not public — a distinction worth nothing to a parent and worth something to somebody probing.
	if !found || !app.publiclyVisible(g) {
		app.NotFoundResponse(w, r)
		return
	}

	ref, ok := glimtVariantRef(g, ordinal, r.URL.Query().Get("variant"))
	if !ok {
		app.NotFoundResponse(w, r)
		return
	}

	// `public`, unlike the authenticated route's `private`. Safe precisely because this handler's
	// answer does not depend on who asked, and valuable because a public link is the one that gets
	// shared widely — a CDN or a corporate proxy serving it is a feature here and a leak there.
	app.streamPublicGlimtMedia(w, r, ref, glimtID)
}

// publiclyVisible re-checks a glimt against the public rule.
//
// # Why this exists when PublicFeed already filters
//
// The media handler does not go through `PublicFeed` — it fetches one glimt by id — so without this
// the id alone would be enough to pull the bytes of a group-scoped photograph off an unauthenticated
// route. That is the same reasoning `glimtmediaserve.go` records for its own re-check, and it is the
// single most important check in this file.
//
// All three conditions, in one place, so the page, the JSON and the media route cannot drift:
// public audience, not hidden, and inside the retention window.
func (app *application) publiclyVisible(g glimt.Glimt) bool {
	if g.Audience != glimt.AudiencePublic {
		return false
	}
	if g.HiddenAt != nil {
		return false
	}
	if cutoff := app.glimtPublicCutoff(); !cutoff.IsZero() && g.CreatedAt.Before(cutoff) {
		return false
	}
	return true
}

// streamPublicGlimtMedia is streamGlimtMedia with the shared-cacheable header.
//
// `streamGlimtMedia` takes the header value as a parameter rather than this function wrapping the
// ResponseWriter to overwrite it. A wrapper would work and would be worse: the authenticated route's
// `private` is a privacy property, and it should be visible at its own call site rather than
// something a later reader has to prove nobody downstream changed.
func (app *application) streamPublicGlimtMedia(
	w http.ResponseWriter, r *http.Request, ref blob.Ref, logID string,
) {
	app.streamGlimtMedia(w, r, ref, logID, publicGlimtMediaCacheControl)
}

// publicGlimtMediaCacheControl is a year, immutable, and **public**.
const publicGlimtMediaCacheControl = "public, max-age=31536000, immutable"

// reportPublicGlimtHandler lets an anonymous visitor report a glimt.
//
// @Summary      Report a public glimt (no login)
// @Description  Flags a publicly shared glimt and hides it from every audience immediately, before any human looks — the same behaviour as the authenticated report, because the public scope has no approval queue in front of it. **Deliberately unauthenticated**: a parent who spots a problem is exactly the person we want to hear from, and they have no session and will not make one. Rate-limited by IP, since there is no member to key on. Only public glimt can be reported here; anything else answers 404 without revealing whether it exists.
// @Tags         glimt-public
// @Accept       json
// @Produce      json
// @Param        glimtId  path      string              true   "glimt id"
// @Param        request  body      reportGlimtRequest  false  "optional reason"
// @Success      204  "reported and hidden"
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string  "not a public glimt"
// @Failure      429  {object}  map[string]string  "report rate limit, by IP"
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /public/glimt/{glimtId}/report [post]
func (app *application) reportPublicGlimtHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.Glimt == nil {
		app.ServiceUnavailableResponse(w, r, errGlimtUnavailable.Error())
		return
	}

	// By IP, because there is nobody to key on. Tighter than the authenticated report limit — an
	// anonymous write needs a lower ceiling — but still generous, and for the same reason recorded
	// in glimtreport.go: the cost of a spurious report is a moderator's glance, while the cost of a
	// throttled one is a photograph somebody objected to staying on the open web.
	if app.publicReportLimiter != nil && !app.publicReportLimiter.Allow(clientIP(r)) {
		app.RateLimitMessageResponse(w, r, "For mange anmeldelser. Prøv igen om lidt.")
		return
	}

	glimtID := httprouter.ParamsFromContext(r.Context()).ByName("glimtId")

	var in reportGlimtRequest
	if r.ContentLength > 0 {
		if err := app.ReadJSON(w, r, &in); err != nil {
			app.BadRequestResponse(w, r, err)
			return
		}
	}
	reason := strings.TrimSpace(in.Reason)
	if len([]rune(reason)) > maxPublicReportReason {
		app.BadRequestResponse(w, r, errors.New("beskrivelsen er for lang"))
		return
	}

	g, found, err := app.models.Glimt.Get(app.config.eventYear, glimtID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	// Only a public glimt is reportable here. Otherwise this endpoint would be an unauthenticated
	// way to take down a group-scoped photograph, and an unauthenticated way to discover that one
	// exists.
	if !found || !app.publiclyVisible(g) {
		app.NotFoundResponse(w, r)
		return
	}

	subject, serr := glimt.Subject(app.config.eventYear, glimtID, glimt.VerbReported)
	if serr != nil {
		app.ServerErrorResponse(w, r, serr)
		return
	}

	if perr := app.commands.Publish(subject, glimt.Reported{
		GlimtID: glimtID,
		Year:    app.config.eventYear,
		// **A sentinel, not an IP address.** `glimt_report` is keyed by reporter, so anonymous
		// reports need *a* value; recording the IP would put a personal identifier of the one
		// participant in this feature who never agreed to anything into an append-only audit
		// table, to solve a duplicate-counting problem that does not matter here.
		//
		// The cost is honest and small: several anonymous reports of the same glimt collapse into
		// one row, so the count under-reports. The glimt is hidden on the first one anyway, which
		// is the outcome that matters.
		ReporterPersonID: publicReporterSentinel,
		Reason:           reason,
		ReportedAt:       time.Now().UTC(),
	}); perr != nil {
		if errors.Is(perr, commands.ErrNoPublisher) {
			app.ServiceUnavailableResponse(w, r, "anmeldelsen kunne ikke gemmes, prøv igen")
			return
		}
		app.ServerErrorResponse(w, r, perr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// publicReporterSentinel stands in for an anonymous reporter in the audit trail.
//
// Recognisable in the moderation queue as "somebody outside the app", which is a useful thing for a
// moderator to know: a report from the open web has different weight to one from a participant.
const publicReporterSentinel = "public"

// allowPublicGlimtRead applies the by-IP read limiter.
//
// Deliberately its own limiter rather than sharing `glimtReadLimiter`, which is keyed by member. An
// IP is a much worse key — a school or a workplace behind one NAT shares a single budget — so the
// ceiling has to be correspondingly generous, and mixing the two would drag the member-keyed limit
// down with it.
func (app *application) allowPublicGlimtRead(w http.ResponseWriter, r *http.Request) bool {
	if app.publicGlimtReadLimiter == nil || app.publicGlimtReadLimiter.Allow(clientIP(r)) {
		return true
	}
	app.RateLimitMessageResponse(w, r, "For mange forespørgsler. Prøv igen om lidt.")
	return false
}

// publicGlimtPageHandler renders the public page.
//
// # Server-rendered, and that is the requirement rather than a preference
//
// No login, no bundle, no client-side state, and it works with JavaScript disabled. The reason is
// reach: this page exists so a grandparent on a ten-year-old browser can see what the weekend looked
// like. Anything that needs the app's bundle to parse defeats the only thing this page is for.
//
// # Every media item is shown — there is no carousel
//
// The maintainer's point (2026-09-18): the public page has to work on a desktop computer, so media
// must be reachable **without a swipe**. The app's strip solves that with prev/next buttons, but
// that is a Vue component driven by Embla and there is no bundle here to run it.
//
// So a glimt with four photographs renders four `<img>` tags. No JavaScript, no gestures, no
// controls to discover, and it works in every browser. A glimt has at most ten items. The rejected
// alternatives are recorded in task 323: CSS scroll-snap with anchor links (works, but anchor
// navigation moves the *page*, and containing it needs per-browser care for no benefit on a page
// whose whole virtue is being dumb) and a small inline script (fails the no-JavaScript requirement,
// and would give some visitors a different page).
//
// @Summary      The public glimt page (HTML)
// @Description  A server-rendered page of the glimt shared publicly: no login, no bundle, no client-side state, and it works with JavaScript disabled — which is the requirement rather than a preference, because the page exists so somebody on an old browser can see what the weekend looked like. **Every media item of every glimt is rendered as its own img tag; there is no carousel**, so multiple photographs are reachable on a desktop without a swipe and without script. Unauthenticated and it ignores the session cookie entirely, so a signed-in member sees exactly what a parent sees. Shows the most recent 60 glimt and is not paginated. Not indexed (robots noindex).
// @Tags         glimt-public
// @Produce      html
// @Success      200  {string}  string  "the page"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Router       /{year}/glimt [get]
func (app *application) publicGlimtPageHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicGlimtRead(w, r) {
		return
	}

	rows, err := app.publicGlimt(publicGlimtPageLimit, 0)
	if err != nil && !errors.Is(err, errGlimtUnavailable) {
		app.Logger.Error("rendering the public glimt page", "err", err)
		// An error page rather than a 500 body: this is HTML, and a JSON error object rendered in
		// a browser tells a parent nothing.
		err = errGlimtUnavailable
	}

	data := publicGlimtPageData{
		Year:        app.config.eventYear,
		Unavailable: err != nil,
		Root:        app.publicRoot(),
	}
	for _, g := range rows {
		data.Glimt = append(data.Glimt, newPublicGlimtResponse(g))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Same short window as the JSON: shareable, but a takedown must land quickly.
	w.Header().Set("Cache-Control", "public, max-age=60")
	if rerr := publicGlimtPageTemplate.Execute(w, data); rerr != nil {
		// The response has already begun. Logged rather than answered.
		app.Logger.Error("executing the public glimt template", "err", rerr)
	}
}

type publicGlimtPageData struct {
	Year        string
	Glimt       []publicGlimtResponse
	Unavailable bool

	// Root is the public site's base path (`/2026`), for the same reason publicPageData carries one: the
	// prefix moved once (task 351) and will move again when PRD 021 lands, and a path typed into a template
	// is one nobody remembers to change.
	Root string
}

// publicGlimtPageTemplate is the whole page: one file, no assets, no script.
//
// The CSS is inline for the same reason there is no bundle — a separate stylesheet is a second
// request that can fail, and there is not enough of it to be worth caching. `html/template` escapes
// every interpolation, which matters here because a caption is participant-authored text on an
// unauthenticated page.
var publicGlimtPageTemplate = template.Must(template.New("publicGlimt").Funcs(template.FuncMap{
	"hold": publicHoldLabel,
	// The same helper the rest of the public site uses (task 358). This used to be a one-line closure over a
	// Go layout string, and it had **two** bugs that a reader would not see: `januar` is not a layout token, so
	// Go copied it through as a literal and every glimt was dated in January; and nothing converted the
	// instant, so the clock was UTC. Both were invisible because the code looked like a format string.
	"date": eventtime.Danish,
}).Parse(`<!DOCTYPE html>
<html lang="da">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Glimt fra Nathejk {{.Year}}</title>
<!-- Not indexed. The photographs were shared publicly by their authors, which is not the same as
     asking for them to be findable by name in a search engine years later. -->
<meta name="robots" content="noindex, nofollow">
<style>
  :root { color-scheme: light dark; }
  body { font-family: system-ui, sans-serif; margin: 0; padding: 1rem;
         max-width: 60rem; margin-inline: auto; line-height: 1.5; }
  h1 { font-size: 1.6rem; margin-bottom: .25rem; }
  .intro { color: #555; margin-top: 0; }
  .glimt { border-top: 1px solid #ddd; padding: 1rem 0; }
  .hold { font-weight: 600; }
  .when { color: #666; font-size: .85rem; }
  .caption { margin: .5rem 0 0; white-space: pre-wrap; overflow-wrap: break-word; }
  /* Every item, side by side where there is room and stacked where there is not. No carousel:
     see the handler comment. */
  .media { display: grid; grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
           gap: .5rem; margin-top: .75rem; }
  .media img { width: 100%; height: auto; border-radius: .25rem; background: #eee; }
  .empty { border: 1px dashed #bbb; border-radius: .5rem; padding: 2rem; text-align: center;
           color: #555; }
  footer { border-top: 1px solid #ddd; margin-top: 2rem; padding-top: 1rem;
           color: #555; font-size: .85rem; }
</style>
</head>
<body>
<h1>Glimt fra Nathejk {{.Year}}</h1>
<p class="intro">
  Billeder som deltagerne selv har valgt at dele offentligt. Der står ikke navne på billederne —
  et glimt vises med patruljen eller klanen, ikke med personen.
</p>

{{if .Unavailable}}
  <p class="empty">Billederne kan ikke vises lige nu. Prøv igen om lidt.</p>
{{else if not .Glimt}}
  <p class="empty">Der er ikke delt nogen offentlige billeder endnu.</p>
{{else}}
  {{range .Glimt}}
  <article class="glimt">
    <p class="hold">{{hold .Hold}}</p>
    <p class="when">{{date .CreatedAt}}</p>
    {{if .Caption}}<p class="caption">{{.Caption}}</p>{{end}}
    {{if .Media}}
    {{$g := .}}
    <div class="media">
      {{range .Media}}
      <!-- The thumbnail, always. This page is read by a lot of people at once on whatever
           connection they have, and a grid of full-size images is the difference between usable
           and not. loading=lazy is a plain attribute, not a script.

           Every item gets its own tag - there is no carousel here. See the handler comment. -->
      <img src="/api/public/glimt/{{$g.ID}}/media/{{.Ordinal}}?variant=thumb"
           alt="Glimt fra {{hold $g.Hold}}" loading="lazy" decoding="async"
           {{if and .Width .Height}}width="{{.Width}}" height="{{.Height}}"{{end}}>
      {{end}}
    </div>
    {{end}}
  </article>
  {{end}}
{{end}}

<!-- No footer here (task 362).
     This page carried a takedown line, the retention period and a privacy link. The maintainer removed them:
     "It's already stated elsewhere and it seems very overwhelming with all these disclaimer everywhere."
     Both statements are still on /{year}/privatliv, and the patrol page keeps the takedown *affordance* —
     the details/summary somebody actually uses when a picture is wrong (task 343). What went is the repetition
     of it on a page that is just photographs. -->
</body>
</html>
`))

// publicHoldLabel is the attribution line, in Danish, matching the app's.
//
// Duplicated from the frontend's `attributionLine` rather than shared, because there is nothing to
// share it through: that one is TypeScript in a bundle this page deliberately does not load. The
// duplication is real and worth naming — if the wording changes in one place it should change in
// both.
func publicHoldLabel(h holdAttribution) string {
	kind := ""
	switch h.Group {
	case "spejder":
		kind = "Patrulje"
	case "bandit":
		kind = "Klan"
	}

	parts := []string{}
	if h.Number != "" {
		parts = append(parts, strings.TrimSpace(kind+" "+h.Number))
	} else if kind != "" {
		parts = append(parts, kind)
	}
	if h.Name != "" && h.Name != h.Number {
		parts = append(parts, h.Name)
	}
	if len(parts) == 0 {
		return "Ukendt hold"
	}
	return strings.Join(parts, " · ")
}
