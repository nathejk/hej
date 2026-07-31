package sms

import (
	"context"
	"log/slog"
)

// LogSender "sends" a message by writing it to the logger. It is a dev/test
// stand-in until a real provider is wired.
//
// WARNING: it logs the full message body (including any PIN). Use only in dev.
type LogSender struct {
	Logger *slog.Logger
}

// Send logs the message and always succeeds.
func (s LogSender) Send(_ context.Context, to, message string) error {
	s.Logger.Info("sms (dev log sender — not really sent)", "to", to, "message", message)
	return nil
}
