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
}

func loadConfig() config {
	var cfg config
	flag.IntVar(&cfg.port, "port", envInt("PORT", 4000), "API server port")
	flag.StringVar(&cfg.env, "env", envStr("ENV", "development"), "Environment (development|staging|production)")
	flag.StringVar(&cfg.webRoot, "web-root", envStr("WEB_ROOT", "./www"), "Directory containing the built SPA")
	flag.StringVar(&cfg.sessionSecret, "session-secret", envStr("SESSION_SECRET", "dev-insecure-secret-change-me"), "HMAC secret for signing session cookies")
	flag.BoolVar(&cfg.sessionSecure, "session-secure", envBool("SESSION_SECURE", true), "Set the Secure flag on the session cookie (true behind HTTPS)")
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
