package pin

import (
	"errors"
	"testing"
	"time"
)

// withClock swaps the store clock for deterministic tests.
func withClock(s *Store, t *time.Time) {
	s.now = func() time.Time { return *t }
}

func TestGenerate_LengthAndDigits(t *testing.T) {
	code, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(code) != Length {
		t.Fatalf("len = %d, want %d", len(code), Length)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("non-digit in pin: %q", code)
		}
	}
}

func TestIssueThenVerify_Success(t *testing.T) {
	s := NewStore()
	code, err := s.Issue("+4520304050")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := s.Verify("+4520304050", code); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Single-use: a second verify finds no PIN.
	if err := s.Verify("+4520304050", code); !errors.Is(err, ErrNoPIN) {
		t.Fatalf("second verify err = %v, want ErrNoPIN", err)
	}
}

func TestVerify_Mismatch(t *testing.T) {
	s := NewStore()
	code, _ := s.Issue("+4520304050")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if err := s.Verify("+4520304050", wrong); !errors.Is(err, ErrMismatch) {
		t.Fatalf("err = %v, want ErrMismatch", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	now := time.Now()
	s := NewStore()
	withClock(s, &now)
	code, _ := s.Issue("+4520304050")
	now = now.Add(TTL + time.Second)
	if err := s.Verify("+4520304050", code); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestVerify_LockoutAfterMaxAttempts(t *testing.T) {
	s := NewStore()
	code, _ := s.Issue("+4520304050")
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < MaxAttempts; i++ {
		if err := s.Verify("+4520304050", wrong); !errors.Is(err, ErrMismatch) {
			t.Fatalf("attempt %d err = %v, want ErrMismatch", i, err)
		}
	}
	// Next attempt is locked out — even the correct code fails.
	if err := s.Verify("+4520304050", code); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("err = %v, want ErrTooManyAttempts", err)
	}
}

func TestIssue_ResendCooldown(t *testing.T) {
	now := time.Now()
	s := NewStore()
	withClock(s, &now)
	if _, err := s.Issue("+4520304050"); err != nil {
		t.Fatalf("first issue: %v", err)
	}
	if _, err := s.Issue("+4520304050"); !errors.Is(err, ErrCooldown) {
		t.Fatalf("second issue err = %v, want ErrCooldown", err)
	}
	now = now.Add(ResendCooldown + time.Second)
	if _, err := s.Issue("+4520304050"); err != nil {
		t.Fatalf("issue after cooldown: %v", err)
	}
}
