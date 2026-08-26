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
	flag.StringVar(&cfg.dbDSN, "db-dsn", envStr("DB_DSN", ""), "MariaDB DSN (empty runs without a database)")
	flag.IntVar(&cfg.dbMaxOpenConns, "db-max-open-conns", envInt("DB_MAX_OPEN_CONNS", 25), "Maximum open database connections")
	flag.IntVar(&cfg.dbMaxIdleConns, "db-max-idle-conns", envInt("DB_MAX_IDLE_CONNS", 25), "Maximum idle database connections")
	flag.DurationVar(&cfg.dbConnMaxLifetime, "db-conn-max-lifetime", envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute), "Maximum lifetime of a pooled database connection")
	flag.DurationVar(&cfg.dbConnectTimeout, "db-connect-timeout", envDuration("DB_CONNECT_TIMEOUT", 10*time.Second), "How long to keep retrying the initial database ping")
	flag.StringVar(&cfg.jetstreamDSN, "jetstream-dsn", envStr("JETSTREAM_DSN", ""), "NATS JetStream DSN (empty runs without a broker)")
	flag.StringVar(&cfg.blobPath, "blob-path", envStr("BLOB_PATH", ""), "Directory for binary objects such as portraits (empty keeps them in memory)")
	flag.StringVar(&cfg.eventYear, "event-year", envStr("EVENT_YEAR", currentYear()), "Event year the member directory reads (defaults to the current year)")
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
