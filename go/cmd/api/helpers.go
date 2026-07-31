package main

import (
	"net"
	"net/http"
	"strings"
)

// clientIP returns the best-effort client IP for rate-limiting. Behind Traefik
// the real client is in X-Forwarded-For; fall back to RemoteAddr otherwise.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
