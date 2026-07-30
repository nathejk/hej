// Package app holds the transport-layer helpers shared across the API binary's
// HTTP handlers: JSON read/write, standard error responses, and the HTTP server
// with graceful shutdown. The application struct in package main embeds
// JsonApi to inherit these methods (called via the `app` receiver in handlers).
package app

import "log/slog"

// JsonApi provides the shared transport helpers. Embed it on the application
// struct. A logger is required for error reporting.
type JsonApi struct {
	Logger *slog.Logger
}

// Envelope is the standard wrapper for JSON responses.
type Envelope map[string]any
