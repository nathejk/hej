package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

// The public site: `/offentligt` and the pages under it (PRD 011 §7, task 332).
//
// # What this is
//
// Three sections, in this order: **curated albums → find din patrulje → recent glimt**. Albums first
// because they are the most inviting thing and need no input from the visitor; find-your-patrol second
// because whoever came for that arrived with a number in their hand; the glimt strip last, with a link,
// because the full feed already has its own page (PRD 019) and does not need reproducing.
//
// # Server-rendered, and that is the requirement rather than a preference
//
// No bundle, no framework, no client-side state, and it works with JavaScript disabled. The reason is
// reach: this page exists so a grandparent on a ten-year-old browser can see what the weekend looked
// like. `.rules` puts the *app's* floor at Safari 16.4 / Chrome 111 because the app needs service
// workers and Web Push — none of which this page uses, so none of which it should require.
//
// The pattern is `glimtpublic.go`'s, deliberately: inline CSS with hex colours, thumbnails always,
// `loading="lazy"` as a plain attribute, no carousel, `noindex`. Those decisions were argued once
// (PRD 019, task 323) and are not re-litigated here.
//
// # The session is not merely ignored — it cannot be read
//
// Every route here is registered without `requireAuth`, which is the **only** place a session enters
// the request context (middleware.go). So `contextGetSession` in this file would return false
// unconditionally, and a signed-in member's browser sending `hej_session` gets exactly the page a
// parent gets. See the longer note in glimtpublic.go — this file inherits it wholesale.
//
// # Relationship to /desktop.html (PRD 013)
//
// That page is the event's **manual**: rules, programme, practical information, privacy. This is the
// event's **memory**. They link to each other and duplicate no content. The line to hold when somebody
// asks "and on the website?": the manual carries what does not change during the race, the memory
// carries what happened.

// publicSiteTitle is the site's name, used in every page title and the wordmark.
const publicSiteTitle = "Nathejk"

// publicPageData is what every page in the public site needs.
//
// Embedded by each page's own data struct rather than passed alongside it, so a new page cannot forget
// the year or the navigation and render a header that says nothing.
type publicPageData struct {
	Year string
	// Title is the page's own title, shown after the site name.
	Title string
}

// publicFrontpageData is the frontpage.
type publicFrontpageData struct {
	publicPageData

	// Albums are the curated collections (task 333/334). Empty until there are any, which is a normal
	// state and not an error — the section says so rather than disappearing.
	Albums []publicAlbumSummary

	// Glimt is the recent public glimt strip (task 336).
	Glimt []publicGlimtResponse
	// GlimtUnavailable distinguishes "the feature is down" from "nobody has shared anything", which
	// want different sentences: one is our problem and one is a fact about the event.
	GlimtUnavailable bool

	// SearchError is set when a patrol search could not be used, e.g. a non-numeric entry.
	//
	// Deliberately vague by design. It never says whether a patrol exists — see notYetPatrolPage.
	SearchError string
}

// publicAlbumSummary is one album as the frontpage lists it.
//
// A summary type rather than the album itself: the frontpage needs a cover and a title, and giving the
// template the whole album would let a future edit render a photograph's coordinate onto the frontpage
// without anybody deciding to.
type publicAlbumSummary struct {
	Slug        string
	Title       string
	Description string
	// CoverOrdinal is which item is the cover; CoverAlbumID addresses the media route.
	CoverAlbumID string
	CoverOrdinal int
	HasCover     bool
	Count        int
}

// publicFrontpageHandler renders `/offentligt`.
//
// @Summary      The public frontpage (HTML)
// @Description  The event's public front door: curated photo albums, a lookup for a patrol's own page, and the most recent publicly shared glimt. Server-rendered, no bundle, and complete with JavaScript disabled — the page exists so somebody on an old browser or a desktop can see what the weekend looked like. **Unauthenticated, and it ignores the session cookie entirely**, so a signed-in member sees exactly what a parent sees. Not indexed (robots noindex).
// @Tags         public-site
// @Produce      html
// @Success      200  {string}  string  "the page"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Router       /offentligt [get]
func (app *application) publicFrontpageHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicSiteRead(w, r) {
		return
	}

	data := publicFrontpageData{
		publicPageData: publicPageData{Year: app.config.eventYear},
		Albums:         app.frontpageAlbums(),
		SearchError:    patrolSearchError(r.URL.Query().Get("fejl")),
	}

	// The glimt strip reuses PRD 019's read wholesale — `publicGlimt` applies the audience filter, the
	// hidden flag and the public retention cutoff in one place. A second query here would be a second
	// chance to forget one of them, and this is the copy nobody would think to test for a leak.
	rows, err := app.publicGlimt(publicFrontpageGlimtLimit, 0)
	if err != nil {
		// Logged, not fatal: a frontpage missing its glimt strip is still worth serving, because the
		// albums and the patrol lookup are unaffected. Reporting the whole page as broken because one
		// section is would be a worse trade than the section saying so.
		app.Logger.Error("reading glimt for the public frontpage", "err", err)
		data.GlimtUnavailable = true
	}
	for _, g := range rows {
		data.Glimt = append(data.Glimt, newPublicGlimtResponse(g))
	}

	app.renderPublicPage(w, "frontpage", data)
}

// publicFrontpageGlimtLimit is how many glimt the strip shows.
//
// Much smaller than the full page's 60: this is a taste of the feed with a link to the rest, not a
// second copy of it. Nine fills three rows of three on a desktop and three on a phone, so the strip
// ends on a complete row at both widths rather than with a ragged gap.
const publicFrontpageGlimtLimit = 9

// patrolSearchLookupHandler turns a submitted patrol number into a redirect to that patrol's page.
//
// # Why a redirect and not a page
//
// A no-JavaScript form can only submit to a fixed action with a query string, while the patrol page's
// address is a path (`/offentligt/patrulje/42`) — chosen because it is memorable and sendable by voice
// (PRD 011 §11 Q2). This is the one-line bridge between those two facts. A `303` so the browser follows
// with a GET and the resulting URL is the shareable one, which is the whole point of the path form.
//
// # It must not answer differently for a patrol that does not exist
//
// This handler deliberately does **no** lookup. It normalises and redirects, and the patrol page itself
// answers identically for "not yet", "no such patrol" and "we cannot tell" (see notYetPatrolPage). A
// validity check here would reintroduce exactly the distinction that page works to remove, on the one
// route a visitor can hammer with guesses.
//
// @Summary      Find a patrol's public page
// @Description  Redirects to /offentligt/patrulje/{number} for a submitted patrol number. Exists because a form without JavaScript submits a query string while the page's address is a path. **It performs no lookup and reveals nothing**: an unknown number redirects exactly like a known one, and the destination answers identically for a patrol that does not exist, one that has not finished, and one we cannot resolve.
// @Tags         public-site
// @Produce      html
// @Param        nummer  query     string  true  "patrol number"
// @Success      303  {string}  string  "redirect to the patrol's page"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Router       /offentligt/patrulje [get]
func (app *application) patrolSearchLookupHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicSiteRead(w, r) {
		return
	}

	number, ok := normalizePatrolNumber(r.URL.Query().Get("nummer"))
	if !ok {
		// Back to the frontpage with a flag rather than an error page: the visitor mistyped, and the
		// form they need is on the page they came from. The flag names the *form's* problem, never the
		// patrol's.
		http.Redirect(w, r, "/offentligt?fejl=nummer", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/offentligt/patrulje/"+number, http.StatusSeeOther)
}

// normalizePatrolNumber trims and validates what someone typed into the lookup form.
//
// Digits only, bounded in length. Not because patrol numbers are digits by law — `teamNumber` is a
// VARCHAR upstream, and the person projection's comment records that it is ordered by length *then*
// value precisely because of that — but because this string is about to become a path segment on an
// unauthenticated route. Accepting arbitrary text here would mean reflecting arbitrary text into a
// redirect, and the narrow rule costs nothing: a patrol whose number is not digits can still be reached
// by its URL directly.
func normalizePatrolNumber(raw string) (string, bool) {
	number := strings.TrimSpace(raw)
	if number == "" || len(number) > 8 {
		return "", false
	}
	for _, r := range number {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	// Leading zeros dropped so "042" and "42" reach one page rather than two, which also keeps the
	// canonical URL the one printed on the patrol's sign. All-zeros collapses to a single "0" rather
	// than to the empty string — otherwise "00" and "0" would be two different URLs, and one of them
	// would be a path with an empty segment.
	if trimmed := strings.TrimLeft(number, "0"); trimmed != "" {
		number = trimmed
	} else {
		number = "0"
	}
	return number, true
}

// patrolSearchError maps the frontpage's error flag to a sentence.
//
// A flag rather than the message itself in the query string: a message would be text from the URL
// rendered into the page, which is a reflected-content problem on an unauthenticated route even with
// `html/template` escaping it. An unknown flag yields no message at all.
func patrolSearchError(flag string) string {
	if flag == "nummer" {
		return "Skriv patruljens nummer med tal."
	}
	return ""
}

// A note on the patrol page, which lives in patrolpage.go.
//
// Task 332 registered `/offentligt/patrulje/:number` here answering the not-yet page for every number, and
// task 341 moved the handler out once there was something to gate. What stayed behind is
// `renderPatrolNotYet` — the single closed answer, called from every refusal, which is the property that
// keeps "not yet", "no such patrol" and "we cannot tell" indistinguishable.

// renderPatrolNotYet is the single closed-gate answer.
//
// Status 200 with a real page, not a 404. The reasoning differs from the crew patrol lookup's, which
// answers 404 so a refusal cannot be told from a nonexistent patrol: there, the caller is an
// authenticated crew member and the response is JSON. Here the caller is a parent who followed a link,
// and "404" in a browser reads as *broken*, which would send them to a leader to ask why. So every
// number gets the same friendly page — which achieves the same indistinguishability by making the
// answers equal rather than by making them both errors.
func (app *application) renderPatrolNotYet(w http.ResponseWriter) {
	app.renderPublicPage(w, "patrol-notyet", publicPageData{
		Year:  app.config.eventYear,
		Title: "Patruljens side",
	})
}

// allowPublicSiteRead applies the by-IP read limit for pages and JSON.
//
// Shares `publicGlimtReadLimiter` deliberately, rather than introducing a second page budget: it is the same
// anonymous audience arriving at the same origin for the same kind of response, and the reason that limiter
// is keyed by IP and set generously — a school or workplace behind one NAT shares a budget — applies
// identically here.
//
// **Media is the one thing that does not share it** (task 347): see `allowPublicMediaRead`, because sixty
// thumbnails per album page would otherwise spend the allowance the next page needs.
func (app *application) allowPublicSiteRead(w http.ResponseWriter, r *http.Request) bool {
	return app.allowPublicGlimtRead(w, r)
}

// allowPublicMediaRead applies the by-IP limit for public media bytes.
//
// Its own budget, an order of magnitude above the page limit, because the request counts differ by an order
// of magnitude: an album page is one HTML response and up to sixty thumbnails. The numbers, and the NAT'd
// school they are sized for, are in env.go at `publicMediaReadsPerMinute`.
//
// The message is Danish and readable for the same reason every other limit here is: this is a route a
// grandparent reaches by following a link, and a bare 429 in a browser reads as *broken*.
func (app *application) allowPublicMediaRead(w http.ResponseWriter, r *http.Request) bool {
	if app.publicMediaReadLimiter == nil || app.publicMediaReadLimiter.Allow(clientIP(r)) {
		return true
	}
	app.RateLimitMessageResponse(w, r, "For mange billeder på én gang. Prøv igen om lidt.")
	return false
}

// renderPublicPage executes one of the site's templates with the shared layout.
//
// Centralised so no page can forget the headers. Two matter:
//
//   - `noindex, nofollow`, following PRD 019's reasoning: participants shared these photographs
//     publicly, which is not the same as asking to be findable by name in a search engine years later.
//     The patrol pages inherit it for the same reason.
//   - `max-age=60`, the same short window the glimt page chose: long enough to absorb the
//     morning-after burst, short enough that a takedown lands quickly. Task 335 depends on that bound,
//     so it must not be lengthened without reading it.
func (app *application) renderPublicPage(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")

	if err := publicSiteTemplates.ExecuteTemplate(w, name, data); err != nil {
		// The response has already begun, so there is nothing to answer with. Logged, as the glimt page
		// does for the same reason.
		app.Logger.Error("executing a public site template", "template", name, "err", err)
	}
}

// publicSiteFuncs are the template helpers shared by every page here.
//
// `hold` is `publicHoldLabel`, the same attribution the glimt page uses — shared rather than
// re-implemented, because two renderings of "Patrulje 42 · Ørnene" would eventually disagree and the
// public surface is where that would be most visible.
var publicSiteFuncs = template.FuncMap{
	"hold": publicHoldLabel,
	"date": publicDanishDateTime,
}

// publicDanishDateTime renders an instant the way the public pages say it.
//
// Danish month names and a 24-hour clock, matching the glimt page's format so the two surfaces do not
// spell a date two ways. Takes a pointer as well as a value through the template's own nil handling: a nil
// `*time.Time` never reaches here, because every call site guards on the field being set.
func publicDanishDateTime(t time.Time) string {
	months := []string{
		"januar", "februar", "marts", "april", "maj", "juni",
		"juli", "august", "september", "oktober", "november", "december",
	}
	return fmt.Sprintf("%d. %s %d kl. %02d:%02d",
		t.Day(), months[int(t.Month())-1], t.Year(), t.Hour(), t.Minute())
}

// publicSiteTemplates is the whole site: one layout plus one template per page.
//
// # Why a shared layout and not a page each
//
// `glimtpublic.go` renders one self-contained page, which was right when there was one. There are now
// four (frontpage, album, patrol, not-yet) and they must agree on the wordmark, the footer, the privacy
// link and the CSS. Four copies of that would diverge, and the first thing to diverge would be the
// footer's takedown line — the one piece of text on this surface somebody needs when something is
// wrong.
//
// The existing glimt page is deliberately **not** migrated here as part of this task. It works, it is
// tested, and rewriting a shipped public page to prove a point about layout sharing is how a refactor
// becomes an outage. If it is ever touched for another reason, this is where it should land.
//
// # The CSS is inline, in hex, with no modern syntax
//
// A separate stylesheet is a second request that can fail. Hex rather than `oklch()`, no nesting, no
// custom-property fallbacks — the constraints task 204 established against a 2013 iPad, which is
// precisely the device this page exists for.
var publicSiteTemplates = template.Must(template.New("publicsite").Funcs(publicSiteFuncs).Parse(`
{{define "layout-head"}}<!DOCTYPE html>
<html lang="da">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .Title}}{{.Title}} · {{end}}` + publicSiteTitle + ` {{.Year}}</title>
<meta name="robots" content="noindex, nofollow">
<style>
  :root { color-scheme: light dark; }
  body { font-family: system-ui, sans-serif; margin: 0; padding: 1rem;
         max-width: 60rem; margin-inline: auto; line-height: 1.5; }
  a { color: #1d4ed8; }
  /* The wordmark and page titles use the Nathejk font per .rules. It is loaded by the app's CSS,
     which this page does not load — so the stack degrades to the narrow-bold fallbacks, which is
     what the app's own --font-nathejk falls back to anyway. Stated rather than left to look like
     an oversight. */
  .wordmark, h1, h2 { font-family: Impact, "Haettenschweiler", "Arial Narrow Bold", sans-serif;
         letter-spacing: .01em; }
  .wordmark { display: block; font-size: 1.1rem; text-transform: uppercase; color: #64748b;
         text-decoration: none; margin-bottom: .75rem; }
  h1 { font-size: 1.9rem; margin: 0 0 .25rem; }
  h2 { font-size: 1.3rem; margin: 0 0 .5rem; }
  .intro { color: #555; margin-top: 0; }
  section { border-top: 1px solid #ddd; padding: 1.25rem 0; }
  .empty { border: 1px dashed #bbb; border-radius: .5rem; padding: 1.5rem; text-align: center;
         color: #555; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
         gap: .75rem; }
  .card { display: block; color: inherit; text-decoration: none; }
  .card img { width: 100%; height: auto; border-radius: .25rem; background: #eee; display: block; }
  .card .name { font-weight: 600; margin-top: .35rem; }
  .card .meta { color: #666; font-size: .85rem; }
  .thumbs { display: grid; grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr)); gap: .5rem; }
  .thumbs img { width: 100%; height: auto; border-radius: .25rem; background: #eee; display: block; }
  /* One photograph per row on a phone, two where there is room. Wider than the thumbnail grid on
     purpose: an album is for looking at, so the pictures get the space. */
  .photos { display: grid; grid-template-columns: repeat(auto-fit, minmax(18rem, 1fr));
         gap: 1rem; }
  .photos figure { margin: 0; }
  .photos img { width: 100%; height: auto; border-radius: .25rem; background: #eee; display: block; }
  .photos figcaption { color: #555; font-size: .9rem; margin-top: .35rem; }
  /* The patrol page's header: who they are on the left, the diploma on the right. Flex rather than
     grid so it collapses to one column on a narrow screen without a media query. */
  .patrolhead { display: flex; flex-wrap: wrap; gap: 1rem; align-items: flex-start;
         justify-content: space-between; }
  .patrolhead .who { flex: 1 1 18rem; }
  .patrolhead .group { color: #555; margin: .1rem 0 0; }
  .patrolhead .distance { font-size: 1.4rem; font-weight: 600; margin: .6rem 0 0; }
  .patrolhead .finished { color: #555; margin: .2rem 0 0; font-size: .9rem; }
  .diploma { flex: 0 0 9rem; border: 1px solid #ddd; border-radius: .25rem; background: #fafafa;
         aspect-ratio: 1 / 1.414; display: flex; align-items: center; justify-content: center; }
  .diploma .soon { text-align: center; color: #94a3b8; font-size: .9rem; margin: 0; }
  .caveat { color: #555; font-size: .85rem; margin: .5rem 0 0; }
  /* The map and the caveat that explains it are **hidden until the island draws** (task 342). A 22rem grey
     box with nothing in it is a broken frame, and a caveat about a route nobody can see is noise — so
     publicmap.js adds .ready to both, and a visitor without JavaScript sees neither. */
  .maparea { display: none; height: 22rem; border-radius: .25rem; background: #eee; }
  .maparea.ready { display: block; }
  .mapcaveat { display: none; }
  .mapcaveat.ready { display: block; }
  /* The photograph pin (task 342). A white-ringed amber square, so it is a different shape as well as a
     different colour from the round scan dots — legible on both the topographic and the aerial base
     layer, which is why the ring is there at all. */
  .photopin { background: #b45309; border: 2px solid #fff; border-radius: .15rem;
         box-shadow: 0 0 0 1px rgba(0,0,0,.35); }
  .scans { margin: 0; padding-left: 1.2rem; }
  .scans li { margin-bottom: .35rem; }
  .scans .what { font-weight: 600; }
  .scans .when { color: #666; font-size: .85rem; margin-left: .4rem; }
  .scans .nopos { color: #94a3b8; font-size: .8rem; margin-left: .4rem; }
  .find label { display: block; font-weight: 600; margin-bottom: .35rem; }
  .find input { font-size: 1.1rem; padding: .5rem; width: 8rem; border: 1px solid #94a3b8;
         border-radius: .25rem; }
  .find button { font-size: 1.1rem; padding: .5rem 1rem; border: 0; border-radius: .25rem;
         background: #1d4ed8; color: #fff; cursor: pointer; }
  .find .hint { color: #666; font-size: .85rem; margin-top: .5rem; }
  .find .problem { color: #b91c1c; font-size: .9rem; margin-top: .5rem; }
  .more { margin-top: .75rem; }
  /* The takedown form (task 343). Quiet by default — a <details> — so the page is not led by an apology,
     but full width and full size once opened: this is the form somebody upset is filling in on a phone. */
  .report { margin-top: 1.5rem; }
  .report summary { cursor: pointer; color: #1d4ed8; }
  .report label { display: block; font-weight: 600; margin: .75rem 0 .35rem; }
  .report textarea { width: 100%; box-sizing: border-box; font: inherit; padding: .5rem;
         border: 1px solid #94a3b8; border-radius: .25rem; }
  .report button { margin-top: .5rem; font-size: 1.1rem; padding: .5rem 1rem; border: 0;
         border-radius: .25rem; background: #1d4ed8; color: #fff; cursor: pointer; }
  .report .thanks { background: #f0fdf4; border: 1px solid #bbf7d0; border-radius: .25rem;
         padding: .6rem .75rem; color: #166534; }
  footer { border-top: 1px solid #ddd; margin-top: 2rem; padding-top: 1rem;
         color: #555; font-size: .85rem; }
</style>
</head>
<body>
<a class="wordmark" href="/offentligt">` + publicSiteTitle + ` {{.Year}}</a>
{{end}}

{{define "layout-foot"}}
<footer>
  <p>
    Er der noget her, der ikke skal ligge offentligt? Skriv til os, så tager vi det ned.
  </p>
  <p>
    <a href="/privatliv">Data og privatliv</a> ·
    <a href="/desktop.html">Om Nathejk, regler og program</a>
  </p>
</footer>
</body>
</html>
{{end}}

{{define "frontpage"}}{{template "layout-head" .}}
<h1>Nathejk {{.Year}}</h1>
<p class="intro">
  Billeder fra løbet, og patruljernes egne sider med deres rute. Du behøver ikke logge ind.
</p>

<section>
  <h2>Billeder</h2>
  {{if .Albums}}
  <div class="grid">
    {{range .Albums}}
    <a class="card" href="/offentligt/album/{{.Slug}}">
      {{if .HasCover}}
      <img src="/api/public/albums/{{.CoverAlbumID}}/media/{{.CoverOrdinal}}?variant=thumb"
           alt="{{.Title}}" loading="lazy" decoding="async">
      {{end}}
      <span class="name">{{.Title}}</span>
      {{if .Description}}<span class="meta">{{.Description}}</span>{{end}}
      {{if .Count}}<span class="meta">{{.Count}} billeder</span>{{end}}
    </a>
    {{end}}
  </div>
  {{else}}
  <p class="empty">Der er ikke lagt billeder op endnu.</p>
  {{end}}
</section>

<section class="find">
  <h2>Find din patrulje</h2>
  <form method="get" action="/offentligt/patrulje">
    <label for="nummer">Patruljens nummer</label>
    <input id="nummer" name="nummer" type="text" inputmode="numeric" autocomplete="off"
           maxlength="8">
    <button type="submit">Vis siden</button>
  </form>
  {{if .SearchError}}<p class="problem">{{.SearchError}}</p>{{end}}
  <p class="hint">
    Patruljens side kommer frem, når patruljen er i mål — og senest når løbet er slut. Den viser holdets
    rute, hvor de blev scannet, og hvor langt de gik.
  </p>
</section>

<section>
  <h2>Glimt</h2>
  {{if .GlimtUnavailable}}
  <p class="empty">Billederne kan ikke vises lige nu. Prøv igen om lidt.</p>
  {{else if .Glimt}}
  <div class="thumbs">
    {{range $g := .Glimt}}{{range .Media}}
    <img src="/api/public/glimt/{{$g.ID}}/media/{{.Ordinal}}?variant=thumb"
         alt="Glimt fra {{hold $g.Hold}}" loading="lazy" decoding="async"
         {{if and .Width .Height}}width="{{.Width}}" height="{{.Height}}"{{end}}>
    {{end}}{{end}}
  </div>
  <p class="more"><a href="/offentligt/glimt">Se alle glimt</a></p>
  {{else}}
  <p class="empty">Der er ikke delt nogen offentlige billeder endnu.</p>
  {{end}}
</section>
{{template "layout-foot" .}}{{end}}

{{define "album"}}{{template "layout-head" .}}
<h1>{{.Album.Title}}</h1>
{{if .Album.Description}}<p class="intro">{{.Album.Description}}</p>{{end}}

{{if .Items}}
{{$album := .Album}}
<div class="photos">
  {{range .Items}}
  <figure>
    <!-- The thumbnail, always: this page is read by a lot of people at once on whatever connection
         they have. Every item gets its own tag — there is no carousel here, so every photograph is
         reachable on a desktop without a swipe and without script. See the handler comment. -->
    <img src="/api/public/albums/{{$album.ID}}/media/{{.Ordinal}}?variant=thumb"
         alt="{{if .Caption}}{{.Caption}}{{else}}Billede fra {{$album.Title}}{{end}}"
         loading="lazy" decoding="async"
         {{if and .Width .Height}}width="{{.Width}}" height="{{.Height}}"{{end}}>
    {{if .Caption}}<figcaption>{{.Caption}}</figcaption>{{end}}
  </figure>
  {{end}}
</div>
{{else}}
<p class="empty">Der er ingen billeder i dette album.</p>
{{end}}

<p class="more"><a href="/offentligt">Tilbage til forsiden</a></p>
{{template "layout-foot" .}}{{end}}

{{define "patrol"}}{{template "layout-head" .}}
<div class="patrolhead">
  <div class="who">
    <h1>Patrulje {{.Patrol.Number}}{{if .Patrol.Name}} · {{.Patrol.Name}}{{end}}</h1>
    {{if or .Patrol.GroupName .KorpsLabel}}
    <p class="group">
      {{.Patrol.GroupName}}{{if and .Patrol.GroupName .KorpsLabel}} · {{end}}{{.KorpsLabel}}
    </p>
    {{end}}
    {{if .Distance}}
    <p class="distance">{{.Distance}}</p>
    {{if .DistanceIncomplete}}
    <p class="caveat">
      Nogle af jeres registreringer havde ingen position, så tallet er lavere end det I gik.
    </p>
    {{end}}
    {{end}}
    {{if .FinishedLabel}}<p class="finished">I mål {{.FinishedLabel}}</p>{{end}}
  </div>

  {{if .HasDiploma}}
  <!-- The diploma slot. Rendered only where the patrol finished: a page the backstop opened has nothing
       to certify, and an empty frame would point at what is missing (task 346). The artwork itself is
       task 345 — until then the slot is a link, not an image, so nothing here pretends to be a diploma. -->
  <div class="diploma">
    <p class="soon">Diplom<br><span>kommer</span></p>
  </div>
  {{end}}
</div>

<section>
  <h2>Kortet</h2>
  <!-- The map is a progressive enhancement (task 342). This paragraph is what a visitor without it sees,
       and it is also what the page says *about* the track — see the handler comment: a gap in a recorded
       route means the phone was in a pocket, not that the patrol stood still. -->
  {{if .TrackAbsent}}
  <p class="empty">
    Der er ingen rute at vise. Appen optager kun, mens den er åben, så mange patruljer har ingen eller
    kun lidt rute — det er helt normalt.
  </p>
  {{else if .TrackSegments}}
  <div id="patrolmap" class="maparea" data-patrol="{{.Patrol.Number}}"></div>
  <p class="caveat mapcaveat">
    Ruten viser, hvor en telefon havde appen åben — ikke hele vejen I gik. Ligger telefonen i lommen,
    holder optagelsen pause, så ruten har huller. Det er ikke fejl.
  </p>
  <!-- The map is a **progressive enhancement** (PRD 011 §8, task 342), and these assets are the only
       script on the public site.

       Everything above and below renders without them: the scan list, the distance, the header. Where
       they do not load — JavaScript off, a browser too old for Leaflet, an asset blocked — the map
       container and its caveat stay hidden (see .maparea in the stylesheet) and the scan list carries the
       same information as a list.

       The defer attribute keeps them from blocking the page, and same-origin means there is no third
       party in a position to log who looked at which patrol's route. See scripts/vendor-leaflet.sh for why
       Leaflet is vendored rather than fetched from a CDN. -->
  <link rel="stylesheet" href="/vendor/leaflet.css">
  <link rel="stylesheet" href="/vendor/MarkerCluster.css">
  <link rel="stylesheet" href="/vendor/MarkerCluster.Default.css">
  <script src="/vendor/leaflet.js" defer></script>
  <script src="/vendor/leaflet.markercluster.js" defer></script>
  <script src="/publicmap.js" defer></script>
  {{end}}
</section>

<section>
  <h2>Undervejs</h2>
  {{if .Scans}}
  <ol class="scans">
    {{range .Scans}}
    <li>
      <span class="what">{{.Label}}</span>
      <span class="when">{{date .At}}</span>
      {{if not .Plottable}}<span class="nopos">ikke på kortet</span>{{end}}
    </li>
    {{end}}
  </ol>
  {{if .UnplottableScans}}
  <p class="caveat">
    En post kan registrere en patrulje i hånden. De registreringer har ingen position, så de står på
    listen men ikke på kortet.
  </p>
  {{end}}
  {{else}}
  <p class="empty">Der er ingen registreringer på denne patrulje.</p>
  {{end}}
</section>

<!-- **The takedown route** (task 343). A plain form, so it works with JavaScript off, and a <details> so
     it is present without shouting: a visitor looking for it finds it, and a family reading the page is
     not greeted by a page apologising for itself.

     It reports rather than removes — see cmd/api/patrolreport.go for why one anonymous request must not
     take a patrol's page down. The wording says so, because a promise of "immediately" that we do not keep
     is worse than an honest "we look at it". -->
<section class="report">
  {{if .Reported}}
  <p class="thanks">
    Tak. Vi har fået din besked og kigger på den.
  </p>
  {{end}}
  <details>
    <summary>Er der noget på denne side, der ikke skal ligge her?</summary>
    <p>
      Skriv til os, så kigger vi på det. Det kan være en registrering, der ikke er jeres, en rute, der ser
      forkert ud, eller noget helt tredje. Du behøver ikke skrive dit navn.
    </p>
    <form method="post" action="/offentligt/patrulje/{{.Patrol.Number}}/anmeld">
      <label for="reason">Hvad er der galt? (frivilligt)</label>
      <textarea id="reason" name="reason" rows="4" maxlength="2000"></textarea>
      <button type="submit">Send besked</button>
    </form>
  </details>
</section>

<p class="more"><a href="/offentligt">Tilbage til forsiden</a></p>
{{template "layout-foot" .}}{{end}}

{{define "patrol-notyet"}}{{template "layout-head" .}}
<h1>Patruljens side</h1>
<p class="intro">
  Den er ikke klar endnu.
</p>
<p>
  Hver patrulje får sin egen side, når de er i mål — og når løbet er slut, får alle patruljer deres.
  Den viser holdets rute på kortet, hvor de blev scannet undervejs, og hvor langt de gik.
</p>
<p>
  Prøv igen efter løbet. Tjek også, at nummeret er skrevet rigtigt.
</p>
<p class="more"><a href="/offentligt">Tilbage til forsiden</a></p>
{{template "layout-foot" .}}{{end}}
`))
