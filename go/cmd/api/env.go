package main

import (
	"flag"
	"os"
	"strconv"
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

	// contactsPollSeconds is how often a client with the contacts pane open asks whether the
	// directory has changed (PRD 007 §8).
	//
	// Configurable for the same reason as installGate, though against a smaller risk: this is
	// the app's first continuous during-race traffic, so if it costs more than expected it has
	// to be widenable mid-event. 0 or less disables the interval while leaving the foreground
	// and reconnect checks in place.
	//
	// 60s by default, per the PRD's "within ~60 seconds while the app is open".
	contactsPollSeconds int

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

	// blobPath is the directory for content-addressed binary objects — portrait
	// bytes today (PRDs 003/007). Empty keeps them in memory, which is fine for
	// tests and for running the API before portraits exist, but means they do not
	// survive a restart. The production choice between a mounted volume and object
	// storage is still open (PRD 008 §11 Q4); this is the volume half.
	blobPath string

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
	flag.IntVar(&cfg.contactsPollSeconds, "contacts-poll-seconds", envInt("CONTACTS_POLL_SECONDS", 60), "How often the contacts pane checks for directory changes while open (PRD 007). 0 disables the interval; foreground and reconnect checks still run.")
	flag.StringVar(&cfg.dbDSN, "db-dsn", envStr("DB_DSN", ""), "MariaDB DSN (empty runs without a database)")
	flag.IntVar(&cfg.dbMaxOpenConns, "db-max-open-conns", envInt("DB_MAX_OPEN_CONNS", 25), "Maximum open database connections")
	flag.IntVar(&cfg.dbMaxIdleConns, "db-max-idle-conns", envInt("DB_MAX_IDLE_CONNS", 25), "Maximum idle database connections")
	flag.DurationVar(&cfg.dbConnMaxLifetime, "db-conn-max-lifetime", envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute), "Maximum lifetime of a pooled database connection")
	flag.DurationVar(&cfg.dbConnectTimeout, "db-connect-timeout", envDuration("DB_CONNECT_TIMEOUT", 10*time.Second), "How long to keep retrying the initial database ping")
	flag.StringVar(&cfg.jetstreamDSN, "jetstream-dsn", envStr("JETSTREAM_DSN", ""), "NATS JetStream DSN (empty runs without a broker)")
	flag.StringVar(&cfg.blobPath, "blob-path", envStr("BLOB_PATH", ""), "Directory for binary objects such as portraits (empty keeps them in memory)")
	flag.StringVar(&cfg.eventYear, "event-year", envStr("EVENT_YEAR", currentYear()), "Event year the member directory reads (defaults to the current year)")
	flag.DurationVar(&cfg.portraitRetention, "portrait-retention", envDuration("PORTRAIT_RETENTION", 30*24*time.Hour), "How long a portrait is kept after capture before it is purged (0 disables the purge)")
	flag.DurationVar(&cfg.cachedDirectoryTTL, "cached-directory-ttl", envDuration("CACHED_DIRECTORY_TTL", 14*24*time.Hour), "How long a device may keep its cached contacts directory (0 disables the deadline)")
	flag.BoolVar(&cfg.portraitKeepOriginal, "portrait-keep-original", envBool("PORTRAIT_KEEP_ORIGINAL", true), "Retain the uploaded image at full resolution (metadata stripped) so renditions can be regenerated later")
	flag.Parse()
	return cfg
}

// currentYear is the default event year.
//
// A function rather than a constant so the default follows the clock, and separated from
// the flag definition so a test can reason about the fallback without reparsing flags.
func currentYear() string {
	return strconv.Itoa(time.Now().Year())
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
