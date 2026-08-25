// Package choice issues and verifies the short-lived token that carries a user
// between "PIN verified" and "which of you is this?".
//
// # Why a token instead of a session
//
// One phone number can belong to several people (siblings sharing a phone, or a
// guardian's number entered as the scout's own — 213 such numbers in the real data,
// roughly one member in eight). A verified PIN proves control of the *number*, not
// which of its owners is holding it, so login cannot end there: issuing a session for
// one of them would be a guess, and a wrong guess means one sibling reading the
// other's profile.
//
// So verification produces a token instead. The token says "this phone number passed
// PIN verification just now", and nothing else. Exchanging it for a session requires
// naming which of that number's owners you are.
//
// # Why it is signed rather than stored
//
// The same reasoning the session cookie uses (PRD 001): an HMAC-signed, self-contained
// value needs no server-side state, so it survives a restart and does not depend on a
// shared store.
//
// The token is deliberately *not* a session: it authorises exactly one action, carries
// no user id, and expires in about a minute. If it leaked, the holder could pick an
// owner of a phone number they had already proven control of — which is the authority
// they already had.
package choice

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	// ErrInvalid covers a malformed token, a bad signature, and a token bound to a
	// different phone number. They collapse deliberately: distinguishing them tells an
	// attacker which part of a forgery attempt was wrong.
	ErrInvalid = errors.New("invalid choice token")
	// ErrExpired is separate because it is genuinely actionable by the client — the
	// user should be asked for a fresh PIN rather than told "invalid".
	ErrExpired = errors.New("choice token expired")
)

// DefaultTTL is how long the user has to choose.
//
// Long enough to read a short list and tap; short enough that a token found later is
// useless. It is not a session lifetime and must not drift towards one.
const DefaultTTL = 2 * time.Minute

type claims struct {
	// Phone is the normalized number that passed verification. The token is bound to
	// it, so a token minted for one number cannot be redeemed against another.
	Phone string `json:"p"`
	// ExpiresAt is a Unix timestamp.
	ExpiresAt int64 `json:"e"`
}

// Manager issues and verifies choice tokens.
type Manager struct {
	secret []byte
	ttl    time.Duration
	// now is injectable so expiry is testable without sleeping.
	now func() time.Time
}

// NewManager returns a Manager.
//
// Pass the same secret the session manager uses: both are server-side signing keys
// with the same blast radius, and a second secret to configure is a second secret to
// forget to set in production.
func NewManager(secret []byte, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Manager{secret: secret, ttl: ttl, now: time.Now}
}

// Issue mints a token for a phone number that has just passed PIN verification.
func (m *Manager) Issue(normalizedPhone string) string {
	payload, _ := json.Marshal(claims{
		Phone:     normalizedPhone,
		ExpiresAt: m.now().Add(m.ttl).Unix(),
	})
	b := base64.RawURLEncoding.EncodeToString(payload)
	return b + "." + m.sign(b)
}

// Verify checks a token and returns the phone number it was issued for.
//
// The caller must then confirm the chosen user is actually an owner of that number.
// This package cannot: it has no directory. The split is deliberate — the token proves
// the *phone* was verified, and the directory decides who that phone belongs to.
func (m *Manager) Verify(token string) (normalizedPhone string, err error) {
	b, sig, ok := strings.Cut(token, ".")
	if !ok {
		return "", ErrInvalid
	}
	// Constant-time compare, as with the session cookie: a timing-distinguishable
	// signature check is one an attacker can walk.
	if !hmac.Equal([]byte(sig), []byte(m.sign(b))) {
		return "", ErrInvalid
	}
	payload, decErr := base64.RawURLEncoding.DecodeString(b)
	if decErr != nil {
		return "", ErrInvalid
	}
	var c claims
	if jsonErr := json.Unmarshal(payload, &c); jsonErr != nil {
		return "", ErrInvalid
	}
	if c.Phone == "" {
		return "", ErrInvalid
	}
	// Expiry is checked after the signature, so a forged token never reports
	// ErrExpired — which would confirm to a forger that their signature was accepted.
	if m.now().Unix() > c.ExpiresAt {
		return "", ErrExpired
	}
	return c.Phone, nil
}

func (m *Manager) sign(msg string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
