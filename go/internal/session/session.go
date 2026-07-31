// Package session issues and verifies stateless, signed session cookies for
// the BFF. A cookie carries the user id + role + expiry, HMAC-SHA256 signed
// with a server secret; validation is cookie-only so sessions survive process
// restarts (no server store needed for the skeleton).
//
// Cookie attributes: HttpOnly, SameSite=Lax, Secure (configurable), Path=/.
package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// CookieName is the session cookie name.
const CookieName = "hej_session"

var (
	// ErrNoSession means no session cookie is present on the request.
	ErrNoSession = errors.New("no session")
	// ErrInvalid means the cookie failed signature/format checks (tampered).
	ErrInvalid = errors.New("invalid session")
	// ErrExpired means the cookie's expiry has passed.
	ErrExpired = errors.New("session expired")
)

// Session is the payload carried by the cookie.
type Session struct {
	UserID    string    `json:"uid"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"exp"`
}

// Manager signs, issues, and verifies session cookies.
type Manager struct {
	secret []byte
	ttl    time.Duration
	secure bool
	now    func() time.Time
}

// NewManager returns a Manager. `secure` toggles the Secure cookie flag
// (production TLS: true; local plain-HTTP tests: false). `ttl` is how long an
// issued session stays valid.
func NewManager(secret []byte, ttl time.Duration, secure bool) *Manager {
	return &Manager{secret: secret, ttl: ttl, secure: secure, now: time.Now}
}

// Issue sets a fresh session cookie carrying userID + role. Returns the
// resulting Session (mainly for logging/testing).
func (m *Manager) Issue(w http.ResponseWriter, userID, role string) Session {
	s := Session{
		UserID:    userID,
		Role:      role,
		ExpiresAt: m.now().Add(m.ttl),
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    m.encode(s),
		Path:     "/",
		Expires:  s.ExpiresAt,
		MaxAge:   int(m.ttl.Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return s
}

// Clear removes the session cookie from the client.
func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Read returns the Session carried by r, or an error if none/invalid/expired.
func (m *Manager) Read(r *http.Request) (Session, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return Session{}, ErrNoSession
	}
	s, err := m.decode(c.Value)
	if err != nil {
		return Session{}, err
	}
	if m.now().After(s.ExpiresAt) {
		return Session{}, ErrExpired
	}
	return s, nil
}

func (m *Manager) encode(s Session) string {
	payload, _ := json.Marshal(s)
	b := base64.RawURLEncoding.EncodeToString(payload)
	return b + "." + m.sign(b)
}

func (m *Manager) decode(value string) (Session, error) {
	b, sig, ok := strings.Cut(value, ".")
	if !ok {
		return Session{}, ErrInvalid
	}
	expected := m.sign(b)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return Session{}, ErrInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(b)
	if err != nil {
		return Session{}, ErrInvalid
	}
	var s Session
	if err := json.Unmarshal(payload, &s); err != nil {
		return Session{}, ErrInvalid
	}
	return s, nil
}

func (m *Manager) sign(msg string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
