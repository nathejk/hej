package main

import (
	"flag"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// config holds all runtime configuration. Per go-bff-layout, configuration is
// read from environment variables here (with sensible dev defaults) and passed
// down through this struct — never read from os.Getenv deeper in the tree.
type config struct {
	port    int
	env     string
	webRoot string // directory containing the built SPA (served in production)

	// Session cookie signing secret and Secure flag. The secret MUST be
	// overridden in production (docker-compose.override.yml); the dev default is
	// intentionally insecure. sessionSecure toggles the cookie Secure attribute.
	sessionSecret string
	sessionSecure bool

	// Web Push VAPID keys. The public key is served to the client; the private
	// key is a secret used to sign push messages (delivery is a later PRD).
	// Empty by default — push is simply unavailable until they are set.
	vapidPublicKey  string
	vapidPrivateKey string

	// Dataforsyningen quota key for the map's WMS base layers, served to the
	// client at runtime via /api/config. Not a credential — it reaches the
	// browser either way — but kept out of the bundle so the same image can be
	// deployed with a different key, and out of git so our quota isn't shared.
	dataforsyningenToken string

	// showBuildId toggles the build identifier overlaid on the bottom nav. It is a
	// diagnostic, not a feature: an installed PWA can sit on a stale service
	// worker, so "which build is this phone running?" is otherwise unanswerable from
	// the device — and a test result against the wrong build proves nothing.
	//
	// Served at runtime rather than baked in by Vite so the same published image can
	// have it on for a test deployment and off for the real event. Defaults to on
	// everywhere except production, which is where it would be noise.
	//
	// Note it only controls the nav overlay. The privacy page shows the build id
	// unconditionally, because a user reporting a problem needs to be able to read it
	// back to us.
	showBuildId bool

	// showLayoutDebug overlays viewport, safe-area inset and shell-geometry values on
	// the app. Strictly a diagnostic for the iOS standalone layout questions that
	// cannot be answered from a screenshot — whether a blank strip is our layout
	// reserving space or iOS drawing its own chrome, which is otherwise pure guesswork.
	//
	// A runtime flag rather than a `?debug=` URL parameter because the manifest's
	// start_url is "/": launching from the home screen drops any query string, so a
	// URL-based switch can never be on in the one mode that needs it.
	//
	// Defaults off everywhere, including development — unlike showBuildId this is
	// genuinely noisy, so it is opt-in with SHOW_LAYOUT_DEBUG=true.
	showLayoutDebug bool

	// installGate switches PRD 005's install-first gate on or off at runtime: the device
	// classification, the install wall redirect and the onboarding redirect, all three.
	//
	// **A kill switch, not a feature toggle.** The failure mode it exists for is
	// participants unable to reach a safety app: an over-eager device classification, or an
	// unreliable `display-mode` in some webview, and a member standing in a forest cannot
	// get to the map, the SOS page or their contacts. A redeploy is not an acceptable
	// response time for that during an event, so this has to be flippable from the server.
	//
	// Defaults **on**, including in development: a gate that is off by default is a gate
	// nobody tests, and the whole point of PRD 005 is that the installed app is the only
	// supported way to use this.
	installGate bool

	// syncIntervalSeconds and syncDebounceSeconds tune the multiplexed freshness check (PRD 017):
	// how often a client re-checks while the app is open, and the minimum gap between checks.
	//
	// Both are **operational levers, not constants**, and served in the `/api/sync` response itself
	// rather than only in `/api/config`. The reason is response time: an operator shedding load at
	// 02:00 needs the change to take effect on the next check, on every device, without waiting for
	// anybody to refetch config.
	//
	// The interval is set by the strictest dataset rather than by the average — scans should surface
	// inside a minute — and everything else rides along for free, because one multiplexed check costs
	// the same whether one dataset changed or none did.
	//
	// Zero interval disables the periodic check and **nothing else**: foreground, reconnect and
	// manual checks keep running, so "poll less" can never silently become "stop updating". Zero
	// debounce disables the debounce, which is the right behaviour for a test and the wrong one for
	// an event.
	syncIntervalSeconds int
	syncDebounceSeconds int

	// MariaDB connection. dbDSN is a go-sql-driver/mysql DSN; empty means "run
	// without a database", which is a legitimate mode: everything served today
	// comes from mocks (PRD 008 is what introduces persistence), so a missing DSN
	// must degrade rather than refuse to boot.
	//
	// The pool bounds are deliberately explicit. Go's default MaxOpenConns is 0
	// (unlimited) while MariaDB's default max_connections is 151, so an unbounded
	// pool turns a traffic spike into "too many connections" for every other
	// consumer of the same server.
	dbDSN             string
	dbMaxOpenConns    int
	dbMaxIdleConns    int
	dbConnMaxLifetime time.Duration
	// dbConnectTimeout bounds the startup ping: we retry within this window and
	// then give up rather than blocking forever. See openDB.
	dbConnectTimeout time.Duration

	// NATS JetStream DSN. The broker is a shared org-level service (owned by the
	// `nathejk` repo, reachable on the external `jetstream` network), not something
	// this repo runs. Empty means "run without a broker", which is deliberately
	// survivable: reads come from SQL projections, so a broker outage degrades the
	// app rather than taking it down (PRD 008 §5). Env var name matches the sibling
	// repos so operators do not have to learn a second one.
	jetstreamDSN string

	// smsDSN decides how login PINs are delivered, and is the *only* thing that
	// decides it: empty (the dev default) logs the message and sends nothing, a
	// `cpsms://<key>@api.cpsms.dk` DSN sends for real. Keeping it a single DSN
	// means a developer cannot accidentally text a participant by setting some
	// unrelated flag, and production cannot silently fall back to logging PINs —
	// an invalid DSN refuses to boot. Env var name matches the sibling repos.
	smsDSN string

	// blobPath is the directory for content-addressed binary objects — portrait
	// bytes today (PRDs 003/007). Empty keeps them in memory, which is fine for
	// tests and for running the API before portraits exist, but means they do not
	// survive a restart. The production choice between a mounted volume and object
	// storage is still open (PRD 008 §11 Q4); this is the volume half.
	blobPath string

	// photoBaseURL is where foto serves photograph objects, e.g. https://foto.nathejk.dk (task 361).
	//
	// # Why this app talks to another service at all
	//
	// It does not, per request. The `photographed` event carries refs rather than bytes, and a diploma is
	// rendered server-side, so the bytes have to be in this process. They are fetched **once per photograph**
	// and kept in this app's own blob store (`internal/photobytes`), on the maintainer's instruction.
	//
	// Verifiable rather than trusted: both stores key objects by the sha256 of their contents, so a fetch is
	// checked against the ref the event named before anything is stored.
	//
	// **Empty disables photographs on diplomas**, which is the safe default for an environment with no foto
	// reachable — the certificate renders without a picture, exactly as it does for a patrol nobody
	// photographed.
	//
	// # It shares hq's name but not its meaning
	//
	// hq has a `PHOTO_BASEURL` too, and it is a **browser-facing** URL: hq hands it to a client and never fetches
	// a byte itself. This one has to be reachable **from this container**, because this app renders the PDF.
	//
	// That difference is not academic — it is the first thing that went wrong. Copying hq's dev value verbatim
	// gave `https://foto.local.nathejk.dk`, which inside the container resolves to its own loopback:
	// `dial tcp 127.0.0.1:443: connect: connection refused`, and every diploma quietly rendered without a
	// photograph. The failure was invisible except as a log line, because that is the designed behaviour for an
	// unreachable foto.
	photoBaseURL string

	// publicAlbums shows or hides the curated photo albums on the public site (task 359).
	//
	// # A content switch, not a kill switch
	//
	// The maintainer's instruction on 2026-09-21, preparing the first production deploy of the public pages:
	// *"hide both, we have no photos at the moment — i want to get the rest in prod"*. The patrol pages, the
	// lookup and the glimt strip are ready; curated albums are a promise with nothing behind it yet, and a
	// heading over a dashed "nothing here yet" box is worse than no heading.
	//
	// So this is deliberately **not** the same thing as an empty album list. Empty means "nobody has uploaded
	// yet" and the section says so. Off means the feature is not part of the site: no section, no heading, and
	// `/{year}/album/{slug}` answers 404, so hiding it hides it from a shared link and a crawler too rather
	// than only from the frontpage.
	//
	// Defaults **on**, like every other flag here: a feature that is off by default is a feature nobody tests,
	// and the albums are a shipped feature with tests and a dev fixture. Production turns it off until there
	// are photographs — see docker-compose.prod.yml — and turning it back on is an env change, not a release,
	// which is the point of it being configuration.
	publicAlbums bool

	// adminUser and adminPassword are the credential for the photographer admin tool (PRD 022 §8.2).
	//
	// # This is the only inbound credential in the service, and that is deliberate
	//
	// Everything else authenticated here goes through `requireAuth`: an HMAC-signed session cookie obtained
	// by SMS PIN, authorized by a per-request lookup in the `person` projection. No bearer token, no API
	// key, no role. Adding a shared password is a genuine exception and is recorded as one.
	//
	// The session mechanism was considered and does not fit. It requires the curator to be a person in this
	// year's `person` projection, with a phone number we hold, reachable by SMS, and assigned to the right
	// Team section upstream. Our photographers are frequently none of those: volunteers with a camera,
	// sometimes not registered as personnel at all — and the tool has to work on the Tuesday after the event,
	// when the SMS pipeline is the last thing anybody wants in the loop.
	//
	// # What it costs, written down rather than discovered
	//
	//   - **No attribution.** A shared credential cannot honestly say who did something, which is why no
	//     projection in this feature has a curator column. See photo/table.sql.
	//   - **Rotation is a config change and a redeploy**, and revoking one person means rotating for
	//     everybody. There is no per-user revocation because there are no users.
	//   - **It will be pasted into a chat message.** Assume it, rather than hoping otherwise; the
	//     mitigations in task 371 exist because this is true.
	//
	// PRD 022 §11 Q4 is still open and is a process question, not a code one: who generates this, where it is
	// kept, who is told, and what happens to it after the event.
	//
	// # There is deliberately no default, in any environment
	//
	// Not even in development. An empty password does not mean "allow everyone", it means **the routes are
	// not registered at all** (`adminRoutesEnabled`, task 370) — the difference between *absent* and *open*.
	// A default value would be the one thing that turns a forgotten config into an anonymous write path into
	// the blob store, which is the only data in this service that cannot be rebuilt from the log.
	//
	// This is why `adminUser` has no fallback either: a default username with an empty password reads, to
	// somebody skimming, like a configured tool.
	adminUser     string
	adminPassword string

	// eventYear selects which event the directory reads. The person projection is
	// keyed per year, so this decides whose phone numbers can log in.
	//
	// Configurable rather than derived from time.Now(), which is what `hq` does. The
	// clock is wrong for this in two ordinary situations: a test event held outside its
	// nominal year, and the days around new year, when the app would stop recognising
	// every participant of an event that has not happened yet. Defaulting to the current
	// year keeps the common case zero-config while leaving an override that does not
	// require a code change (PRD 006 §11 Q7).
	eventYear string

	// eventRoute overrides the route line on a diploma: "fra Lundby til Glumsø" (task 345).
	//
	// # It used to be the only source, and that reasoning was wrong
	//
	// This field's comment said "why configuration and not a projection: because nothing upstream carries it".
	// That was false — hq's year entity has carried `cityDeparture` and `cityDestination` all along, and the
	// maintainer pointed at it on 2026-09-21. Task 357 copied the fold, so the **projection is now the source**
	// and this is an override (see app.diplomaRoute).
	//
	// Kept, rather than deleted, for the one thing a projection cannot do: correct a wrong line without waiting
	// for somebody to fix the data upstream and a replay to reach it. That is the same argument as every other
	// operational override in this file.
	//
	// **Empty defers to the projection**, which is the default. If neither has anything, the line is omitted: a
	// diploma with no route reads fine; one naming the wrong places is a certificate somebody frames with a
	// mistake in it.
	//
	// Written as the whole phrase rather than as a start and a destination, because an override exists to be
	// exact — including its grammar. The projection's two cities are composed into the phrase by the handler.
	eventRoute string

	// portraitRetention is how long a portrait is kept after it was captured, before the
	// purge job deletes it (task 109).
	//
	// The *rule* was decided by the maintainer (task 102): the portrait is an in-race
	// safety feature and does not outlive the event. The number implementing that rule
	// lives here because it is the part that wants a human's answer, and because
	// shortening it must not require a deploy.
	//
	// Measured from **capture**, not from a configured event end date. That is a
	// deliberate simplification: an end date is one more thing to keep correct every
	// year, and getting it wrong fails in the bad direction (photos kept). Capture time
	// is already on the row and is replay-stable.
	//
	// The 30-day default is conservative rather than chosen: it is comfortably past any
	// post-race need and well short of "indefinitely". **Flagged for a maintainer
	// number.** Zero or negative disables the purge, which exists for a database-only
	// diagnostic run — not as a supported production setting.
	portraitRetention time.Duration

	// cachedDirectoryTTL is how long a device may keep its copy of the contacts directory
	// before it must throw it away (PRD 009 §6, task 193).
	//
	// # Why the server issues this rather than the client computing it
	//
	// A client-side TTL is defeated by the thing most likely to be wrong on a phone at 03:00:
	// the clock. A device whose date is set back a month would extend its own retention, and a
	// user who wants to keep a directory of other people's phone numbers has an easy way to do
	// it. So the deadline is a timestamp the server puts in the payload, and the client's job is
	// only to obey it.
	//
	// # Why an expiry exists at all
	//
	// It is the only lever we hold over a **dormant device** — a phone that never reopens the app
	// after the event, where no purge, no service worker and no push will ever run again (PRD 009
	// §11.5, PRD 007 §11.8). A baked-in deadline is checked the next time the app opens at all,
	// whenever that is, which is more than any server-side purge can promise.
	//
	// Fourteen days, **approved by the maintainer 2026-09-01**, matching
	// `PORTRAIT_CACHE_MAX_AGE_SECONDS` on the client — the index and the faces expire together on
	// purpose, because a directory of names with no photos and a set of photos with no names are
	// both worse than neither. Long enough for a participant who prepares a fortnight early; short
	// enough that the data is gone within a fortnight of the race whatever the device does
	// afterwards.
	//
	// Unlike `portraitRetention` above, this number is settled rather than a placeholder. If it
	// changes, change the client constant in the same commit or the two halves of one purge drift.
	//
	// Zero or negative disables the deadline. That is for a diagnostic run, not a supported
	// production setting: it means "keep other people's phone numbers on this phone forever".
	cachedDirectoryTTL time.Duration

	// portraitKeepOriginal decides whether the uploaded image is retained at its own
	// resolution alongside the display renditions (task 111).
	//
	// Default true: without the original, a change to the rendition set can only ever
	// apply to portraits taken after the change, so "add a smaller thumbnail for the
	// identification grid" would silently mean "for next year's members".
	//
	// Configurable because it is the one setting with a large storage consequence. The
	// blob store is the only thing in this service that cannot be rebuilt from the log
	// and therefore the only thing that must be backed up (PRD 008 §8); an original is
	// roughly two orders of magnitude larger than the renditions, so a full event turns
	// megabytes of backup into gigabytes. An operator who cannot afford that should be
	// able to say so without a deploy.
	portraitKeepOriginal bool

	// glimtRetention is how long a glimt is kept after it was posted, before the purge job
	// deletes it and its media (PRD 019 §6, task 310).
	//
	// # Why this is much longer than portraitRetention
	//
	// The portrait is a safety feature whose purpose expires with the race. A glimt is the
	// opposite: its **peak use is after the event**, when spejdere spend hours going through the
	// night's photos (PRD 019 §0a). A window measured against the race's end would delete the
	// thing at the moment it is most wanted.
	//
	// 90 days by default — long enough for the post-race browse and for showing family, short
	// enough to be a real limit rather than an archive. PRD 019 §4 is explicit that this is not
	// a photo backup: what a member wants to keep, they save to their device (task 318).
	//
	// Measured from **creation**, not from a configured event end date. The same simplification
	// portraitRetention makes, and it fails in the safe direction: one fewer thing to keep
	// correct every year, and getting it wrong deletes early rather than never.
	//
	// **Zero or negative disables the purge entirely**, which dev and CI run with — a fixture
	// posted last month must still be there tomorrow. Guarded with `<= 0` so a negative value
	// cannot be read as "immediately", and logged at startup: "off for dev" and "off because
	// somebody fat-fingered the env in production" look identical otherwise, and this is the one
	// setting whose failure mode is keeping children's photographs forever.
	glimtRetention time.Duration

	// glimtPublicRetention is how long a public glimt stays on the public page, which can be
	// shorter than how long the glimt itself lives.
	//
	// # Why this is a read-time cutoff and not a state change
	//
	// It would be tidier to "unpublish" a glimt when this expires. It would also be wrong: the
	// audience is **immutable** (PRD 019 §6) and hiding is a moderation act with a moderator's
	// name on it. Neither should be repurposed by a timer. So this is applied where the public
	// feed is read — a glimt older than the cutoff simply is not returned to the open web, while
	// remaining exactly what it was inside the app.
	//
	// That also makes it reversible: lengthening the window brings the older glimt back, which a
	// state change could not do without inventing an "unexpire" event.
	//
	// 30 days by default, closing the public window before the internal one. **Zero means "as
	// long as the glimt itself"** — not "immediately", which is the reading that would silently
	// empty the public page in a dev environment where everything else is set to 0.
	glimtPublicRetention time.Duration

	// publicPagePatrolOverride names patrols whose public page is open regardless of the gate
	// (PRD 011 §0b.3, task 330).
	//
	// # What this is for
	//
	// A patrol's page opens when it is scanned at the last checkgroup, or when the last checkpoint
	// closes. The first trigger depends on the scan being *attributed* to a checkpoint, which happens
	// by asking which post its scanner was on shift at — and an unattributed scan is a normal outcome,
	// not an error, because the rota is fed from outside this repo. So a patrol can finish and not be
	// recognised as having finished. This is how løbsledelsen fixes that by hand.
	//
	// # Why it is only a convenience
	//
	// The second trigger is guaranteed: the last checkpoint always carries absolute opening hours, so
	// every patrol's page opens when the race ends whatever happens here. Without an override the worst
	// case is a page arriving at closing time instead of at the finish line — late, not absent. That is
	// why this is one environment variable rather than a moderation surface, and why nothing in the
	// feature degrades if it is never set.
	//
	// It can only ever **open** a page. There is deliberately no way to close one from configuration: a
	// second, quieter mechanism for withholding a page would mean two places to look when one is
	// missing.
	//
	// Patrol **ids**, comma-separated. Ids rather than the numbers a human reads off a sign, because the
	// gate is evaluated against the patrol id the rest of the read side uses, and translating here would
	// put a directory lookup in the configuration layer.
	publicPagePatrolOverride []string

	// Glimt write limits and storage ceilings (PRD 019 §8, §11 Q8, task 311).
	//
	// # Two limits per member, because a count and a size answer different questions
	//
	// `glimtMediaPerHour` counts uploads and `glimtBytesPerHour` sums them. Each lets through
	// exactly what the other exists to stop: sixty thumbnails and sixty 12 MiB videos are the same
	// number of events and nowhere near the same cost, while a byte cap alone would let a client
	// hammer the decode path with tiny images. `glimtPerHour` is separate again and protects the
	// broker rather than the disk — a looping client publishing creation events costs no upload.
	//
	// # Reads are limited separately and much more loosely, on purpose
	//
	// `glimtReadsPerMinute` exists so the read endpoints cannot be hammered, and for **no other
	// reason**. The post-race browse is a legitimate flood (PRD 019 §0a.3): a thousand people at the
	// finish line pulling thumbnails by the hundred is the use this feature was built for, and a
	// limiter tuned for uploads would throttle precisely that. Per *minute* rather than per hour for
	// the same reason — a grid of 60 thumbnails is one screen, so an hourly budget would be spent by
	// a member scrolling for two minutes and then locked out for fifty-eight.
	//
	// **3000, raised from 600 after task 324 measured it.** The arithmetic that forced the change: a
	// hold page is 20 glimt averaging 2.1 media, so ~43 requests including the JSON. At 600/minute
	// that is fourteen pages a minute — one every 4.3 seconds — which a member flicking through grids
	// at the finish line beats comfortably. The limit would therefore have fired for exactly the
	// browse it exists to protect, and the comment above says that means it is set wrong. At 3000 it
	// is one page every 0.85s, still some sixty times below what a script does.
	//
	// # The ceilings
	//
	// The blob store is the only non-rebuildable data in the service and lives on one volume
	// (PRD 008 §8), so it needs a floor under it that is not "the disk filled up". Two ceilings: per
	// member, so no one account can consume the event's storage, and total, so the volume cannot be
	// filled at all.
	//
	// **Exceeding one rejects the upload; nothing is ever evicted.** PRD 019 §11 Q8 asks whether to
	// reject or evict oldest, and rejecting is the only defensible answer: refusing a photo in a
	// field is a bad experience, and silently deleting somebody else's memories to make room is
	// worse — it would also mean this feature's one irreversible operation firing with no human
	// involved. Retention is what frees space, on a schedule everybody was told about.
	//
	// **Zero means unlimited** for all five, matching the retention windows: the disabling value is
	// the zero value, so an unset variable cannot impose a limit nobody chose. Defaults are
	// deliberately generous — the point is a ceiling, not a ration.
	glimtPerHour        int
	glimtMediaPerHour   int
	glimtBytesPerHour   int64
	glimtReadsPerMinute int
	// glimtPublicReadsPerMinute and glimtPublicReportsPerHour bound the unauthenticated public page
	// and its API (task 323), keyed by IP because there is no member to key on.
	//
	// Looser again than the member read limit: one IP may legitimately be a whole school, and this
	// is the surface a link in a parents' group chat lands on.
	glimtPublicReadsPerMinute int
	glimtPublicReportsPerHour int
	// publicMediaReadsPerMinute bounds the public **media** routes per IP — album photographs and public
	// glimt thumbnails (PRD 011 §8, task 347).
	//
	// # Why it is its own ceiling, and why it is this high
	//
	// A page load is a handful of requests; an album page is up to sixty thumbnails. So the arithmetic that
	// matters is the worst *honest* case, which is a NAT'd school rather than an attack:
	//
	//	one album page      ≈  61 requests (1 HTML + 60 thumbnails)
	//	one family, 4 devices, 5 albums in a minute   ≈  1,220
	//	30 devices behind one school NAT doing the same ≈  9,150
	//
	// 12,000/minute (200/second) sits above that and still stops a scripted scrape, which would run at
	// thousands per second rather than hundreds per minute. The asymmetry is deliberate: the cost of a
	// throttled thumbnail is a broken-looking page for a family the morning after, and the cost of an
	// unthrottled one is some bandwidth.
	//
	// **Zero means unlimited**, as with every other ceiling here.
	publicMediaReadsPerMinute int
	// glimtMemberStorageBytes is one member's total media allowance for the year.
	glimtMemberStorageBytes int64
	// glimtTotalStorageBytes is the whole year's allowance, protecting the volume itself.
	glimtTotalStorageBytes int64
	// adminDiskFloorBytes is how much free space on the blob volume must survive an organizer upload.
	//
	// A floor under the *volume*, not a quota over the feature — see `adminstorage.go` for why PRD 022 §11
	// Q5 is answered that way. **Zero disables it**, as with every other ceiling here.
	adminDiskFloorBytes int64
}

func loadConfig() config {
	var cfg config
	flag.IntVar(&cfg.port, "port", envInt("PORT", 4000), "API server port")
	flag.StringVar(&cfg.env, "env", envStr("ENV", "development"), "Environment (development|staging|production)")
	flag.StringVar(&cfg.webRoot, "web-root", envStr("WEB_ROOT", "./www"), "Directory containing the built SPA")
	flag.StringVar(&cfg.sessionSecret, "session-secret", envStr("SESSION_SECRET", "dev-insecure-secret-change-me"), "HMAC secret for signing session cookies")
	flag.BoolVar(&cfg.sessionSecure, "session-secure", envBool("SESSION_SECURE", true), "Set the Secure flag on the session cookie (true behind HTTPS)")
	flag.StringVar(&cfg.vapidPublicKey, "vapid-public-key", envStr("VAPID_PUBLIC_KEY", ""), "Web Push VAPID public key (served to clients)")
	flag.StringVar(&cfg.vapidPrivateKey, "vapid-private-key", envStr("VAPID_PRIVATE_KEY", ""), "Web Push VAPID private key (secret)")
	flag.StringVar(&cfg.dataforsyningenToken, "dataforsyningen-token", envStr("DATAFORSYNINGEN_TOKEN", ""), "Dataforsyningen API token for the map's WMS base layers")
	// Default derived from ENV read directly, not from cfg.env: flags are not parsed
	// yet at this point, so cfg.env still holds its zero value.
	flag.BoolVar(&cfg.showBuildId, "show-build-id", envBool("SHOW_BUILD_ID", envStr("ENV", "development") != "production"), "Overlay the build id on the bottom nav (diagnostic)")
	flag.BoolVar(&cfg.showLayoutDebug, "show-layout-debug", envBool("SHOW_LAYOUT_DEBUG", false), "Overlay viewport/safe-area/geometry values on the client (diagnostic)")
	flag.BoolVar(&cfg.installGate, "install-gate", envBool("INSTALL_GATE", true), "Require the app to be installed before it can be used (PRD 005). Set INSTALL_GATE=false to disable the gate without a redeploy.")
	flag.IntVar(&cfg.syncIntervalSeconds, "sync-interval-seconds", envInt("SYNC_INTERVAL_SECONDS", 60), "How often a client re-runs the multiplexed freshness check while the app is open (PRD 017). 0 disables the interval; foreground, reconnect and manual checks still run.")
	flag.IntVar(&cfg.syncDebounceSeconds, "sync-debounce-seconds", envInt("SYNC_DEBOUNCE_SECONDS", 5), "Minimum seconds between freshness checks (PRD 017). Absorbs repeated foregrounding; a user-requested refresh ignores it. 0 disables the debounce.")
	flag.StringVar(&cfg.dbDSN, "db-dsn", envStr("DB_DSN", ""), "MariaDB DSN (empty runs without a database)")
	flag.IntVar(&cfg.dbMaxOpenConns, "db-max-open-conns", envInt("DB_MAX_OPEN_CONNS", 25), "Maximum open database connections")
	flag.IntVar(&cfg.dbMaxIdleConns, "db-max-idle-conns", envInt("DB_MAX_IDLE_CONNS", 25), "Maximum idle database connections")
	flag.DurationVar(&cfg.dbConnMaxLifetime, "db-conn-max-lifetime", envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute), "Maximum lifetime of a pooled database connection")
	flag.DurationVar(&cfg.dbConnectTimeout, "db-connect-timeout", envDuration("DB_CONNECT_TIMEOUT", 10*time.Second), "How long to keep retrying the initial database ping")
	flag.StringVar(&cfg.jetstreamDSN, "jetstream-dsn", envStr("JETSTREAM_DSN", ""), "NATS JetStream DSN (empty runs without a broker)")
	flag.StringVar(&cfg.smsDSN, "sms-dsn", envStr("SMS_DSN", ""), "SMS provider DSN, e.g. cpsms://<api-key>@api.cpsms.dk (empty logs messages instead of sending)")
	flag.StringVar(&cfg.blobPath, "blob-path", envStr("BLOB_PATH", ""), "Directory for binary objects such as portraits (empty keeps them in memory)")
	flag.StringVar(&cfg.eventYear, "event-year", envStr("EVENT_YEAR", currentYear()), "Event year the member directory reads (defaults to the current year)")
	flag.StringVar(&cfg.photoBaseURL, "photo-baseurl", envStr("PHOTO_BASEURL", ""), "Base URL of the foto service, which serves photograph objects at /photos/<ref> (empty means diplomas carry no photograph)")
	flag.BoolVar(&cfg.publicAlbums, "public-albums", envBool("PUBLIC_ALBUMS", true), "Show the curated photo albums on the public site (PUBLIC_ALBUMS=false hides the section and answers 404 for an album page)")
	flag.StringVar(&cfg.eventRoute, "event-route", envStr("EVENT_ROUTE", ""), "Overrides the route line printed on a diploma, e.g. \"fra Lundby til Glums\u00f8\" (empty uses the year projection's two cities)")
	// No fallback for either, in any environment: an unset password means the admin routes do not exist
	// rather than that they are open. See the fields' doc and task 370.
	flag.StringVar(&cfg.adminUser, "admin-user", envStr("ADMIN_USER", ""), "Username for the photographer admin tool (empty, with ADMIN_PASSWORD, means the tool is not served at all)")
	flag.StringVar(&cfg.adminPassword, "admin-password", envStr("ADMIN_PASSWORD", ""), "Password for the photographer admin tool (empty means /admin and /api/admin/* are not registered; there is no default in any environment)")
	flag.DurationVar(&cfg.portraitRetention, "portrait-retention", envDuration("PORTRAIT_RETENTION", 30*24*time.Hour), "How long a portrait is kept after capture before it is purged (0 disables the purge)")
	flag.DurationVar(&cfg.cachedDirectoryTTL, "cached-directory-ttl", envDuration("CACHED_DIRECTORY_TTL", 14*24*time.Hour), "How long a device may keep its cached contacts directory (0 disables the deadline)")
	flag.BoolVar(&cfg.portraitKeepOriginal, "portrait-keep-original", envBool("PORTRAIT_KEEP_ORIGINAL", true), "Retain the uploaded image at full resolution (metadata stripped) so renditions can be regenerated later")
	flag.DurationVar(&cfg.glimtRetention, "glimt-retention", envDuration("GLIMT_RETENTION", 90*24*time.Hour), "How long a glimt is kept after it was posted before it is purged (0 disables the purge)")
	flag.DurationVar(&cfg.glimtPublicRetention, "glimt-public-retention", envDuration("GLIMT_PUBLIC_RETENTION", 30*24*time.Hour), "How long a public glimt stays on the public page (0 means as long as the glimt itself)")
	flag.IntVar(&cfg.glimtPerHour, "glimt-per-hour", envInt("GLIMT_PER_HOUR", 20), "Glimt one member may create per hour (0 disables the limit)")
	flag.IntVar(&cfg.glimtMediaPerHour, "glimt-media-per-hour", envInt("GLIMT_MEDIA_PER_HOUR", 60), "Media files one member may upload per hour (0 disables the limit)")
	flag.Int64Var(&cfg.glimtBytesPerHour, "glimt-bytes-per-hour", envInt64("GLIMT_BYTES_PER_HOUR", 200<<20), "Media bytes one member may upload per hour (0 disables the limit)")
	flag.IntVar(&cfg.glimtReadsPerMinute, "glimt-reads-per-minute", envInt("GLIMT_READS_PER_MINUTE", 3000), "Glimt read requests one member may make per minute (0 disables the limit)")
	flag.IntVar(&cfg.glimtPublicReadsPerMinute, "glimt-public-reads-per-minute", envInt("GLIMT_PUBLIC_READS_PER_MINUTE", 3000), "Public glimt read requests one IP may make per minute (0 disables the limit)")
	flag.IntVar(&cfg.glimtPublicReportsPerHour, "glimt-public-reports-per-hour", envInt("GLIMT_PUBLIC_REPORTS_PER_HOUR", 30), "Anonymous glimt reports one IP may make per hour (0 disables the limit)")
	flag.IntVar(&cfg.publicMediaReadsPerMinute, "public-media-reads-per-minute", envInt("PUBLIC_MEDIA_READS_PER_MINUTE", 12000), "Public media requests (album photographs, public glimt thumbnails) one IP may make per minute (0 disables the limit)")
	flag.Int64Var(&cfg.glimtMemberStorageBytes, "glimt-member-storage-bytes", envInt64("GLIMT_MEMBER_STORAGE_BYTES", 500<<20), "Total media bytes one member may have stored (0 disables the ceiling)")
	flag.Int64Var(&cfg.glimtTotalStorageBytes, "glimt-total-storage-bytes", envInt64("GLIMT_TOTAL_STORAGE_BYTES", 0), "Total media bytes the event may have stored (0 disables the ceiling)")
	// 2 GiB, which is reserve rather than budget: roughly twice a full card's hand-in (PRD 022 §6 puts one at
	// order 1 GB), so the refusal lands with a whole batch's worth of room still on the volume for everything
	// else that shares it. Defaulted **on**, unlike the glimt total ceiling, because the thing it protects
	// against needs no adversary — one photographer with a card does it by following instructions.
	flag.Int64Var(&cfg.adminDiskFloorBytes, "admin-disk-floor-bytes", envInt64("ADMIN_DISK_FLOOR_BYTES", 2<<30), "Free bytes that must remain on the blob volume after an admin photo upload (0 disables the check)")
	publicOverride := flag.String("public-page-patrol-override", envStr("PUBLIC_PAGE_PATROL_OVERRIDE", ""), "Patrol ids whose public page is open regardless of the gate, comma-separated (PRD 011)")
	flag.Parse()
	cfg.publicPagePatrolOverride = splitCSV(*publicOverride)
	return cfg
}

// splitCSV parses a comma-separated list, dropping empties and surrounding space.
//
// Empty in, nil out — not a one-element list containing "". That distinction is the whole reason this is
// a function: the value it feeds is the public-page override, and an unset variable yielding a list with
// an empty id in it would make the gate's "is this patrol overridden?" lookup true for the empty patrol
// id, which is every personnel user.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// currentYear is the default event year.
//
// A function rather than a constant so the default follows the clock, and separated from
// the flag definition so a test can reason about the fallback without reparsing flags.
func currentYear() string {
	return strconv.Itoa(time.Now().Year())
}

// smsProviderName reports just the scheme of an SMS DSN, so a startup log line can
// say what will happen without putting the API key in the log.
func smsProviderName(dsn string) string {
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		return u.Scheme
	}
	return "unknown"
}

func envStr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// envInt64 reads a byte-sized setting.
//
// Separate from envInt because the storage ceilings are naturally larger than a 32-bit int on the
// platforms this may run on, and a silently truncated ceiling is the kind of bug that only appears
// once the disk is nearly full.
//
// Plain digits only — no "500MB" suffix parsing. A suffix that a typo turns into a different
// magnitude ("500Mb", "500 MB", "500mib") is worse than a long number, and these are written once in
// a compose file.
func envInt64(key string, fallback int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
