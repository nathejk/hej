package choice

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testPhone = "+4530112233"

func managerAt(t *testing.T, now time.Time) *Manager {
	t.Helper()
	m := NewManager([]byte("test-secret"), time.Minute)
	m.now = func() time.Time { return now }
	return m
}

func TestIssueThenVerifyRoundTrips(t *testing.T) {
	m := managerAt(t, time.Now())

	got, err := m.Verify(m.Issue(testPhone))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != testPhone {
		t.Fatalf("phone = %q, want %q", got, testPhone)
	}
}

// The token must be useless after its window. It is a hand-off, not a session.
func TestExpiredTokenIsRejected(t *testing.T) {
	issued := time.Now()
	m := managerAt(t, issued)
	token := m.Issue(testPhone)

	m.now = func() time.Time { return issued.Add(2 * time.Minute) }
	if _, err := m.Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

// A token signed with a different secret must not verify. This is the whole security
// property: without it, anyone could mint a token for any phone number.
func TestTokenFromAnotherSecretIsRejected(t *testing.T) {
	attacker := NewManager([]byte("attacker-secret"), time.Minute)
	victim := NewManager([]byte("real-secret"), time.Minute)

	if _, err := victim.Verify(attacker.Issue(testPhone)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a foreign signature, got %v", err)
	}
}

// Tampering with the payload must invalidate the signature — otherwise the phone
// binding is decorative and a token for one number could be edited into another.
func TestTamperedPayloadIsRejected(t *testing.T) {
	m := managerAt(t, time.Now())
	token := m.Issue(testPhone)

	payload, sig, _ := strings.Cut(token, ".")
	// Flip a character in the payload, keeping the original signature.
	tampered := payload[:len(payload)-1] + string(payload[len(payload)-1]^1) + "." + sig

	if _, err := m.Verify(tampered); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid for a tampered payload, got %v", err)
	}
}

func TestMalformedTokensAreRejected(t *testing.T) {
	m := managerAt(t, time.Now())

	for _, token := range []string{
		"",
		"nodot",
		".",
		"not-base64.signature",
		strings.Repeat("a", 64),
	} {
		if _, err := m.Verify(token); err == nil {
			t.Errorf("token %q should not verify", token)
		}
	}
}

// An expired token with a broken signature must report ErrInvalid, not ErrExpired.
// Reporting expiry would confirm to a forger that their signature was accepted.
func TestForgedAndExpiredReportsInvalidNotExpired(t *testing.T) {
	issued := time.Now()
	attacker := NewManager([]byte("attacker-secret"), time.Minute)
	token := attacker.Issue(testPhone)

	victim := managerAt(t, issued.Add(2*time.Minute))
	if _, err := victim.Verify(token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("want ErrInvalid (signature checked first), got %v", err)
	}
}

// A token carries no user id by design: it authorises "pick one of this number's
// owners", not "be this user". Pinned so nobody adds one for convenience.
func TestTokenCarriesNoUserID(t *testing.T) {
	m := managerAt(t, time.Now())
	token := m.Issue(testPhone)

	if strings.Contains(token, "user") {
		t.Fatal("the token payload must not carry a user id")
	}
}

func TestZeroTTLFallsBackToDefault(t *testing.T) {
	m := NewManager([]byte("s"), 0)
	if m.ttl != DefaultTTL {
		t.Fatalf("ttl = %v, want the default %v", m.ttl, DefaultTTL)
	}
}
