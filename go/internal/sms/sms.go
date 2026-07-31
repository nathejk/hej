// Package sms abstracts sending SMS messages. The BFF codes against Sender; a
// real provider (e.g. cpsms) can be dropped in later. This skeleton ships only
// a dev LogSender — no real provider is assumed to be configured yet.
package sms

import "context"

// Sender delivers a text message to a phone number.
type Sender interface {
	Send(ctx context.Context, to, message string) error
}
