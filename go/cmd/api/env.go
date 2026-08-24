package main

import (
	"flag"
	"os"
	"strconv"
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
	flag.Parse()
	return cfg
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
