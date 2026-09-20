package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// routes registers all HTTP handlers on an httprouter. API routes live under
// /api; everything else falls through to the SPA handler so the Vue app's
// client-side router can take over. New resource handlers (auth, push, …) are
// added here, grouped with related routes, as the API grows.
func (app *application) routes() http.Handler {
	router := httprouter.New()

	// Unknown routes: /api/* → JSON 404; anything else → SPA fallback so a hard
	// reload on a client-side route still returns index.html.
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			app.notFoundAPIHandler(w, r)
			return
		}
		app.spaHandler().ServeHTTP(w, r)
	})
	router.MethodNotAllowed = http.HandlerFunc(app.MethodNotAllowedResponse)

	// API routes (JSON).
	router.HandlerFunc(http.MethodGet, "/api/healthcheck", app.healthcheckHandler)
	router.HandlerFunc(http.MethodGet, "/api/config", app.runtimeConfigHandler)
	router.HandlerFunc(http.MethodPost, "/api/auth/request-pin", app.requestPinHandler)
	router.HandlerFunc(http.MethodPost, "/api/auth/verify", app.verifyPinHandler)
	// Second half of login for a phone number shared by several people (task 079).
	// Public like the rest of /auth: it is authorised by the short-lived choice token
	// from /auth/verify, not by a session — there is no session yet.
	router.HandlerFunc(http.MethodPost, "/api/auth/choose", app.chooseHandler)
	// Switching profile within a session (PRD 012). Behind requireAuth, and it mints a choice
	// token for the number the caller is *already* authenticated as — the switch then completes
	// through /auth/choose above, unchanged. No new SMS: see the handler comment and PRD 012 §8.
	router.HandlerFunc(http.MethodPost, "/api/auth/switch", app.requireAuth(app.switchProfileHandler))
	router.HandlerFunc(http.MethodPost, "/api/auth/logout", app.logoutHandler)
	router.HandlerFunc(http.MethodGet, "/api/me", app.requireAuth(app.meHandler))
	// The caller's own details (PRD 003). Session-scoped by construction: there is no
	// user id in the path, so no caller can ask for somebody else's profile.
	router.HandlerFunc(http.MethodGet, "/api/me/profile", app.requireAuth(app.showProfileHandler))
	// PRD 005's guardian-number confirmation. Session-scoped like the profile read: the member
	// is taken from the cookie, so nobody can confirm on somebody else's behalf. Writes no SQL —
	// it publishes a domain event (PRD 008 §8).
	router.HandlerFunc(http.MethodPost, "/api/me/profile/confirm", app.requireAuth(app.confirmProfileHandler))
	// The member supplying a guardian number themselves, when they cannot recognise ours
	// (task 148). Publishes the same event as /confirm, with the two numbers differing.
	router.HandlerFunc(http.MethodPost, "/api/me/profile/guardian", app.requireAuth(app.setGuardianHandler))
	// Giving up on the check (PRD 015, task 228). Publishes the same event as the two above, with
	// no contact number — which is what tells check-in to ask this member. An endpoint rather than
	// a client-side skip, because "nobody recorded anything" is indistinguishable from "never
	// opened the app", and those want different things from the counter.
	router.HandlerFunc(http.MethodPost, "/api/me/profile/skip", app.requireAuth(app.skipProfileCheckHandler))
	// The caller's own portrait (PRD 003). Both are session-scoped: no user id in the
	// path, so neither can be pointed at somebody else's face. Cross-person viewing is
	// PRD 007, with its own access matrix and audit — it must not arrive as a parameter
	// on these.
	router.HandlerFunc(http.MethodPut, "/api/me/photo", app.requireAuth(app.updatePhotoHandler))
	router.HandlerFunc(http.MethodGet, "/api/me/photo", app.requireAuth(app.showPhotoHandler))
	router.HandlerFunc(http.MethodGet, "/api/patrol/scans", app.requireAuth(app.listPatrolScansHandler))
	// The map sheets this patrol has been handed (PRD 016). Patrol-scoped like the scans and checkpoints
	// reads: the read behind it takes only the patrol id, so this route cannot be asked for another
	// team's sheets, and it never names the team a reassigned sheet moved to.
	router.HandlerFunc(http.MethodGet, "/api/patrol/handouts", app.requireAuth(app.listPatrolHandoutsHandler))
	// The caller's own vehicles (PRD 010). Session-scoped like the profile and portrait
	// routes above: no user id in the path, so nobody can read another member's
	// registrations. Scoped by custodianship rather than by who is driving, so a car lent
	// out for a pickup stays with the person who answers for it.
	router.HandlerFunc(http.MethodGet, "/api/me/vehicles", app.requireAuth(app.listOwnVehiclesHandler))
	// Registering one (PRD 010). The custodian is the session's user, never a body field,
	// so nobody can file a car under somebody else's name — which is what task 239's
	// authorisation then rests on. Role-gated server-side: every role except spejder.
	router.HandlerFunc(http.MethodPost, "/api/me/vehicles", app.requireAuth(app.registerVehicleHandler))
	// Editing and withdrawing one (PRD 010). These are the first vehicle routes carrying an
	// id, so they are the first that *can* be pointed at another member's row: both are
	// custodian-only, and a vehicle belonging to somebody else answers 404 rather than 403 so
	// the endpoints cannot be used to discover which registrations exist.
	router.HandlerFunc(http.MethodPatch, "/api/me/vehicles/:id", app.requireAuth(app.updateVehicleHandler))
	router.HandlerFunc(http.MethodDelete, "/api/me/vehicles/:id", app.requireAuth(app.deleteVehicleHandler))
	// Glimt (PRD 019). Media is uploaded one item at a time and the refs returned are then
	// named when the glimt is created — two steps rather than one big multipart request,
	// because a post carries up to ten items from a field on one bar of signal, so a failure
	// should cost one item rather than the whole post (and the client's outbox can resume).
	router.HandlerFunc(http.MethodPost, "/api/glimt/media", app.requireAuth(app.uploadGlimtMediaHandler))
	router.HandlerFunc(http.MethodPost, "/api/glimt", app.requireAuth(app.createGlimtHandler))
	// The feed. Note there is deliberately **no** `/api/glimt/version` — freshness is a `glimt`
	// key on /api/sync above, per the instruction there.
	router.HandlerFunc(http.MethodGet, "/api/glimt/feed", app.requireAuth(app.listGlimtHandler))
	// Browsing by hold (PRD 019 §0a.1) — the post-race surface, and the load peak of the feature.
	// The collection is **oldest first**: a race reads forward in time, which is deliberately the
	// opposite of the feed's order.
	router.HandlerFunc(http.MethodGet, "/api/glimt/hold", app.requireAuth(app.listGlimtHoldsHandler))
	router.HandlerFunc(http.MethodGet, "/api/glimt/hold/:number", app.requireAuth(app.listGlimtByHoldHandler))
	// Media bytes. The visibility check here is the same one the feed applies — a media URL is
	// guessable, so it must not be a bearer token (PRD 019 §8).
	//
	// Under `/items/` rather than `/api/glimt/:glimtId/media/:ordinal`, which is what PRD 019 §8
	// asks for and what httprouter refuses: a wildcard segment cannot sit alongside the static
	// `feed` and `media` siblings (it panics at construction — confirmed while wiring task 305).
	// The contacts pane hit the same wall and answered it the same way, with `/people/:personId`.
	router.HandlerFunc(http.MethodGet, "/api/glimt/items/:glimtId/media/:ordinal", app.requireAuth(app.showGlimtMediaHandler))
	// Deleting is the author's alone. Moderators hide (below) — the person who took the
	// photograph is the only one who gets to destroy it.
	router.HandlerFunc(http.MethodDelete, "/api/glimt/items/:glimtId", app.requireAuth(app.deleteGlimtHandler))
	// Reporting. With no approval queue in front of the public scope (PRD 019 §0) this is the
	// safety mechanism, so it hides the glimt immediately rather than queueing a decision — see
	// the projection's handleReported, which records and hides in one fold.
	router.HandlerFunc(http.MethodPost, "/api/glimt/items/:glimtId/report", app.requireAuth(app.reportGlimtHandler))
	// Team-section moderation (PRD 019 §0). The queue takes no visibility filter — it is the
	// widest read in the service — so the section check inside each handler is the only thing
	// standing between it and every photograph in the event. It is a per-request lookup of the
	// caller's current section, never a session claim, so revoking the assignment revokes access.
	router.HandlerFunc(http.MethodGet, "/api/glimt/moderation", app.requireAuth(app.listGlimtModerationHandler))
	router.HandlerFunc(http.MethodPost, "/api/glimt/items/:glimtId/hide", app.requireAuth(app.hideGlimtHandler))
	router.HandlerFunc(http.MethodPost, "/api/glimt/items/:glimtId/unhide", app.requireAuth(app.unhideGlimtHandler))
	// Album curation: removal (PRD 011 §0b.2, task 335). Gated on the **Team section**, re-checked per
	// request by the same `isGlimtModerator` the glimt moderation routes use — not a second notion of
	// "curator", which would be a second thing to assign and a second thing to get wrong.
	//
	// Authenticated, unlike everything else about albums. That asymmetry is the feature: albums are
	// *read* by the open web and *changed* by the Team section.
	//
	// Where curation as a whole lives is still open (PRD 011 §11 Q5); removal could not wait for that
	// answer, because it is the safety valve behind §0b.2's "permission is withdrawable".
	router.HandlerFunc(http.MethodDelete, "/api/albums/:albumId", app.requireAuth(app.deleteAlbumHandler))
	router.HandlerFunc(http.MethodDelete, "/api/albums/:albumId/items/:ordinal", app.requireAuth(app.removeAlbumItemHandler))
	// The public Glimt page and its API (PRD 019 §0, task 323). Served by this service on
	// hej.nathejk.dk, not exported into the marketing site — one store, one takedown.
	//
	// **Registered without `requireAuth`, and that is the security property rather than a
	// convenience.** A logged-in member's browser *will* send `hej_session` to these routes, and if
	// any of them read it the public page would silently become a different page for members than
	// for parents — at which point "is this public-safe?" stops being a testable question
	// (PRD 019 §8).
	//
	// They cannot read it: `requireAuth` is the **only** place a session enters the request context
	// (middleware.go), so `contextGetSession` in a bare handler returns false unconditionally. Do
	// not wrap these, and do not add a middleware that populates the session globally — that one
	// change would undo this without any test failing except the ones in glimtpublic_test.go, which
	// exist for exactly this.
	//
	// The audience is re-checked per glimt inside the handlers (`publiclyVisible`), so the media
	// route cannot be used to reach a group-scoped item by id.
	router.HandlerFunc(http.MethodGet, "/api/public/glimt", app.listPublicGlimtHandler)
	router.HandlerFunc(http.MethodGet, "/api/public/glimt/:glimtId/media/:ordinal", app.showPublicGlimtMediaHandler)
	// Unauthenticated by design: a parent who spots a problem is exactly the person we want to hear
	// from, and they have no session and will not make one. Rate-limited by IP.
	router.HandlerFunc(http.MethodPost, "/api/public/glimt/:glimtId/report", app.reportPublicGlimtHandler)
	// The server-rendered page. Not under /api, and deliberately **not** an SPA route: it must work
	// with JavaScript disabled on a browser that cannot parse the app's bundle. Registered here
	// rather than left to the SPA fallback, which would serve index.html — and note that the
	// service worker needs a `navigateFallbackDenylist` entry for the same reason, or an installed
	// member following a public link gets the app shell instead (see vite.config.ts).
	router.HandlerFunc(http.MethodGet, "/offentligt/glimt", app.publicGlimtPageHandler)
	// The public site (PRD 011, task 332). Same properties as the glimt page above and for the same
	// reasons — registered bare so no session can be read, server-rendered so no bundle is needed, and
	// outside the SPA fallback so a hard load does not get index.html.
	//
	// Note the ordering constraint httprouter imposes: `/offentligt/patrulje` (the form's target) and
	// `/offentligt/patrulje/:number` (the page) are different routes, not one with an optional segment,
	// which is why the lookup is its own handler rather than the page treating an empty number as "show
	// the form".
	router.HandlerFunc(http.MethodGet, "/offentligt", app.publicFrontpageHandler)
	router.HandlerFunc(http.MethodGet, "/offentligt/album/:slug", app.albumPageHandler)
	router.HandlerFunc(http.MethodGet, "/offentligt/patrulje", app.patrolSearchLookupHandler)
	router.HandlerFunc(http.MethodGet, "/offentligt/patrulje/:number", app.publicPatrolPageHandler)
	// Album media (task 334). Under /api/public/ with the glimt media route rather than under
	// /offentligt/, because it serves bytes rather than a page — and it shares `streamGlimtMedia`, so
	// the ETag handling and the missing-object degradation cannot diverge between the two.
	router.HandlerFunc(http.MethodGet, "/api/public/albums/:albumId/media/:ordinal", app.albumMediaHandler)
	// The contacts directory (PRD 007). Cached by the client and worked from offline, so
	// it carries everything the pane needs and nothing it does not: no guardian numbers, no
	// postal addresses. Spejdere are refused — they do not get this pane, and crew reach
	// them only through the patrol lookup, which is a separate, uncached surface.
	router.HandlerFunc(http.MethodGet, "/api/contacts/manifest", app.requireAuth(app.contactsManifestHandler))
	// The multiplexed freshness check (PRD 017): one request answers "did anything I hold
	// change?" for every dataset this caller has. It replaced a per-dataset version endpoint
	// (`/api/contacts/version`, retired in task 292) — do not add another one; add a key here.
	// Deliberately the cheapest authenticated endpoint in the API — it is called on every
	// foreground by every device, so nothing in it may build a payload.
	router.HandlerFunc(http.MethodGet, "/api/sync", app.requireAuth(app.syncHandler))
	// A directory member's portrait, authorized per request. Under /people/ rather than
	// directly under /contacts/{personId} because httprouter refuses a wildcard segment
	// alongside the static `manifest` and `version` siblings — and the extra segment reads
	// better anyway next to the patrol routes.
	router.HandlerFunc(http.MethodGet, "/api/contacts/people/:personId/photo", app.requireAuth(app.contactsPhotoHandler))
	// The crew-only patrol lookup (PRD 007). Everything about these two is the opposite of
	// the directory above: live, never cached, never a sync dataset, and audited per call —
	// they are the only path by which a spejder's details are reachable in this app. A
	// refusal answers 404, identical to a nonexistent patrol, so the endpoint cannot be used
	// to discover which numbers exist.
	router.HandlerFunc(http.MethodGet, "/api/contacts/patrols/:number", app.requireAuth(app.patrolLookupHandler))
	router.HandlerFunc(http.MethodGet, "/api/contacts/patrols/:number/photo/:personId", app.requireAuth(app.patrolPhotoHandler))
	// Position track ingest (PRD 002 §11.1, task 084). Publishes to the telemetry stream
	// and writes no SQL; the person is taken from the session, never from the body.
	router.HandlerFunc(http.MethodPost, "/api/track", app.requireAuth(app.createTrackHandler))
	// The region the client caches map tiles for. Authenticated deliberately: unlike
	// /api/config it is not public — the event area is not fully known to participants.
	router.HandlerFunc(http.MethodGet, "/api/race-area", app.requireAuth(app.raceAreaHandler))
	// The checkpoints this patrol has earned sight of (PRD 016). Patrol-scoped in the BFF:
	// the read behind it cannot be asked for checkpoints the caller has not been shown, so
	// this route cannot leak a position even if a future edit here got the filtering wrong.
	router.HandlerFunc(http.MethodGet, "/api/checkpoints", app.requireAuth(app.listCheckpointsHandler))
	router.HandlerFunc(http.MethodGet, "/api/push/public-key", app.pushPublicKeyHandler)
	router.HandlerFunc(http.MethodPost, "/api/push/subscription", app.requireAuth(app.createPushSubscriptionHandler))

	// Development-only routes (PRD 014 §8). Registered rather than guarded, so outside
	// ENV=development they do not exist at all — see dev.go.
	if devRoutesEnabled(app.config) {
		router.HandlerFunc(http.MethodGet, "/api/dev/pin", app.devPinHandler)
		// Glimt fixture data (task 327). Authenticated, because it authors the glimt *as* the
		// caller so the group-scoped ones are visible to whoever asked — a fixture nobody can
		// see would be worse than none.
		router.HandlerFunc(http.MethodPost, "/api/dev/glimt-fixture", app.requireAuth(app.devGlimtFixtureHandler))
		// Album fixture data (task 333). Nothing else in the app creates an album — there is no
		// participant-facing path that produces one — so without this the frontpage covers, the album
		// grid, the unpublished-album state and the bounds verdicts cannot be looked at before the
		// curation tool exists (PRD 011 §11 Q5).
		router.HandlerFunc(http.MethodPost, "/api/dev/album-fixture", app.requireAuth(app.devAlbumFixtureHandler))
	}

	return router
}

// spaHandler serves the built single-page app from the configured web root. In
// production this directory holds the compiled Vue bundle; in dev it's a
// placeholder (the Vite dev server serves the real SPA and proxies /api here).
func (app *application) spaHandler() http.Handler {
	root := app.config.webRoot
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	})
}
