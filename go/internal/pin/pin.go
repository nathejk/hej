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

	// plaintext is the issued code, kept ONLY when the store was built with
	// NewDevStoreWithPlaintextRecall. Empty in every other store, including every
	// production one — see that constructor for why.
	plaintext string
}

// Store holds active PINs keyed by normalized phone number.
type Store struct {
	mu      sync.Mutex
	records map[string]*record
	now     func() time.Time

	// recallPlaintext enables IssuedPlaintextForDev. Set at construction and never
	// mutated afterwards, so a store cannot become readable at runtime.
	recallPlaintext bool
}

// NewStore returns an empty in-memory PIN store. PINs are hashed and cannot be
// read back.
func NewStore() *Store {
	return &Store{
		records: make(map[string]*record),
		now:     time.Now,
	}
}

// NewDevStoreWithPlaintextRecall returns a store that additionally keeps each
// issued PIN in plaintext so it can be read back with IssuedPlaintextForDev.
//
// This exists for one reason: the development-only GET /api/dev/pin endpoint
// (PRD 014 §8), which lets a developer log in on a laptop without tailing the
// API logs. It must never be used outside ENV=development — the whole point of
// hashing PINs is that a memory dump or an accidental log line cannot yield a
// usable credential, and this constructor gives that up.
//
// A separate constructor rather than a setter, so that whether a store is
// readable is decided once, at the single call site that knows the environment.
func NewDevStoreWithPlaintextRecall() *Store {
	s := NewStore()
	s.recallPlaintext = true
	return s
}

// IssuedPlaintextForDev returns the currently-issued, unexpired PIN for phone.
//
// Deliberately narrow: it reports only the code, never the attempt count, expiry
// or send time, and it reports nothing at all unless the store was built with
// NewDevStoreWithPlaintextRecall. It does not consume the PIN or count as an
// attempt, so a developer reading it does not disturb the login it is used for.
func (s *Store) IssuedPlaintextForDev(phone string) (string, bool) {
	if !s.recallPlaintext {
		return "", false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[phone]
	if !ok || rec.plaintext == "" || s.now().After(rec.expiresAt) {
		return "", false
	}
	return rec.plaintext, true
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

	rec := &record{
		hash:      hash,
		expiresAt: now.Add(TTL),
		sentAt:    now,
	}
	if s.recallPlaintext {
		rec.plaintext = code
	}
	s.records[phone] = rec
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
