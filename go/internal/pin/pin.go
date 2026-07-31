// Package pin issues and verifies short-lived, single-use SMS login PINs.
//
// Policy (per PRD 001): 6-digit PIN, 10-minute TTL, max 5 verify attempts, and a
// 60-second resend cooldown. PINs are stored hashed (bcrypt). The store here is
// in-memory with TTL; a persistent store can implement the same behaviour later.
package pin

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// Length is the number of digits in a PIN.
	Length = 6
	// TTL is how long an issued PIN stays valid.
	TTL = 10 * time.Minute
	// MaxAttempts is the number of verify attempts before lockout.
	MaxAttempts = 5
	// ResendCooldown is the minimum time between issuing PINs for one phone.
	ResendCooldown = 60 * time.Second
)

var (
	// ErrCooldown is returned by Issue when a PIN was issued too recently.
	ErrCooldown = errors.New("resend cooldown active")
	// ErrNoPIN is returned by Verify when no active PIN exists for the phone.
	ErrNoPIN = errors.New("no active pin")
	// ErrExpired is returned by Verify when the PIN has expired.
	ErrExpired = errors.New("pin expired")
	// ErrTooManyAttempts is returned by Verify once the attempt limit is hit.
	ErrTooManyAttempts = errors.New("too many attempts")
	// ErrMismatch is returned by Verify when the submitted PIN is wrong.
	ErrMismatch = errors.New("pin mismatch")
)

type record struct {
	hash      []byte
	expiresAt time.Time
	sentAt    time.Time
	attempts  int
}

// Store holds active PINs keyed by normalized phone number.
type Store struct {
	mu      sync.Mutex
	records map[string]*record
	now     func() time.Time
}

// NewStore returns an empty in-memory PIN store.
func NewStore() *Store {
	return &Store{
		records: make(map[string]*record),
		now:     time.Now,
	}
}

// Generate returns a fresh cryptographically-random numeric PIN.
func Generate() (string, error) {
	const digits = "0123456789"
	buf := make([]byte, Length)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		buf[i] = digits[n.Int64()]
	}
	return string(buf), nil
}

// Issue creates and stores a hashed PIN for phone, replacing any previous one,
// and returns the plaintext PIN to send by SMS. If a PIN was issued within the
// resend cooldown, it returns ErrCooldown and leaves the existing PIN intact.
func (s *Store) Issue(phone string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if rec, ok := s.records[phone]; ok && now.Sub(rec.sentAt) < ResendCooldown {
		return "", ErrCooldown
	}

	code, err := Generate()
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	s.records[phone] = &record{
		hash:      hash,
		expiresAt: now.Add(TTL),
		sentAt:    now,
	}
	return code, nil
}

// Verify checks a submitted PIN for phone. On success the PIN is consumed
// (single-use). It enforces expiry and the attempt limit; a wrong PIN counts as
// an attempt and returns ErrMismatch.
func (s *Store) Verify(phone, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[phone]
	if !ok {
		return ErrNoPIN
	}
	if s.now().After(rec.expiresAt) {
		delete(s.records, phone)
		return ErrExpired
	}
	if rec.attempts >= MaxAttempts {
		delete(s.records, phone)
		return ErrTooManyAttempts
	}

	rec.attempts++
	if err := bcrypt.CompareHashAndPassword(rec.hash, []byte(code)); err != nil {
		return ErrMismatch
	}

	delete(s.records, phone)
	return nil
}
