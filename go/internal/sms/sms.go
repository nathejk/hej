// Package sms abstracts sending SMS messages. The BFF codes against Sender;
// which implementation it gets is decided by a DSN in the environment (SMS_DSN)
// — an empty DSN keeps the dev LogSender, `cpsms://` sends for real.
package sms

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
)

// Sender delivers a text message to a phone number.
type Sender interface {
	Send(ctx context.Context, to, message string) error
}

// NewSender builds a Sender from a DSN. The DSN is the single deciding factor,
// deliberately: nothing else in the configuration can make the app send real
// messages, and nothing else can stop it.
//
// Supported forms:
//
//	""                                   → LogSender (dev; logs, sends nothing)
//	log://                               → LogSender, stated explicitly
//	cpsms://<api-key>@api.cpsms.dk       → CPSMS, sender name "Nathejk"
//	cpsms://<api-key>@api.cpsms.dk?from=X → CPSMS with an approved sender name
//
// The api-key is the pre-encoded HTTP Basic credential CPSMS issues; same value
// the sibling `tilmelding` repo uses, so one account serves both. URL-escape it
// if it contains characters that are special in a userinfo field.
func NewSender(dsn string, logger *slog.Logger) (Sender, error) {
	if dsn == "" {
		return LogSender{Logger: logger}, nil
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("sms: invalid DSN: %w", err)
	}
	switch u.Scheme {
	case "log":
		return LogSender{Logger: logger}, nil
	case "cpsms":
		key := ""
		if u.User != nil {
			key = u.User.Username()
		}
		return NewCpsms(u.Host, key, u.Query().Get("from"))
	}
	return nil, fmt.Errorf("sms: unknown provider %q", u.Scheme)
}
